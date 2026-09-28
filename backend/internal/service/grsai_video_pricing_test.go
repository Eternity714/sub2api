package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrsaiVideoPriceRequiresExactConfiguredTier(t *testing.T) {
	p := func(v float64) *float64 { return &v }
	group := &Group{ID: 1, ModelPricing: []ChannelModelPricing{{Models: []string{"minimax-h3"}, BillingMode: BillingModeVideo, PerRequestPrice: p(9), Intervals: []PricingInterval{
		{TierLabel: "480p", PerRequestPrice: p(.10)}, {TierLabel: "768p", PerRequestPrice: p(.14)}, {TierLabel: "1080p", PerRequestPrice: p(.30)},
	}}}}
	resolver := &GrsaiModelPricingResolver{Resolver: NewModelPricingResolver(nil, nil)}
	got, err := resolver.ResolveGrsaiTaskPrice(context.Background(), "minimax-h3", group, "768p")
	require.NoError(t, err)
	require.Equal(t, BillingModeVideo, got.Mode)
	require.Equal(t, "768p", got.Resolution)
	require.InDelta(t, .14, got.UnitPrice, 1e-10)
	for _, tier := range []string{"", "720p", "768P"} {
		_, err = resolver.ResolveGrsaiTaskPrice(context.Background(), "minimax-h3", group, tier)
		require.ErrorIs(t, err, ErrGrsaiSettlementPricingMissing)
	}
	group.ModelPricing[0].Intervals = append(group.ModelPricing[0].Intervals, PricingInterval{TierLabel: "768p", PerRequestPrice: p(.14)})
	_, err = resolver.ResolveGrsaiTaskPrice(context.Background(), "minimax-h3", group, "768p")
	require.ErrorIs(t, err, ErrGrsaiSettlementPricingMissing)
}

func TestGrsaiVideoPriceFlatAndExplicitZero(t *testing.T) {
	zero := 0.0
	group := &Group{ID: 1, ModelPricing: []ChannelModelPricing{{Models: []string{"future-video"}, BillingMode: BillingModeVideo, PerRequestPrice: &zero}}}
	resolver := &GrsaiModelPricingResolver{Resolver: NewModelPricingResolver(nil, nil)}
	got, err := resolver.ResolveGrsaiTaskPrice(context.Background(), "future-video", group, "custom")
	require.NoError(t, err)
	require.Equal(t, BillingModeVideo, got.Mode)
	require.Zero(t, got.UnitPrice)
	group.ModelPricing[0].PerRequestPrice = nil
	_, err = resolver.ResolveGrsaiTaskPrice(context.Background(), "future-video", group, "custom")
	require.ErrorIs(t, err, ErrGrsaiSettlementPricingMissing)
}

func TestGrsaiVideoRequestValidation(t *testing.T) {
	for _, tt := range []struct {
		model, resolution string
		duration          int
		valid             bool
	}{
		{"minimax-h3", "480p", 1, true}, {"minimax-h3", "768p", 15, true}, {"minimax-h3", "1080p", 10, true},
		{"minimax-h3", "1080p", 11, false}, {"minimax-h3", "720p", 2, false}, {"minimax-h3", "480p", 16, false},
		{"future-video", "cinema", 120, true}, {"future-video", "", 1, true}, {"future-video", "", 0, false},
	} {
		err := ValidateGrsaiVideoRequest(tt.model, tt.resolution, tt.duration)
		if tt.valid {
			require.NoError(t, err)
		} else {
			require.ErrorIs(t, err, ErrGrsaiInvalidRequest)
		}
	}
}
