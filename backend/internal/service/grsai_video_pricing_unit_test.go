//go:build unit

package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGrsaiVideoPriceUsesGroupBeforeChannel(t *testing.T) {
	channel := newResolverWithChannel(t, []ChannelModelPricing{{Platform: "anthropic", Models: []string{"minimax-h3"}, BillingMode: BillingModeVideo, Intervals: []PricingInterval{{TierLabel: "768p", PerRequestPrice: testPtrFloat64(.14)}}}})
	group := &Group{ID: 100}
	resolver := &GrsaiModelPricingResolver{Resolver: channel}
	got, err := resolver.ResolveGrsaiTaskPrice(context.Background(), "minimax-h3", group, "768p")
	require.NoError(t, err)
	require.InDelta(t, .14, got.UnitPrice, 1e-10)
	group.ModelPricing = []ChannelModelPricing{{Models: []string{"minimax-h3"}, BillingMode: BillingModeVideo, Intervals: []PricingInterval{{TierLabel: "768p", PerRequestPrice: testPtrFloat64(.11)}}}}
	got, err = resolver.ResolveGrsaiTaskPrice(context.Background(), "minimax-h3", group, "768p")
	require.NoError(t, err)
	require.InDelta(t, .11, got.UnitPrice, 1e-10)
	group.ModelPricing[0].Intervals = []PricingInterval{{TierLabel: "480p", PerRequestPrice: testPtrFloat64(.10)}}
	_, err = resolver.ResolveGrsaiTaskPrice(context.Background(), "minimax-h3", group, "768p")
	require.ErrorIs(t, err, ErrGrsaiSettlementPricingMissing)
}
