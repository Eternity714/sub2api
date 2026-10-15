//go:build unit

package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestUnifiedMediaPricingMatchesOutputSpecification(t *testing.T) {
	price := func(v float64) *float64 { return &v }
	group := &Group{ID: 1, ModelPricing: []ChannelModelPricing{{Models: []string{"public-image"}, BillingMode: BillingModeImage, PerRequestPrice: price(99), Intervals: []PricingInterval{{TierLabel: "1K", PerRequestPrice: price(.1)}, {TierLabel: "4K", PerRequestPrice: price(.4)}}}}}
	resolver := &UnifiedMediaPricingResolver{Resolver: NewModelPricingResolver(nil, nil)}
	got, err := resolver.ResolveGrsaiTaskPrice(context.Background(), "public-image", group, "4K")
	require.NoError(t, err)
	require.Equal(t, .4, got.UnitPrice)
	for _, spec := range []string{"", "2K", "4k"} {
		_, err = resolver.ResolveGrsaiTaskPrice(context.Background(), "public-image", group, spec)
		require.Error(t, err)
	}
}
