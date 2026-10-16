//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type mediaDiscoveryUnavailableRouteRepo struct {
	CompositeModelRouteRepository
}

func (*mediaDiscoveryUnavailableRouteRepo) ListByGroup(context.Context, int64, bool) ([]CompositeModelRoute, error) {
	return nil, errors.New("route repository unavailable")
}

// AC-029: a discovery dependency failure cannot remove accepted model IDs.
func TestGatewayMediaDiscoveryKeepsCatalogOnRouteQueryFailure(t *testing.T) {
	const groupID int64 = 22
	id := groupID
	channels := &ChannelService{}
	channels.cache.Store(populateChannelCache([]Channel{{
		ID: 1, Status: StatusActive, GroupIDs: []int64{groupID},
	}}, map[int64]string{groupID: PlatformComposite}))
	svc := &GatewayService{
		channelService:    channels,
		compositeResolver: NewCompositeRouteResolver(&mediaDiscoveryUnavailableRouteRepo{}),
	}
	models := []string{"image", "toapis/image", "other"}
	before := append([]string(nil), models...)
	require.Equal(t, before, svc.FilterCompositeMediaDiscoveryModels(context.Background(), &id, models))
	require.Equal(t, before, models)
}

// AC-029: equivalence ignores storage identity while retaining all price semantics,
// and never mutates channel pricing shared with billing or admin reads.
func TestGatewayMediaDiscoveryPriceComparisonPreservesSource(t *testing.T) {
	basePrice, tierPrice, zero := 1.0, 2.0, 0.0
	a := ChannelModelPricing{
		ID: 1, ChannelID: 3, Platform: PlatformToAPIs, Models: []string{"image"},
		BillingMode: BillingModeImage, PerRequestPrice: &basePrice,
		Intervals: []PricingInterval{{ID: 4, PricingID: 1, TierLabel: "2K", PerRequestPrice: &tierPrice}},
		TimePricing: &ChannelTimePricing{Timezone: "Asia/Shanghai", Periods: []ChannelTimePricingPeriod{
			{StartTime: "10:00", EndTime: "11:00", Multiplier: 0.5},
		}},
		CreatedAt: time.Unix(100, 0), UpdatedAt: time.Unix(200, 0),
	}
	b := a.Clone()
	b.ID, b.ChannelID, b.Platform, b.Models = 5, 6, PlatformGrsai, []string{"toapis/image"}
	b.CreatedAt, b.UpdatedAt = time.Unix(300, 0), time.Unix(400, 0)
	b.Intervals[0].ID, b.Intervals[0].PricingID = 7, 5
	b.Intervals[0].CreatedAt = time.Unix(500, 0)
	aBefore, bBefore := a.Clone(), b.Clone()
	require.True(t, equivalentMediaDiscoveryPrices(&a, &b))
	require.Equal(t, aBefore, a)
	require.Equal(t, bBefore, b)

	t.Run("tier price matters", func(t *testing.T) {
		changed := b.Clone()
		changed.Intervals[0].PerRequestPrice = &zero
		require.False(t, equivalentMediaDiscoveryPrices(&a, &changed))
	})
	t.Run("time pricing matters", func(t *testing.T) {
		changed := b.Clone()
		changed.TimePricing.Periods[0].Multiplier = 0.8
		require.False(t, equivalentMediaDiscoveryPrices(&a, &changed))
	})
	t.Run("nil is not a free price", func(t *testing.T) {
		changed := b.Clone()
		changed.InputPrice = &zero
		require.False(t, equivalentMediaDiscoveryPrices(&a, &changed))
	})
	require.Equal(t, aBefore, a)
	require.Equal(t, bBefore, b)
}
