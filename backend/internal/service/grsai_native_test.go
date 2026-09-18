package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGrsaiNativeClientGeneratePreservesExplicitJSONBody(t *testing.T) {
	wantBody := []byte("{\n  \"model\": \"gpt-image-2\", \"prompt\": \"private prompt\",\n  \"aspectRatio\": \"16:9\", \"imageSize\": \"2K\", \"quality\": \"high\",\n  \"background\": \"transparent\", \"images\": [\"data:image/png;base64,abc\"],\n  \"stream\": false, \"async\": false, \"futureOption\": {\"enabled\":true}, \"replyType\": \"json\"\n}\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/tenant/root/v1/api/generate", r.URL.EscapedPath())
		require.Empty(t, r.URL.RawQuery)
		require.Equal(t, "Bearer upstream-secret", r.Header.Get("Authorization"))
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Equal(t, "application/json", r.Header.Get("Accept"))
		gotBody, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, wantBody, gotBody)

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"task-1","status":"succeeded","results":[{"url":"https://example.test/image.png"}]}`)
	}))
	defer server.Close()

	client := NewGrsaiNativeClient(server.Client())
	result, err := client.Generate(context.Background(), grsaiTestAccount(server.URL+"/tenant/root/", "upstream-secret"), wantBody)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, result.HTTPStatus)
	require.Equal(t, "task-1", result.TaskID)
	require.Equal(t, GrsaiUpstreamStatusSucceeded, result.Status)
	require.JSONEq(t, `{"id":"task-1","status":"succeeded","results":[{"url":"https://example.test/image.png"}]}`, string(result.RawBody))
}

func TestGrsaiNativeClientGenerateAddsOnlyDefaultReplyType(t *testing.T) {
	original := []byte("{ \"model\" : \"nano-banana\", \"future\" : { \"answer\" : 42 }, \"images\" : [\"one\", \"two\"] }\r\n")
	want := []byte("{ \"model\" : \"nano-banana\", \"future\" : { \"answer\" : 42 }, \"images\" : [\"one\", \"two\"] ,\"replyType\":\"json\"}\r\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, want, got)
		var decoded map[string]any
		require.NoError(t, json.Unmarshal(got, &decoded))
		require.Equal(t, "json", decoded["replyType"])
		require.Equal(t, map[string]any{"answer": float64(42)}, decoded["future"])
		_, _ = io.WriteString(w, `{"id":"task-default","status":"succeeded"}`)
	}))
	defer server.Close()

	result, err := NewGrsaiNativeClient(server.Client()).Generate(
		context.Background(),
		grsaiTestAccount(server.URL, "key"),
		original,
	)
	require.NoError(t, err)
	require.Equal(t, "task-default", result.TaskID)
}

func TestGrsaiNativeClientGenerateRejectsProtocolBoundaryViolations(t *testing.T) {
	const secret = "sk-this-must-never-appear-in-errors"
	const prompt = "private prompt must never appear in errors"
	tests := []struct {
		name string
		body string
	}{
		{name: "not json", body: `not-json ` + prompt},
		{name: "json array", body: `[{"model":"x"}]`},
		{name: "stream true", body: `{"prompt":"` + prompt + `","stream":true}`},
		{name: "stream null", body: `{"prompt":"` + prompt + `","stream":null}`},
		{name: "stream string", body: `{"prompt":"` + prompt + `","stream":"false"}`},
		{name: "stream object", body: `{"prompt":"` + prompt + `","stream":{}}`},
		{name: "async true", body: `{"prompt":"` + prompt + `","async":true}`},
		{name: "async null", body: `{"prompt":"` + prompt + `","async":null}`},
		{name: "async number", body: `{"prompt":"` + prompt + `","async":0}`},
		{name: "async array", body: `{"prompt":"` + prompt + `","async":[]}`},
		{name: "reply type stream", body: `{"prompt":"` + prompt + `","replyType":"stream"}`},
		{name: "reply type async", body: `{"prompt":"` + prompt + `","replyType":"async"}`},
		{name: "reply type wrong type", body: `{"prompt":"` + prompt + `","replyType":1}`},
		{name: "duplicate stream", body: `{"stream":false,"stream":false}`},
		{name: "duplicate async", body: `{"async":false,"async":true}`},
		{name: "duplicate reply type", body: `{"replyType":"json","replyType":"json"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NewGrsaiNativeClient(nil).Generate(
				context.Background(),
				grsaiTestAccount("https://api.grsai.example", secret),
				[]byte(tt.body),
			)
			require.Nil(t, result)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrGrsaiInvalidRequest)
			require.NotContains(t, err.Error(), secret)
			require.NotContains(t, err.Error(), prompt)
		})
	}
}

func TestGrsaiNativeClientResultBuildsEscapedQueryAndParsesStatuses(t *testing.T) {
	tests := []struct {
		name             string
		taskID           string
		statusCode       int
		body             string
		wantStatus       string
		wantErrorCode    string
		wantErrorMessage string
		wantHTTPError    bool
	}{
		{
			name:       "running",
			taskID:     "task/running ?&",
			statusCode: http.StatusOK,
			body:       `{"id":"task/running ?&","status":"running","progress":45}`,
			wantStatus: GrsaiUpstreamStatusRunning,
		},
		{
			name:       "succeeded terminal",
			taskID:     "task-success",
			statusCode: http.StatusOK,
			body:       `{"id":"task-success","status":"succeeded","results":[{"url":"https://example.test/image.png"}]}`,
			wantStatus: GrsaiUpstreamStatusSucceeded,
		},
		{
			name:             "failed terminal",
			taskID:           "task-failed",
			statusCode:       http.StatusOK,
			body:             `{"id":"task-failed","status":"failed","error":"render failed"}`,
			wantStatus:       GrsaiUpstreamStatusFailed,
			wantErrorCode:    GrsaiUpstreamStatusFailed,
			wantErrorMessage: "render failed",
		},
		{
			name:             "violation terminal on http error",
			taskID:           "task-violation",
			statusCode:       http.StatusBadRequest,
			body:             `{"id":"task-violation","status":"violation","error":"input moderation"}`,
			wantStatus:       GrsaiUpstreamStatusViolation,
			wantErrorCode:    GrsaiUpstreamStatusViolation,
			wantErrorMessage: "input moderation",
			wantHTTPError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodGet, r.Method)
				require.Equal(t, "/base/v1/api/result", r.URL.EscapedPath())
				require.Equal(t, tt.taskID, r.URL.Query().Get("id"))
				require.Equal(t, "Bearer result-key", r.Header.Get("Authorization"))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()

			result, err := NewGrsaiNativeClient(server.Client()).Result(
				context.Background(),
				grsaiTestAccount(server.URL+"/base", "result-key"),
				tt.taskID,
			)
			if tt.wantHTTPError {
				var httpErr *GrsaiHTTPError
				require.ErrorAs(t, err, &httpErr)
				require.Equal(t, tt.statusCode, httpErr.StatusCode)
			} else {
				require.NoError(t, err)
			}
			require.NotNil(t, result)
			require.Equal(t, tt.statusCode, result.HTTPStatus)
			require.Equal(t, tt.taskID, result.TaskID)
			require.Equal(t, tt.wantStatus, result.Status)
			require.Equal(t, tt.wantErrorCode, result.ErrorCode)
			require.Equal(t, tt.wantErrorMessage, result.ErrorMessage)
			require.Equal(t, []byte(tt.body), result.RawBody)
		})
	}
}

func TestGrsaiNativeClientPreservesNestedLegacyResponseFields(t *testing.T) {
	body := `{"code":0,"msg":"success","data":{"id":"legacy-task","status":"failed","failure_reason":"output_moderation","error":"blocked"}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer server.Close()

	result, err := NewGrsaiNativeClient(server.Client()).Result(
		context.Background(),
		grsaiTestAccount(server.URL, "key"),
		"legacy-task",
	)
	require.NoError(t, err)
	require.Equal(t, "legacy-task", result.TaskID)
	require.Equal(t, GrsaiUpstreamStatusFailed, result.Status)
	require.Equal(t, "output_moderation", result.ErrorCode)
	require.Equal(t, "blocked", result.ErrorMessage)
	require.Equal(t, []byte(body), result.RawBody)
}

func TestGrsaiNativeClientReturnsRawBodyWithMalformedResponseError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":`)
	}))
	defer server.Close()

	result, err := NewGrsaiNativeClient(server.Client()).Result(
		context.Background(),
		grsaiTestAccount(server.URL, "key"),
		"task-malformed",
	)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrGrsaiInvalidResponse)
	require.NotNil(t, result)
	require.Equal(t, []byte(`{"id":`), result.RawBody)
}

func TestGrsaiNativeClientHTTPErrorStringDoesNotExposeResponseOrCredentials(t *testing.T) {
	const secret = "opaque-real-account-token"
	const echoedPrompt = "echoed private prompt"
	body := `{"status":"failed","code":"unauthorized-` + secret + `","error":"` + secret + ` ` + echoedPrompt + `"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, body)
	}))
	defer server.Close()

	result, err := NewGrsaiNativeClient(server.Client()).Generate(
		context.Background(),
		grsaiTestAccount(server.URL, secret),
		[]byte(`{"prompt":"`+echoedPrompt+`","replyType":"json"}`),
	)
	require.Equal(t, []byte(body), result.RawBody)
	var httpErr *GrsaiHTTPError
	require.ErrorAs(t, err, &httpErr)
	require.NotContains(t, result.ErrorCode, secret)
	require.Contains(t, result.ErrorCode, "REDACTED")
	require.Equal(t, result.ErrorCode, httpErr.ErrorCode)
	require.NotContains(t, result.ErrorMessage, secret)
	require.Contains(t, result.ErrorMessage, "REDACTED")
	require.NotContains(t, err.Error(), secret)
	require.NotContains(t, err.Error(), echoedPrompt)
}

func TestGrsaiNativeClientNetworkErrorDoesNotExposeAPIKey(t *testing.T) {
	const secret = "sk-network-secret-value"
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	client := server.Client()
	baseURL := server.URL
	server.Close()

	result, err := NewGrsaiNativeClient(client).Result(
		context.Background(),
		grsaiTestAccount(baseURL, secret),
		"task-network",
	)
	require.Nil(t, result)
	require.Error(t, err)
	require.NotContains(t, err.Error(), secret)
}

func TestNewGrsaiNativeClientConfiguresTimeout(t *testing.T) {
	input := &http.Client{}
	client := NewGrsaiNativeClient(input)
	httpClient, ok := client.(*GrsaiNativeHTTPClient)

	require.True(t, ok)
	require.NotSame(t, input, httpClient.client)
	require.Equal(t, grsaiRequestTimeout, httpClient.client.Timeout)
	require.Greater(t, httpClient.client.Timeout, time.Duration(0))
}

func TestGrsaiNativeClientPreservesPartialResponseOnReadError(t *testing.T) {
	const secret = "account-key-without-known-prefix"
	partial := []byte(`{"id":"partial-task"`)
	client := NewGrsaiNativeClient(&http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Header:     make(http.Header),
			Body: &failingReadCloser{
				payload: partial,
				err:     errors.New("upstream read failed for " + secret),
			},
		}, nil
	})})

	result, err := client.Result(
		context.Background(),
		grsaiTestAccount("https://api.grsai.example", secret),
		"partial-task",
	)

	require.Error(t, err)
	require.NotContains(t, err.Error(), secret)
	require.Contains(t, err.Error(), "REDACTED")
	require.NotNil(t, result)
	require.Equal(t, http.StatusBadGateway, result.HTTPStatus)
	require.Equal(t, "partial-task", result.TaskID)
	require.Equal(t, partial, result.RawBody)
}

func grsaiTestAccount(baseURL, apiKey string) *Account {
	return &Account{
		ID:       42,
		Platform: PlatformGrsai,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": baseURL,
			"api_key":  apiKey,
		},
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type failingReadCloser struct {
	payload []byte
	err     error
}

func (r *failingReadCloser) Read(p []byte) (int, error) {
	if len(r.payload) == 0 {
		return 0, r.err
	}
	n := copy(p, r.payload)
	r.payload = r.payload[n:]
	return n, nil
}

func (*failingReadCloser) Close() error {
	return nil
}

func TestGrsaiNativeClientValidatesAccountAndTaskID(t *testing.T) {
	client := NewGrsaiNativeClient(nil)
	tests := []struct {
		name    string
		account *Account
		taskID  string
	}{
		{name: "nil account", account: nil, taskID: "task"},
		{name: "wrong platform", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, taskID: "task"},
		{name: "missing base url", account: grsaiTestAccount("", "key"), taskID: "task"},
		{name: "missing api key", account: grsaiTestAccount("https://api.grsai.example", ""), taskID: "task"},
		{name: "missing task id", account: grsaiTestAccount("https://api.grsai.example", "key"), taskID: "  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := client.Result(context.Background(), tt.account, tt.taskID)
			require.Nil(t, result)
			require.Error(t, err)
			require.True(t,
				strings.Contains(err.Error(), "account") ||
					strings.Contains(err.Error(), "base URL") ||
					strings.Contains(err.Error(), "API key") ||
					strings.Contains(err.Error(), "task ID"),
			)
		})
	}
}
