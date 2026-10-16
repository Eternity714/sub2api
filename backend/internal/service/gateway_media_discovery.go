package service

import (
	"context"
	"reflect"
	"strings"
	"time"
)

// FilterCompositeMediaDiscoveryModels hides only proven equivalent media aliases
// from a caller's final, allowlisted catalog. It never changes model acceptance.
// Missing dependencies, prices or route evidence preserve the original IDs.
func (s *GatewayService) FilterCompositeMediaDiscoveryModels(ctx context.Context, groupID *int64, models []string) []string {
	if s == nil || s.channelService == nil || s.compositeResolver == nil || s.compositeResolver.repo == nil || groupID == nil || *groupID <= 0 {
		return models
	}
	visible := make(map[string][]string, len(models))
	for _, model := range models {
		visible[strings.ToLower(model)] = append(visible[strings.ToLower(model)], model)
	}
	channel, err := s.channelService.GetChannelForGroup(ctx, *groupID)
	if err != nil || channel == nil {
		return models
	}
	routes, err := s.compositeResolver.repo.ListByGroup(ctx, *groupID, false)
	if err != nil {
		return models
	}
	out := make([]string, 0, len(models))
	for _, model := range models {
		provider, neutral, qualified := strings.Cut(strings.ToLower(model), "/")
		hide := false
		if qualified && isMediaRoutePlatform(provider) && neutral != "" {
			aliasRoutes, proven := mediaDiscoveryCandidates(routes, *groupID, model)
			if proven {
				aliasPrice := s.channelService.GetChannelModelPricing(ctx, *groupID, model)
				for _, publicModel := range visible[neutral] {
					publicRoutes, publicProven := mediaDiscoveryCandidates(routes, *groupID, publicModel)
					if !publicProven || !mediaDiscoveryCovers(publicRoutes, aliasRoutes) {
						continue
					}
					publicPrice := s.channelService.GetChannelModelPricing(ctx, *groupID, publicModel)
					if equivalentMediaDiscoveryPrices(aliasPrice, publicPrice) {
						hide = true
						break
					}
				}
			}
		}
		if !hide {
			out = append(out, model)
		}
	}
	return out
}

type mediaDiscoveryCandidate struct {
	endpoint, platform, upstream, capability string
}

func mediaDiscoveryCandidates(routes []CompositeModelRoute, groupID int64, model string) (map[mediaDiscoveryCandidate]struct{}, bool) {
	out := make(map[mediaDiscoveryCandidate]struct{})
	for _, route := range routes {
		if !route.Enabled || route.GroupID != groupID {
			continue
		}
		if route.MatchType == CompositeRouteMatchPrefix && route.PublicModel != "" && strings.HasPrefix(model, route.PublicModel) {
			return nil, false
		}
		if route.PublicModel != model || route.MatchType != CompositeRouteMatchExact {
			continue
		}
		// Catalog IDs can also serve chat/any routes. Those are not media aliases.
		if !isMediaRouteEndpoint(route.Endpoint) || !isMediaRoutePlatform(route.TargetPlatform) {
			return nil, false
		}
		upstream := strings.TrimSpace(route.UpstreamModel)
		if upstream == "" {
			upstream = model
		}
		capability := route.CapabilityModel
		if capability == "" {
			capability = model
		}
		out[mediaDiscoveryCandidate{route.Endpoint, route.TargetPlatform, upstream, capability}] = struct{}{}
	}
	return out, len(out) > 0
}

func mediaDiscoveryCovers(public, alias map[mediaDiscoveryCandidate]struct{}) bool {
	for candidate := range alias {
		if _, exists := public[candidate]; !exists {
			return false
		}
	}
	return len(alias) > 0
}

func equivalentMediaDiscoveryPrices(a, b *ChannelModelPricing) bool {
	if a == nil || b == nil || (a.BillingMode != BillingModeImage && a.BillingMode != BillingModeVideo && a.BillingMode != BillingModePerRequest) {
		return false
	}
	if a.PerRequestPrice == nil && len(a.Intervals) == 0 {
		return false
	}
	return reflect.DeepEqual(mediaDiscoveryPriceSemantics(a), mediaDiscoveryPriceSemantics(b))
}

func mediaDiscoveryPriceSemantics(price *ChannelModelPricing) ChannelModelPricing {
	copy := price.Clone()
	copy.ID, copy.ChannelID, copy.Platform, copy.Models = 0, 0, "", nil
	copy.CreatedAt, copy.UpdatedAt = time.Time{}, time.Time{}
	for index := range copy.Intervals {
		copy.Intervals[index].ID, copy.Intervals[index].PricingID = 0, 0
		copy.Intervals[index].CreatedAt, copy.Intervals[index].UpdatedAt = time.Time{}, time.Time{}
	}
	return copy
}
