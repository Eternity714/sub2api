package mediacapability

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

const grsPublicParametersSchema = `{"version":1,"fields":{"prompt":{"type":"string","required":true},"images":{"type":"array","items":{"type":"string","format":"http_url"},"min_items":1,"max_items":16},"aspectRatio":{"type":"string","default":"1:1"},"imageSize":{"type":"string","enum":["1K","2K"],"default":"1K"},"quality":{"type":"string","enum":["auto","low","medium","high"],"default":"auto"},"replyType":{"type":"string","enum":["async"],"default":"async"},"n":{"type":"integer","minimum":1,"maximum":8,"default":1}},"batch":{"native_max_results":1,"allow_split":true}}`

func TestGRSParametersPublicSchemaNormalizesBothRequestSpellings(t *testing.T) {
	compiled, err := Compile("image", []byte(grsPublicParametersSchema))
	require.NoError(t, err)
	for _, body := range []string{
		`{"model":"gpt-image-2.6","prompt":"cup","images":["https://example.com/ref.png"],"aspectRatio":"3:2","imageSize":"1K","quality":"high","replyType":"async","n":2}`,
		`{"model":"gpt-image-2.6","prompt":"cup","image_urls":["https://example.com/ref.png"],"size":"3:2","metadata":{"resolution":"1K","quality":"high"},"n":2}`,
	} {
		normalized, fields := compiled.Validate([]byte(body))
		require.Empty(t, fields)
		values, err := json.Marshal(normalized.Values)
		require.NoError(t, err)
		require.JSONEq(t, `{"model":"gpt-image-2.6","prompt":"cup","image_urls":["https://example.com/ref.png"],"size":"3:2","metadata":{"resolution":"1K","quality":"high"},"n":2}`, string(values))
		require.Equal(t, 2, normalized.RequestedCount)
		snapshot, err := json.Marshal(normalized)
		require.NoError(t, err)
		require.Contains(t, string(snapshot), `"reply_type":"async"`)
	}
}

func TestGRSParametersPixelAspectRatioPreservesRequestedGeometry(t *testing.T) {
	compiled, err := Compile("image", []byte(grsPublicParametersSchema))
	require.NoError(t, err)
	normalized, fields := compiled.Validate([]byte(`{"model":"gpt-image-2.6","prompt":"cup","aspectRatio":"1536x1024"}`))
	require.Empty(t, fields)
	require.Equal(t, "3:2", normalized.Values["size"])
	require.Equal(t, "1K", normalized.Values["metadata"].(map[string]any)["resolution"])
	snapshot, err := json.Marshal(normalized)
	require.NoError(t, err)
	require.Contains(t, string(snapshot), `"requested_image_spec"`)
	require.Contains(t, string(snapshot), `"width":1536`)
	require.Contains(t, string(snapshot), `"height":1024`)
}

func TestGRSParametersRejectConflictingAliasesAndInvalidReplyType(t *testing.T) {
	compiled, err := Compile("image", []byte(grsPublicParametersSchema))
	require.NoError(t, err)
	for _, body := range []string{
		`{"model":"x","prompt":"p","aspectRatio":"1:1","size":"3:2"}`,
		`{"model":"x","prompt":"p","quality":"high","metadata":{"quality":"low"}}`,
		`{"model":"x","prompt":"p","images":["https://example.com/a"],"image_urls":["https://example.com/b"]}`,
		`{"model":"x","prompt":"p","aspectRatio":"0x1024"}`,
		`{"model":"x","prompt":"p","replyType":"stream"}`,
	} {
		_, fields := compiled.Validate([]byte(body))
		require.NotEmpty(t, fields, body)
	}
}

func TestGRSParametersAlsoWorkWithLegacyCanonicalSchema(t *testing.T) {
	compiled, err := Compile("image", []byte(`{"version":1,"fields":{"prompt":{"type":"string","required":true},"size":{"type":"string"},"image_urls":{"type":"array","items":{"type":"string","format":"http_url"}},"metadata":{"type":"object","properties":{"quality":{"type":"string"},"resolution":{"type":"string"}}}}}`))
	require.NoError(t, err)
	normalized, fields := compiled.Validate([]byte(`{"model":"x","prompt":"p","aspectRatio":"3:2","quality":"auto","imageSize":"1K","images":["https://example.com/a"]}`))
	require.Empty(t, fields)
	require.Equal(t, "3:2", normalized.Values["size"])
}

func TestGRSParametersRoundedDocumentedPixelsNeedNoPublicGrid(t *testing.T) {
	compiled, err := Compile("image", []byte(`{"version":1,"fields":{"prompt":{"type":"string"},"aspectRatio":{"type":"string","enum":["16:9","5:4","21:9"]},"imageSize":{"type":"string","enum":["1K"]}}}`))
	require.NoError(t, err)
	for pixels, ratio := range map[string]string{"1672x941": "16:9", "1408x1120": "5:4", "1920x832": "21:9"} {
		normalized, fields := compiled.Validate([]byte(`{"model":"x","aspectRatio":"` + pixels + `"}`))
		require.Empty(t, fields, pixels)
		require.Equal(t, ratio, normalized.Values["size"])
	}
}
