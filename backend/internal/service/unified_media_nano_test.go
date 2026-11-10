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

func nanoPublicSchemas(t *testing.T) map[string]json.RawMessage {
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
	for _, entry := range catalog.Models {
		if entry.Model == "nano-banana-pro" || entry.Model == "nano-banana-2" {
			out[entry.Model] = entry.Schema
		}
	}
	require.Len(t, out, 2)
	return out
}

func TestNanoPublicContractFreezesGRSPayloadAndFlatPrice(t *testing.T) {
	ratios := []string{"auto", "1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3", "5:4", "4:5", "21:9", "9:21"}
	for model, schema := range nanoPublicSchemas(t) {
		price := map[string]float64{"nano-banana-pro": .2, "nano-banana-2": .15}[model]
		caps := &mediaCapabilityMemoryRepository{rows: map[int64]*MediaCapability{1: {ID: 1, PublicModel: model, MediaType: "image", Enabled: true, Schema: schema}}}
		routes := NewCompositeMediaRouteService(NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{{ID: 1, GroupID: 22, PublicModel: model, MatchType: "exact", Endpoint: "images", TargetPlatform: PlatformGrsai, UpstreamModel: model, Priority: 10, Enabled: true}}}), caps)
		pricing := unifiedMediaChannelPricingResolver(22, StatusActive, []ChannelModelPricing{{Platform: PlatformGrsai, Models: []string{model}, BillingMode: BillingModeImage, PerRequestPrice: float64Ptr(price)}})
		svc := NewUnifiedMediaTaskService(&unifiedMediaMemoryRepo{}, NewMediaCapabilityService(caps), routes, nil, pricing, nil, &unifiedMediaPersister{})
		id := int64(22)
		for _, tier := range []string{"1k", "2k", "4k", "1K", "2K", "4K"} {
			for _, ratio := range ratios {
				for _, reference := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%s/%s/reference=%t", model, tier, ratio, reference), func(t *testing.T) {
						body := map[string]any{"model": model, "prompt": "cup", "aspectRatio": ratio, "imageSize": tier, "n": 1, "replyType": "async"}
						if reference {
							body["images"] = []string{"https://example.com/reference.png"}
						}
						raw, _ := json.Marshal(body)
						task, err := svc.Create(context.Background(), UnifiedMediaTaskCreateInput{APIKey: &APIKey{ID: 33, UserID: 1, GroupID: &id, Group: &Group{ID: id, Platform: PlatformComposite, RateMultiplier: 1}}, MediaType: "image", Body: raw})
						require.NoError(t, err)
						require.Equal(t, price, task.UnitPrice)
						require.Equal(t, price, task.HeldAmount)
						require.Len(t, task.Payload.Candidates, 1)
						require.Equal(t, tier, task.Payload.Normalized.Values["metadata"].(map[string]any)["resolution"])
						require.Nil(t, task.Payload.Normalized.ImageSpec)
						require.Nil(t, task.Payload.Normalized.RequestedImageSpec)
						frozen, _ := json.Marshal(task.Payload)
						var restored UnifiedMediaTaskPayload
						require.NoError(t, json.Unmarshal(frozen, &restored))
						candidate := restored.Candidates[0]
						require.Equal(t, PlatformGrsai, candidate.Platform)
						require.Equal(t, model, candidate.Model)
						require.True(t, candidate.AdapterProfile.ResolutionUppercase)
						require.Equal(t, "imageSize", candidate.AdapterProfile.FieldMap["metadata.resolution"])
						n, err := unifiedMediaCandidateRequest(restored.Normalized, candidate)
						require.NoError(t, err)
						encoded, path, err := BuildUnifiedMediaPayload(candidate.Platform, "image", model, n, 1)
						require.NoError(t, err)
						require.Equal(t, "/v1/api/generate", path)
						want := map[string]any{"model": model, "prompt": "cup", "aspectRatio": ratio, "imageSize": strings.ToUpper(tier), "replyType": "async"}
						if reference {
							want["images"] = []string{"https://example.com/reference.png"}
						}
						expected, _ := json.Marshal(want)
						require.JSONEq(t, string(expected), string(encoded))
						after, _ := json.Marshal(task.Payload)
						require.JSONEq(t, string(frozen), string(after), "provider adaptation must not mutate captured public values")
					})
				}
			}
		}
	}
}

func TestNanoDefaultsAndInvalidInputsRemainBounded(t *testing.T) {
	for model, schema := range nanoPublicSchemas(t) {
		compiled, err := mediacapability.Compile("image", schema)
		require.NoError(t, err)
		n, fields := compiled.Validate([]byte(fmt.Sprintf(`{"model":%q,"prompt":"cup"}`, model)))
		require.Empty(t, fields)
		require.Equal(t, "auto", n.Values["size"])
		require.Equal(t, "1k", n.Values["metadata"].(map[string]any)["resolution"])
		require.Equal(t, 1, n.RequestedCount)
		require.Equal(t, "async", n.ReplyType)
		plan, err := planUnifiedMediaProviderRequest(PlatformGrsai, "image", model, n)
		require.NoError(t, err)
		raw, _, err := BuildUnifiedMediaPayload(PlatformGrsai, "image", model, plan.Request, 1)
		require.NoError(t, err)
		require.JSONEq(t, fmt.Sprintf(`{"model":%q,"prompt":"cup","aspectRatio":"auto","imageSize":"1K","replyType":"async"}`, model), string(raw))
		for _, extra := range []map[string]any{
			{"quality": "auto"}, {"quality": "high"}, {"metadata": map[string]any{"quality": "auto"}},
			{"imageSize": "8k"}, {"imageSize": "1"}, {"imageSize": 1}, {"aspectRatio": "1024x1024"}, {"size": "1024x1024"},
			{"aspectRatio": "1:4"}, {"aspectRatio": "4:1"}, {"aspectRatio": "1:8"}, {"aspectRatio": "8:1"},
			{"n": 2}, {"replyType": "json"}, {"replyType": "stream"},
			{"images": []string{"data:image/png;base64,AA=="}}, {"images": []string{"file:///tmp/a.png"}}, {"images": []string{"https://user:pass@example.com/a.png"}},
			{"images": []string{}}, {"imageSize": "1k", "resolution": "2K"}, {"resolution_uppercase": false},
		} {
			extra["model"], extra["prompt"] = model, "cup"
			raw, _ := json.Marshal(extra)
			_, fields := compiled.Validate(raw)
			require.NotEmpty(t, fields, string(raw))
		}
		for _, platform := range []string{PlatformKuaipao, PlatformToAPIs} {
			_, err := planUnifiedMediaProviderRequest(platform, "image", model, n)
			require.Error(t, err, "only the GRS provider is registered for these models")
		}
	}
}
