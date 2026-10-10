package service

import (
	"context"
	"testing"
	"time"

	gocache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"
)

type userGroupRateResolverRepoStub struct {
	UserGroupRateRepository

	rate  *float64
	err   error
	calls int
}

func (s *userGroupRateResolverRepoStub) GetByUserAndGroup(ctx context.Context, userID, groupID int64) (*float64, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.rate, nil
}

func TestNewUserGroupRateResolver_Defaults(t *testing.T) {
	resolver := newUserGroupRateResolver(nil, nil, 0, nil, "")

	require.NotNil(t, resolver)
	require.NotNil(t, resolver.cache)
	require.Equal(t, defaultUserGroupRateCacheTTL, resolver.cacheTTL)
	require.NotNil(t, resolver.sf)
	require.Equal(t, "service.gateway", resolver.logComponent)
}

func TestUserGroupRateResolverResolve_FallbackForNilResolverAndInvalidIDs(t *testing.T) {
	var nilResolver *userGroupRateResolver
	require.Equal(t, 1.4, nilResolver.Resolve(context.Background(), 101, 202, 1.4))

	resolver := newUserGroupRateResolver(nil, nil, time.Second, nil, "service.test")
	require.Equal(t, 1.4, resolver.Resolve(context.Background(), 0, 202, 1.4))
	require.Equal(t, 1.4, resolver.Resolve(context.Background(), 101, 0, 1.4))
}

func TestUserGroupRateResolverResolve_InvalidCacheEntryLoadsRepoAndCaches(t *testing.T) {
	resetGatewayHotpathStatsForTest()

	rate := 1.7
	repo := &userGroupRateResolverRepoStub{rate: &rate}
	cache := gocache.New(time.Minute, time.Minute)
	cache.Set("101:202", "bad-cache", time.Minute)
	resolver := newUserGroupRateResolver(repo, cache, time.Minute, nil, "service.test")

	got := resolver.Resolve(context.Background(), 101, 202, 1.2)
	require.Equal(t, rate, got)
	require.Equal(t, 1, repo.calls)

	cached, ok := cache.Get("101:202")
	require.True(t, ok)
	require.Equal(t, rate, cached)

	hit, miss, load, _, fallback := GatewayUserGroupRateCacheStats()
	require.Equal(t, int64(0), hit)
	require.Equal(t, int64(1), miss)
	require.Equal(t, int64(1), load)
	require.Equal(t, int64(0), fallback)
}

func TestGatewayServiceGetUserGroupRateMultiplier_FallbacksAndUsesExistingResolver(t *testing.T) {
	var nilSvc *GatewayService
	require.Equal(t, 1.3, nilSvc.getUserGroupRateMultiplier(context.Background(), 101, 202, 1.3))

	rate := 1.9
	repo := &userGroupRateResolverRepoStub{rate: &rate}
	resolver := newUserGroupRateResolver(repo, nil, time.Minute, nil, "service.gateway")
	svc := &GatewayService{userGroupRateResolver: resolver}

	got := svc.getUserGroupRateMultiplier(context.Background(), 101, 202, 1.2)
	require.Equal(t, rate, got)
	require.Equal(t, 1, repo.calls)
}

func TestGatewayRateDefault_NoPerRequestAllocations(t *testing.T) {
	ctx := context.Background()
	const defaultRate = 1.3
	tests := []struct {
		name    string
		resolve func(context.Context, int64, int64, float64) float64
	}{
		{"gateway", (&GatewayService{}).getUserGroupRateMultiplier},
		{"openai", (&OpenAIGatewayService{}).ResolveUserGroupRateMultiplier},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var rate float64
			allocs := testing.AllocsPerRun(100, func() {
				rate = test.resolve(ctx, 101, 202, defaultRate)
			})
			require.Equal(t, defaultRate, rate)
			require.Zero(t, allocs, "default-only lookups must not allocate caches and cleanup goroutines per request")
		})
	}
}

func TestGatewayRateDefault_PreservesPrewarmedCache(t *testing.T) {
	cache := gocache.New(time.Minute, 0)
	cache.Set("101:202", 1.9, time.Minute)
	ctx := context.Background()

	gateway := &GatewayService{userGroupRateCache: cache}
	require.Equal(t, 1.9, gateway.getUserGroupRateMultiplier(ctx, 101, 202, 1.3))
	require.Equal(t, 1.3, gateway.getUserGroupRateMultiplier(ctx, 101, 203, 1.3))

	openai := &OpenAIGatewayService{
		userGroupRateResolver: newUserGroupRateResolver(nil, cache, time.Minute, nil, "service.openai_gateway"),
	}
	require.Equal(t, 1.9, openai.ResolveUserGroupRateMultiplier(ctx, 101, 202, 1.3))
	require.Equal(t, 1.3, openai.ResolveUserGroupRateMultiplier(ctx, 101, 203, 1.3))
}
