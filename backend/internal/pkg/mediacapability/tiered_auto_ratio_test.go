package mediacapability

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

const tieredAutoSchema = `{"version":1,"allow_pixel_size":false,"auto_image_aspect_ratio":"1:1","fields":{"prompt":{"type":"string","required":true},"aspectRatio":{"type":"string","enum":["auto","1:1"],"default":"1:1"},"imageSize":{"type":"string","enum":["1K","2K"],"default":"1K"}},"image_specs":[{"aspect_ratio":"1:1","resolution":"1K","width":1024,"height":1024},{"aspect_ratio":"1:1","resolution":"2K","width":2048,"height":2048}]}`

func TestTieredAutoRatioCapturesSelectedGeometryAndRejectsPixelInput(t *testing.T) {
	compiled, err := Compile("image", []byte(tieredAutoSchema))
	require.NoError(t, err)
	for _, tier := range []string{"1K", "2K"} {
		raw, _ := json.Marshal(map[string]any{"model": "image", "prompt": "cup", "aspectRatio": "auto", "imageSize": tier})
		n, fields := compiled.Validate(raw)
		require.Empty(t, fields)
		require.Equal(t, tier, n.ImageSpec.Resolution)
		require.Equal(t, "1:1", n.ImageSpec.AspectRatio)
		require.Equal(t, n.ImageSpec, n.RequestedImageSpec)
		require.Equal(t, "auto", n.Values["size"])
		encoded, _ := json.Marshal(n)
		var restored NormalizedRequest
		require.NoError(t, json.Unmarshal(encoded, &restored))
		require.Equal(t, n.RequestedImageSpec, restored.RequestedImageSpec)
	}
	_, fields := compiled.Validate([]byte(`{"model":"image","prompt":"cup","aspectRatio":"1024x1024"}`))
	require.Contains(t, fields, FieldError{Path: "aspectRatio", Code: "unsupported_pixel_size", Message: "this model accepts aspect ratios, not pixel dimensions"})
}

func TestTieredAutoRatioRequiresMappingForEveryAllowedTier(t *testing.T) {
	var schema map[string]any
	require.NoError(t, json.Unmarshal([]byte(tieredAutoSchema), &schema))
	schema["image_specs"] = schema["image_specs"].([]any)[:1]
	raw, _ := json.Marshal(schema)
	_, err := Compile("image", raw)
	require.Error(t, err)
	for _, value := range []any{nil, "false", 1} {
		require.NoError(t, json.Unmarshal([]byte(tieredAutoSchema), &schema))
		schema["allow_pixel_size"] = value
		raw, _ = json.Marshal(schema)
		_, err = Compile("image", raw)
		require.Error(t, err)
	}
}
