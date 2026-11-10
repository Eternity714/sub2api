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

func seriesPublicSchemas(t *testing.T) map[string]json.RawMessage {
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
	for _, model := range catalog.Models {
		if strings.HasSuffix(model.Model, "-flare") || strings.HasSuffix(model.Model, "-sunburst") {
			out[model.Model] = model.Schema
		}
	}
	require.ElementsMatch(t, []string{"gpt-image-2.6-flare", "gpt-image-2.6-sunburst", "gpt-image-2.5-flare", "gpt-image-2.5-sunburst"}, mapKeys(out))
	return out
}

func mapKeys(values map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func TestGPTImageSeriesEveryTierRatioQualityPreservesProviderPayloadAndSnapshot(t *testing.T) {
	grids := map[string][][2]int{
		"1K": {{1024, 1024}, {1280, 720}, {720, 1280}, {1152, 864}, {864, 1152}, {1248, 832}, {832, 1248}, {1120, 896}, {896, 1120}, {1456, 624}, {624, 1456}, {720, 1440}, {1440, 720}, {576, 1728}, {1728, 576}},
		"2K": {{2048, 2048}, {2560, 1440}, {1440, 2560}, {2304, 1728}, {1728, 2304}, {2496, 1664}, {1664, 2496}, {2240, 1792}, {1792, 2240}, {3024, 1296}, {1296, 3024}, {1440, 2880}, {2880, 1440}, {1168, 3504}, {3504, 1168}},
		"4K": {{2880, 2880}, {3840, 2160}, {2160, 3840}, {3264, 2448}, {2448, 3264}, {3504, 2336}, {2336, 3504}, {3200, 2560}, {2560, 3200}, {3808, 1632}, {1632, 3808}, {1920, 3840}, {3840, 1920}, {1280, 3840}, {3840, 1280}},
	}
	ratios := []string{"1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3", "5:4", "4:5", "21:9", "9:21", "1:2", "2:1", "1:3", "3:1", "auto"}
	for model, schema := range seriesPublicSchemas(t) {
		compiled, err := mediacapability.Compile("image", schema)
		require.NoError(t, err)
		for tier, pixels := range grids {
			for index, ratio := range ratios {
				position := index
				if ratio == "auto" {
					position = 0
				}
				for _, quality := range []string{"auto", "low", "medium", "high"} {
					for _, platform := range []string{PlatformKuaipao, PlatformGrsai} {
						t.Run(model+"/"+tier+"/"+ratio+"/"+quality+"/"+platform, func(t *testing.T) {
							raw, _ := json.Marshal(map[string]any{"model": model, "prompt": "blue cup", "aspectRatio": ratio, "imageSize": tier, "quality": quality, "replyType": "async"})
							n, fields := compiled.Validate(raw)
							require.Empty(t, fields)
							require.Empty(t, n.FixedImageResolution)
							require.NotNil(t, n.RequestedImageSpec)
							require.Equal(t, pixels[position][0], n.ImageSpec.Width)
							require.Equal(t, pixels[position][1], n.ImageSpec.Height)
							before, _ := json.Marshal(n)
							plan, err := planUnifiedMediaProviderRequest(platform, "image", model, n)
							require.NoError(t, err)
							candidate := UnifiedMediaCandidateSnapshot{ProviderManaged: true, Platform: platform, Model: model, Capability: MediaCapabilitySnapshot{MediaType: "image", Schema: plan.Schema}, AdapterProfile: plan.Profile}
							snapshot, _ := json.Marshal(candidate)
							var restored UnifiedMediaCandidateSnapshot
							require.NoError(t, json.Unmarshal(snapshot, &restored))
							var request mediacapability.NormalizedRequest
							require.NoError(t, json.Unmarshal(before, &request))
							adapted, err := unifiedMediaCandidateRequest(&request, restored)
							require.NoError(t, err)
							body, _, err := BuildUnifiedMediaPayload(platform, "image", model, adapted, 1)
							require.NoError(t, err)
							var payload map[string]any
							require.NoError(t, json.Unmarshal(body, &payload))
							require.Equal(t, quality, payload["quality"])
							require.Equal(t, model, payload["model"])
							if platform == PlatformGrsai {
								require.Equal(t, fmt.Sprintf("%dx%d", pixels[position][0], pixels[position][1]), payload["aspectRatio"])
								require.Equal(t, "async", payload["replyType"])
							} else {
								want := ratio
								if ratio == "auto" {
									want = "1:1"
								}
								require.Equal(t, want, payload["aspect_ratio"])
								require.Equal(t, tier, payload["resolution"])
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
}

func TestGPTImageSeriesDefaultsTierBillingFastProviderPriorityAndInvalidInput(t *testing.T) {
	for model, schema := range seriesPublicSchemas(t) {
		compiled, err := mediacapability.Compile("image", schema)
		require.NoError(t, err)
		n, fields := compiled.Validate([]byte(fmt.Sprintf(`{"model":%q,"prompt":"cup"}`, model)))
		require.Empty(t, fields)
		require.Equal(t, "1K", n.ImageSpec.Resolution)
		require.Equal(t, 1024, n.ImageSpec.Width)
		require.Equal(t, "auto", n.Values["metadata"].(map[string]any)["quality"])
		for _, extra := range []map[string]any{{"aspectRatio": "1024x1024"}, {"aspectRatio": "4:1"}, {"quality": "xhigh"}, {"quality": "max"}, {"imageSize": "8K"}, {"imageSize": "2k"}, {"n": 2}, {"replyType": "json"}, {"allow_pixel_size": true}, {"images": []string{"data:image/png;base64,AA=="}}, {"imageSize": "1K", "resolution": "4K"}} {
			extra["model"], extra["prompt"] = model, "cup"
			raw, _ := json.Marshal(extra)
			_, fields = compiled.Validate(raw)
			require.NotEmpty(t, fields, string(raw))
		}
		capRepo := &mediaCapabilityMemoryRepository{rows: map[int64]*MediaCapability{1: {ID: 1, PublicModel: model, MediaType: "image", Enabled: true, Schema: schema}}}
		routeRows := []CompositeModelRoute{{ID: 1, GroupID: 22, PublicModel: model, MatchType: "exact", Endpoint: "images", TargetPlatform: PlatformGrsai, UpstreamModel: model, Enabled: true, Priority: 20}, {ID: 2, GroupID: 22, PublicModel: model, MatchType: "exact", Endpoint: "images", TargetPlatform: PlatformKuaipao, UpstreamModel: model, Enabled: true, Priority: 10}}
		routes := NewCompositeMediaRouteService(NewCompositeRouteResolver(compositeRouteRepoStub{routes: routeRows}), capRepo)
		pricing := unifiedMediaChannelPricingResolver(22, StatusActive, []ChannelModelPricing{{Platform: PlatformKuaipao, Models: []string{model}, BillingMode: BillingModeImage, Intervals: []PricingInterval{{TierLabel: "1K", PerRequestPrice: float64Ptr(.12)}, {TierLabel: "2K", PerRequestPrice: float64Ptr(.2)}, {TierLabel: "4K", PerRequestPrice: float64Ptr(.35)}}}})
		svc := NewUnifiedMediaTaskService(&unifiedMediaMemoryRepo{}, NewMediaCapabilityService(capRepo), routes, nil, pricing, nil, &unifiedMediaPersister{})
		groupID := int64(22)
		for tier, price := range map[string]float64{"1K": .12, "2K": .2, "4K": .35} {
			task, err := svc.Create(context.Background(), UnifiedMediaTaskCreateInput{APIKey: &APIKey{ID: 33, UserID: 1, GroupID: &groupID, Group: &Group{ID: groupID, Platform: PlatformComposite, RateMultiplier: 1}}, MediaType: "image", Body: []byte(fmt.Sprintf(`{"model":%q,"prompt":"cup","imageSize":%q,"aspectRatio":"auto"}`, model, tier))})
			require.NoError(t, err)
			require.Equal(t, price, task.UnitPrice)
			require.Equal(t, price, task.HeldAmount)
			require.Len(t, task.Payload.Candidates, 2)
			require.Equal(t, PlatformKuaipao, task.Payload.Candidates[0].Platform)
			require.Equal(t, PlatformGrsai, task.Payload.Candidates[1].Platform)
		}
	}
}
