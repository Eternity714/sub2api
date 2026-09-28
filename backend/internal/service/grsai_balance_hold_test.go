package service

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGrsaiVideoHoldQuotaAndRateUseSameSingleRequestAmount(t *testing.T) {
	id := "video-billing"
	record := &GrsaiSettlement{ID: 1, LocalTaskID: &id, UserID: 1, APIKeyID: 1, AccountID: 1, GroupID: 1, Model: "minimax-h3", Currency: "USD", BillingIdempotencyKey: "video-billing", MediaKind: "video", VideoDurationSeconds: 5, BaseUnitPrice: .14, GroupRateMultiplier: 1.5, AccountRateMultiplier: 1.2, BillableUnitPrice: 1.26, RequestedImageCount: 99}
	amount, err := GrsaiTaskHoldAmount(record)
	require.NoError(t, err)
	require.InDelta(t, 1.26, amount, 1e-9)
	cmd, err := GrsaiTaskUsageCommand(record)
	require.NoError(t, err)
	require.InDelta(t, amount, cmd.APIKeyQuotaCost, 1e-9)
	require.InDelta(t, amount, cmd.APIKeyRateLimitCost, 1e-9)
	require.InDelta(t, .84, cmd.AccountQuotaCost, 1e-9)
	require.Zero(t, cmd.BalanceCost)
}
