//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
	"github.com/stretchr/testify/require"
)

// AC-014 (adapter scope): translate normalized image fields/metadata without mutating the accepted request.
func TestUnifiedMediaPayloadGRSNano(t *testing.T) {
	req := &mediacapability.NormalizedRequest{Values: map[string]any{"model": "public", "prompt": "cat", "size": "16:9", "n": json.Number("3"), "image_urls": []any{"https://example.com/in.png"}, "metadata": map[string]any{"resolution": "2K"}}, RequestedCount: 3}
	body, path, err := BuildUnifiedMediaPayload("grsai", "image", "nano-banana-2", req, 1)
	require.NoError(t, err)
	require.Equal(t, "/v1/api/generate", path)
	var v map[string]any
	require.NoError(t, json.Unmarshal(body, &v))
	require.Equal(t, "async", v["replyType"])
	require.Equal(t, "nano-banana-2", v["model"])
	require.Equal(t, "16:9", v["aspectRatio"])
	require.Equal(t, "2K", v["imageSize"])
	require.NotContains(t, v, "n")
	require.NotContains(t, v, "metadata")
	require.Equal(t, json.Number("3"), req.Values["n"])
}

func TestUnifiedMediaPayloadGRSPixelAndVideo(t *testing.T) {
	req := &mediacapability.NormalizedRequest{Values: map[string]any{"prompt": "cat", "size": "16:9", "metadata": map[string]any{"resolution": "4K", "mask_url": "https://example.com/mask.png"}}, ImageSpec: &mediacapability.ImageSpec{Width: 3840, Height: 2160}}
	body, _, err := BuildUnifiedMediaPayload("grsai", "image", "gpt-image-2-vip", req, 1)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"gpt-image-2-vip","prompt":"cat","aspectRatio":"3840x2160","mask":"https://example.com/mask.png","replyType":"async"}`, string(body))
	req.Values = map[string]any{"prompt": "fly", "duration": json.Number("5"), "aspect_ratio": "9:16", "metadata": map[string]any{"resolution": "768p"}}
	body, _, err = BuildUnifiedMediaPayload("grsai", "video", "minimax-h3", req, 1)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"minimax-h3","prompt":"fly","duration":5,"aspectRatio":"portrait","resolution":"768p","replyType":"async"}`, string(body))
}

func TestUnifiedMediaPayloadToAPIsBasic(t *testing.T) {
	req := &mediacapability.NormalizedRequest{Values: map[string]any{"prompt": "cat", "size": "1:1", "metadata": map[string]any{"resolution": "2K", "watermark": false}}, RequestedCount: 3}
	body, p, err := BuildUnifiedMediaPayload("toapis", "image", "doubao-seedream-4-0", req, 3)
	require.NoError(t, err)
	require.Equal(t, "/v1/images/generations", p)
	require.JSONEq(t, `{"model":"doubao-seedream-4-0","prompt":"cat","size":"1:1","n":3,"metadata":{"resolution":"2K","watermark":false}}`, string(body))
}

func TestUnifiedMediaUpstreamNoRepostOnUncertain(t *testing.T) {
	posts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts++
		w.WriteHeader(502)
		_, _ = w.Write([]byte("bad gateway"))
	}))
	defer srv.Close()
	account := &Account{Platform: "toapis", Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": srv.URL, "api_key": "secret"}}
	result, _ := NewUnifiedMediaUpstream(srv.Client()).Submit(context.Background(), account, "image", "model", &mediacapability.NormalizedRequest{Values: map[string]any{"prompt": "x"}}, 1)
	require.Equal(t, 1, posts)
	require.Equal(t, "unknown", result.Disposition)
}

func TestUnifiedMediaUpstreamQueryToAPIsAndMidjourney(t *testing.T) {
	paths := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		require.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/midjourney/task-2" {
			_, _ = w.Write([]byte(`{"id":"task-2","status":"completed","result":{"grid_image_url":"https://example.com/grid.png","image_urls":["https://example.com/cell.png"]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"task-1","status":"completed","result":{"type":"image","data":[{"url":"https://example.com/a.png"}]}}`))
	}))
	defer srv.Close()
	account := &Account{Platform: "toapis", Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": srv.URL + "/v1", "api_key": "secret"}}
	client := NewUnifiedMediaUpstream(srv.Client())
	result, err := client.Poll(context.Background(), account, "image", "model", "task-1")
	require.NoError(t, err)
	require.Equal(t, []string{"https://example.com/a.png"}, result.URLs)
	result, err = client.Poll(context.Background(), account, "image", "midjourney", "task-2")
	require.NoError(t, err)
	require.Equal(t, []string{"https://example.com/grid.png"}, result.URLs)
	require.Equal(t, []string{"/v1/images/generations/task-1", "/v1/midjourney/task-2"}, paths)
}

func TestUnifiedMediaUpstreamAcceptedFailureMustNotFailOver(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"id":"upstream-id","status":"failed","error":"secret"}`))
	}))
	defer srv.Close()
	account := &Account{Platform: PlatformGrsai, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": srv.URL, "api_key": "secret"}}
	result, err := NewUnifiedMediaUpstream(srv.Client()).Submit(context.Background(), account, "image", "nano-banana-2", &mediacapability.NormalizedRequest{Values: map[string]any{"prompt": "x"}}, 1)
	require.NoError(t, err)
	require.Equal(t, "accepted", result.Disposition)
	require.Equal(t, "completed", result.Status)
	require.NotContains(t, result.Error, "secret")
}
func TestUnifiedMediaUpstreamPollMalformedOrHTTPFailureRetries(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
	}{{200, `{"hello":"world"}`}, {404, `{"error":{"message":"not found"}}`}} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		account := &Account{Platform: PlatformToAPIs, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": srv.URL, "api_key": "secret"}}
		_, err := NewUnifiedMediaUpstream(srv.Client()).Poll(context.Background(), account, "image", "example", "id")
		require.Error(t, err)
		srv.Close()
	}
}

func TestUnifiedMediaUpstreamRejectsErrorTrackingIDs(t *testing.T) {
	for _, status := range []int{401, 403, 404, 429, 500, 502} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"id":"error-tracking-id","status":"completed","result":{"type":"image","data":[{"url":"https://example.com/not-generated.png"}]}}`))
		}))
		account := &Account{Platform: PlatformToAPIs, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": srv.URL, "api_key": "secret"}}
		result, _ := NewUnifiedMediaUpstream(srv.Client()).Submit(context.Background(), account, "image", "example", &mediacapability.NormalizedRequest{Values: map[string]any{"prompt": "x"}}, 1)
		require.NotEqual(t, "accepted", result.Disposition)
		require.Empty(t, result.URLs)
		srv.Close()
	}
}
