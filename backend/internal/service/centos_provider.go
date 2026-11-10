package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
)

const (
	CentOSDefaultBaseURL    = "https://frapi.centos.hk"
	CentOSDefaultImageModel = "gpt-image-2.6-flare"
	centosMaxResponseBytes  = 64 << 20
)

// CentOS has its own account identity and protocol boundary. Upstream streaming
// keeps long generations alive; clients still receive the durable async media API.
type centosMediaProvider struct{ transport *unifiedMediaHTTPUpstream }

var _ UnifiedMediaUpstream = (*centosMediaProvider)(nil)

func buildUnifiedCentOSImagePayload(model string, request *mediacapability.NormalizedRequest, values, metadata map[string]any, count int) ([]byte, string, error) {
	if isCentOSNanoModel(model) {
		return buildUnifiedCentOSNanoPayload(model, request, values, metadata, count)
	}
	if model != "gpt-image-2.6-flare" && model != "gpt-image-2.6-sunburst" {
		return nil, "", errors.New("CentOS model has no verified image protocol")
	}
	if count != 1 || request.ImageSpec == nil {
		return nil, "", errors.New("CentOS images require one output and a reviewed pixel specification")
	}
	path := "/v1/images/generations"
	out := map[string]any{"model": model, "prompt": values["prompt"], "n": 1, "stream": true,
		"size": fmt.Sprintf("%dx%d", request.ImageSpec.Width, request.ImageSpec.Height), "response_format": "url"}
	if images, exists := values["image_urls"]; exists {
		refs, ok := images.([]any)
		if !ok || len(refs) < 1 || len(refs) > 16 {
			return nil, "", errors.New("CentOS supports one to sixteen reference images")
		}
		out["image"] = images
		path = "/v1/images/edits"
	}
	for field, value := range metadata {
		switch field {
		case "resolution":
		case "quality":
			out[field] = value
		default:
			return nil, "", errors.New("unsupported CentOS image parameter")
		}
	}
	body, err := json.Marshal(out)
	return body, path, err
}

func (p *centosMediaProvider) Submit(ctx context.Context, account *Account, mediaType, model string, normalized *mediacapability.NormalizedRequest, count int) (*UnifiedMediaUpstreamResult, error) {
	result := &UnifiedMediaUpstreamResult{Disposition: "rejected"}
	if p == nil || p.transport == nil || account == nil || account.Platform != PlatformCentOS || account.Type != AccountTypeAPIKey || mediaType != "image" {
		return result, errors.New("invalid CentOS image account")
	}
	key := strings.TrimSpace(account.GetCredential("api_key"))
	if key == "" {
		return result, errors.New("CentOS API key is not configured")
	}
	body, path, err := BuildUnifiedMediaPayload(PlatformCentOS, mediaType, model, normalized, count)
	if err != nil {
		return result, err
	}
	base := strings.TrimSpace(account.GetCredential("base_url"))
	if base == "" {
		base = CentOSDefaultBaseURL
	}
	if _, err := buildGrsaiEndpointURL(base, path); err != nil {
		return result, err
	}
	target := buildOpenAIEndpointURL(base, path)
	contentType := "application/json"
	if path == "/v1/images/edits" {
		body, contentType, err = p.prepareImageEdit(ctx, account, body)
		if err != nil {
			return result, err
		}
	}
	if isCentOSNanoModel(model) {
		body, err = p.prepareNanoReferences(ctx, account, body)
		if err != nil {
			return result, err
		}
	}
	ctx, cancel := context.WithTimeout(WithHTTPUpstreamRedirectsDisabled(WithHTTPUpstreamProfile(ctx, HTTPUpstreamProfileLongStream)), 15*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return result, errors.New("CentOS image request setup failed")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "text/event-stream, application/json")
	result.Disposition = "unknown"
	var response *http.Response
	if p.transport.transport != nil {
		proxyURL := ""
		if account.ProxyID != nil && account.Proxy != nil {
			proxyURL = account.Proxy.URL()
		}
		response, err = p.transport.transport.Do(req, proxyURL, account.ID, account.Concurrency)
	} else {
		client := *p.transport.client
		client.Timeout = 15 * time.Minute
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		response, err = client.Do(req)
	}
	if err != nil {
		return result, errors.New("CentOS connection failed; submission acceptance is unknown")
	}
	defer func() { _ = response.Body.Close() }()
	result.HTTPStatus = response.StatusCode
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(response.Body, 65536))
		// This upstream can report a generation timeout as HTTP 400. A timeout
		// does not prove rejection, so it must not trigger a second paid POST.
		message := strings.ToLower(string(raw))
		ambiguous := strings.Contains(message, "timeout") || strings.Contains(message, "timed out") || strings.Contains(message, "stream error")
		if !ambiguous && (response.StatusCode == 400 || response.StatusCode == 401 || response.StatusCode == 403 || response.StatusCode == 404 || response.StatusCode == 422 || response.StatusCode == 429) {
			result.Disposition = "rejected"
			result.Error = fmt.Sprintf("Upstream rejected generation (HTTP %d)", response.StatusCode)
			return result, nil
		}
		return result, errors.New("CentOS response did not confirm submission acceptance")
	}
	if strings.HasPrefix(strings.ToLower(response.Header.Get("Content-Type")), "text/event-stream") {
		result.URLs, err = parseCentOSImageStream(response.Body)
	} else {
		var raw []byte
		raw, err = io.ReadAll(io.LimitReader(response.Body, centosMaxResponseBytes+1))
		if len(raw) > centosMaxResponseBytes {
			err = errors.New("CentOS image response is too large")
		}
		if err == nil {
			if isCentOSNanoModel(model) {
				result.URLs = parseCentOSNanoImages(raw)
			} else {
				result.URLs = parseUnifiedImageResults(raw, true)
			}
		}
	}
	if err != nil || len(result.URLs) != 1 {
		result.URLs = nil
		return result, errors.New("CentOS did not return one completed image; submission acceptance is unknown")
	}
	result.Disposition = "accepted"
	result.Status = "completed"
	return result, nil
}

func isCentOSNanoModel(model string) bool {
	return model == "gemini-3-pro-image-preview" || model == "gemini-3.1-flash-image-preview"
}

// Default geometry is an explicit, captured policy for this backup only.
// Public Nano validation still bounds ratio/tier, and the GRS candidate keeps them.
func buildUnifiedCentOSNanoPayload(model string, request *mediacapability.NormalizedRequest, values, metadata map[string]any, count int) ([]byte, string, error) {
	profile := mediaAdapterProfile(PlatformCentOS, "image", model)
	if len(request.AdapterProfile) > 0 {
		profile = UnifiedMediaAdapterProfile{}
		if json.Unmarshal(request.AdapterProfile, &profile) != nil {
			return nil, "", errors.New("captured CentOS Nano profile is invalid")
		}
	}
	if !profile.DefaultImageGeometry || count != 1 || request.ImageSpec != nil || request.RequestedImageSpec != nil || request.FixedImageResolution != "" {
		return nil, "", errors.New("CentOS Nano requires the approved default-geometry backup policy")
	}
	for field := range metadata {
		if field != "resolution" {
			return nil, "", errors.New("unsupported CentOS Nano image parameter")
		}
	}
	var content any = values["prompt"]
	if images, exists := values["image_urls"]; exists {
		refs, ok := images.([]any)
		if !ok || len(refs) < 1 || len(refs) > 16 {
			return nil, "", errors.New("CentOS Nano supports one to sixteen reference images")
		}
		parts := []any{map[string]any{"type": "text", "text": values["prompt"]}}
		for _, reference := range refs {
			parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]any{"url": reference}})
		}
		content = parts
	}
	body, err := json.Marshal(map[string]any{"model": model, "messages": []any{map[string]any{"role": "user", "content": content}}, "stream": false, "modalities": []string{"text", "image"}})
	return body, "/v1/chat/completions", err
}

var centosNanoInlineImagePattern = regexp.MustCompile(`data:image/(?:png|jpe?g|webp|gif);base64,[A-Za-z0-9+/=]+`)

func parseCentOSNanoImages(raw []byte) []string {
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(raw, &response) != nil || len(response.Choices) != 1 || response.Choices[0].FinishReason != "stop" || (len(response.Error) > 0 && string(response.Error) != "null") {
		return nil
	}
	images := centosNanoInlineImagePattern.FindAllString(response.Choices[0].Message.Content, 2)
	if len(images) != 1 {
		return nil
	}
	return images
}

func (p *centosMediaProvider) prepareNanoReferences(ctx context.Context, account *Account, payload []byte) ([]byte, error) {
	var body struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
		Stream     bool     `json:"stream"`
		Modalities []string `json:"modalities"`
	}
	if json.Unmarshal(payload, &body) != nil || len(body.Messages) != 1 {
		return nil, errors.New("invalid CentOS Nano request")
	}
	var parts []struct {
		Type     string `json:"type"`
		Text     string `json:"text,omitempty"`
		ImageURL *struct {
			URL string `json:"url"`
		} `json:"image_url,omitempty"`
	}
	if json.Unmarshal(body.Messages[0].Content, &parts) != nil {
		return payload, nil // Text-only messages do not download references.
	}
	total := 0
	for i := range parts {
		if parts[i].Type != "image_url" || parts[i].ImageURL == nil {
			continue
		}
		data, err := p.downloadReference(ctx, account, parts[i].ImageURL.URL, &total)
		if err != nil {
			return nil, err
		}
		parts[i].ImageURL.URL = "data:" + http.DetectContentType(data) + ";base64," + base64.StdEncoding.EncodeToString(data)
	}
	content, err := json.Marshal(parts)
	if err != nil {
		return nil, err
	}
	body.Messages[0].Content = content
	return json.Marshal(body)
}

// Reference URLs never receive provider credentials. The shared transport checks
// public DNS destinations on every redirect hop, before files enter multipart.
func (p *centosMediaProvider) prepareImageEdit(ctx context.Context, account *Account, payload []byte) ([]byte, string, error) {
	if p.transport.transport == nil {
		return nil, "", errors.New("secure reference image transport is unavailable")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(payload, &fields) != nil {
		return nil, "", errors.New("invalid CentOS edit payload")
	}
	var references []string
	if json.Unmarshal(fields["image"], &references) != nil || len(references) < 1 || len(references) > 16 {
		return nil, "", errors.New("invalid CentOS reference list")
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, field := range []string{"model", "prompt", "n", "size", "quality", "response_format", "stream"} {
		raw, exists := fields[field]
		if !exists {
			continue
		}
		value := string(raw)
		var text string
		if json.Unmarshal(raw, &text) == nil {
			value = text
		}
		if err := writer.WriteField(field, value); err != nil {
			return nil, "", errors.New("failed to prepare CentOS edit fields")
		}
	}
	total := 0
	for i, source := range references {
		data, err := p.downloadReference(ctx, account, source, &total)
		if err != nil {
			return nil, "", err
		}
		fieldName := "image"
		if len(references) > 1 {
			fieldName = "image[]"
		}
		header := textproto.MIMEHeader{}
		contentType := http.DetectContentType(data)
		extension := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp", "image/gif": ".gif"}[contentType]
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="reference-%d%s"`, fieldName, i+1, extension))
		header.Set("Content-Type", contentType)
		part, err := writer.CreatePart(header)
		if err != nil {
			return nil, "", errors.New("failed to prepare reference upload")
		}
		if _, err = part.Write(data); err != nil {
			return nil, "", errors.New("failed to prepare reference upload")
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", errors.New("failed to finish reference upload")
	}
	return body.Bytes(), writer.FormDataContentType(), nil
}

func (p *centosMediaProvider) downloadReference(ctx context.Context, account *Account, source string, total *int) ([]byte, error) {
	if p.transport.transport == nil {
		return nil, errors.New("secure reference image transport is unavailable")
	}
	parsed, err := url.Parse(source)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || rejectPrivateImageHost(source) != nil {
		return nil, errors.New("invalid public reference image URL")
	}
	requestCtx, cancel := context.WithTimeout(WithHTTPUpstreamPublicHostsOnly(ctx), 45*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, source, nil)
	if err != nil {
		return nil, errors.New("failed to prepare reference download")
	}
	req.Header.Set("Accept", "image/*")
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	response, err := p.transport.transport.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return nil, errors.New("reference image download failed")
	}
	defer func() { _ = response.Body.Close() }()
	data, readErr := io.ReadAll(io.LimitReader(response.Body, (20<<20)+1))
	*total += len(data)
	if readErr != nil || response.StatusCode < 200 || response.StatusCode >= 300 || len(data) > 20<<20 || *total > 32<<20 || !isBackfillImageContent(data) {
		return nil, errors.New("reference image is unavailable or exceeds the upload limit")
	}
	return data, nil
}

func (*centosMediaProvider) Poll(context.Context, *Account, string, string, string) (*UnifiedMediaUpstreamResult, error) {
	return nil, errors.New("CentOS image results are delivered by the submission stream")
}

func parseCentOSImageStream(reader io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(io.LimitReader(reader, centosMaxResponseBytes+1))
	scanner.Buffer(make([]byte, 64*1024), centosMaxResponseBytes)
	var data strings.Builder
	process := func() ([]string, error) {
		if data.Len() == 0 {
			return nil, nil
		}
		fragment := data.String()
		data.Reset()
		if fragment == "[DONE]" {
			return nil, nil
		}
		var event struct {
			Type   string `json:"type"`
			URL    string `json:"url"`
			Base64 string `json:"b64_json"`
		}
		if json.Unmarshal([]byte(fragment), &event) != nil {
			return nil, errors.New("invalid CentOS stream event")
		}
		if event.Type != "image_generation.completed" && event.Type != "image_edit.completed" {
			return nil, nil
		}
		raw, _ := json.Marshal(map[string]any{"data": []any{map[string]string{"url": event.URL, "b64_json": event.Base64}}})
		urls := parseUnifiedImageResults(raw, true)
		if len(urls) != 1 {
			return nil, errors.New("invalid completed CentOS image")
		}
		return urls, nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			urls, err := process()
			if err != nil || len(urls) > 0 {
				return urls, err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
			if data.Len() > centosMaxResponseBytes {
				return nil, errors.New("CentOS stream event is too large")
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return process()
}
