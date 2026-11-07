package service

import (
	"context"
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
