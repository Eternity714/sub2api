package mediacapability

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

const fixed2KSchema = `{"version":1,"fixed_image_resolution":"2K","auto_image_aspect_ratio":"1:1","fields":{"prompt":{"type":"string","required":true},"aspectRatio":{"type":"string","enum":["auto","1:1","16:9"],"default":"1:1"},"quality":{"type":"string","enum":["auto","low","medium","high"],"default":"auto"}},"image_specs":[{"aspect_ratio":"1:1","resolution":"2K","width":2048,"height":2048},{"aspect_ratio":"16:9","resolution":"2K","width":2560,"height":1440}]}`

func TestFixedImageResolutionBindsGeometryWithoutAcceptingTierInput(t *testing.T) {
	c, err := Compile("image", []byte(fixed2KSchema))
	require.NoError(t, err)
	for _, ratio := range []string{"auto", "1:1", "16:9"} {
		raw, _ := json.Marshal(map[string]any{"model": "image-2k", "prompt": "cup", "aspectRatio": ratio})
		n, fields := c.Validate(raw)
		require.Empty(t, fields)
		require.Equal(t, "2K", n.FixedImageResolution)
		require.Equal(t, ratio, n.Values["size"])
		require.NotContains(t, n.Values["metadata"], "resolution")
		require.NotNil(t, n.ImageSpec)
		require.Equal(t, "2K", n.ImageSpec.Resolution)
		if ratio == "auto" {
			require.Equal(t, ImageSpec{AspectRatio: "1:1", Resolution: "2K", Width: 2048, Height: 2048}, *n.ImageSpec)
		}
		snapshot, err := json.Marshal(n)
		require.NoError(t, err)
		var restored NormalizedRequest
		require.NoError(t, json.Unmarshal(snapshot, &restored))
		require.Equal(t, n.FixedImageResolution, restored.FixedImageResolution)
		require.Equal(t, n.ImageSpec, restored.ImageSpec)
	}
	for _, extra := range []map[string]any{{"imageSize": "2K"}, {"resolution": "2K"}, {"metadata": map[string]any{"resolution": "2K"}}, {"fixed_image_resolution": "4K"}, {"auto_image_aspect_ratio": "16:9"}, {"aspectRatio": "2048x2048"}} {
		extra["model"], extra["prompt"] = "image-2k", "cup"
		raw, _ := json.Marshal(extra)
		_, fields := c.Validate(raw)
		require.NotEmpty(t, fields, string(raw))
	}
}

func TestFixedImageResolutionRejectsIncompleteOrConflictingConfiguration(t *testing.T) {
	for _, mutate := range []func(map[string]any){
		func(s map[string]any) { s["fixed_image_resolution"] = "8K" },
		func(s map[string]any) { delete(s, "image_specs") },
		func(s map[string]any) { delete(s, "auto_image_aspect_ratio") },
		func(s map[string]any) { s["auto_image_aspect_ratio"] = "3:2" },
		func(s map[string]any) { s["fields"].(map[string]any)["imageSize"] = map[string]any{"type": "string"} },
		func(s map[string]any) { s["image_specs"].([]any)[0].(map[string]any)["resolution"] = "1K" },
	} {
		var schema map[string]any
		require.NoError(t, json.Unmarshal([]byte(fixed2KSchema), &schema))
		mutate(schema)
		raw, _ := json.Marshal(schema)
		_, err := Compile("image", raw)
		require.Error(t, err, string(raw))
	}
}
