//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
	"github.com/stretchr/testify/require"
)

func TestCentOSNanoBackupFreezesGeometryExceptionWithoutChangingGRSOrPublicValues(t *testing.T) {
	for public, schema := range nanoPublicSchemas(t) {
		upstream := map[string]string{"nano-banana-pro": "gemini-3-pro-image-preview", "nano-banana-2.1": "gemini-3.1-flash-image-preview"}[public]
		for _, tier := range []string{"1k", "2K", "4k"} {
			for _, ratio := range []string{"auto", "1:1", "9:21"} {
				t.Run(public+"/"+tier+"/"+ratio, func(t *testing.T) {
					compiled, err := mediacapability.Compile("image", schema)
					require.NoError(t, err)
					raw, _ := json.Marshal(map[string]any{"model": public, "prompt": "cup", "aspectRatio": ratio, "imageSize": tier})
					original, fields := compiled.Validate(raw)
					require.Empty(t, fields)
					before, _ := json.Marshal(original)
					caps := &mediaCapabilityMemoryRepository{rows: map[int64]*MediaCapability{1: {ID: 1, PublicModel: public, MediaType: "image", Enabled: true, Schema: schema}}}
					routes := NewCompositeMediaRouteService(NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{
						{ID: 2, GroupID: 22, PublicModel: public, MatchType: "exact", Endpoint: "images", TargetPlatform: PlatformCentOS, UpstreamModel: upstream, Priority: 20, Enabled: true},
						{ID: 1, GroupID: 22, PublicModel: public, MatchType: "exact", Endpoint: "images", TargetPlatform: PlatformGrsai, UpstreamModel: public, Priority: 10, Enabled: true},
					}}), caps)
					candidates, err := routes.MediaCandidates(context.Background(), 22, public, "image", original)
					require.NoError(t, err)
					require.Len(t, candidates, 2)
					require.Equal(t, PlatformGrsai, candidates[0].Route.TargetPlatform)
					require.False(t, candidates[0].AdapterProfile.DefaultImageGeometry)
					require.Equal(t, PlatformCentOS, candidates[1].Route.TargetPlatform)
					require.True(t, candidates[1].AdapterProfile.DefaultImageGeometry)
					snapshot, _ := json.Marshal(UnifiedMediaCandidateSnapshot{Platform: PlatformCentOS, Model: upstream, ProviderManaged: true, Capability: candidates[1].Capability, AdapterProfile: candidates[1].AdapterProfile})
					var restored UnifiedMediaCandidateSnapshot
					require.NoError(t, json.Unmarshal(snapshot, &restored))
					adapted, err := unifiedMediaCandidateRequest(original, restored)
					require.NoError(t, err)
					body, path, err := BuildUnifiedMediaPayload(PlatformCentOS, "image", upstream, adapted, 1)
					require.NoError(t, err)
					require.Equal(t, "/v1/chat/completions", path)
					require.JSONEq(t, fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"cup"}],"stream":false,"modalities":["text","image"]}`, upstream), string(body))
					restored.AdapterProfile.DefaultImageGeometry = false
					_, err = unifiedMediaCandidateRequest(original, restored)
					require.NoError(t, err)
					disabled, _ := json.Marshal(restored.AdapterProfile)
					adapted.AdapterProfile = disabled
					_, _, err = BuildUnifiedMediaPayload(PlatformCentOS, "image", upstream, adapted, 1)
					require.Error(t, err, "captured policy must not silently inherit a newer default")
					after, _ := json.Marshal(original)
					require.JSONEq(t, string(before), string(after))
				})
			}
		}
	}
	require.False(t, mediaAdapterProfile(PlatformCentOS, "image", "gpt-image-2.6-flare").DefaultImageGeometry)
}

func TestCentOSNanoDefiniteGRSRejectionFallsBackAndStorageRecoveryDoesNotRegenerate(t *testing.T) {
	var pixels bytes.Buffer
	require.NoError(t, png.Encode(&pixels, image.NewRGBA(image.Rect(0, 0, 8, 4))))
	inline := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pixels.Bytes())
	var posts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts = append(posts, r.URL.Path)
		if r.URL.Path == "/v1/api/generate" {
			w.WriteHeader(429)
			fmt.Fprint(w, `{"error":"busy"}`)
			return
		}
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, false, body["stream"])
		require.NotContains(t, body, "image_config")
		require.NotContains(t, body, "aspectRatio")
		require.NotContains(t, body, "imageSize")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "![image](" + inline + ")"}, "finish_reason": "stop"}}})
	}))
	defer server.Close()
	schema := nanoPublicSchemas(t)["nano-banana-pro"]
	compiled, err := mediacapability.Compile("image", schema)
	require.NoError(t, err)
	original, fields := compiled.Validate([]byte(`{"model":"nano-banana-pro","prompt":"cup","aspectRatio":"9:21","imageSize":"4K"}`))
	require.Empty(t, fields)
	var candidates []UnifiedMediaCandidateSnapshot
	for _, pair := range [][2]string{{PlatformGrsai, "nano-banana-pro"}, {PlatformCentOS, "gemini-3-pro-image-preview"}} {
		plan, err := planUnifiedMediaProviderRequest(pair[0], "image", pair[1], original)
		require.NoError(t, err)
		candidates = append(candidates, UnifiedMediaCandidateSnapshot{Platform: pair[0], Model: pair[1], ProviderManaged: true, Capability: MediaCapabilitySnapshot{MediaType: "image", Schema: plan.Schema}, AdapterProfile: plan.Profile, NativeMaxResults: 1})
	}
	accounts := &unifiedMediaAccounts{rows: []Account{
		{ID: 19, Platform: PlatformGrsai, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"api_key": "test-key", "base_url": server.URL}},
		{ID: 22, Platform: PlatformCentOS, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"api_key": "test-key", "base_url": server.URL}},
	}}
	storage := &grsaiMetadataStorage{failAt: 1}
	uploader := NewImageResultUploader(storage, "images/", 0, nil)
	now := time.Now()
	svc := NewUnifiedMediaTaskService(&unifiedMediaMemoryRepo{}, nil, nil, accounts, nil, NewUnifiedMediaUpstream(server.Client()), &UnifiedMediaS3Persister{resolve: func() (*ImageResultUploader, bool) { return uploader, true }})
	svc.Now = func() time.Time { return now }
	task := &UnifiedMediaTask{ID: "media_nano_backup", GroupID: 22, Model: "nano-banana-pro", MediaType: "image", RequestedCount: 1, UnitPrice: .2, AccountUnitPrice: .2, HeldAmount: .2, Deadline: now.Add(time.Hour), Payload: UnifiedMediaTaskPayload{Normalized: original, Candidates: candidates}, Units: []UnifiedMediaUnit{{State: "queued", Count: 1}}}
	require.NoError(t, svc.submit(context.Background(), task, 0))
	require.Equal(t, PlatformCentOS, task.Units[0].Platform)
	require.NoError(t, svc.persist(context.Background(), task, 0))
	require.Equal(t, "persisting", task.Units[0].State)
	require.Zero(t, UnifiedMediaTaskCharge(task))
	frozen, _ := json.Marshal(task)
	var restored UnifiedMediaTask
	require.NoError(t, json.Unmarshal(frozen, &restored))
	storage.failAt = 0
	now = now.Add(time.Minute)
	require.NoError(t, svc.persist(context.Background(), &restored, 0))
	require.Equal(t, .2, UnifiedMediaTaskCharge(&restored))
	require.Equal(t, []string{"/v1/api/generate", "/v1/chat/completions"}, posts)
	require.Equal(t, pixels.Bytes(), storage.saved[0].data)
	require.True(t, restored.Units[0].AdapterProfile.DefaultImageGeometry)
}

func TestCentOSNanoReferencesBecomeInlineWithoutDisclosingKeyAndUnsafeInputNeverSubmits(t *testing.T) {
	var pixels bytes.Buffer
	require.NoError(t, png.Encode(&pixels, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	inline := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pixels.Bytes())
	gets, posts := 0, 0
	upstream := NewUnifiedMediaUpstream(nil).(*unifiedMediaHTTPUpstream)
	upstream.transport = &systemOneHTTPUpstream{do: func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet {
			gets++
			require.Empty(t, req.Header.Get("Authorization"))
			require.True(t, HTTPUpstreamPublicHostsOnly(req.Context()))
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(pixels.Bytes()))}, nil
		}
		posts++
		require.Equal(t, "/v1/chat/completions", req.URL.Path)
		require.Equal(t, "Bearer test-key", req.Header.Get("Authorization"))
		require.True(t, HTTPUpstreamRedirectsDisabled(req.Context()))
		var body map[string]any
		require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
		parts := body["messages"].([]any)[0].(map[string]any)["content"].([]any)
		require.Len(t, parts, 3)
		for _, part := range parts[1:] {
			require.Equal(t, inline, part.(map[string]any)["image_url"].(map[string]any)["url"])
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"choices":[{"message":{"content":%q},"finish_reason":"stop"}]}`, inline)))}, nil
	}}
	request := &mediacapability.NormalizedRequest{Values: map[string]any{"prompt": "edit cup", "size": "9:21", "metadata": map[string]any{"resolution": "4K"}, "image_urls": []any{"https://reference.example/a.png", "https://reference.example/b.png"}}, RequestedCount: 1}
	before, _ := json.Marshal(request)
	account := &Account{Platform: PlatformCentOS, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test-key"}}
	result, err := upstream.Submit(context.Background(), account, "image", "gemini-3.1-flash-image-preview", request, 1)
	require.NoError(t, err)
	require.Equal(t, "accepted", result.Disposition)
	require.Equal(t, []string{inline}, result.URLs)
	after, _ := json.Marshal(request)
	require.JSONEq(t, string(before), string(after))
	require.Equal(t, 2, gets)
	require.Equal(t, 1, posts)
	request.Values["image_urls"] = []any{"http://127.0.0.1/private.png"}
	result, err = upstream.Submit(context.Background(), account, "image", "gemini-3.1-flash-image-preview", request, 1)
	require.Error(t, err)
	require.Equal(t, "rejected", result.Disposition)
	require.Equal(t, 1, posts)
}

func TestCentOSNanoRequiresOneCompleteInlineImage(t *testing.T) {
	valid := `{"choices":[{"message":{"content":"![image](data:image/png;base64,AAAA)"},"finish_reason":"stop"}]}`
	require.Equal(t, []string{"data:image/png;base64,AAAA"}, parseCentOSNanoImages([]byte(valid)))
	for _, raw := range []string{`{}`, strings.Replace(valid, `"stop"`, `"length"`, 1), strings.Replace(valid, "AAAA)", "AAAA) data:image/jpeg;base64,BBBB", 1), strings.Replace(valid, "data:image/png;base64,AAAA", "https://example.com/image.png", 1), strings.TrimSuffix(valid, "}")} {
		require.Empty(t, parseCentOSNanoImages([]byte(raw)), raw)
	}
}
