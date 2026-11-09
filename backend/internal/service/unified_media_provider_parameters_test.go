//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
	"github.com/stretchr/testify/require"
)

const providerPublicTestSchema = `{"version":1,"fields":{"prompt":{"type":"string","required":true},"size":{"type":"string","enum":["1:1","3:2","16:9"],"default":"1:1"},"n":{"type":"integer","minimum":1,"maximum":8,"default":1},"image_urls":{"type":"array","items":{"type":"string","format":"http_url"},"min_items":1,"max_items":16},"metadata":{"type":"object","default":{},"properties":{"resolution":{"type":"string","enum":["1K","2K","4K"],"default":"1K"},"quality":{"type":"string","enum":["auto","low","medium","high"],"default":"auto"}}}},"batch":{"native_max_results":1,"allow_split":true},"image_specs":[{"aspect_ratio":"1:1","resolution":"1K","width":1024,"height":1024},{"aspect_ratio":"3:2","resolution":"1K","width":1536,"height":1024},{"aspect_ratio":"16:9","resolution":"1K","width":1280,"height":720},{"aspect_ratio":"1:1","resolution":"2K","width":2048,"height":2048},{"aspect_ratio":"3:2","resolution":"2K","width":2048,"height":1360},{"aspect_ratio":"1:1","resolution":"4K","width":2880,"height":2880}]}`

type providerCapabilityLookups struct {
	MediaCapabilityRepository
	models []string
}

func (r *providerCapabilityLookups) GetByModel(ctx context.Context, model, mediaType string) (*MediaCapability, error) {
	r.models = append(r.models, model)
	return r.MediaCapabilityRepository.GetByModel(ctx, model, mediaType)
}

func providerPublicRequest(t *testing.T, body string) *mediacapability.NormalizedRequest {
	t.Helper()
	compiled, err := mediacapability.Compile("image", []byte(providerPublicTestSchema))
	require.NoError(t, err)
	normalized, fields := compiled.Validate([]byte(body))
	require.Empty(t, fields)
	return normalized
}

func providerRoute(id int64, platform, upstream string) CompositeModelRoute {
	return CompositeModelRoute{ID: id, GroupID: 22, PublicModel: "gpt-image-2.6", MatchType: "exact", Endpoint: "images", TargetPlatform: platform, UpstreamModel: upstream, Priority: int(id), Enabled: true}
}

func TestUnifiedProviderParametersUseOnePublicCapabilityAndProviderSupport(t *testing.T) {
	for _, tc := range []struct{ name, body, platform string }{
		{"quality", `{"model":"gpt-image-2.6","prompt":"cup","metadata":{"quality":"high"}}`, PlatformKuaipao},
		{"resolution", `{"model":"gpt-image-2.6","prompt":"cup","metadata":{"resolution":"2K"}}`, PlatformToAPIs},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &providerCapabilityLookups{MediaCapabilityRepository: &mediaCapabilityMemoryRepository{rows: map[int64]*MediaCapability{170: {ID: 170, PublicModel: "gpt-image-2.6", MediaType: "image", Enabled: true, Revision: 1, SchemaVersion: 1, Schema: json.RawMessage(providerPublicTestSchema)}}}}
			routes := []CompositeModelRoute{providerRoute(1, PlatformGrsai, "gpt-image-2.6")}
			if tc.name == "quality" {
				routes = append(routes, providerRoute(2, PlatformKuaipao, "gpt-image-2.6-1k"))
			} else {
				routes = append(routes, providerRoute(2, PlatformKuaipao, "gpt-image-2.6-1k"), providerRoute(3, PlatformToAPIs, "gpt-image-2.6-flare"))
			}
			svc := NewCompositeMediaRouteService(NewCompositeRouteResolver(compositeRouteRepoStub{routes: routes}), repo)
			normalized := providerPublicRequest(t, tc.body)
			before, _ := json.Marshal(normalized)
			candidates, err := svc.MediaCandidates(context.Background(), 22, "gpt-image-2.6", "image", normalized)
			require.NoError(t, err)
			require.Len(t, candidates, 1)
			require.Equal(t, tc.platform, candidates[0].Route.TargetPlatform)
			require.Empty(t, repo.models, "native provider candidates must not look up upstream-named DB capability records")
			after, _ := json.Marshal(normalized)
			require.JSONEq(t, string(before), string(after))
		})
	}
}

func TestUnifiedProviderParametersRejectFixedTierDowngradeBeforeCreation(t *testing.T) {
	normalized := providerPublicRequest(t, `{"model":"gpt-image-2.6","prompt":"cup","metadata":{"resolution":"2K"}}`)
	repo := &mediaCapabilityMemoryRepository{rows: map[int64]*MediaCapability{170: {ID: 170, PublicModel: "gpt-image-2.6", MediaType: "image", Enabled: true, Schema: json.RawMessage(providerPublicTestSchema)}}}
	svc := NewCompositeMediaRouteService(NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{providerRoute(1, PlatformKuaipao, "gpt-image-2.6-1k")}}), repo)
	_, err := svc.MediaCandidates(context.Background(), 22, "gpt-image-2.6", "image", normalized)
	require.Error(t, err, "a fixed 1K provider must not accept a 2K public request")
}

func TestUnifiedProviderParametersKuaipaoMapsRatioTierAndReferences(t *testing.T) {
	normalized := providerPublicRequest(t, `{"model":"gpt-image-2.6","prompt":"cup","size":"3:2","image_urls":["https://example.com/ref.png"]}`)
	before, _ := json.Marshal(normalized)
	body, path, err := BuildUnifiedMediaPayload(PlatformKuaipao, "image", "gpt-image-2.6-1k", normalized, 1)
	require.NoError(t, err)
	require.Equal(t, "/v1/images/generations", path)
	require.JSONEq(t, `{"model":"gpt-image-2.6-1k","prompt":"cup","aspect_ratio":"3:2","resolution":"1K","quality":"auto","n":1,"response_format":"url","image":["https://example.com/ref.png"]}`, string(body))
	after, _ := json.Marshal(normalized)
	require.JSONEq(t, string(before), string(after))
}

func TestUnifiedProviderParametersKuaipaoPayloadRejectsWrongTier(t *testing.T) {
	normalized := providerPublicRequest(t, `{"model":"gpt-image-2.6","prompt":"cup","metadata":{"resolution":"2K"}}`)
	_, _, err := BuildUnifiedMediaPayload(PlatformKuaipao, "image", "gpt-image-2.6-1k", normalized, 1)
	require.Error(t, err)
}

func TestUnifiedProviderParametersWorkerUsesFrozenRulesAndPreservesPublicInput(t *testing.T) {
	original := providerPublicRequest(t, `{"model":"gpt-image-2.6","prompt":"cup","metadata":{"resolution":"2K"}}`)
	before, _ := json.Marshal(original)
	// A snapshot with a future/custom model proves execution does not consult
	// the current catalog or mutable database capability configuration.
	schema := json.RawMessage(`{"version":1,"fields":{"prompt":{"type":"string","required":true},"size":{"type":"string"},"n":{"type":"integer","minimum":1,"maximum":8},"metadata":{"type":"object","default":{},"properties":{"resolution":{"type":"string","enum":["2K"]},"quality":{"type":"string","enum":["high"],"default":"high"},"output_format":{"type":"string","default":"png"}}}},"batch":{"native_max_results":1,"allow_split":true}}`)
	raw, err := json.Marshal(map[string]any{"platform": "toapis", "model": "captured-model-v1", "provider_managed": true, "capability": MediaCapabilitySnapshot{MediaType: "image", Schema: schema}})
	require.NoError(t, err)
	var candidate UnifiedMediaCandidateSnapshot
	require.NoError(t, json.Unmarshal(raw, &candidate))
	adapted, err := unifiedMediaCandidateRequest(original, candidate)
	require.NoError(t, err)
	metadata := adapted.Values["metadata"].(map[string]any)
	require.Equal(t, "high", metadata["quality"])
	require.Equal(t, "png", metadata["output_format"])
	after, _ := json.Marshal(original)
	require.JSONEq(t, string(before), string(after))
}

type providerCreateRepo struct {
	UnifiedMediaTaskRepository
	creates int
}

func (r *providerCreateRepo) Create(context.Context, *UnifiedMediaTask) error {
	r.creates++
	return nil
}

type providerUncalledPricing struct{ GrsaiPricingResolver }

func TestUnifiedProviderParametersUnsupportedRequestCannotCreateOrFreeze(t *testing.T) {
	caps := &mediaCapabilityMemoryRepository{rows: map[int64]*MediaCapability{170: {ID: 170, PublicModel: "gpt-image-2.6", MediaType: "image", Enabled: true, Schema: json.RawMessage(providerPublicTestSchema)}}}
	routes := NewCompositeMediaRouteService(NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{providerRoute(1, PlatformKuaipao, "gpt-image-2.6-1k")}}), caps)
	repo := &providerCreateRepo{}
	service := NewUnifiedMediaTaskService(repo, NewMediaCapabilityService(caps), routes, nil, providerUncalledPricing{}, nil, &unifiedMediaPersister{})
	id := int64(22)
	_, err := service.Create(context.Background(), UnifiedMediaTaskCreateInput{APIKey: &APIKey{ID: 33, UserID: 1, GroupID: &id, Group: &Group{ID: id, Platform: PlatformComposite, AllowImageGeneration: true}}, MediaType: "image", Body: []byte(`{"model":"gpt-image-2.6","prompt":"cup","imageSize":"2K","replyType":"async"}`)})
	require.Error(t, err)
	require.Zero(t, repo.creates)
}

func TestUnifiedProviderParametersCannotSplitPublicAtomicBatch(t *testing.T) {
	normalized := providerPublicRequest(t, `{"model":"gpt-image-2.6","prompt":"cup","n":2}`)
	// Atomicity is frozen by public validation, independently of provider rules.
	raw, _ := json.Marshal(normalized)
	var object map[string]any
	require.NoError(t, json.Unmarshal(raw, &object))
	object["atomic_batch"] = true
	raw, _ = json.Marshal(object)
	require.NoError(t, json.Unmarshal(raw, &normalized))
	svc := NewCompositeMediaRouteService(NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{providerRoute(1, PlatformGrsai, "gpt-image-2.6")}}), &mediaCapabilityMemoryRepository{})
	_, err := svc.MediaCandidates(context.Background(), 22, "gpt-image-2.6", "image", normalized)
	require.Error(t, err)
}

func TestUnifiedProviderParametersUnknownNativeModelIsRejected(t *testing.T) {
	normalized := providerPublicRequest(t, `{"model":"gpt-image-2.6","prompt":"cup","imageSize":"2K"}`)
	repo := &providerCapabilityLookups{}
	svc := NewCompositeMediaRouteService(NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{providerRoute(1, PlatformGrsai, "custom-image-v1")}}), repo)
	_, err := svc.MediaCandidates(context.Background(), 22, "gpt-image-2.6", "image", normalized)
	require.ErrorContains(t, err, "no verified parameter rules")
	require.Empty(t, repo.models)
}

func TestUnifiedProviderParametersExactPixelsMustBePreserved(t *testing.T) {
	compiled, err := mediacapability.Compile("image", []byte(grsProviderPixelTestSchema))
	require.NoError(t, err)
	for _, tc := range []struct {
		platform, model, pixels string
		supported               bool
	}{
		{PlatformGrsai, "gpt-image-2.6", "1536x1024", true},
		{PlatformKuaipao, "gpt-image-2.6-1k", "1536x1024", false},
		{PlatformKuaipao, "gpt-image-2.6-1k", "1248x832", true},
		{PlatformToAPIs, "gpt-image-2.5-flare-official", "512x512", false},
	} {
		t.Run(tc.platform+"/"+tc.pixels, func(t *testing.T) {
			raw, _ := json.Marshal(map[string]any{"model": "gpt-image-2.6", "prompt": "cup", "aspectRatio": tc.pixels})
			normalized, fields := compiled.Validate(raw)
			require.Empty(t, fields)
			plan, err := planUnifiedMediaProviderRequest(tc.platform, "image", tc.model, normalized)
			if tc.supported {
				require.NoError(t, err)
				require.NotNil(t, plan)
			} else {
				require.Error(t, err)
			}
		})
	}
}

const grsProviderPixelTestSchema = `{"version":1,"fields":{"prompt":{"type":"string","required":true},"aspectRatio":{"type":"string"},"imageSize":{"type":"string","enum":["1K"],"default":"1K"},"quality":{"type":"string","default":"auto"},"n":{"type":"integer","default":1}},"batch":{"native_max_results":1,"allow_split":true}}`

func TestUnifiedProviderParametersWorkerKeepsCapturedFieldMapping(t *testing.T) {
	original := providerPublicRequest(t, `{"model":"gpt-image-2.6","prompt":"cup","image_urls":["https://example.com/ref.png"]}`)
	var candidate UnifiedMediaCandidateSnapshot
	raw, _ := json.Marshal(map[string]any{"platform": "toapis", "model": "captured-model-v1", "provider_managed": true, "capability": MediaCapabilitySnapshot{MediaType: "image", Schema: json.RawMessage(providerPublicTestSchema)}, "adapter_profile": UnifiedMediaAdapterProfile{FieldMap: map[string]string{"image_urls": "captured_references"}, SubmitPath: "/v1/captured-generations"}})
	require.NoError(t, json.Unmarshal(raw, &candidate))
	request, err := unifiedMediaCandidateRequest(original, candidate)
	require.NoError(t, err)
	body, path, err := BuildUnifiedMediaPayload(PlatformToAPIs, "image", candidate.Model, request, 1)
	require.NoError(t, err)
	require.Equal(t, "/v1/captured-generations", path)
	require.Contains(t, string(body), `"captured_references"`)
	require.NotContains(t, string(body), `"image_urls"`)
}

func TestUnifiedProviderParametersWorkerPollUsesFrozenQueryPath(t *testing.T) {
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"processing"}`))
	}))
	defer server.Close()
	var task UnifiedMediaTask
	require.NoError(t, json.Unmarshal([]byte(`{"ID":"media_poll_snapshot","MediaType":"video","Units":[{"state":"polling","count":1,"account_id":7,"platform":"toapis","upstream_model":"gemini-omni-1.1-flash-preview","upstream_task_id":"accepted-id","adapter_profile":{"query_path":"/v1/captured/{id}"}}]}`), &task))
	task.Deadline = time.Now().Add(time.Hour)
	account := Account{ID: 7, Platform: PlatformToAPIs, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": server.URL, "api_key": "test-key"}}
	service := NewUnifiedMediaTaskService(&unifiedMediaMemoryRepo{}, nil, nil, &unifiedMediaAccounts{rows: []Account{account}}, nil, NewUnifiedMediaUpstream(server.Client()), &unifiedMediaPersister{})
	require.NoError(t, service.process(context.Background(), &task))
	require.Equal(t, "/v1/captured/accepted-id", path)
}
