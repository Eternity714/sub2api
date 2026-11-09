package service

import (
	"bytes"
	"encoding/json"
	"errors"

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
		request, err := adaptUnifiedMediaProviderRequest(mediaType, entry.Schema, original)
		if err != nil {
			return nil, err
		}
		profile := mediaAdapterProfile(platform, mediaType, model)
		request.AdapterProfile, _ = json.Marshal(profile)
		if original.RequestedImageSpec != nil {
			// Exact pixels are a stronger promise than a ratio and resolution tier.
			wanted, actual := original.RequestedImageSpec, request.ImageSpec
			if actual == nil || wanted.Width != actual.Width || wanted.Height != actual.Height {
				return nil, mediaCapabilityFieldError("aspectRatio", "unsupported_image_spec", "provider cannot preserve the requested pixel dimensions")
			}
			if platform == PlatformToAPIs && !profile.PixelSize {
				return nil, mediaCapabilityFieldError("aspectRatio", "unsupported_image_spec", "provider accepts ratio and tier, not exact pixel dimensions")
			}
		}
		var schema mediacapability.Schema
		if err := json.Unmarshal(entry.Schema, &schema); err != nil {
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
		return &unifiedMediaProviderPlan{Schema: append(json.RawMessage(nil), entry.Schema...), Request: request, Match: match, Profile: &profile}, nil
	}
	return nil, mediaCapabilityFieldError("model", "unsupported_parameters", "provider model has no verified parameter rules")
}

// Replay the frozen schema, never the current catalog. The sole value adaptation
// is quality:auto -> the documented provider default; explicit levels stay exact.
func adaptUnifiedMediaProviderRequest(mediaType string, schemaJSON json.RawMessage, original *mediacapability.NormalizedRequest) (*mediacapability.NormalizedRequest, error) {
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
	return validated, nil
}
