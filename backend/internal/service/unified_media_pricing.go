package service

import "context"

// UnifiedMediaPricingResolver prices the public model and the normalized output
// tier. A configured tier list is exhaustive; it never falls back to another
// resolution or to a token price.
type UnifiedMediaPricingResolver struct{ Resolver *ModelPricingResolver }

func (r *UnifiedMediaPricingResolver) GrsaiUnitPrice(ctx context.Context, model string, group *Group) (float64, error) {
	p, err := r.ResolveGrsaiTaskPrice(ctx, model, group, "")
	return p.UnitPrice, err
}
func (r *UnifiedMediaPricingResolver) ResolveGrsaiTaskPrice(ctx context.Context, model string, group *Group, resolution string) (GrsaiTaskPrice, error) {
	if r == nil || r.Resolver == nil || group == nil {
		return GrsaiTaskPrice{}, ErrGrsaiSettlementPricingMissing
	}
	if r.Resolver.billingService == nil && matchGroupModelPricing(group, model) == nil && r.Resolver.channelService == nil {
		return GrsaiTaskPrice{}, ErrGrsaiSettlementPricingMissing
	}
	p := r.Resolver.Resolve(ctx, PricingInput{Model: model, GroupID: &group.ID, Group: group})
	if p == nil || (p.Source != PricingSourceGroup && p.Source != PricingSourceChannel) || p.channelPricing == nil {
		return GrsaiTaskPrice{}, ErrGrsaiSettlementPricingMissing
	}
	if p.Mode != BillingModeImage && p.Mode != BillingModeVideo && p.Mode != BillingModePerRequest {
		return GrsaiTaskPrice{}, ErrGrsaiSettlementPricingMissing
	}
	configured := p.channelPricing.PerRequestPrice
	if len(p.channelPricing.Intervals) > 0 {
		configured = nil
		for _, tier := range p.channelPricing.Intervals {
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
	return GrsaiTaskPrice{Mode: p.Mode, UnitPrice: *configured, Resolution: resolution}, nil
}
