//go:build unit

package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type resetTokenCacheStub struct {
	emailCacheStub
	stored       *PasswordResetTokenData
	consumedHash string
}

func (s *resetTokenCacheStub) GetPasswordResetToken(context.Context, string) (*PasswordResetTokenData, error) {
	return s.stored, nil
}

func (s *resetTokenCacheStub) SetPasswordResetToken(_ context.Context, _ string, data *PasswordResetTokenData, _ time.Duration) error {
	copy := *data
	s.stored = &copy
	return nil
}

func (s *resetTokenCacheStub) ConsumePasswordResetToken(_ context.Context, _ string, expected *PasswordResetTokenData) (bool, error) {
	s.consumedHash = expected.TokenHash
	if s.stored == nil || *s.stored != *expected {
		return false, nil
	}
	s.stored = nil
	return true, nil
}

func TestSendPasswordResetEmailReadsLegacyCompatAtEachWrite(t *testing.T) {
	ctx := context.Background()
	smtpServer := startNotificationEmailTestSMTPServer(t)
	repo := &settingRepoStub{values: smtpServer.settings()}
	cache := &resetTokenCacheStub{}
	svc := NewEmailService(repo, cache)
	const key = "password_reset_token_legacy_compat"
	for _, legacy := range []bool{false, true, false} {
		if legacy {
			repo.values[key] = "true"
		} else {
			delete(repo.values, key)
		}
		require.NoError(t, svc.SendPasswordResetEmail(ctx, "a@b.c", "test", "https://example.com/reset-password"))
		matches := regexp.MustCompile(`token=([a-f0-9]{64})`).FindStringSubmatch(smtpServer.lastMessageBody(t))
		require.Len(t, matches, 2)
		token := matches[1]
		raw, err := json.Marshal(cache.stored)
		require.NoError(t, err)
		var fields map[string]any
		require.NoError(t, json.Unmarshal(raw, &fields))
		if legacy {
			// This is the exact Token field read by the old stable binary.
			require.Equal(t, token, cache.stored.Token)
			require.Equal(t, "legacy", fields["TokenFormat"])
			require.Empty(t, fields["TokenHash"])
		} else {
			sum := sha256.Sum256([]byte(token))
			require.Empty(t, cache.stored.Token)
			require.Equal(t, "sha256", fields["TokenFormat"])
			require.Equal(t, hex.EncodeToString(sum[:]), fields["TokenHash"])
			require.NotContains(t, string(raw), token)
		}
	}
}

func TestSendPasswordResetEmailCompatReadErrorDoesNotWriteToken(t *testing.T) {
	cache := &resetTokenCacheStub{}
	svc := NewEmailService(&settingRepoStub{err: errors.New("settings unavailable")}, cache)
	require.Error(t, svc.SendPasswordResetEmail(context.Background(), "a@b.c", "test", "https://example.com/reset-password"))
	require.Nil(t, cache.stored)
}

func TestConsumePasswordResetToken_ComparesHashNotPlaintext(t *testing.T) {
	token := "deadbeef"
	sum := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(sum[:])
	require.Equal(t, hash, hashPasswordResetToken(token))
	require.NotEqual(t, token, hashPasswordResetToken(token))

	cache := &resetTokenCacheStub{stored: &PasswordResetTokenData{TokenHash: hash, TokenFormat: PasswordResetTokenFormatSHA256}}
	svc := NewEmailService(nil, cache)

	require.ErrorIs(t, svc.ConsumePasswordResetToken(context.Background(), "a@b.c", hash), ErrInvalidResetToken)
	require.NoError(t, svc.ConsumePasswordResetToken(context.Background(), "a@b.c", token))
	require.Equal(t, hash, cache.consumedHash)
	require.ErrorIs(t, svc.ConsumePasswordResetToken(context.Background(), "a@b.c", token), ErrInvalidResetToken)

	// Links issued by the stable binary remain usable after the bridge is off.
	cache.stored = &PasswordResetTokenData{Token: token}
	require.NoError(t, svc.ConsumePasswordResetToken(context.Background(), "a@b.c", token))
}
