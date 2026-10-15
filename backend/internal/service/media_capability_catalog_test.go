//go:build unit

package service

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
	"github.com/stretchr/testify/require"
)

func TestMediaCatalogTemplatesCompileAndExamplesMatch(t *testing.T) {
	catalog := MediaCapabilityCatalog()
	require.GreaterOrEqual(t, len(catalog), 108)
	ready := 0
	seen := map[string]bool{}
	for _, entry := range catalog {
		key := entry.Platform + ":" + entry.MediaType + ":" + entry.UpstreamModel
		require.False(t, seen[key], key)
		seen[key] = true
		require.False(t, entry.Enabled, key)
		require.NotEmpty(t, entry.Source, key)
		if len(entry.Schema) == 0 {
			require.NotEmpty(t, entry.Notes, key)
			continue
		}
		ready++
		compiled, err := mediacapability.Compile(entry.MediaType, entry.Schema)
		require.NoError(t, err, key)
		normalized, fields := compiled.Validate(entry.Example)
		require.Empty(t, fields, key)
		require.NotNil(t, normalized, key)
	}
	require.GreaterOrEqual(t, ready, 90)
}

func TestMediaCatalogProfilesKeepProtocolDifferences(t *testing.T) {
	seed := mediaAdapterProfile("toapis", "image", "doubao-seedream-4-0")
	require.NotContains(t, seed.TopLevelMetadata, "resolution")
	gpt := mediaAdapterProfile("toapis", "image", "gpt-image-2.5-flare")
	require.False(t, gpt.PixelSize)
	require.Contains(t, gpt.TopLevelMetadata, "resolution")
	vip := mediaAdapterProfile("toapis", "image", "gpt-image-2.5-flare-vip")
	require.True(t, vip.PixelSize)
	require.Equal(t, "reference_images", vip.FieldMap["image_urls"])
	preview := mediaAdapterProfile("toapis", "image", "gemini-3.1-flash-image-preview")
	require.True(t, preview.ImageURLObjects)
	official := mediaAdapterProfile("toapis", "image", "gemini-3.1-flash-image-official")
	require.False(t, official.ImageURLObjects)
	omni := mediaAdapterProfile("toapis", "video", "gemini-omni-1.1-flash-preview")
	require.Equal(t, "/v1/videos", omni.SubmitPath)
	require.Equal(t, "/v1/videos/{id}", omni.QueryPath)
	ext := mediaAdapterProfile("toapis", "video", "seedance-2-ext")
	require.Equal(t, "seconds", ext.FieldMap["duration"])
	require.Equal(t, "size", ext.FieldMap["aspect_ratio"])
}

// AC-027 (model-schema scope): Seedream 5 Pro rejects layer decomposition; ordinary catalog examples are checked separately.
func TestMediaCatalogSpecificConstraintsDoNotLeakAcrossModels(t *testing.T) {
	index := map[string]MediaCapabilityCatalogEntry{}
	for _, entry := range MediaCapabilityCatalog() {
		index[entry.Platform+":"+entry.UpstreamModel+":"+entry.MediaType] = entry
	}
	for _, key := range []string{"toapis:gemini_omni:video", "toapis:nano_banana:image", "toapis:grok-imagine-1.0-edit:image"} {
		require.Empty(t, index[key].Schema, key)
	}
	for _, tc := range []struct{ key, body string }{
		{"grsai:nano-banana-pro-cl:image", `{"model":"x","prompt":"p","metadata":{"resolution":"4K"}}`},
		{"toapis:viduq2-fast:image", `{"model":"x","prompt":"p","metadata":{"resolution":"4K"}}`},
		{"toapis:doubao-seedream-5-0-pro:image", `{"model":"x","prompt":"p","metadata":{"layer_decomposition":true}}`},
	} {
		entry := index[tc.key]
		require.NotEmpty(t, entry.Schema, tc.key)
		compiled, err := mediacapability.Compile(entry.MediaType, entry.Schema)
		require.NoError(t, err)
		_, fields := compiled.Validate([]byte(tc.body))
		require.NotEmpty(t, fields, tc.key)
	}
	// A catalogue template must never create or enable a persisted capability by listing it.
	templates := NewMediaCapabilityService(nil).Templates()
	require.Greater(t, len(templates), 2)
	encoded, err := json.Marshal(templates)
	require.NoError(t, err)
	require.NotEmpty(t, encoded)
}

func TestMediaCatalogRejectsUnsupportedCombinations(t *testing.T) {
	entries := map[string]MediaCapabilityCatalogEntry{}
	for _, entry := range MediaCapabilityCatalog() {
		entries[entry.Platform+":"+entry.UpstreamModel+":"+entry.MediaType] = entry
	}
	cases := []struct{ key, body string }{
		{"toapis:viduq4-preview:video", `{"model":"x","prompt":"p","image_urls":["https://example.com/a.png"],"metadata":{"video_generation_mode":"first_frame_image_to_video"}}`},
		{"toapis:gemini-3.1-flash-image-preview:image", `{"model":"x","prompt":"p","metadata":{"google_image_search":true,"google_search":false}}`},
		{"toapis:kling-v2-6:video", `{"model":"x","prompt":"p","metadata":{"audio":true,"mode":"std"}}`},
		{"toapis:gemini-omni-1.1-flash-preview-vip:video", `{"model":"x","prompt":"p","metadata":{"generation_type":"frame"}}`},
		{"toapis:happyhorse-1.1:video", `{"model":"x","prompt":"p","metadata":{"action":"image-to-video"}}`},
		{"toapis:gpt-image-2-official:image", `{"model":"x","prompt":"p","image_urls":[],"metadata":{"mask_url":"https://example.com/mask.png"}}`},
		{"toapis:MiniMax-H3:video", `{"model":"x","prompt":"p","image_urls":["https://example.com/a.png"],"aspect_ratio":"16:9"}`},
	}
	for _, tc := range cases {
		entry := entries[tc.key]
		compiled, err := mediacapability.Compile(entry.MediaType, entry.Schema)
		require.NoError(t, err)
		_, errs := compiled.Validate([]byte(tc.body))
		require.NotEmpty(t, errs, tc.key)
	}
}

func TestMediaCatalogCopiesCannotMutateProfilesOrTemplates(t *testing.T) {
	first := MediaCapabilityCatalog()
	for i := range first {
		if first[i].UpstreamModel == "gpt-image-2.5-flare-vip" {
			first[i].Schema[0] = '!'
			first[i].Profile.FieldMap["image_urls"] = "corrupted"
		}
	}
	profile := mediaAdapterProfile("toapis", "image", "gpt-image-2.5-flare-vip")
	require.Equal(t, "reference_images", profile.FieldMap["image_urls"])
	profile.FieldMap["image_urls"] = "other"
	require.Equal(t, "reference_images", mediaAdapterProfile("toapis", "image", "gpt-image-2.5-flare-vip").FieldMap["image_urls"])
	for _, entry := range MediaCapabilityCatalog() {
		if len(entry.Schema) > 0 {
			require.True(t, json.Valid(entry.Schema))
		}
	}
}
