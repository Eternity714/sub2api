//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type mediaReservationAuthRepo struct {
	*authRepoStub
	amount  float64
	readErr error
	reads   int
}

func (r *mediaReservationAuthRepo) GetReservedMediaAmount(context.Context, int64) (float64, error) {
	r.reads++
	return r.amount, r.readErr
}

func TestAPIKeyMediaReservationsRefreshAfterCachedAuthentication(t *testing.T) {
	entry := &APIKeyAuthCacheEntry{Snapshot: &APIKeyAuthSnapshot{Version: apiKeyAuthSnapshotVersion, APIKeyID: 1, UserID: 2, Status: StatusAPIKeyActive, Quota: 10, QuotaUsed: 2, User: APIKeyAuthUserSnapshot{ID: 2, Status: StatusActive, Balance: 100}}}
	cache := &authCacheStub{getAuthCache: func(context.Context, string) (*APIKeyAuthCacheEntry, error) { return entry, nil }}
	repo := &mediaReservationAuthRepo{authRepoStub: &authRepoStub{}, amount: 3}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, &config.Config{APIKeyAuth: config.APIKeyAuthCacheConfig{L2TTLSeconds: 60}})
	first, err := svc.GetByKey(context.Background(), "synthetic-media-key")
	require.NoError(t, err)
	require.Equal(t, 3.0, first.ReservedMediaAmount)
	require.False(t, first.IsQuotaExhausted())
	repo.amount = 8
	second, err := svc.GetByKey(context.Background(), "synthetic-media-key")
	require.NoError(t, err)
	require.Equal(t, 8.0, second.ReservedMediaAmount)
	require.True(t, second.IsQuotaExhausted())
	require.Zero(t, second.GetQuotaRemaining())
	require.Equal(t, 3.0, first.ReservedMediaAmount)
	require.Equal(t, 2.0, entry.Snapshot.QuotaUsed)
	require.Equal(t, 2, repo.reads)
	repo.readErr = errors.New("database unavailable")
	third, err := svc.GetByKey(context.Background(), "synthetic-media-key")
	require.NoError(t, err)
	require.True(t, third.MediaReservationReadFailed)
	require.ErrorIs(t, (&BillingCacheService{}).CheckBillingEligibility(context.Background(), nil, third, nil, nil, ""), ErrBillingServiceUnavailable)
}

func TestMediaReservationsCountAgainstEveryKeyMonetaryWindow(t *testing.T) {
	now := time.Now()
	svc := &BillingCacheService{}
	for _, tc := range []struct {
		key  APIKey
		want error
	}{
		{APIKey{RateLimit5h: 5, ReservedMediaAmount: 3}, ErrAPIKeyRateLimit5hExceeded},
		{APIKey{RateLimit1d: 5, ReservedMediaAmount: 3}, ErrAPIKeyRateLimit1dExceeded},
		{APIKey{RateLimit7d: 5, ReservedMediaAmount: 3}, ErrAPIKeyRateLimit7dExceeded},
	} {
		key := tc.key
		require.ErrorIs(t, svc.evaluateRateLimits(context.Background(), &key, 2, 2, 2, &now, &now, &now, 0), tc.want)
	}
	key := &APIKey{Quota: 10, QuotaUsed: 2, ReservedMediaAmount: 7}
	require.ErrorIs(t, svc.checkFixedChargeAPIKey(context.Background(), key, 2), ErrAPIKeyQuotaExhausted)
	require.NoError(t, svc.checkFixedChargeAPIKey(context.Background(), key, 1))
	key.MediaReservationReadFailed = true
	require.ErrorIs(t, svc.checkFixedChargeAPIKey(context.Background(), key, 1), ErrBillingServiceUnavailable)
}
