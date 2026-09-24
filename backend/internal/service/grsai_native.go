package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
)

const (
	grsaiGeneratePath       = "/v1/api/generate"
	grsaiResultPath         = "/v1/api/result"
	grsaiRequestTimeout     = 30 * time.Minute
	grsaiHeaderTimeout      = 60 * time.Second
	grsaiMaxResponseBodyLen = 16 << 20

	GrsaiUpstreamStatusRunning   = "running"
	GrsaiUpstreamStatusSucceeded = "succeeded"
	GrsaiUpstreamStatusFailed    = "failed"
	GrsaiUpstreamStatusViolation = "violation"
)

var (
	ErrGrsaiInvalidAccount        = errors.New("invalid grsai account")
	ErrGrsaiInvalidRequest        = errors.New("invalid grsai request")
	ErrGrsaiInvalidResponse       = errors.New("invalid grsai response")
	ErrGrsaiInvalidStreamResponse = errors.New("invalid grsai stream response")
)

// GrsaiUpstreamResult preserves the exact upstream response while exposing the
// small status surface needed by settlement and recovery code.
type GrsaiUpstreamResult struct {
	HTTPStatus   int
	RawBody      []byte
	TaskID       string
	Status       string
	Progress     int
	ResultURLs   []string
	ErrorCode    string
	ErrorMessage string
}

// GrsaiHTTPError describes a non-2xx response without including the upstream
// body in Error(). RawBody and parsed details remain available on the returned
// GrsaiUpstreamResult for controlled forwarding and handling.
type GrsaiHTTPError struct {
	Operation  string
	StatusCode int
	ErrorCode  string
}

func (e *GrsaiHTTPError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("grsai %s request failed: status=%d", e.Operation, e.StatusCode)
}

// GrsaiNativeClient is the injectable native GRS.AI protocol boundary used by
// handlers and recovery workers.
type GrsaiNativeClient interface {
	Generate(ctx context.Context, account *Account, body []byte) (*GrsaiUpstreamResult, error)
	Result(ctx context.Context, account *Account, taskID string) (*GrsaiUpstreamResult, error)
}

// GrsaiUpstreamStream is the validated HTTP response boundary for the native
// provider stream. Callers own Body and must close it after consumption.
type GrsaiUpstreamStream struct {
	StatusCode  int
	ContentType string
	Body        io.ReadCloser
}

type GrsaiStreamClient interface {
	OpenGenerateStream(context.Context, *Account, []byte) (*GrsaiUpstreamStream, error)
}

// GrsaiNativeHTTPClient implements GrsaiNativeClient over the native GRS.AI
// HTTP protocol. It is deliberately independent from OpenAI-compatible
// transports.
type GrsaiNativeHTTPClient struct {
	client *http.Client
}

func NewGrsaiNativeHTTPClient(client *http.Client) *GrsaiNativeHTTPClient {
	return &GrsaiNativeHTTPClient{client: configureGrsaiHTTPClient(client)}
}

func NewGrsaiNativeClient(client *http.Client) GrsaiNativeClient {
	return NewGrsaiNativeHTTPClient(client)
}

func configureGrsaiHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		shared, err := httpclient.GetClient(httpclient.Options{
			Timeout:               grsaiRequestTimeout,
			ResponseHeaderTimeout: grsaiHeaderTimeout,
		})
		if err == nil {
			return shared
		}
		fallback := *http.DefaultClient
		fallback.Timeout = grsaiRequestTimeout
		return &fallback
	}

	configured := *client
	if configured.Timeout <= 0 {
		configured.Timeout = grsaiRequestTimeout
	}
	return &configured
}

func (c *GrsaiNativeHTTPClient) Generate(ctx context.Context, account *Account, body []byte) (*GrsaiUpstreamResult, error) {
	baseURL, apiKey, err := grsaiAccountCredentials(account)
	if err != nil {
		return nil, err
	}
	delivery, err := ParseGrsaiDeliveryRequest(body)
	if err != nil {
		return nil, err
	}
	targetURL, err := buildGrsaiEndpointURL(baseURL, grsaiGeneratePath)
	if err != nil {
		return nil, err
	}
	return c.do(ctx, "generate", http.MethodPost, targetURL, apiKey, delivery.UpstreamBody, "")
}

// OpenGenerateStream sends the provider-safe request body and exposes a body
// only after both the HTTP status and SSE content type have been validated.
// Invalid responses are drained only up to the bounded limit before closing,
// so an upstream error can never leave an open response body behind.
func (c *GrsaiNativeHTTPClient) OpenGenerateStream(ctx context.Context, account *Account, body []byte) (*GrsaiUpstreamStream, error) {
	baseURL, apiKey, err := grsaiAccountCredentials(account)
	if err != nil {
		return nil, err
	}
	delivery, err := ParseGrsaiDeliveryRequest(body)
	if err != nil {
		return nil, err
	}
	targetURL, err := buildGrsaiEndpointURL(baseURL, grsaiGeneratePath)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(delivery.UpstreamBody))
	if err != nil {
		return nil, fmt.Errorf("grsai stream request setup failed: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	client := c.client
	if client == nil {
		client = configureGrsaiHTTPClient(nil)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("grsai stream transport failed: %s", sanitizeGrsaiErrorValue(err.Error(), apiKey))
	}
	contentType := resp.Header.Get("Content-Type")
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices && strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		return &GrsaiUpstreamStream{StatusCode: resp.StatusCode, ContentType: contentType, Body: resp.Body}, nil
	}
	defer func() { _ = resp.Body.Close() }()
	errorBody, readErr := io.ReadAll(io.LimitReader(resp.Body, grsaiMaxResponseBodyLen+1))
	if len(errorBody) > grsaiMaxResponseBodyLen {
		errorBody = errorBody[:grsaiMaxResponseBodyLen]
	}
	detail := sanitizeGrsaiErrorValue(strings.TrimSpace(string(errorBody)), apiKey)
	if readErr != nil {
		detail = sanitizeGrsaiErrorValue(readErr.Error(), apiKey)
	}
	if detail == "" {
		detail = "empty upstream response"
	}
	return nil, fmt.Errorf("%w: status=%d content_type=%q body=%s", ErrGrsaiInvalidStreamResponse, resp.StatusCode, contentType, detail)
}

// PrepareGrsaiGenerateBody remains a compatibility boundary for callers that
// validate a request before handing it to the native client. The returned body
// is the independent downstream snapshot; only Generate uses UpstreamBody.
func PrepareGrsaiGenerateBody(body []byte) ([]byte, error) {
	delivery, err := ParseGrsaiDeliveryRequest(body)
	if err != nil {
		return nil, err
	}
	return delivery.OriginalBody, nil
}

func (c *GrsaiNativeHTTPClient) Result(ctx context.Context, account *Account, taskID string) (*GrsaiUpstreamResult, error) {
	baseURL, apiKey, err := grsaiAccountCredentials(account)
	if err != nil {
		return nil, err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("%w: task ID is required", ErrGrsaiInvalidRequest)
	}
	targetURL, err := buildGrsaiEndpointURL(baseURL, grsaiResultPath)
	if err != nil {
		return nil, err
	}
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("%w: result URL is invalid", ErrGrsaiInvalidAccount)
	}
	query := parsedURL.Query()
	query.Set("id", taskID)
	parsedURL.RawQuery = query.Encode()
	return c.do(ctx, "result", http.MethodGet, parsedURL.String(), apiKey, nil, taskID)
}

func (c *GrsaiNativeHTTPClient) do(
	ctx context.Context,
	operation string,
	method string,
	targetURL string,
	apiKey string,
	body []byte,
	knownTaskID string,
) (*GrsaiUpstreamResult, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, targetURL, reader)
	if err != nil {
		return nil, fmt.Errorf("grsai %s request setup failed: %w", operation, err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if operation == "generate" {
		req.Header.Set("Accept", "text/event-stream")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	var client *http.Client
	if c != nil {
		client = c.client
	}
	if client == nil {
		client = configureGrsaiHTTPClient(nil)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("grsai %s transport failed: %s", operation, sanitizeGrsaiErrorValue(err.Error(), apiKey))
	}
	defer func() { _ = resp.Body.Close() }()

	rawBody, readErr := io.ReadAll(io.LimitReader(resp.Body, grsaiMaxResponseBodyLen+1))
	if readErr != nil {
		return &GrsaiUpstreamResult{
			HTTPStatus: resp.StatusCode,
			RawBody:    append([]byte(nil), rawBody...),
			TaskID:     knownTaskID,
		}, fmt.Errorf("grsai %s response read failed: %s", operation, sanitizeGrsaiErrorValue(readErr.Error(), apiKey))
	}
	if len(rawBody) > grsaiMaxResponseBodyLen {
		return &GrsaiUpstreamResult{
			HTTPStatus: resp.StatusCode,
			RawBody:    append([]byte(nil), rawBody[:grsaiMaxResponseBodyLen]...),
			TaskID:     knownTaskID,
		}, fmt.Errorf("%w: response body exceeds %d bytes", ErrGrsaiInvalidResponse, grsaiMaxResponseBodyLen)
	}

	result, parseErr := parseGrsaiUpstreamResult(resp.StatusCode, rawBody, apiKey)
	if result != nil && result.TaskID == "" {
		result.TaskID = knownTaskID
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		code := ""
		if result != nil {
			code = result.ErrorCode
		}
		return result, &GrsaiHTTPError{
			Operation:  operation,
			StatusCode: resp.StatusCode,
			ErrorCode:  code,
		}
	}
	if parseErr != nil {
		return result, parseErr
	}
	return result, nil
}

func grsaiAccountCredentials(account *Account) (string, string, error) {
	if account == nil || account.Platform != PlatformGrsai || account.Type != AccountTypeAPIKey {
		return "", "", fmt.Errorf("%w: API key account on platform grsai is required", ErrGrsaiInvalidAccount)
	}
	baseURL := strings.TrimSpace(account.GetCredential("base_url"))
	if baseURL == "" {
		return "", "", fmt.Errorf("%w: base URL is required", ErrGrsaiInvalidAccount)
	}
	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
	if apiKey == "" {
		return "", "", fmt.Errorf("%w: API key is required", ErrGrsaiInvalidAccount)
	}
	return baseURL, apiKey, nil
}

func buildGrsaiEndpointURL(baseURL, endpointPath string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("%w: base URL is invalid", ErrGrsaiInvalidAccount)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("%w: base URL scheme must be HTTP or HTTPS", ErrGrsaiInvalidAccount)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("%w: base URL must not contain credentials, query, or fragment", ErrGrsaiInvalidAccount)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + endpointPath
	parsed.RawPath = ""
	return parsed.String(), nil
}

func parseGrsaiUpstreamResult(httpStatus int, rawBody []byte, apiKey string) (*GrsaiUpstreamResult, error) {
	result := &GrsaiUpstreamResult{
		HTTPStatus: httpStatus,
		RawBody:    append([]byte(nil), rawBody...),
	}
	var root map[string]json.RawMessage
	if len(bytes.TrimSpace(rawBody)) == 0 || json.Unmarshal(rawBody, &root) != nil || root == nil {
		return result, fmt.Errorf("%w: body must be a JSON object", ErrGrsaiInvalidResponse)
	}

	data := rawJSONObject(root["data"])
	result.TaskID = firstRawScalar(root, data, "id", "taskId", "task_id")
	result.Status = normalizeGrsaiStatus(firstRawScalar(root, data, "status", "state"))
	if raw := firstRawMessage(root, data, "progress", "percent", "percentage"); len(raw) > 0 {
		progress, err := parseGrsaiProgress(raw)
		if err != nil || progress < 0 || progress > 100 {
			return result, fmt.Errorf("%w: invalid progress", ErrGrsaiInvalidResponse)
		}
		result.Progress = progress
	}
	result.ResultURLs = extractGrsaiResultURLs(root, data)
	result.ErrorCode, result.ErrorMessage = parseGrsaiError(root, data, result.Status, httpStatus, apiKey)

	if httpStatus >= http.StatusOK && httpStatus < http.StatusMultipleChoices && result.Status == "" {
		return result, fmt.Errorf("%w: response status is missing", ErrGrsaiInvalidResponse)
	}
	return result, nil
}

func parseGrsaiError(root, data map[string]json.RawMessage, status string, httpStatus int, apiKey string) (string, string) {
	code := firstRawScalar(root, data, "errorCode", "error_code", "failure_reason")
	message := ""
	for _, object := range []map[string]json.RawMessage{root, data} {
		raw, ok := object["error"]
		if !ok {
			continue
		}
		if scalar := rawScalar(raw); scalar != "" {
			message = scalar
			break
		}
		if errorObject := rawJSONObject(raw); errorObject != nil {
			if code == "" {
				code = firstRawScalar(errorObject, nil, "code", "type")
			}
			message = firstRawScalar(errorObject, nil, "message", "msg", "error")
			if message != "" {
				break
			}
		}
	}
	if code == "" {
		code = firstRawScalar(root, data, "code")
		if code == "0" {
			code = ""
		}
	}
	if message == "" && (status == GrsaiUpstreamStatusFailed || status == GrsaiUpstreamStatusViolation || httpStatus >= 400) {
		message = firstRawScalar(root, data, "message", "msg")
	}
	if code == "" && status == GrsaiUpstreamStatusViolation {
		code = GrsaiUpstreamStatusViolation
	}
	if code == "" && status == GrsaiUpstreamStatusFailed {
		code = GrsaiUpstreamStatusFailed
	}
	return sanitizeGrsaiErrorValue(code, apiKey), sanitizeGrsaiErrorValue(message, apiKey)
}

func sanitizeGrsaiErrorValue(value, apiKey string) string {
	if apiKey = strings.TrimSpace(apiKey); apiKey != "" {
		value = strings.ReplaceAll(value, apiKey, "REDACTED")
	}
	return sanitizeErrorMessage(value)
}

func rawJSONObject(raw json.RawMessage) map[string]json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil {
		return nil
	}
	return object
}

func firstRawScalar(primary, secondary map[string]json.RawMessage, keys ...string) string {
	for _, object := range []map[string]json.RawMessage{primary, secondary} {
		for _, key := range keys {
			if value := rawScalar(object[key]); value != "" {
				return value
			}
		}
	}
	return ""
}

func rawScalar(raw json.RawMessage) string {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return ""
	}
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return strings.TrimSpace(value)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var scalar any
	if decoder.Decode(&scalar) != nil {
		return ""
	}
	switch value := scalar.(type) {
	case json.Number:
		return value.String()
	case bool:
		return fmt.Sprintf("%t", value)
	default:
		return ""
	}
}

func normalizeGrsaiStatus(status string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch normalized {
	case "pending", "queued", "processing", "in_progress", "running":
		return GrsaiUpstreamStatusRunning
	case "success", "completed", "complete", "succeeded":
		return GrsaiUpstreamStatusSucceeded
	case "failure", "error", "failed":
		return GrsaiUpstreamStatusFailed
	case "blocked", "moderation", "input_moderation", "output_moderation", "violation":
		return GrsaiUpstreamStatusViolation
	default:
		return normalized
	}
}

var _ GrsaiNativeClient = (*GrsaiNativeHTTPClient)(nil)
