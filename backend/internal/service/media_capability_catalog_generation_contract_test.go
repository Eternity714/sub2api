//go:build unit

package service

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
	"github.com/stretchr/testify/require"
)

func TestMediaCatalogHailuoFastRequiresFirstFrame(t *testing.T) {
	for _, model := range []string{"MiniMax-Hailuo-2.3", "MiniMax-Hailuo-2.3-Fast"} {
		var entry MediaCapabilityCatalogEntry
		for _, candidate := range MediaCapabilityCatalog() {
			if candidate.Platform == "toapis" && candidate.MediaType == "video" && candidate.UpstreamModel == model {
				entry = candidate
				break
			}
		}
		require.NotEmpty(t, entry.Schema, model)
		compiled, err := mediacapability.Compile("video", entry.Schema)
		require.NoError(t, err)
		_, errors := compiled.Validate([]byte(`{"model":"test","prompt":"A quiet mountain lake at sunrise","duration":6,"metadata":{"resolution":"768P"}}`))
		if model == "MiniMax-Hailuo-2.3-Fast" {
			require.Contains(t, errors, mediacapability.FieldError{Path: "image_urls", Code: "required", Message: "field is required"}, "official Fast model requires exactly one first frame")
		} else {
			require.Empty(t, errors, "ordinary 2.3 still supports text-to-video")
		}
		_, errors = compiled.Validate([]byte(`{"model":"test","prompt":"Animate this landscape","duration":6,"image_urls":["https://example.com/first-frame.png"],"metadata":{"resolution":"768P"}}`))
		require.Empty(t, errors, model)
	}
}

func TestMediaCatalogVideoResolutionPlacementMatchesOfficialExamples(t *testing.T) {
	for _, tc := range []struct {
		model      string
		resolution string
		topLevel   bool
	}{
		{"gemini-omni-1.1-flash-preview-official", "360p", true},
		{"gemini-omni-flash-preview-official", "720p", true},
		{"doubao-seedance-1-5-pro", "480p", false},
		{"veo3.1-fast", "720p", false},
		{"veo3.1-lite", "720p", false},
		{"veo3.1-quality", "720p", false},
	} {
		t.Run(tc.model, func(t *testing.T) {
			request := &mediacapability.NormalizedRequest{Values: map[string]any{
				"model": "public-model", "prompt": "A quiet mountain lake at sunrise",
				"duration": 6, "metadata": map[string]any{"resolution": tc.resolution},
			}}
			payload, endpoint, err := BuildUnifiedMediaPayload("toapis", "video", tc.model, request, 1)
			require.NoError(t, err)
			require.Equal(t, "/v1/videos/generations", endpoint)
			var body map[string]any
			require.NoError(t, json.Unmarshal(payload, &body))
			if tc.topLevel {
				require.Equal(t, tc.resolution, body["resolution"], "official request example declares resolution at top level")
				require.NotContains(t, body, "metadata", "empty metadata is removed after moving resolution")
			} else {
				require.NotContains(t, body, "resolution", "this model documents resolution inside metadata")
				require.Equal(t, map[string]any{"resolution": tc.resolution}, body["metadata"])
			}
		})
	}
}
