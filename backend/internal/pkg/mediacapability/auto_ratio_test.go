package mediacapability

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAutoImageRatioRequiresExplicitSchemaSupport(t *testing.T) {
	for _, tc := range []struct {
		name, media, schema string
		valid               bool
	}{
		{"image opt in", "image", `{"version":1,"fields":{"aspectRatio":{"type":"string","enum":["auto","1:1"]}}}`, true},
		{"image unconstrained", "image", `{"version":1,"fields":{"aspectRatio":{"type":"string"}}}`, false},
		{"image omitted", "image", `{"version":1,"fields":{"aspectRatio":{"type":"string","enum":["1:1"]}}}`, false},
		{"video", "video", `{"version":1,"fields":{"aspectRatio":{"type":"string","enum":["auto","1:1"]}}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			compiled, err := Compile(tc.media, []byte(tc.schema))
			require.NoError(t, err)
			_, fields := compiled.Validate([]byte(`{"model":"test","aspectRatio":"auto"}`))
			require.Equal(t, tc.valid, len(fields) == 0, fields)
		})
	}
}

func TestAutoImageRatioDoesNotInventPixelsOrBypassResolution(t *testing.T) {
	compiled, err := Compile("image", []byte(`{"version":1,"fields":{"aspectRatio":{"type":"string","enum":["auto","1:1"]},"imageSize":{"type":"string","enum":["1K"],"default":"1K"}},"image_specs":[{"aspect_ratio":"1:1","resolution":"1K","width":1024,"height":1024}]}`))
	require.NoError(t, err)
	normalized, fields := compiled.Validate([]byte(`{"model":"test","aspectRatio":"auto"}`))
	require.Empty(t, fields)
	require.Equal(t, "auto", normalized.Values["size"])
	require.Nil(t, normalized.ImageSpec)
	require.Nil(t, normalized.RequestedImageSpec)
	_, fields = compiled.Validate([]byte(`{"model":"test","aspectRatio":"auto","imageSize":"4K"}`))
	require.NotEmpty(t, fields)
}
