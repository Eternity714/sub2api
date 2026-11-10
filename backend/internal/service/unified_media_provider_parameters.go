package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
)

type unifiedMediaProviderPlan struct {
	Schema  json.RawMessage
	Request *mediacapability.NormalizedRequest
	Match   mediacapability.CandidateMatch
	Profile *UnifiedMediaAdapterProfile
}

// Only explicit native routes use these reviewed provider rules. Catalog entries
// do not enable accounts, routes, public capabilities, or pricing.
func planUnifiedMediaProviderRequest(platform, mediaType, model string, original *mediacapability.NormalizedRequest) (*unifiedMediaProviderPlan, error) {
	if platform != PlatformGrsai && platform != PlatformToAPIs && platform != PlatformKuaipao {
		return nil, nil
	}
	loadMediaCapabilityCatalog()
	for _, entry := range mediaCatalogEntries {
		if entry.Platform != platform || entry.MediaType != mediaType || entry.UpstreamModel != model {
			continue
		}
		if len(entry.Schema) == 0 {
			return nil, mediaCapabilityFieldError("model", "unsupported_parameters", "provider model has no verified parameter rules")
		}
		profile := mediaAdapterProfile(platform, mediaType, model)
		providerSchema, err := fixedImageProviderSchema(platform, mediaType, model, entry.Schema, profile, original)
		if err != nil {
			return nil, err
		}
		request, err := adaptUnifiedMediaProviderRequest(mediaType, providerSchema, original, &profile)
		if err != nil {
			return nil, err
		}
		request.AdapterProfile, _ = json.Marshal(profile)
		wanted := original.RequestedImageSpec
		if original.FixedImageResolution != "" {
			wanted = original.ImageSpec
		}
		if wanted != nil {
			// Exact pixels are a stronger promise than a ratio and resolution tier.
			actual := request.ImageSpec
			if actual == nil || wanted.Width != actual.Width || wanted.Height != actual.Height {
				return nil, mediaCapabilityFieldError("aspectRatio", "unsupported_image_spec", "provider cannot preserve the requested pixel dimensions")
			}
			if platform == PlatformToAPIs && !profile.PixelSize {
				return nil, mediaCapabilityFieldError("aspectRatio", "unsupported_image_spec", "provider accepts ratio and tier, not exact pixel dimensions")
			}
		}
		var schema mediacapability.Schema
		if err := json.Unmarshal(providerSchema, &schema); err != nil {
			return nil, err
		}
		native := schema.Batch.NativeMaxResults
		if native < 1 {
			native = 1
		}
		match := mediacapability.CandidateMatch{Supported: true, NativeBatchSize: native}
		if original.RequestedCount > native {
			if original.AtomicBatch || !schema.Batch.AllowSplit || schema.Batch.SemanticallyLinked {
				return nil, mediaCapabilityFieldError("n", "unsupported_batch", "provider cannot preserve this batch without independent splitting")
			}
			match.NeedsSplit = true
		}
		count := original.RequestedCount
		if count < 1 || match.NeedsSplit {
			count = 1
		}
		if _, _, err := BuildUnifiedMediaPayload(platform, mediaType, model, request, count); err != nil {
			return nil, mediaCapabilityFieldError("model", "unsupported_parameters", err.Error())
		}
		return &unifiedMediaProviderPlan{Schema: append(json.RawMessage(nil), providerSchema...), Request: request, Match: match, Profile: &profile}, nil
	}
	return nil, mediaCapabilityFieldError("model", "unsupported_parameters", "provider model has no verified parameter rules")
}

// Replay the frozen schema, never the current catalog. Image models bind their
// captured request tier/geometry; explicit quality levels stay exact.
func adaptUnifiedMediaProviderRequest(mediaType string, schemaJSON json.RawMessage, original *mediacapability.NormalizedRequest, profile *UnifiedMediaAdapterProfile) (*mediacapability.NormalizedRequest, error) {
	if original == nil || original.Values == nil {
		return nil, errors.New("media normalized payload is unavailable")
	}
	compiled, err := mediacapability.Compile(mediaType, schemaJSON)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(original.Values)
	if err != nil {
		return nil, err
	}
	var values map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&values); err != nil {
		return nil, err
	}
	// Canonicalize only the provider's copied request. The accepted public
	// spelling stays in the snapshot, and replay uses this frozen profile.
	if profile != nil && profile.ResolutionUppercase {
		if metadata, ok := values["metadata"].(map[string]any); ok {
			if resolution, ok := metadata["resolution"].(string); ok {
				metadata["resolution"] = strings.ToUpper(resolution)
			}
		}
	}
	if original.FixedImageResolution != "" || original.RequestedImageSpec != nil {
		if mediaType != "image" || original.ImageSpec == nil || (original.FixedImageResolution != "" && original.ImageSpec.Resolution != original.FixedImageResolution) {
			return nil, errors.New("fixed image specification is unavailable")
		}
		metadata, _ := values["metadata"].(map[string]any)
		if metadata == nil {
			metadata = map[string]any{}
		}
		if resolution, exists := metadata["resolution"]; exists && resolution != original.ImageSpec.Resolution {
			return nil, errors.New("resolution conflicts with the fixed image model")
		}
		metadata["resolution"] = original.ImageSpec.Resolution
		values["metadata"] = metadata
		values["size"] = original.ImageSpec.AspectRatio
	}
	var schema mediacapability.Schema
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		return nil, err
	}
	if metadata, ok := values["metadata"].(map[string]any); ok && metadata["quality"] == "auto" {
		field := schema.Fields["metadata"].Properties["quality"]
		acceptsAuto := len(field.Enum) == 0
		for _, value := range field.Enum {
			if value == "auto" {
				acceptsAuto = true
			}
		}
		if !acceptsAuto && len(field.Default) > 0 {
			delete(metadata, "quality")
		}
	}
	raw, err = json.Marshal(values)
	if err != nil {
		return nil, err
	}
	validated, fields := compiled.Validate(raw)
	if len(fields) > 0 {
		return nil, mediaCapabilityValidationError(fields)
	}
	validated.AtomicBatch = original.AtomicBatch
	validated.ReplyType = original.ReplyType
	validated.RequestedImageSpec = original.RequestedImageSpec
	validated.FixedImageResolution = original.FixedImageResolution
	return validated, nil
}

// GRS Flare/Sunburst accept concrete pixels. Capture reviewed request geometry in
// the provider schema so workers never recalculate it from a mutable catalogue.
func fixedImageProviderSchema(platform, mediaType, model string, raw json.RawMessage, profile UnifiedMediaAdapterProfile, original *mediacapability.NormalizedRequest) (json.RawMessage, error) {
	if original == nil || (original.FixedImageResolution == "" && original.RequestedImageSpec == nil) || platform != PlatformGrsai || mediaType != "image" || !grsaiRequiresPixelSize(model) || !profile.PixelSize {
		return raw, nil
	}
	spec := original.ImageSpec
	if spec == nil || (original.FixedImageResolution != "" && spec.Resolution != original.FixedImageResolution) || spec.Width < 1 || spec.Height < 1 || spec.Width > 3840 || spec.Height > 3840 || spec.Width%16 != 0 || spec.Height%16 != 0 || spec.Width > 3*spec.Height || spec.Height > 3*spec.Width || spec.Width*spec.Height < 655360 || spec.Width*spec.Height > 8294400 {
		return nil, mediaCapabilityFieldError("aspectRatio", "unsupported_image_spec", "GRS cannot preserve the fixed image dimensions")
	}
	var schema mediacapability.Schema
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, err
	}
	found := false
	for i, configured := range schema.ImageSpecs {
		if configured.AspectRatio == spec.AspectRatio && configured.Resolution == spec.Resolution {
			schema.ImageSpecs[i] = *spec
			found = true
			break
		}
	}
	if !found {
		return nil, mediaCapabilityFieldError("aspectRatio", "unsupported_image_spec", "GRS has no reviewed ratio and tier rule")
	}
	return json.Marshal(schema)
}
