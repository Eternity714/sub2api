//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGrsaiBalanceHoldCapturesOrReleasesOnce(t *testing.T) {
	for _, terminal := range []string{"succeeded", "failed"} {
		t.Run(terminal, func(t *testing.T) {
			ctx := context.Background()
			client := testEntClient(t)
			user := mustCreateUser(t, client, &service.User{
				Email:        fmt.Sprintf("grsai-hold-%d@example.com", time.Now().UnixNano()),
				PasswordHash: "hash", Balance: 10,
			})
			account := mustCreateAccount(t, client, &service.Account{
				Name: "grsai-hold-" + uuid.NewString(), Type: service.AccountTypeAPIKey,
			})
			apiKey := mustCreateApiKey(t, client, &service.APIKey{
				UserID: user.ID, Key: "sk-grsai-hold-" + uuid.NewString(), Name: "grsai-hold",
			})
			billing := &usageBillingRepository{db: integrationDB}
			repo := grsaiV2TestRepo(t)
			params := grsaiV2TestParams(t)
			params.UserID = user.ID
			params.APIKeyID = apiKey.ID
			params.AccountID = account.ID
			params.BillableUnitPrice = 0.25
			record, err := repo.CreateV2GrsaiTask(ctx, params, billing.ReserveGrsaiBalance)
			require.NoError(t, err)
			cleanupCreatedGrsaiSettlements(t, record)
			t.Cleanup(func() {
				_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM grsai_balance_holds WHERE local_task_id = $1`, params.LocalTaskID)
			})
			assertGrsaiBalance(t, user.ID, 9.5, 0.5)
			claimed, err := repo.ClaimDueV2(ctx, time.Now().UTC(), 1, time.Now().UTC().Add(time.Minute), 3)
			require.NoError(t, err)
			require.Len(t, claimed, 1)
			if terminal == "succeeded" {
				bound, bindErr := repo.BindV2UpstreamTask(ctx, record.ID, claimed[0].ClaimVersion, "provider-id")
				require.NoError(t, bindErr)
				require.True(t, bound)
				completed, completeErr := repo.CompleteV2(ctx, record.ID, claimed[0].ClaimVersion,
					[]byte(`{"images":[]}`), []byte(`[]`), nil, 0.5, billing.CaptureGrsaiBalanceTx)
				require.NoError(t, completeErr)
				require.True(t, completed)
				assertGrsaiBalance(t, user.ID, 9.5, 0)
				var quotaUsed, usage5h float64
				require.NoError(t, integrationDB.QueryRowContext(ctx,
					`SELECT quota_used, usage_5h FROM api_keys WHERE id = $1`, apiKey.ID).Scan(&quotaUsed, &usage5h))
				require.InDelta(t, 0.5, quotaUsed, 0.000001)
				require.InDelta(t, 0.5, usage5h, 0.000001)
			} else {
				failed, failErr := repo.FailV2(ctx, record.ID, claimed[0].ClaimVersion, "test", "", billing.ReleaseGrsaiBalanceTx)
				require.NoError(t, failErr)
				require.True(t, failed)
				assertGrsaiBalance(t, user.ID, 10, 0)
			}
			tx, err := integrationDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			var replay func(context.Context, *sql.Tx, *service.GrsaiSettlement) error
			var opposite func(context.Context, *sql.Tx, *service.GrsaiSettlement) error
			if terminal == "succeeded" {
				replay, opposite = billing.CaptureGrsaiBalanceTx, billing.ReleaseGrsaiBalanceTx
			} else {
				replay, opposite = billing.ReleaseGrsaiBalanceTx, billing.CaptureGrsaiBalanceTx
			}
			require.NoError(t, replay(ctx, tx, record))
			require.ErrorIs(t, opposite(ctx, tx, record), service.ErrGrsaiSettlementInvalidState)
			require.NoError(t, tx.Rollback())
			var markers int
			require.NoError(t, integrationDB.QueryRowContext(ctx,
				`SELECT count(*) FROM usage_billing_dedup WHERE request_id = $1 AND api_key_id = $2`,
				"grsai_task:"+params.LocalTaskID, apiKey.ID).Scan(&markers))
			if terminal == "succeeded" {
				require.Equal(t, 1, markers)
			} else {
				require.Zero(t, markers)
			}
		})
	}
}

func TestGrsaiBalanceHoldInsufficientFundsRollsBackTask(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("grsai-hold-low-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash", Balance: 0.1,
	})
	params := grsaiV2TestParams(t)
	params.UserID = user.ID
	params.APIKeyID = user.ID
	params.BillableUnitPrice = 0.25
	_, err := grsaiV2TestRepo(t).CreateV2GrsaiTask(ctx, params,
		(&usageBillingRepository{db: integrationDB}).ReserveGrsaiBalance)
	require.ErrorIs(t, err, service.ErrGrsaiInsufficientBalance)
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		`SELECT count(*) FROM grsai_settlements WHERE local_task_id = $1`, params.LocalTaskID).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		`SELECT count(*) FROM grsai_balance_holds WHERE local_task_id = $1`, params.LocalTaskID).Scan(&count))
	require.Zero(t, count)
	assertGrsaiBalance(t, user.ID, 0.1, 0)
}

func assertGrsaiBalance(t *testing.T, userID int64, wantBalance, wantFrozen float64) {
	t.Helper()
	var balance, frozen float64
	require.NoError(t, integrationDB.QueryRowContext(context.Background(),
		`SELECT balance, frozen_balance FROM users WHERE id = $1`, userID).Scan(&balance, &frozen))
	require.InDelta(t, wantBalance, balance, 0.000001)
	require.InDelta(t, wantFrozen, frozen, 0.000001)
}

// GRSAI-VIDEO-02/05/06: one request charge for three results, capture is idempotent.
func TestGrsaiVideoBalanceSingleRequestMultipleResults(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{Email: "video-" + uuid.NewString() + "@example.com", PasswordHash: "hash", Balance: 10})
	account := mustCreateAccount(t, client, &service.Account{Name: "video-" + uuid.NewString(), Type: service.AccountTypeAPIKey})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-video-" + uuid.NewString(), Name: "video"})
	repo := grsaiV2TestRepo(t)
	params := grsaiV2TestParams(t)
	params.UserID = user.ID
	params.APIKeyID = key.ID
	params.AccountID = account.ID
	params.RequestedImageCount = 1
	params.MediaKind = "video"
	params.TaskVersion = 3
	params.VideoDurationSeconds = 5
	params.VideoResolution = "768p"
	params.BaseUnitPrice = .14
	params.GroupRateMultiplier = 1.5
	params.AccountRateMultiplier = 1.2
	params.BillableUnitPrice = 1.26
	billing := &usageBillingRepository{db: integrationDB}
	record, err := repo.CreateV2GrsaiTask(ctx, params, billing.ReserveGrsaiBalance)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, record)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM grsai_balance_holds WHERE local_task_id=$1`, params.LocalTaskID)
	})
	assertGrsaiBalance(t, user.ID, 8.74, 1.26)
	claims, err := repo.ClaimDueV2(ctx, time.Now().UTC(), 1, time.Now().UTC().Add(time.Minute), 3)
	require.NoError(t, err)
	require.Len(t, claims, 1)
	bound, err := repo.BindV2UpstreamTask(ctx, record.ID, claims[0].ClaimVersion, "private-video")
	require.NoError(t, err)
	require.True(t, bound)
	result := []byte(`{"results":[{"url":"https://local/a.mp4"},{"url":"https://local/b.mp4"},{"url":"https://local/c.mp4"}]}`)
	done, err := repo.CompleteV2(ctx, record.ID, claims[0].ClaimVersion, result, []byte(`[]`), nil, 1.26, billing.CaptureGrsaiBalanceTx)
	require.NoError(t, err)
	require.True(t, done)
	assertGrsaiBalance(t, user.ID, 8.74, 0)
	var quota, rate float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT quota_used,usage_5h FROM api_keys WHERE id=$1`, key.ID).Scan(&quota, &rate))
	require.InDelta(t, 1.26, quota, 1e-8)
	require.InDelta(t, 1.26, rate, 1e-8)
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, billing.CaptureGrsaiBalanceTx(ctx, tx, record))
	require.NoError(t, tx.Commit())
	assertGrsaiBalance(t, user.ID, 8.74, 0)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT quota_used FROM api_keys WHERE id=$1`, key.ID).Scan(&quota))
	require.InDelta(t, 1.26, quota, 1e-8)
	owned, err := repo.GetOwnedV2(ctx, user.ID, key.ID, params.LocalTaskID)
	require.NoError(t, err)
	require.Equal(t, "video", owned.MediaKind)
	require.Equal(t, "succeeded", owned.PublicStatus)
	_, err = repo.GetOwnedV2(ctx, user.ID, key.ID+100000, params.LocalTaskID)
	require.ErrorIs(t, err, service.ErrGrsaiSettlementNotFound)
}
