//go:build unit

package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
	"github.com/stretchr/testify/require"
)

func fixed1KPublicSchemas(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "legacy_fixed_1k_schemas.json"))
	require.NoError(t, err)
	var catalog struct {
		Models []struct {
			Model  string          `json:"model"`
			Schema json.RawMessage `json:"schema"`
		} `json:"models"`
	}
	require.NoError(t, json.Unmarshal(raw, &catalog))
	result := map[string]json.RawMessage{}
	for _, model := range catalog.Models {
		if model.Model == "gpt-image-2.5-1k" || model.Model == "gpt-image-2.6-1k" {
			result[model.Model] = model.Schema
		}
		require.NotContains(t, []string{"gpt-image-2.5", "gpt-image-2.6"}, model.Model)
	}
	require.Len(t, result, 2)
	return result
}

func TestFixed1KPublicRatiosReachBothProvidersAndFreezeCorrectPayload(t *testing.T) {
	ratios := []string{"auto", "1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3", "5:4", "4:5", "21:9", "9:21", "1:2", "2:1"}
	for model, schema := range fixed1KPublicSchemas(t) {
		compiled, err := mediacapability.Compile("image", schema)
		require.NoError(t, err)
		for _, ratio := range ratios {
			for _, platform := range []string{PlatformGrsai, PlatformKuaipao} {
				t.Run(model+"/"+platform+"/"+ratio, func(t *testing.T) {
					raw, _ := json.Marshal(map[string]any{"model": model, "prompt": "blue cup", "aspectRatio": ratio, "replyType": "async"})
					normalized, fields := compiled.Validate(raw)
					require.Empty(t, fields)
					before, _ := json.Marshal(normalized)
					upstream := model
					if platform == PlatformGrsai {
						upstream = strings.TrimSuffix(model, "-1k")
					}
					plan, err := planUnifiedMediaProviderRequest(platform, "image", upstream, normalized)
					require.NoError(t, err)
					request, err := unifiedMediaCandidateRequest(normalized, UnifiedMediaCandidateSnapshot{ProviderManaged: true, Platform: platform, Model: upstream, Capability: MediaCapabilitySnapshot{MediaType: "image", Schema: plan.Schema}, AdapterProfile: plan.Profile})
					require.NoError(t, err)
					body, _, err := BuildUnifiedMediaPayload(platform, "image", upstream, request, 1)
					require.NoError(t, err)
					var payload map[string]any
					require.NoError(t, json.Unmarshal(body, &payload))
					require.Equal(t, upstream, payload["model"])
					require.Equal(t, "auto", payload["quality"])
					require.NotContains(t, payload, "imageSize")
					if platform == PlatformGrsai {
						require.Equal(t, ratio, payload["aspectRatio"])
						require.Equal(t, "async", payload["replyType"])
						require.NotContains(t, payload, "resolution")
					} else if ratio == "auto" {
						require.Equal(t, "auto", payload["size"])
						require.NotContains(t, payload, "aspect_ratio")
						require.NotContains(t, payload, "resolution")
						require.Nil(t, request.ImageSpec)
					} else {
						require.Equal(t, ratio, payload["aspect_ratio"])
						require.Equal(t, "1K", payload["resolution"])
						require.NotContains(t, payload, "size")
					}
					after, _ := json.Marshal(normalized)
					require.JSONEq(t, string(before), string(after))
				})
			}
		}
	}
}

func TestFixed1KPublicContractRejectsRemovedAndUnsupportedParameters(t *testing.T) {
	for model, schema := range fixed1KPublicSchemas(t) {
		compiled, err := mediacapability.Compile("image", schema)
		require.NoError(t, err)
		for _, extra := range []map[string]any{
			{"imageSize": "1K"}, {"resolution": "1K"}, {"metadata": map[string]any{"resolution": "1K"}},
			{"quality": "high"}, {"aspectRatio": "3:1"}, {"aspectRatio": "1536x1024"}, {"replyType": "json"},
		} {
			extra["model"], extra["prompt"] = model, "cup"
			raw, _ := json.Marshal(extra)
			_, fields := compiled.Validate(raw)
			require.NotEmpty(t, fields, string(raw))
		}
	}
}

func TestFixed1KAutoCannotDiscardAnExplicitConflictingTier(t *testing.T) {
	request := &mediacapability.NormalizedRequest{Values: map[string]any{"prompt": "cup", "size": "auto", "metadata": map[string]any{"resolution": "2K"}}}
	_, _, err := BuildUnifiedMediaPayload(PlatformKuaipao, "image", "gpt-image-2.6-1k", request, 1)
	require.Error(t, err)
	_, _, err = BuildUnifiedMediaPayload(PlatformKuaipao, "image", "gpt-image-2.6", request, 1)
	require.Error(t, err, "an unfixed model cannot preserve resolution when size:auto forbids resolution")
}
