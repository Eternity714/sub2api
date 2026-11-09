package service

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
)

const (
	KuaipaoDefaultBaseURL    = "https://image.kuaipao.pro/v2"
	KuaipaoDefaultImageModel = "gpt-image-2.6-1k"
)

// kuaipaoMediaProvider is the independent Kuaipao adapter for unified tasks.
// The vendor uses OpenAI-compatible JSON, while its account, route, quota and
// audit identities remain kuaipao. Synchronous URL results enter the same
// durable storage and per-output settlement path as the other media providers.
type kuaipaoMediaProvider struct{ transport *unifiedMediaHTTPUpstream }

var _ UnifiedMediaUpstream = (*kuaipaoMediaProvider)(nil)

func buildUnifiedKuaipaoImagePayload(model string, values, metadata map[string]any, count int) ([]byte, string, error) {
	if count > 10 {
		return nil, "", errors.New("Kuaipao GPT images support at most 10 results per request")
	}
	resolution, _ := metadata["resolution"].(string)
	if resolution == "" {
		resolution = "1K"
	}
	for _, tier := range []string{"1k", "2k", "4k"} {
		if strings.HasSuffix(model, "-"+tier) {
			if _, explicit := metadata["resolution"]; !explicit {
				resolution = strings.ToUpper(tier)
			}
			if resolution != strings.ToUpper(tier) {
				return nil, "", errors.New("requested resolution conflicts with the fixed Kuaipao model tier")
			}
		}
	}
	if resolution != "1K" && resolution != "2K" && resolution != "4K" {
		return nil, "", errors.New("unsupported Kuaipao image resolution")
	}
	out := map[string]any{"model": model, "n": count, "response_format": "url", "resolution": resolution}
	if prompt, exists := values["prompt"]; exists {
		out["prompt"] = prompt
	}
	if ratio, exists := values["size"]; exists {
		out["aspect_ratio"] = ratio
	} else {
		out["aspect_ratio"] = "1:1"
	}
	if images, exists := values["image_urls"]; exists {
		out["image"] = images
	}
	for key, value := range metadata {
		if key == "resolution" {
			continue
		}
		// Public validation and captured provider rules bound these fields. Never
		// let a metadata property override the request envelope.
		switch key {
		case "quality", "background", "output_format", "output_compression", "style", "moderation", "input_fidelity", "watermark":
			out[key] = value
		default:
			return nil, "", errors.New("unsupported Kuaipao image parameter")
		}
	}
	encoded, err := json.Marshal(out)
	// The shared URL joiner replaces /v1 with the account's /v2 prefix.
	return encoded, "/v1/images/generations", err
}

func (p *kuaipaoMediaProvider) Submit(ctx context.Context, account *Account, mediaType, model string, normalized *mediacapability.NormalizedRequest, count int) (*UnifiedMediaUpstreamResult, error) {
	if p == nil || p.transport == nil || account == nil || account.Platform != PlatformKuaipao || account.Type != AccountTypeAPIKey || mediaType != "image" {
		return &UnifiedMediaUpstreamResult{Disposition: "rejected"}, errors.New("invalid Kuaipao image account")
	}
	copy := *account
	if strings.TrimSpace(copy.GetCredential("base_url")) == "" {
		copy.Credentials = maps.Clone(account.Credentials)
		if copy.Credentials == nil {
			copy.Credentials = make(map[string]any)
		}
		copy.Credentials["base_url"] = KuaipaoDefaultBaseURL
	}
	body, path, err := BuildUnifiedMediaPayload(PlatformKuaipao, mediaType, model, normalized, count)
	if err != nil {
		return &UnifiedMediaUpstreamResult{Disposition: "rejected"}, err
	}
	return p.transport.do(ctx, &copy, "POST", path, body, mediaType, model)
}

func (p *kuaipaoMediaProvider) Poll(context.Context, *Account, string, string, string) (*UnifiedMediaUpstreamResult, error) {
	return nil, errors.New("Kuaipao image results are synchronous and cannot be polled upstream")
}
