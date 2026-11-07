//go:build unit

package service

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
	"github.com/stretchr/testify/require"
)

func TestMediaCatalogGPTImage26Generation(t *testing.T) {
	for _, model := range []string{"gpt-image-2.6-flare", "gpt-image-2.6-sunburst"} {
		t.Run(model, func(t *testing.T) {
			var entry MediaCapabilityCatalogEntry
			for _, candidate := range MediaCapabilityCatalog() {
				if candidate.Platform == PlatformToAPIs && candidate.UpstreamModel == model && candidate.MediaType == "image" {
					entry = candidate
					break
				}
			}
			require.NotEmpty(t, entry.Schema)
			compiled, err := mediacapability.Compile("image", entry.Schema)
			require.NoError(t, err)
			for _, quality := range []string{"low", "medium", "high"} {
				request := `{"model":"public","prompt":"A lake","n":2,"size":"16:9","image_urls":["https://example.com/reference.png"],"metadata":{"resolution":"4K","quality":"` + quality + `","background":"transparent","output_format":"webp"}}`
				normalized, fields := compiled.Validate([]byte(request))
				require.Empty(t, fields)
				body, endpoint, err := BuildUnifiedMediaPayload(PlatformToAPIs, "image", model, normalized, 1)
				require.NoError(t, err)
				require.Equal(t, "/v1/images/generations", endpoint)
				var payload map[string]any
				require.NoError(t, json.Unmarshal(body, &payload))
				require.Equal(t, model, payload["model"])
				require.Equal(t, float64(1), payload["n"])
				require.Equal(t, "4K", payload["resolution"])
				require.Equal(t, quality, payload["quality"])
				require.Equal(t, "transparent", payload["background"])
				require.Equal(t, "webp", payload["output_format"])
				require.Equal(t, []any{"https://example.com/reference.png"}, payload["reference_images"])
				require.NotContains(t, payload, "metadata")
			}
			normalized, fields := compiled.Validate([]byte(`{"model":"public","prompt":"A lake"}`))
			require.Empty(t, fields)
			require.Equal(t, "1K", normalized.Values["metadata"].(map[string]any)["resolution"])
			images := make([]string, 11)
			for i := range images {
				images[i] = "https://example.com/reference.png"
			}
			request, err := json.Marshal(map[string]any{"model": "public", "prompt": "A lake", "image_urls": images})
			require.NoError(t, err)
			_, fields = compiled.Validate(request)
			require.NotEmpty(t, fields, "the provider accepts at most ten reference images")
		})
	}
}
