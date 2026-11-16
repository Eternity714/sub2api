//go:build unit

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

	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
	"github.com/stretchr/testify/require"
)

func seedanceUnifiedAccount(base string) *Account {
	return &Account{ID: 53, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": base, "api_key": "test-secret"}, Extra: map[string]any{"grok_video_protocol": "openai"}}
}
func TestUnifiedSeedancePayloadUsesUnifiedFields(t *testing.T) {
	input := &mediacapability.NormalizedRequest{Values: map[string]any{"model": "seedance-2.5", "prompt": "blue cup", "duration": json.Number("4"), "aspect_ratio": "16:9", "metadata": map[string]any{"resolution": "480p"}}}
	body, path, err := BuildUnifiedMediaPayload(PlatformGrok, "video", "seedance-2.5", input, 1)
	require.NoError(t, err)
	require.Equal(t, "/v1/videos", path)
	require.JSONEq(t, `{"model":"seedance-2.5","prompt":"blue cup","seconds":4,"aspect_ratio":"16:9","resolution":"480p"}`, string(body))
	require.Equal(t, json.Number("4"), input.Values["duration"])
	require.Equal(t, map[string]any{"resolution": "480p"}, input.Values["metadata"])
}
func TestUnifiedSeedanceNativeCompletionUsesProtectedContent(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		require.Equal(t, "Bearer test-secret", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.WriteHeader(202)
			_, _ = w.Write([]byte(`{"id":"task_native","status":"queued"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"task_native","status":"completed","result":{"data":[{"url":"https://untrusted.example/not-used.mp4"}]}}`))
	}))
	defer srv.Close()
	upstream := NewUnifiedMediaUpstream(srv.Client())
	a := seedanceUnifiedAccount(srv.URL + "/v1")
	result, err := upstream.Submit(context.Background(), a, "video", "seedance-2.5", &mediacapability.NormalizedRequest{Values: map[string]any{"prompt": "cup", "duration": 4, "metadata": map[string]any{"resolution": "480p"}}}, 1)
	require.NoError(t, err)
	require.Equal(t, "accepted", result.Disposition)
	result, err = upstream.Poll(context.Background(), a, "video", "seedance-2.5", "task_native")
	require.NoError(t, err)
	require.Equal(t, "completed", result.Status)
	require.Empty(t, result.URLs, "native response URLs must never receive provider credentials")
	require.Equal(t, []string{"/v1/videos", "/v1/videos/task_native"}, paths)
}
func TestUnifiedSeedanceRouteProvider(t *testing.T) {
	require.True(t, isMediaRoutePlatform(PlatformGrok))
}
func TestUnifiedSeedanceRequiresExplicitProtocolOptIn(t *testing.T) {
	a := seedanceUnifiedAccount("https://example.com")
	a.Extra = nil
	r, err := NewUnifiedMediaUpstream(nil).Submit(context.Background(), a, "video", "seedance-2.5", &mediacapability.NormalizedRequest{Values: map[string]any{"prompt": "cup"}}, 1)
	require.Error(t, err)
	require.Equal(t, "rejected", r.Disposition)
}

type unifiedSeedanceContentPersister struct {
	ordinaryCalls, contentCalls int
	fail                        bool
}

func (p *unifiedSeedanceContentPersister) Persist(context.Context, string, string, int, int, string) (*UnifiedMediaStoredResult, error) {
	p.ordinaryCalls++
	return nil, errors.New("protected content requires authenticated transfer")
}
func (p *unifiedSeedanceContentPersister) PersistOpenAIVideo(_ context.Context, task string, unit, output int, a *Account, id string) (*UnifiedMediaStoredResult, error) {
	p.contentCalls++
	if p.fail {
		return nil, errors.New("storage temporarily unavailable")
	}
	if a.ID != 53 || id != "task_native" {
		return nil, errors.New("wrong task account")
	}
	return &UnifiedMediaStoredResult{URL: "https://s3.example/seedance.mp4"}, nil
}
func TestUnifiedSeedanceWorkerRetriesTransferWithoutRecreatingVideo(t *testing.T) {
	posts, polls := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			posts++
			_, _ = w.Write([]byte(`{"id":"task_native","status":"queued"}`))
			return
		}
		polls++
		_, _ = w.Write([]byte(`{"id":"task_native","status":"completed"}`))
	}))
	defer srv.Close()
	repo := &unifiedMediaMemoryRepo{}
	now := time.Now()
	task := &UnifiedMediaTask{ID: "media_seedance", Model: "seedance-2.5", MediaType: "video", Status: "in_progress", RequestedCount: 1, UnitPrice: 1.2, HeldAmount: 1.2, Deadline: now.Add(time.Hour), Units: []UnifiedMediaUnit{{State: "polling", Count: 1, AccountID: 53, Platform: PlatformGrok, UpstreamModel: "seedance-2.5", UpstreamTaskID: "task_native"}}}
	persister := &unifiedSeedanceContentPersister{fail: true}
	svc := NewUnifiedMediaTaskService(repo, nil, nil, &unifiedMediaAccounts{rows: []Account{*seedanceUnifiedAccount(srv.URL)}}, nil, NewUnifiedMediaUpstream(srv.Client()), persister)
	svc.Now = func() time.Time { return now }
	require.NoError(t, svc.process(context.Background(), task))
	require.Equal(t, "persisting", task.Units[0].State)
	require.Empty(t, task.PublicView().Result.Data)
	snapshot, err := json.Marshal(task)
	require.NoError(t, err)
	// Simulate a restart after the protected download intent was persisted.
	var restored UnifiedMediaTask
	require.NoError(t, json.Unmarshal(snapshot, &restored))
	now = now.Add(time.Minute)
	persister.fail = false
	require.NoError(t, svc.process(context.Background(), &restored))
	require.Equal(t, "completed", restored.Status)
	require.Equal(t, []string{"https://s3.example/seedance.mp4"}, restored.PublicView().Result.Data)
	require.Equal(t, 1.2, UnifiedMediaTaskCharge(&restored))
	require.Zero(t, posts)
	require.Equal(t, 1, polls)
	require.Equal(t, 2, persister.contentCalls)
	require.Zero(t, persister.ordinaryCalls)
	require.Equal(t, 1, repo.finalized)
}
func TestUnifiedSeedanceContentDownloadBlocksCredentialRedirects(t *testing.T) {
	hits := 0
	client := &http.Client{Transport: videoRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		hits++
		require.Equal(t, "api.example", r.URL.Hostname())
		require.Equal(t, "Bearer test-secret", r.Header.Get("Authorization"))
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://other.example/video.mp4"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})}
	_, _, err := newOpenAIVideoContentDownloader(client, "test-secret").Download(context.Background(), "https://api.example/v1/videos/task_native/content")
	require.Error(t, err)
	require.Equal(t, 1, hits)
}
func TestUnifiedSeedanceContentDownloadValidatesMP4(t *testing.T) {
	for _, body := range []string{"....ftypisompayload", "<html>bad gateway</html>"} {
		client := &http.Client{Transport: videoRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"video/mp4"}}, ContentLength: int64(len(body)), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
		})}
		reader, _, err := newOpenAIVideoContentDownloader(client, "test-secret").Download(context.Background(), "https://api.example/v1/videos/task_native/content")
		if strings.Contains(body, "ftyp") {
			require.NoError(t, err)
			require.NoError(t, reader.Close())
		} else {
			require.Error(t, err)
		}
	}
}
func TestUnifiedSeedanceKeepsExistingGrokChatRoutes(t *testing.T) {
	_, err := compositeRouteFromInput(51, CompositeRouteInput{PublicModel: "grok-4", TargetPlatform: PlatformGrok, Endpoint: CompositeRouteEndpointChatCompletions})
	require.NoError(t, err)
	_, err = compositeRouteFromInput(51, CompositeRouteInput{PublicModel: "seedance-2.5", TargetPlatform: PlatformGrok, Endpoint: CompositeRouteEndpointVideos, CapabilityModel: "seedance-2.5"})
	require.NoError(t, err)
}

func TestUnifiedSeedanceCatalogRejectsUnsupportedModes(t *testing.T) {
	var schema json.RawMessage
	for _, entry := range MediaCapabilityCatalog() {
		if entry.Platform == PlatformGrok && entry.UpstreamModel == "seedance-2.5" && entry.MediaType == "video" {
			schema = entry.Schema
		}
	}
	require.NotEmpty(t, schema)
	compiled, err := mediacapability.Compile("video", schema)
	require.NoError(t, err)
	good := `{"model":"seedance-2.5","prompt":"cup","duration":4,"aspect_ratio":"16:9","metadata":{"resolution":"480p"}}`
	normalized, fields := compiled.Validate([]byte(good))
	require.Empty(t, fields)
	require.NotNil(t, normalized)
	for _, bad := range []string{
		`{"model":"seedance-2.5","prompt":"cup","duration":3}`,
		`{"model":"seedance-2.5","prompt":"cup","duration":31}`,
		`{"model":"seedance-2.5","prompt":"cup","duration":4,"image_urls":["https://example.com/first.png"]}`,
		`{"model":"seedance-2.5","prompt":"cup","duration":4,"metadata":{"resolution":"4K"}}`,
		`{"model":"seedance-2.5","prompt":"cup","duration":4,"metadata":{"reference_videos":["https://example.com/video.mp4"]}}`,
	} {
		_, errs := compiled.Validate([]byte(bad))
		require.NotEmpty(t, errs, bad)
	}
}

func TestUnifiedSeedanceDoesNotCaptureExistingGrokVideoRoutes(t *testing.T) {
	resolver := NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{
		{ID: 1, GroupID: 51, PublicModel: "grok-imagine-video", UpstreamModel: "grok-imagine-video", TargetPlatform: PlatformGrok, Endpoint: "videos", MatchType: "exact", Enabled: true},
	}})
	enabled, err := resolver.HasMediaRoutes(context.Background(), 51, "videos")
	require.NoError(t, err)
	require.False(t, enabled)
}
