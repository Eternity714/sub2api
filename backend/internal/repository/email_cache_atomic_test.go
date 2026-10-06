package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newMiniredisEmailCache(t *testing.T) (service.EmailCache, *miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewEmailCache(rdb), mr, rdb
}

func TestEmailCache_ConcurrentWrongCodesCannotExceedAttemptCap(t *testing.T) {
	cache, _, _ := newMiniredisEmailCache(t)
	ctx := context.Background()
	email := "user@example.com"

	svc := service.NewEmailService(nil, cache)
	require.NoError(t, cache.SetVerificationCode(ctx, email, &service.VerificationCodeData{
		Code:      "123456",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}, 15*time.Minute))

	const workers = 50
	var invalid, maxed atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := svc.VerifyCode(ctx, email, "000000")
			switch {
			case errors.Is(err, service.ErrInvalidVerifyCode):
				invalid.Add(1)
			case errors.Is(err, service.ErrVerifyCodeMaxAttempts):
				maxed.Add(1)
			default:
				t.Errorf("unexpected result: %v", err)
			}
		}()
	}
	wg.Wait()

	// Only attempts 1..4 may return "invalid"; every other guess is rejected by the cap.
	require.LessOrEqual(t, int(invalid.Load()), 4)
	require.Equal(t, workers, int(invalid.Load()+maxed.Load()))

	// Even the correct code is now rejected.
	require.ErrorIs(t, svc.VerifyCode(ctx, email, "123456"), service.ErrVerifyCodeMaxAttempts)

	data, err := cache.GetVerificationCode(ctx, email)
	require.NoError(t, err)
	require.GreaterOrEqual(t, data.Attempts, 5)
}

func TestEmailCache_AttemptsResetOnNewCodeAndTTLFollowsCode(t *testing.T) {
	cache, mr, _ := newMiniredisEmailCache(t)
	ctx := context.Background()
	email := "User@Example.com"

	require.NoError(t, cache.SetVerificationCode(ctx, email, &service.VerificationCodeData{Code: "1"}, time.Minute))
	n, err := cache.IncrVerificationCodeAttempts(ctx, email)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Greater(t, mr.TTL(verifyCodeKey(email)+attemptsKeySuffix), time.Duration(0))

	require.NoError(t, cache.SetVerificationCode(ctx, email, &service.VerificationCodeData{Code: "2"}, time.Minute))
	data, err := cache.GetVerificationCode(ctx, email)
	require.NoError(t, err)
	require.Equal(t, 0, data.Attempts)

	require.NoError(t, cache.DeleteVerificationCode(ctx, email))
	_, err = cache.IncrVerificationCodeAttempts(ctx, email)
	require.Error(t, err)
	require.False(t, mr.Exists(verifyCodeKey(email)+attemptsKeySuffix))
}

func TestEmailCache_PasswordResetTokenHashedAndSingleUse(t *testing.T) {
	cache, mr, _ := newMiniredisEmailCache(t)
	ctx := context.Background()
	email := "reset@example.com"

	svc := service.NewEmailService(nil, cache)

	// Seed the token the same way SendPasswordResetEmail does (hash only).
	token, err := svc.GeneratePasswordResetToken()
	require.NoError(t, err)
	sum := sha256.Sum256([]byte(token))
	require.NoError(t, cache.SetPasswordResetToken(ctx, email, &service.PasswordResetTokenData{
		TokenHash: hex.EncodeToString(sum[:]), TokenFormat: service.PasswordResetTokenFormatSHA256, CreatedAt: time.Now(),
	}, 30*time.Minute))

	raw, err := mr.Get(passwordResetKey(email))
	require.NoError(t, err)
	require.False(t, strings.Contains(raw, token), "plaintext token must not be stored")

	require.NoError(t, svc.VerifyPasswordResetToken(ctx, email, token))
	require.ErrorIs(t, svc.ConsumePasswordResetToken(ctx, email, hex.EncodeToString(sum[:])), service.ErrInvalidResetToken)
	require.ErrorIs(t, svc.ConsumePasswordResetToken(ctx, email, "wrong"), service.ErrInvalidResetToken)

	const workers = 30
	var ok atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if svc.ConsumePasswordResetToken(ctx, email, token) == nil {
				ok.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), ok.Load())
	require.False(t, mr.Exists(passwordResetKey(email)))
}

func TestEmailCache_ConsumePasswordResetTokenMismatchKeepsToken(t *testing.T) {
	cache, mr, _ := newMiniredisEmailCache(t)
	ctx := context.Background()
	email := "keep@example.com"
	require.NoError(t, cache.SetPasswordResetToken(ctx, email, &service.PasswordResetTokenData{Token: "abc"}, time.Minute))

	ok, err := cache.ConsumePasswordResetToken(ctx, email, &service.PasswordResetTokenData{Token: "xyz"})
	require.NoError(t, err)
	require.False(t, ok)
	require.True(t, mr.Exists(passwordResetKey(email)))

	ok, err = cache.ConsumePasswordResetToken(ctx, email, &service.PasswordResetTokenData{Token: "abc"})
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = cache.ConsumePasswordResetToken(ctx, email, &service.PasswordResetTokenData{Token: "abc"})
	require.NoError(t, err)
	require.False(t, ok)
}

func TestEmailCache_LegacyPasswordResetTokenSingleUse(t *testing.T) {
	cache, mr, _ := newMiniredisEmailCache(t)
	ctx := context.Background()
	email := "legacy@example.com"
	token := strings.Repeat("a", 64)
	require.NoError(t, cache.SetPasswordResetToken(ctx, email, &service.PasswordResetTokenData{
		Token: token, CreatedAt: time.Now(),
	}, 30*time.Minute))
	svc := service.NewEmailService(nil, cache)
	const workers = 30
	var won atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if svc.ConsumePasswordResetToken(ctx, email, token) == nil {
				won.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), won.Load())
	require.ErrorIs(t, svc.ConsumePasswordResetToken(ctx, email, token), service.ErrInvalidResetToken)
	require.False(t, mr.Exists(passwordResetKey(email)))
}

func TestEmailCache_PasswordResetRejectsUnknownAndAmbiguousFormats(t *testing.T) {
	cache, mr, _ := newMiniredisEmailCache(t)
	ctx := context.Background()
	svc := service.NewEmailService(nil, cache)
	token := strings.Repeat("a", 64)
	sum := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(sum[:])
	for _, data := range []*service.PasswordResetTokenData{
		{Token: token, TokenFormat: "unknown"},
		{Token: token, TokenHash: hash},
		{TokenHash: hash},
		{Token: token, TokenHash: hash, TokenFormat: service.PasswordResetTokenFormatSHA256},
		{TokenHash: "invalid", TokenFormat: service.PasswordResetTokenFormatSHA256},
		{},
	} {
		t.Run(data.TokenFormat+"/"+data.TokenHash, func(t *testing.T) {
			require.NoError(t, cache.SetPasswordResetToken(ctx, "invalid@example.com", data, time.Minute))
			require.ErrorIs(t, svc.ConsumePasswordResetToken(ctx, "invalid@example.com", token), service.ErrInvalidResetToken)
			require.True(t, mr.Exists(passwordResetKey("invalid@example.com")))
		})
	}
}

type resetTokenReplacementCache struct {
	service.EmailCache
	replacement *service.PasswordResetTokenData
}

func (c *resetTokenReplacementCache) ConsumePasswordResetToken(ctx context.Context, email string, expected *service.PasswordResetTokenData) (bool, error) {
	if err := c.SetPasswordResetToken(ctx, email, c.replacement, time.Minute); err != nil {
		return false, err
	}
	return c.EmailCache.ConsumePasswordResetToken(ctx, email, expected)
}

func TestEmailCache_PasswordResetReplacementAfterVerificationIsPreserved(t *testing.T) {
	cache, _, _ := newMiniredisEmailCache(t)
	ctx := context.Background()
	email := "replacement@example.com"
	oldToken := strings.Repeat("a", 64)
	newToken := strings.Repeat("b", 64)
	old := &service.PasswordResetTokenData{Token: oldToken, CreatedAt: time.Now()}
	require.NoError(t, cache.SetPasswordResetToken(ctx, email, old, time.Minute))
	sum := sha256.Sum256([]byte(newToken))
	newData := &service.PasswordResetTokenData{
		TokenHash: hex.EncodeToString(sum[:]), TokenFormat: service.PasswordResetTokenFormatSHA256, CreatedAt: old.CreatedAt.Add(time.Second),
	}
	svc := service.NewEmailService(nil, &resetTokenReplacementCache{EmailCache: cache, replacement: newData})
	require.ErrorIs(t, svc.ConsumePasswordResetToken(ctx, email, oldToken), service.ErrInvalidResetToken)
	fresh := service.NewEmailService(nil, cache)
	require.NoError(t, fresh.ConsumePasswordResetToken(ctx, email, newToken))
}

func TestEmailCache_PasswordResetConsumptionChecksFormatAndIssueTime(t *testing.T) {
	cache, _, _ := newMiniredisEmailCache(t)
	ctx := context.Background()
	email := "snapshot@example.com"
	issuedAt := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	current := &service.PasswordResetTokenData{Token: "same-token", TokenFormat: service.PasswordResetTokenFormatLegacy, CreatedAt: issuedAt}
	require.NoError(t, cache.SetPasswordResetToken(ctx, email, current, time.Minute))
	for _, expected := range []*service.PasswordResetTokenData{
		{Token: "same-token", CreatedAt: issuedAt},
		{Token: "same-token", TokenFormat: service.PasswordResetTokenFormatLegacy, CreatedAt: issuedAt.Add(-time.Second)},
	} {
		ok, err := cache.ConsumePasswordResetToken(ctx, email, expected)
		require.NoError(t, err)
		require.False(t, ok)
	}
	ok, err := cache.ConsumePasswordResetToken(ctx, email, current)
	require.NoError(t, err)
	require.True(t, ok)
}
