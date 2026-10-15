package service

import "context"

// UnifiedMediaPricingResolver prices the public model and the normalized output
// tier from the group's active channel. Group model overrides and catalog prices
// do not apply to unified media. A configured tier list is exhaustive; it never
// falls back to another resolution or to a token price.
type UnifiedMediaPricingResolver struct{ Resolver *ModelPricingResolver }

func (r *UnifiedMediaPricingResolver) GrsaiUnitPrice(ctx context.Context, model string, group *Group) (float64, error) {
	p, err := r.ResolveGrsaiTaskPrice(ctx, model, group, "")
	return p.UnitPrice, err
}
func (r *UnifiedMediaPricingResolver) ResolveGrsaiTaskPrice(ctx context.Context, model string, group *Group, resolution string) (GrsaiTaskPrice, error) {
	if r == nil || r.Resolver == nil || r.Resolver.channelService == nil || group == nil {
		return GrsaiTaskPrice{}, ErrGrsaiSettlementPricingMissing
	}
	p := r.Resolver.channelService.GetChannelModelPricing(ctx, group.ID, model)
	if p == nil {
		return GrsaiTaskPrice{}, ErrGrsaiSettlementPricingMissing
	}
	if p.BillingMode != BillingModeImage && p.BillingMode != BillingModeVideo && p.BillingMode != BillingModePerRequest {
		return GrsaiTaskPrice{}, ErrGrsaiSettlementPricingMissing
	}
	configured := p.PerRequestPrice
	if len(p.Intervals) > 0 {
		configured = nil
		for _, tier := range p.Intervals {
			if tier.TierLabel == resolution && resolution != "" {
				if configured != nil || tier.PerRequestPrice == nil {
					return GrsaiTaskPrice{}, ErrGrsaiSettlementPricingMissing
				}
				configured = tier.PerRequestPrice
			}
		}
	}
	if configured == nil || !grsaiFiniteNonNegative(*configured) {
		return GrsaiTaskPrice{}, ErrGrsaiSettlementPricingMissing
	}
	return GrsaiTaskPrice{Mode: p.BillingMode, UnitPrice: *configured, Resolution: resolution}, nil
}
