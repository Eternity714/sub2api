//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
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
			billing := &usageBillingRepository{db: integrationDB}
			repo := grsaiV2TestRepo(t)
			params := grsaiV2TestParams(t)
			params.UserID = user.ID
			params.APIKeyID = user.ID
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
