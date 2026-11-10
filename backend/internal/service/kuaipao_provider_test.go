//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

func TestKuaipaoProviderTranslatesPublicModelToUpstream(t *testing.T) {
	route, err := compositeRouteFromInput(22, CompositeRouteInput{PublicModel: "gpt-image-2.6", TargetPlatform: "kuaipao", UpstreamModel: "gpt-image-2.6-1k", CapabilityModel: "gpt-image-2.6-1k", Endpoint: "images", Enabled: true})
	require.NoError(t, err)
	route.ID = 134
	resolver := NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{*route}})
	candidates, err := resolver.ResolveMediaCandidates(context.Background(), 22, "gpt-image-2.6", "images")
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, "kuaipao", candidates[0].TargetPlatform)
	require.Equal(t, "gpt-image-2.6-1k", candidates[0].UpstreamModel)
	posts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts++
		require.Equal(t, "/v2/images/generations", r.URL.Path)
		require.Equal(t, "Bearer provider-key", r.Header.Get("Authorization"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.JSONEq(t, `{"model":"gpt-image-2.6-1k","prompt":"blue cup","aspect_ratio":"1:1","resolution":"1K","quality":"auto","n":1,"response_format":"url"}`, string(body))
		_, _ = w.Write([]byte(`{"data":[{"url":"https://example.com/image.png"}]}`))
	}))
	defer srv.Close()
	req := &mediacapability.NormalizedRequest{Values: map[string]any{"model": "gpt-image-2.6", "prompt": "blue cup", "size": "1:1", "metadata": map[string]any{"resolution": "1K", "quality": "auto"}}, ImageSpec: &mediacapability.ImageSpec{Width: 1024, Height: 1024}}
	account := &Account{Platform: "kuaipao", Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": srv.URL + "/v2", "api_key": "provider-key"}}
	result, err := NewUnifiedMediaUpstream(srv.Client()).Submit(context.Background(), account, "image", candidates[0].UpstreamModel, req, 1)
	require.NoError(t, err)
	require.Equal(t, "accepted", result.Disposition)
	require.Equal(t, "completed", result.Status)
	require.Equal(t, []string{"https://example.com/image.png"}, result.URLs)
	require.Equal(t, 1, posts)
	require.Equal(t, "gpt-image-2.6", req.Values["model"])
}

func TestKuaipaoProviderCatalogHasExecutable1KTemplate(t *testing.T) {
	var entry *MediaCapabilityCatalogEntry
	for _, candidate := range MediaCapabilityCatalog() {
		if candidate.Platform == "kuaipao" && candidate.UpstreamModel == "gpt-image-2.6-1k" {
			copy := candidate
			entry = &copy
			break
		}
	}
	require.NotNil(t, entry)
	require.Equal(t, "image", entry.MediaType)
	compiled, err := mediacapability.Compile("image", entry.Schema)
	require.NoError(t, err)
	_, fields := compiled.Validate([]byte(`{"model":"gpt-image-2.6","prompt":"cup","size":"1:1","metadata":{"resolution":"1K"}}`))
	require.Empty(t, fields)
	_, fields = compiled.Validate([]byte(`{"model":"gpt-image-2.6","prompt":"cup","size":"1:1","metadata":{"resolution":"4K"}}`))
	require.NotEmpty(t, fields)
}

func TestKuaipaoAccountCreationKeepsProviderAndRequiresAPIKey(t *testing.T) {
	repo := &upstreamBillingProbeAccountRepo{}
	a, err := (&adminServiceImpl{accountRepo: repo}).CreateAccount(context.Background(), &CreateAccountInput{Name: "快跑", Platform: "kuaipao", Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "provider-key", "base_url": "https://image.kuaipao.pro/v2"}, SkipDefaultGroupBind: true})
	require.NoError(t, err)
	require.Equal(t, "kuaipao", a.Platform)
	require.Equal(t, "https://image.kuaipao.pro/v2", a.GetCredential("base_url"))
	_, err = (&adminServiceImpl{accountRepo: repo}).CreateAccount(context.Background(), &CreateAccountInput{Name: "invalid", Platform: "kuaipao", Type: AccountTypeOAuth, Credentials: map[string]any{"api_key": "provider-key"}, SkipDefaultGroupBind: true})
	require.Error(t, err)
	require.False(t, IsUpstreamBillingProbeIdentity("kuaipao", AccountTypeAPIKey))
	_, err = (&adminServiceImpl{accountRepo: repo}).UpdateAccount(context.Background(), a.ID, &UpdateAccountInput{Type: AccountTypeOAuth})
	require.Error(t, err)
}

func TestKuaipaoProviderDoesNotReplayUncertainSubmission(t *testing.T) {
	posts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts++
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"temporarily unavailable"}`))
	}))
	defer srv.Close()
	account := &Account{Platform: PlatformKuaipao, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "key", "base_url": srv.URL + "/v2"}}
	normalized := &mediacapability.NormalizedRequest{Values: map[string]any{"prompt": "cup"}, ImageSpec: &mediacapability.ImageSpec{Width: 1024, Height: 1024}}
	result, err := NewUnifiedMediaUpstream(srv.Client()).Submit(context.Background(), account, "image", KuaipaoDefaultImageModel, normalized, 1)
	require.Error(t, err)
	require.Equal(t, "unknown", result.Disposition)
	require.Equal(t, 1, posts)
}

type kuaipaoAccountTestUpstream struct {
	HTTPUpstream
	requests []*http.Request
}

func (u *kuaipaoAccountTestUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.requests = append(u.requests, req)
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"data":[{"url":"https://example.com/cup.png"}]}`))}, nil
}

func (u *kuaipaoAccountTestUpstream) DoWithTLS(req *http.Request, proxy string, id int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxy, id, concurrency)
}

func TestKuaipaoAccountTestUsesItsImageProvider(t *testing.T) {
	u := &kuaipaoAccountTestUpstream{}
	a := Account{ID: 21, Platform: "kuaipao", Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://image.example.com/v2", "api_key": "test-key", "model_mapping": map[string]any{"gpt-image-2.6": "gpt-image-2.6-1k"}}}
	s := &AccountTestService{accountRepo: &unifiedMediaAccounts{rows: []Account{a}}, httpUpstream: u, cfg: &config.Config{}}
	c, w := newTestContext()
	err := s.TestAccountConnection(c, 21, "gpt-image-2.6", "blue cup", "")
	require.NoError(t, err)
	require.Len(t, u.requests, 1)
	require.Equal(t, "/v2/images/generations", u.requests[0].URL.Path)
	body, err := io.ReadAll(u.requests[0].Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `"model":"gpt-image-2.6-1k"`)
	require.Contains(t, w.Body.String(), `"type":"image"`)
	require.Contains(t, w.Body.String(), `"success":true`)
}

func TestKuaipaoAccountTestPreservesMappedFixedTier(t *testing.T) {
	for _, tier := range []string{"2k", "4k"} {
		t.Run(tier, func(t *testing.T) {
			u := &kuaipaoAccountTestUpstream{}
			model := "gpt-image-2.6-" + tier
			a := Account{ID: 21, Platform: PlatformKuaipao, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://image.example.com/v2", "api_key": "test-key", "model_mapping": map[string]any{model: model}}}
			s := &AccountTestService{accountRepo: &unifiedMediaAccounts{rows: []Account{a}}, httpUpstream: u, cfg: &config.Config{}}
			c, w := newTestContext()
			require.NoError(t, s.TestAccountConnection(c, 21, model, "cup", ""))
			require.Len(t, u.requests, 1)
			body, err := io.ReadAll(u.requests[0].Body)
			require.NoError(t, err)
			require.Contains(t, string(body), `"resolution":"`+strings.ToUpper(tier)+`"`)
			require.Contains(t, w.Body.String(), `"success":true`)
		})
	}
}
