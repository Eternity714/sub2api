//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnifiedMediaPricingMatchesOutputSpecification(t *testing.T) {
	price := func(v float64) *float64 { return &v }
	group := &Group{ID: 1, Platform: PlatformComposite}
	resolver := unifiedMediaChannelPricingResolver(group.ID, StatusActive, []ChannelModelPricing{{Platform: PlatformToAPIs, Models: []string{"public-image"}, BillingMode: BillingModeImage, PerRequestPrice: price(99), Intervals: []PricingInterval{{TierLabel: "1K", PerRequestPrice: price(.1)}, {TierLabel: "4K", PerRequestPrice: price(.4)}}}})
	got, err := resolver.ResolveGrsaiTaskPrice(context.Background(), "public-image", group, "4K")
	require.NoError(t, err)
	require.Equal(t, .4, got.UnitPrice)
	for _, spec := range []string{"", "2K", "4k"} {
		_, err = resolver.ResolveGrsaiTaskPrice(context.Background(), "public-image", group, spec)
		require.Error(t, err)
	}
}

func unifiedMediaChannelPricingResolver(groupID int64, status string, prices []ChannelModelPricing) *UnifiedMediaPricingResolver {
	channel := Channel{ID: 1, Status: status, GroupIDs: []int64{groupID}, ModelPricing: prices}
	channels := &ChannelService{}
	channels.cache.Store(populateChannelCache([]Channel{channel}, map[int64]string{groupID: PlatformComposite}))
	return &UnifiedMediaPricingResolver{Resolver: NewModelPricingResolver(channels, nil)}
}

// AC-028: unified media uses explicit channel prices without group overrides.
func TestUnifiedMediaPricingUsesChannelInsteadOfGroupOverride(t *testing.T) {
	group := &Group{ID: 1, Platform: PlatformComposite, ModelPricing: []ChannelModelPricing{{Models: []string{"public-image"}, BillingMode: BillingModeImage, PerRequestPrice: float64Ptr(99)}}}
	resolver := unifiedMediaChannelPricingResolver(group.ID, StatusActive, []ChannelModelPricing{{Platform: PlatformToAPIs, Models: []string{"public-image"}, BillingMode: BillingModeImage, PerRequestPrice: float64Ptr(.7)}})

	got, err := resolver.ResolveGrsaiTaskPrice(context.Background(), "public-image", group, "")
	require.NoError(t, err)
	require.Equal(t, .7, got.UnitPrice)

	// Other gateway paths retain their existing group override semantics.
	regular := resolver.Resolver.Resolve(context.Background(), PricingInput{Model: "public-image", GroupID: &group.ID, Group: group})
	require.Equal(t, PricingSourceGroup, regular.Source)
	require.Equal(t, 99.0, regular.DefaultPerRequestPrice)
}

func TestUnifiedMediaPricingRequiresActiveMatchingChannel(t *testing.T) {
	group := &Group{ID: 1, Platform: PlatformComposite, ModelPricing: []ChannelModelPricing{{Models: []string{"public-image"}, BillingMode: BillingModeImage, PerRequestPrice: float64Ptr(99)}}}
	for name, resolver := range map[string]*UnifiedMediaPricingResolver{
		"no channel service": {Resolver: NewModelPricingResolver(nil, nil)},
		"disabled channel":   unifiedMediaChannelPricingResolver(group.ID, StatusDisabled, []ChannelModelPricing{{Platform: PlatformToAPIs, Models: []string{"public-image"}, BillingMode: BillingModeImage, PerRequestPrice: float64Ptr(.7)}}),
		"unrelated model":    unifiedMediaChannelPricingResolver(group.ID, StatusActive, []ChannelModelPricing{{Platform: PlatformToAPIs, Models: []string{"another-image"}, BillingMode: BillingModeImage, PerRequestPrice: float64Ptr(.7)}}),
		"unassociated group": unifiedMediaChannelPricingResolver(2, StatusActive, []ChannelModelPricing{{Platform: PlatformToAPIs, Models: []string{"public-image"}, BillingMode: BillingModeImage, PerRequestPrice: float64Ptr(.7)}}),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := resolver.ResolveGrsaiTaskPrice(context.Background(), "public-image", group, "")
			require.ErrorIs(t, err, ErrGrsaiSettlementPricingMissing)
		})
	}
}

func TestUnifiedMediaPricingSupportsProviderMediaModes(t *testing.T) {
	group := &Group{ID: 1, Platform: PlatformComposite}
	for _, platform := range []string{PlatformGrsai, PlatformToAPIs} {
		for _, mode := range []BillingMode{BillingModeImage, BillingModeVideo, BillingModePerRequest} {
			t.Run(platform+"/"+string(mode), func(t *testing.T) {
				resolver := unifiedMediaChannelPricingResolver(group.ID, StatusActive, []ChannelModelPricing{{Platform: platform, Models: []string{"public-media"}, BillingMode: mode, PerRequestPrice: float64Ptr(.7)}})
				got, err := resolver.ResolveGrsaiTaskPrice(context.Background(), "public-media", group, "")
				require.NoError(t, err)
				require.Equal(t, mode, got.Mode)
				require.Equal(t, .7, got.UnitPrice)
			})
		}
	}
}
