//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
	"github.com/stretchr/testify/require"
)

var fixed2KDimensions = map[string][2]int{
	"auto": {2048, 2048}, "1:1": {2048, 2048}, "16:9": {2560, 1440}, "9:16": {1440, 2560},
	"4:3": {2304, 1728}, "3:4": {1728, 2304}, "3:2": {2496, 1664}, "2:3": {1664, 2496},
	"5:4": {2240, 1792}, "4:5": {1792, 2240}, "21:9": {3024, 1296}, "9:21": {1296, 3024},
	"1:2": {1440, 2880}, "2:1": {2880, 1440}, "1:3": {1168, 3504}, "3:1": {3504, 1168},
}

func fixed2KPublicSchemas(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "media-api-catalog.json"))
	require.NoError(t, err)
	var catalog struct {
		Models []struct {
			Model  string          `json:"model"`
			Schema json.RawMessage `json:"schema"`
		} `json:"models"`
	}
	require.NoError(t, json.Unmarshal(raw, &catalog))
	out := map[string]json.RawMessage{}
	for _, m := range catalog.Models {
		if m.Model == "gpt-image-2.5-2k" || m.Model == "gpt-image-2.6-2k" {
			out[m.Model] = m.Schema
		}
	}
	require.Len(t, out, 2)
	return out
}

func fixed2KRequest(t *testing.T, schema json.RawMessage, model, ratio, quality string) *mediacapability.NormalizedRequest {
	t.Helper()
	compiled, err := mediacapability.Compile("image", schema)
	require.NoError(t, err)
	raw, _ := json.Marshal(map[string]any{"model": model, "prompt": "blue cup", "aspectRatio": ratio, "quality": quality, "images": []string{"https://example.com/ref.png"}, "replyType": "async"})
	n, fields := compiled.Validate(raw)
	require.Empty(t, fields)
	return n
}

func TestFixed2KBothModelsPreserveEveryRatioAndQualityAcrossProviders(t *testing.T) {
	for model, schema := range fixed2KPublicSchemas(t) {
		for ratio, pixels := range fixed2KDimensions {
			for _, quality := range []string{"auto", "low", "medium", "high"} {
				for _, platform := range []string{PlatformGrsai, PlatformKuaipao} {
					t.Run(model+"/"+ratio+"/"+quality+"/"+platform, func(t *testing.T) {
						n := fixed2KRequest(t, schema, model, ratio, quality)
						before, _ := json.Marshal(n)
						upstream := model
						if platform == PlatformGrsai {
							upstream = strings.TrimSuffix(model, "-2k") + "-flare"
						}
						plan, err := planUnifiedMediaProviderRequest(platform, "image", upstream, n)
						require.NoError(t, err)
						candidate := UnifiedMediaCandidateSnapshot{ProviderManaged: true, Platform: platform, Model: upstream, Capability: MediaCapabilitySnapshot{MediaType: "image", Schema: plan.Schema}, AdapterProfile: plan.Profile}
						// Workers retain only persisted JSON, including the selected pixel rule.
						snapshot, _ := json.Marshal(candidate)
						var restored UnifiedMediaCandidateSnapshot
						require.NoError(t, json.Unmarshal(snapshot, &restored))
						var restoredRequest mediacapability.NormalizedRequest
						require.NoError(t, json.Unmarshal(before, &restoredRequest))
						request, err := unifiedMediaCandidateRequest(&restoredRequest, restored)
						require.NoError(t, err)
						require.Equal(t, pixels[0], request.ImageSpec.Width)
						require.Equal(t, pixels[1], request.ImageSpec.Height)
						body, _, err := BuildUnifiedMediaPayload(platform, "image", upstream, request, 1)
						require.NoError(t, err)
						var payload map[string]any
						require.NoError(t, json.Unmarshal(body, &payload))
						require.Equal(t, upstream, payload["model"])
						require.Equal(t, quality, payload["quality"])
						require.NotContains(t, payload, "imageSize")
						if platform == PlatformGrsai {
							require.Equal(t, fmt.Sprintf("%dx%d", pixels[0], pixels[1]), payload["aspectRatio"])
							require.Equal(t, "async", payload["replyType"])
							require.Equal(t, []any{"https://example.com/ref.png"}, payload["images"])
							require.NotContains(t, payload, "resolution")
						} else {
							wantRatio := ratio
							if ratio == "auto" {
								wantRatio = "1:1"
							}
							require.Equal(t, wantRatio, payload["aspect_ratio"])
							require.Equal(t, "2K", payload["resolution"])
							require.Equal(t, []any{"https://example.com/ref.png"}, payload["image"])
							require.NotContains(t, payload, "size")
						}
						after, _ := json.Marshal(n)
						require.JSONEq(t, string(before), string(after))
					})
				}
			}
		}
	}
}

func TestFixed2KCannotOverridePrivateTierOrCreateUnsupportedTasks(t *testing.T) {
	for model, schema := range fixed2KPublicSchemas(t) {
		repo := &providerCreateRepo{}
		caps := &mediaCapabilityMemoryRepository{rows: map[int64]*MediaCapability{1: {ID: 1, PublicModel: model, MediaType: "image", Enabled: true, Schema: schema}}}
		routes := NewCompositeMediaRouteService(NewCompositeRouteResolver(compositeRouteRepoStub{}), caps)
		svc := NewUnifiedMediaTaskService(repo, NewMediaCapabilityService(caps), routes, nil, providerUncalledPricing{}, nil, &unifiedMediaPersister{})
		id := int64(22)
		for _, extra := range []map[string]any{{"imageSize": "2K"}, {"resolution": "2K"}, {"metadata": map[string]any{"resolution": "2K"}}, {"aspectRatio": "2048x2048"}, {"aspectRatio": "4:1"}, {"quality": "max"}, {"n": 2}, {"replyType": "stream"}, {"fixed_image_resolution": "4K"}} {
			extra["model"], extra["prompt"] = model, "cup"
			raw, _ := json.Marshal(extra)
			_, err := svc.Create(context.Background(), UnifiedMediaTaskCreateInput{APIKey: &APIKey{ID: 33, UserID: 1, GroupID: &id, Group: &Group{ID: id, Platform: PlatformComposite}}, MediaType: "image", Body: raw})
			require.Error(t, err, string(raw))
			require.Zero(t, repo.creates)
		}
	}
}

func TestFixed2KPricingUsesCapturedTierAndWorkerDoesNotRecalculatePixels(t *testing.T) {
	model := "gpt-image-2.6-2k"
	schema := fixed2KPublicSchemas(t)[model]
	caps := &mediaCapabilityMemoryRepository{rows: map[int64]*MediaCapability{1: {ID: 1, PublicModel: model, MediaType: "image", Enabled: true, Schema: schema}}}
	route := CompositeModelRoute{ID: 1, GroupID: 22, PublicModel: model, MatchType: "exact", Endpoint: "images", TargetPlatform: PlatformGrsai, UpstreamModel: "gpt-image-2.6-flare", Enabled: true}
	routes := NewCompositeMediaRouteService(NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{route}}), caps)
	pricing := unifiedMediaChannelPricingResolver(22, StatusActive, []ChannelModelPricing{{Platform: PlatformGrsai, Models: []string{model}, BillingMode: BillingModeImage, Intervals: []PricingInterval{{TierLabel: "2K", PerRequestPrice: float64Ptr(.2)}}}})
	svc := NewUnifiedMediaTaskService(&unifiedMediaMemoryRepo{}, NewMediaCapabilityService(caps), routes, nil, pricing, nil, &unifiedMediaPersister{})
	id := int64(22)
	task, err := svc.Create(context.Background(), UnifiedMediaTaskCreateInput{APIKey: &APIKey{ID: 33, UserID: 1, GroupID: &id, Group: &Group{ID: id, Platform: PlatformComposite, RateMultiplier: 1}}, MediaType: "image", Body: []byte(`{"model":"gpt-image-2.6-2k","prompt":"cup","aspectRatio":"16:9","quality":"high"}`)})
	require.NoError(t, err)
	require.Equal(t, .2, task.UnitPrice)
	require.Equal(t, .2, task.HeldAmount)
	// This name is deliberately absent from the catalogue. Captured provider
	// rules must still reproduce the accepted geometry after a worker restart.
	task.Payload.Candidates[0].Model = "gpt-image-2.6-flare-captured"
	snapshot, _ := json.Marshal(task)
	var restored UnifiedMediaTask
	require.NoError(t, json.Unmarshal(snapshot, &restored))
	request, err := unifiedMediaCandidateRequest(restored.Payload.Normalized, restored.Payload.Candidates[0])
	require.NoError(t, err)
	body, _, err := BuildUnifiedMediaPayload(PlatformGrsai, "image", restored.Payload.Candidates[0].Model, request, 1)
	require.NoError(t, err)
	require.Contains(t, string(body), `"aspectRatio":"2560x1440"`)
	require.Contains(t, string(body), `"quality":"high"`)
	// Planning a fixed public model must not overwrite generic Flare defaults.
	for _, entry := range MediaCapabilityCatalog() {
		if entry.Platform == PlatformGrsai && entry.UpstreamModel == "gpt-image-2.6-flare" {
			var generic mediacapability.Schema
			require.NoError(t, json.Unmarshal(entry.Schema, &generic))
			for _, spec := range generic.ImageSpecs {
				if spec.AspectRatio == "16:9" && spec.Resolution == "2K" {
					require.Equal(t, 2048, spec.Width)
					require.Equal(t, 1152, spec.Height)
				}
			}
		}
	}
}
