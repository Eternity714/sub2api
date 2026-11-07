//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
	"github.com/stretchr/testify/require"
)

func TestCompositeMediaOpenAIExplicitCapabilityRoute(t *testing.T) {
	route, err := compositeRouteFromInput(22, CompositeRouteInput{PublicModel: "gpt-image-2.6-1k", TargetPlatform: PlatformOpenAI, UpstreamModel: "gpt-image-2.6-1k", Endpoint: "images", CapabilityModel: "gpt-image-2.6-1k", Enabled: true})
	require.NoError(t, err)
	route.ID = 1
	legacy := CompositeModelRoute{ID: 2, GroupID: 22, PublicModel: "legacy", TargetPlatform: PlatformOpenAI, Endpoint: "images", MatchType: "exact", Enabled: true}
	resolver := NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{*route, legacy}})
	candidates, err := resolver.ResolveMediaCandidates(context.Background(), 22, route.PublicModel, "images")
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, PlatformOpenAI, candidates[0].TargetPlatform)
	candidates, err = resolver.ResolveMediaCandidates(context.Background(), 22, "legacy", "images")
	require.NoError(t, err)
	require.Empty(t, candidates)
	legacyResolver := NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{legacy}})
	has, err := legacyResolver.HasMediaRoutes(context.Background(), 22, "images")
	require.NoError(t, err)
	require.False(t, has, "ordinary OpenAI image routes keep their existing handler")
	_, err = compositeRouteFromInput(22, CompositeRouteInput{PublicModel: "chat", TargetPlatform: PlatformOpenAI, Endpoint: "responses", Enabled: true})
	require.NoError(t, err, "ordinary OpenAI routes remain valid")
	for _, tc := range []struct{ endpoint, match string }{{"videos", "exact"}, {"responses", "exact"}, {"images", "prefix"}} {
		_, err = compositeRouteFromInput(22, CompositeRouteInput{PublicModel: "bad", TargetPlatform: PlatformOpenAI, Endpoint: tc.endpoint, MatchType: tc.match, CapabilityModel: "profile", Enabled: true})
		require.Error(t, err)
	}
}

func TestUnifiedMediaPayloadOpenAICompatibleImage(t *testing.T) {
	req := &mediacapability.NormalizedRequest{Values: map[string]any{"model": "public", "prompt": "a blue cup", "size": "3:2", "n": json.Number("2"), "metadata": map[string]any{"resolution": "1K", "quality": "auto", "output_format": "png"}}, ImageSpec: &mediacapability.ImageSpec{Width: 1536, Height: 1024}, RequestedCount: 2}
	body, path, err := BuildUnifiedMediaPayload(PlatformOpenAI, "image", "gpt-image-2.6-1k", req, 1)
	require.NoError(t, err)
	require.Equal(t, "/v1/images/generations", path)
	require.JSONEq(t, `{"model":"gpt-image-2.6-1k","prompt":"a blue cup","n":1,"size":"1536x1024","quality":"auto","output_format":"png","response_format":"url"}`, string(body))
	require.Equal(t, "3:2", req.Values["size"])
	require.Equal(t, json.Number("2"), req.Values["n"])
	require.Equal(t, "1K", req.Values["metadata"].(map[string]any)["resolution"])
	_, _, err = BuildUnifiedMediaPayload(PlatformOpenAI, "video", "model", req, 1)
	require.Error(t, err)
	req.Values["metadata"].(map[string]any)["response_format"] = "b64_json"
	_, _, err = BuildUnifiedMediaPayload(PlatformOpenAI, "image", "model", req, 1)
	require.Error(t, err, "metadata cannot override the URL response contract")
	delete(req.Values["metadata"].(map[string]any), "response_format")
	req.Values["image_urls"] = []any{"https://example.com/reference.png"}
	_, _, err = BuildUnifiedMediaPayload(PlatformOpenAI, "image", "model", req, 1)
	require.Error(t, err, "reference-image editing is outside this adapter's scope")
}

func TestUnifiedMediaUpstreamOpenAICompatibleSynchronousImage(t *testing.T) {
	posts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts++
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v2/images/generations", r.URL.Path)
		require.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.JSONEq(t, `{"model":"gpt-image-2.6-1k","prompt":"cup","n":1,"size":"1024x1024","response_format":"url"}`, string(raw))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"created":1,"data":[{"url":"https://example.com/image.png"}]}`))
	}))
	defer srv.Close()
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": srv.URL + "/v2", "api_key": "secret"}}
	req := &mediacapability.NormalizedRequest{Values: map[string]any{"prompt": "cup", "size": "1:1"}, ImageSpec: &mediacapability.ImageSpec{Width: 1024, Height: 1024}}
	result, err := NewUnifiedMediaUpstream(srv.Client()).Submit(context.Background(), account, "image", "gpt-image-2.6-1k", req, 1)
	require.NoError(t, err)
	require.Equal(t, "accepted", result.Disposition)
	require.Equal(t, "completed", result.Status)
	require.Empty(t, result.TaskID)
	require.Equal(t, []string{"https://example.com/image.png"}, result.URLs)
	require.Equal(t, 1, posts)
}

func TestUnifiedMediaUpstreamOpenAICompatibleDoesNotAcceptOrRepeatErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"rejected", 429, `{"id":"tracking-id","data":[{"url":"https://example.com/not-generated.png"}]}`},
		{"server_error", 502, `{"id":"tracking-id","data":[{"url":"https://example.com/not-generated.png"}]}`},
		{"unexpected_base64", 200, `{"data":[{"b64_json":"aW1hZ2U="}]}`},
		{"async_not_supported", 200, `{"id":"tracking-id","status":"running"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				posts++
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": srv.URL, "api_key": "secret"}}
			result, _ := NewUnifiedMediaUpstream(srv.Client()).Submit(context.Background(), account, "image", "gpt-image-2.6-1k", &mediacapability.NormalizedRequest{Values: map[string]any{"prompt": "cup"}}, 1)
			require.NotEqual(t, "accepted", result.Disposition)
			require.Empty(t, result.URLs)
			require.Empty(t, result.TaskID)
			require.Equal(t, 1, posts)
		})
	}
}
