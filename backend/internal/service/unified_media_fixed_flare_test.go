//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
	"github.com/stretchr/testify/require"
)

func TestFixedFlarePublicModelsRouteFreezeAndPriceIndependently(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "media-api-catalog.json"))
	require.NoError(t, err)
	var catalog struct {
		Models []struct {
			Model   string          `json:"model"`
			Schema  json.RawMessage `json:"schema"`
			Pricing struct {
				UnitPrice float64 `json:"unit_price"`
			} `json:"pricing"`
		} `json:"models"`
	}
	require.NoError(t, json.Unmarshal(raw, &catalog))
	found := 0
	ratios := []string{"auto", "1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3", "5:4", "4:5", "21:9", "9:21", "1:2", "2:1"}
	for _, entry := range catalog.Models {
		if entry.Model != "gpt-image-2.5" && entry.Model != "gpt-image-2.6" {
			continue
		}
		found++
		model, upstream := entry.Model, entry.Model+"-flare"
		require.Equal(t, .08, entry.Pricing.UnitPrice)
		compiled, err := mediacapability.Compile("image", entry.Schema)
		require.NoError(t, err)
		capRepo := &mediaCapabilityMemoryRepository{rows: map[int64]*MediaCapability{1: {ID: 1, PublicModel: model, MediaType: "image", Enabled: true, Schema: entry.Schema}}}
		routes := NewCompositeMediaRouteService(NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{
			{ID: 1, GroupID: 22, PublicModel: model, MatchType: "exact", Endpoint: "images", TargetPlatform: PlatformGrsai, UpstreamModel: upstream, Enabled: true, Priority: 20},
			{ID: 2, GroupID: 22, PublicModel: model, MatchType: "exact", Endpoint: "images", TargetPlatform: PlatformKuaipao, UpstreamModel: upstream, Enabled: true, Priority: 10},
		}}), capRepo)
		pricing := unifiedMediaChannelPricingResolver(22, StatusActive, []ChannelModelPricing{
			{Platform: PlatformKuaipao, Models: []string{upstream}, BillingMode: BillingModeImage, Intervals: []PricingInterval{{TierLabel: "1K", PerRequestPrice: float64Ptr(.12)}}},
			{Platform: PlatformKuaipao, Models: []string{model}, BillingMode: BillingModeImage, PerRequestPrice: float64Ptr(.08)},
		})
		svc := NewUnifiedMediaTaskService(&unifiedMediaMemoryRepo{}, NewMediaCapabilityService(capRepo), routes, nil, pricing, nil, &unifiedMediaPersister{})
		groupID := int64(22)
		for _, ratio := range ratios {
			t.Run(model+"/"+ratio, func(t *testing.T) {
				body, _ := json.Marshal(map[string]any{"model": model, "prompt": "cup", "aspectRatio": ratio})
				task, err := svc.Create(context.Background(), UnifiedMediaTaskCreateInput{APIKey: &APIKey{ID: 33, UserID: 1, GroupID: &groupID, Group: &Group{ID: groupID, Platform: PlatformComposite, RateMultiplier: 1}}, MediaType: "image", Body: body})
				require.NoError(t, err)
				require.Equal(t, model, task.Model)
				require.Equal(t, .08, task.UnitPrice)
				require.Equal(t, .08, task.HeldAmount)
				require.Equal(t, "1K", task.Payload.Normalized.FixedImageResolution)
				require.Len(t, task.Payload.Candidates, 2)
				require.Equal(t, PlatformKuaipao, task.Payload.Candidates[0].Platform)
				require.Equal(t, PlatformGrsai, task.Payload.Candidates[1].Platform)
				snapshot, _ := json.Marshal(task.Payload)
				var restored UnifiedMediaTaskPayload
				require.NoError(t, json.Unmarshal(snapshot, &restored))
				for _, candidate := range restored.Candidates {
					require.Equal(t, upstream, candidate.Model)
					n, err := unifiedMediaCandidateRequest(restored.Normalized, candidate)
					require.NoError(t, err)
					payload, _, err := BuildUnifiedMediaPayload(candidate.Platform, "image", candidate.Model, n, 1)
					require.NoError(t, err)
					var output map[string]any
					require.NoError(t, json.Unmarshal(payload, &output))
					require.Equal(t, upstream, output["model"])
					require.Equal(t, "auto", output["quality"])
					require.NotContains(t, output, "imageSize")
					if candidate.Platform == PlatformGrsai {
						spec := restored.Normalized.ImageSpec
						require.Equal(t, fmt.Sprintf("%dx%d", spec.Width, spec.Height), output["aspectRatio"])
						require.Equal(t, "async", output["replyType"])
					} else {
						want := ratio
						if want == "auto" {
							want = "1:1"
						}
						require.Equal(t, want, output["aspect_ratio"])
						require.Equal(t, "1K", output["resolution"])
					}
				}
			})
		}
		for _, extra := range []map[string]any{
			{"imageSize": "1K"}, {"imageSize": "2K"}, {"resolution": "1K"}, {"metadata": map[string]any{"resolution": "1K"}},
			{"quality": "low"}, {"quality": "medium"}, {"quality": "high"}, {"metadata": map[string]any{"quality": "high"}},
			{"aspectRatio": "1:3"}, {"aspectRatio": "3:1"}, {"aspectRatio": "1024x1024"}, {"size": "1024x1024"},
			{"n": 2}, {"replyType": "json"}, {"images": []string{"data:image/png;base64,AA=="}},
		} {
			extra["model"], extra["prompt"] = model, "cup"
			body, _ := json.Marshal(extra)
			_, fields := compiled.Validate(body)
			require.NotEmpty(t, fields, string(body))
		}
	}
	require.Equal(t, 2, found)
}
