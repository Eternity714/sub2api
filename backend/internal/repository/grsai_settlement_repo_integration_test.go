//go:build integration

package repository

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestGrsaiSettlementRepository_UniqueBillingIdempotencyKey(t *testing.T) {
	ctx := context.Background()
	repo := NewGrsaiSettlementRepository(integrationDB)
	first := grsaiSettlementTestParams(t, "billing-key")
	second := grsaiSettlementTestParams(t, "billing-key-duplicate")
	second.BillingIdempotencyKey = first.BillingIdempotencyKey
	cleanupGrsaiSettlements(t, first.BillingIdempotencyKey)

	_, err := repo.Create(ctx, first)
	require.NoError(t, err)
	_, err = repo.Create(ctx, second)
	requirePostgresUniqueViolation(t, err, "grsai_settlements_billing_idempotency_key_uq")
}

func TestGrsaiSettlementRepository_DeduplicatesNonEmptyTaskWithinAccount(t *testing.T) {
	ctx := context.Background()
	repo := NewGrsaiSettlementRepository(integrationDB)
	first := grsaiSettlementTestParams(t, "task-dedup-a")
	second := grsaiSettlementTestParams(t, "task-dedup-b")
	otherAccount := grsaiSettlementTestParams(t, "task-dedup-other-account")
	taskID := uniqueTestValue(t, "grsai-task")
	first.UpstreamTaskID = &taskID
	second.UpstreamTaskID = &taskID
	otherAccount.UpstreamTaskID = &taskID
	otherAccount.AccountID = first.AccountID + 1
	cleanupGrsaiSettlements(t, first.BillingIdempotencyKey, second.BillingIdempotencyKey, otherAccount.BillingIdempotencyKey)

	_, err := repo.Create(ctx, first)
	require.NoError(t, err)
	_, err = repo.Create(ctx, second)
	requirePostgresUniqueViolation(t, err, "grsai_settlements_account_upstream_task_uq")
	_, err = repo.Create(ctx, otherAccount)
	require.NoError(t, err, "different accounts may receive the same provider task identifier")
}

func TestGrsaiSettlementRepository_AllowsMultipleEmptyTaskIDs(t *testing.T) {
	ctx := context.Background()
	repo := NewGrsaiSettlementRepository(integrationDB)
	first := grsaiSettlementTestParams(t, "null-task-a")
	second := grsaiSettlementTestParams(t, "null-task-b")
	third := grsaiSettlementTestParams(t, "null-task-c")
	emptyTaskID := ""
	first.UpstreamTaskID = &emptyTaskID
	second.UpstreamTaskID = &emptyTaskID
	second.AccountID = first.AccountID
	third.AccountID = first.AccountID
	cleanupGrsaiSettlements(t, first.BillingIdempotencyKey, second.BillingIdempotencyKey, third.BillingIdempotencyKey)

	_, err := repo.Create(ctx, first)
	require.NoError(t, err)
	_, err = repo.Create(ctx, second)
	require.NoError(t, err)
	_, err = repo.Create(ctx, third)
	require.NoError(t, err)
}

func TestGrsaiSettlementRepository_BindAndUpdateSanitizedResult(t *testing.T) {
	ctx := context.Background()
	repo := NewGrsaiSettlementRepository(integrationDB)
	params := grsaiSettlementTestParams(t, "bind-result")
	params.NextAttemptAt = time.Now().UTC().Add(-time.Minute)
	cleanupGrsaiSettlements(t, params.BillingIdempotencyKey)
	record, err := repo.Create(ctx, params)
	require.NoError(t, err)
	claimed, err := repo.ClaimDue(ctx, time.Now().UTC(), 1, time.Now().UTC().Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, record.ID, claimed[0].ID)

	taskID := uniqueTestValue(t, "grsai-task-bind")
	bound, err := repo.BindUpstreamTask(ctx, record.ID, claimed[0].ClaimVersion, taskID, "queued")
	require.NoError(t, err)
	require.True(t, bound)

	nextAttempt := time.Now().UTC().Add(time.Minute)
	updated, err := repo.UpdateResult(ctx, record.ID, claimed[0].ClaimVersion, "failed", "provider timeout", nextAttempt)
	require.NoError(t, err)
	require.True(t, updated)
	got, err := repo.GetByID(ctx, record.ID)
	require.NoError(t, err)
	require.NotNil(t, got.UpstreamTaskID)
	require.Equal(t, taskID, *got.UpstreamTaskID)
	require.Equal(t, "failed", got.UpstreamStatus)
	require.NotNil(t, got.LastErrorSummary)
	require.Equal(t, "provider timeout", *got.LastErrorSummary)
	require.WithinDuration(t, nextAttempt, got.NextAttemptAt, time.Millisecond)
}

func TestGrsaiSettlementRepository_ConcurrentClaimHasSingleWinner(t *testing.T) {
	ctx := context.Background()
	repo := NewGrsaiSettlementRepository(integrationDB)
	params := grsaiSettlementTestParams(t, "claim-single-winner")
	params.NextAttemptAt = time.Now().UTC().Add(-time.Minute)
	cleanupGrsaiSettlements(t, params.BillingIdempotencyKey)
	created, err := repo.Create(ctx, params)
	require.NoError(t, err)

	start := make(chan struct{})
	results := make(chan []*GrsaiSettlement, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			claimed, claimErr := repo.ClaimDue(ctx, time.Now().UTC(), 1, time.Now().UTC().Add(time.Minute))
			results <- claimed
			errs <- claimErr
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)

	for claimErr := range errs {
		require.NoError(t, claimErr)
	}
	winners := 0
	for claimed := range results {
		for _, record := range claimed {
			if record.ID == created.ID {
				winners++
			}
		}
	}
	require.Equal(t, 1, winners)
}

func TestGrsaiSettlementRepository_TerminalRecordsAreNotClaimed(t *testing.T) {
	ctx := context.Background()
	repo := NewGrsaiSettlementRepository(integrationDB)
	now := time.Now().UTC()

	settledParams := grsaiSettlementTestParams(t, "terminal-settled")
	settledParams.NextAttemptAt = now.Add(-time.Minute)
	freeParams := grsaiSettlementTestParams(t, "terminal-free")
	reviewParams := grsaiSettlementTestParams(t, "terminal-review")
	cleanupGrsaiSettlements(t,
		settledParams.BillingIdempotencyKey,
		freeParams.BillingIdempotencyKey,
		reviewParams.BillingIdempotencyKey,
	)
	settled, err := repo.Create(ctx, settledParams)
	require.NoError(t, err)
	free, err := repo.Create(ctx, freeParams)
	require.NoError(t, err)
	review, err := repo.Create(ctx, reviewParams)
	require.NoError(t, err)

	claimed, err := repo.ClaimDue(ctx, now, 1, now.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, settled.ID, claimed[0].ID)
	settledClaim := claimed[0]
	require.NoError(t, repo.MarkPendingSettlement(ctx, settled.ID, settledClaim.ClaimVersion, now.Add(-time.Minute)))
	claimed, err = repo.ClaimDue(ctx, now, 1, now.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, settled.ID, claimed[0].ID)
	require.Greater(t, claimed[0].ClaimVersion, settledClaim.ClaimVersion)
	settledClaim = claimed[0]
	didSettle, err := repo.Settle(ctx, settled.ID, settledClaim.ClaimVersion, 0.02, noOpGrsaiSettlementTx)
	require.NoError(t, err)
	require.True(t, didSettle)
	didSettle, err = repo.Settle(ctx, settled.ID, settledClaim.ClaimVersion, 0.02, noOpGrsaiSettlementTx)
	require.NoError(t, err)
	require.False(t, didSettle, "settlement is idempotent after the terminal transition")

	due, err := repo.ClaimDue(ctx, now.Add(25*time.Hour), 2, now.Add(26*time.Hour))
	require.NoError(t, err)
	require.Len(t, due, 2)
	claimByID := map[int64]*GrsaiSettlement{due[0].ID: due[0], due[1].ID: due[1]}
	require.Contains(t, claimByID, free.ID)
	require.Contains(t, claimByID, review.ID)
	require.NoError(t, repo.CloseNoCharge(ctx, free.ID, claimByID[free.ID].ClaimVersion, "no billable output"))
	require.NoError(t, repo.MarkManualReview(ctx, review.ID, claimByID[review.ID].ClaimVersion, "ambiguous provider result"))
	claimed, err = repo.ClaimDue(ctx, now.Add(time.Hour), 10, now.Add(2*time.Hour))
	require.NoError(t, err)
	for _, record := range claimed {
		require.NotContains(t, []int64{settled.ID, free.ID, review.ID}, record.ID)
	}
}

func TestGrsaiSettlementRepository_ConcurrentSettleHasSingleWinner(t *testing.T) {
	ctx := context.Background()
	repo := NewGrsaiSettlementRepository(integrationDB)
	now := time.Now().UTC()
	params := grsaiSettlementTestParams(t, "settle-single-winner")
	params.NextAttemptAt = now.Add(-time.Minute)
	cleanupGrsaiSettlements(t, params.BillingIdempotencyKey)
	record, err := repo.Create(ctx, params)
	require.NoError(t, err)
	claimed, err := repo.ClaimDue(ctx, now, 1, now.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, record.ID, claimed[0].ID)

	start := make(chan struct{})
	results := make(chan bool, 2)
	errs := make(chan error, 2)
	var callbacks atomic.Int32
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			settled, settleErr := repo.Settle(ctx, record.ID, claimed[0].ClaimVersion, 0.02,
				func(context.Context, *sql.Tx, *GrsaiSettlement) error {
					callbacks.Add(1)
					return nil
				})
			results <- settled
			errs <- settleErr
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)

	for settleErr := range errs {
		require.NoError(t, settleErr)
	}
	winners := 0
	for settled := range results {
		if settled {
			winners++
		}
	}
	require.Equal(t, 1, winners)
	require.Equal(t, int32(1), callbacks.Load(), "the billing callback must run for only the winning settlement")
}

func TestGrsaiSettlementRepository_RejectsExpiredClaimHolder(t *testing.T) {
	tests := []struct {
		name string
		run  func(*testing.T, context.Context, *grsaiSettlementRepository, int64, int64, int64)
	}{
		{
			name: "update_result",
			run: func(t *testing.T, ctx context.Context, repo *grsaiSettlementRepository, id, staleVersion, currentVersion int64) {
				updated, err := repo.UpdateResult(ctx, id, staleVersion, "failed", "stale", time.Now().UTC().Add(time.Minute))
				require.False(t, updated)
				require.ErrorIs(t, err, ErrGrsaiSettlementClaimLost)
				updated, err = repo.UpdateResult(ctx, id, currentVersion, "running", "", time.Now().UTC().Add(time.Minute))
				require.NoError(t, err)
				require.True(t, updated)
			},
		},
		{
			name: "close_no_charge",
			run: func(t *testing.T, ctx context.Context, repo *grsaiSettlementRepository, id, staleVersion, currentVersion int64) {
				require.ErrorIs(t, repo.CloseNoCharge(ctx, id, staleVersion, "stale"), ErrGrsaiSettlementClaimLost)
				require.NoError(t, repo.CloseNoCharge(ctx, id, currentVersion, "no billable output"))
			},
		},
		{
			name: "settle",
			run: func(t *testing.T, ctx context.Context, repo *grsaiSettlementRepository, id, staleVersion, currentVersion int64) {
				var callbacks atomic.Int32
				settled, err := repo.Settle(ctx, id, staleVersion, 0.02, func(context.Context, *sql.Tx, *GrsaiSettlement) error {
					callbacks.Add(1)
					return nil
				})
				require.False(t, settled)
				require.ErrorIs(t, err, ErrGrsaiSettlementClaimLost)
				require.Zero(t, callbacks.Load())
				settled, err = repo.Settle(ctx, id, currentVersion, 0.02, func(context.Context, *sql.Tx, *GrsaiSettlement) error {
					callbacks.Add(1)
					return nil
				})
				require.NoError(t, err)
				require.True(t, settled)
				require.Equal(t, int32(1), callbacks.Load())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := NewGrsaiSettlementRepository(integrationDB)
			now := time.Now().UTC()
			params := grsaiSettlementTestParams(t, "expired-claim-"+tt.name)
			params.NextAttemptAt = now.Add(-time.Minute)
			cleanupGrsaiSettlements(t, params.BillingIdempotencyKey)
			record, err := repo.Create(ctx, params)
			require.NoError(t, err)

			first, err := repo.ClaimDue(ctx, now, 1, now.Add(time.Minute))
			require.NoError(t, err)
			require.Len(t, first, 1)
			require.Equal(t, record.ID, first[0].ID)
			second, err := repo.ClaimDue(ctx, now.Add(2*time.Minute), 1, now.Add(3*time.Minute))
			require.NoError(t, err)
			require.Len(t, second, 1)
			require.Equal(t, record.ID, second[0].ID)
			require.Greater(t, second[0].ClaimVersion, first[0].ClaimVersion)

			tt.run(t, ctx, repo, record.ID, first[0].ClaimVersion, second[0].ClaimVersion)
		})
	}
}

func TestGrsaiSettlementRepository_SettleCommitsBillingAndTerminalStateAtomically(t *testing.T) {
	ctx := context.Background()
	repo := NewGrsaiSettlementRepository(integrationDB)
	now := time.Now().UTC()
	params := grsaiSettlementTestParams(t, "atomic-settle")
	params.NextAttemptAt = now.Add(-time.Minute)
	cleanupGrsaiSettlements(t, params.BillingIdempotencyKey)
	cleanupGrsaiBillingMarker(t, params.BillingIdempotencyKey, params.APIKeyID)
	record, err := repo.Create(ctx, params)
	require.NoError(t, err)
	claimed, err := repo.ClaimDue(ctx, now, 1, now.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	fingerprint := hashedTestValue(t, "grsai-atomic-settle")

	billingErr := errors.New("billing failed")
	settled, err := repo.Settle(ctx, record.ID, claimed[0].ClaimVersion, 0.02,
		func(ctx context.Context, tx *sql.Tx, locked *GrsaiSettlement) error {
			require.Equal(t, params.BillingIdempotencyKey, locked.BillingIdempotencyKey)
			_, insertErr := tx.ExecContext(ctx, `
INSERT INTO usage_billing_dedup (request_id, api_key_id, request_fingerprint)
VALUES ($1, $2, $3)`, locked.BillingIdempotencyKey, locked.APIKeyID, fingerprint)
			require.NoError(t, insertErr)
			return billingErr
		})
	require.False(t, settled)
	require.ErrorIs(t, err, billingErr)
	require.Equal(t, 0, countGrsaiBillingMarkers(t, params.BillingIdempotencyKey, params.APIKeyID))
	got, err := repo.GetByID(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, GrsaiSettlementStatusProcessing, got.InternalStatus)

	settled, err = repo.Settle(ctx, record.ID, claimed[0].ClaimVersion, 0.02,
		func(ctx context.Context, tx *sql.Tx, locked *GrsaiSettlement) error {
			_, insertErr := tx.ExecContext(ctx, `
INSERT INTO usage_billing_dedup (request_id, api_key_id, request_fingerprint)
VALUES ($1, $2, $3)`, locked.BillingIdempotencyKey, locked.APIKeyID, fingerprint)
			return insertErr
		})
	require.NoError(t, err)
	require.True(t, settled)
	require.Equal(t, 1, countGrsaiBillingMarkers(t, params.BillingIdempotencyKey, params.APIKeyID))
	got, err = repo.GetByID(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, GrsaiSettlementStatusSettled, got.InternalStatus)
	require.NotNil(t, got.SettledAmount)
	require.InDelta(t, 0.02, *got.SettledAmount, 0.0000000001)
}

func TestGrsaiSettlementRepository_RejectsNonFinitePrices(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CreateGrsaiSettlementParams)
	}{
		{name: "nan_base", mutate: func(p *CreateGrsaiSettlementParams) { p.BaseUnitPrice = math.NaN() }},
		{name: "positive_infinite_group_rate", mutate: func(p *CreateGrsaiSettlementParams) { p.GroupRateMultiplier = math.Inf(1) }},
		{name: "negative_infinite_account_rate", mutate: func(p *CreateGrsaiSettlementParams) { p.AccountRateMultiplier = math.Inf(-1) }},
		{name: "nan_billable", mutate: func(p *CreateGrsaiSettlementParams) { p.BillableUnitPrice = math.NaN() }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := grsaiSettlementTestParams(t, "non-finite-"+tt.name)
			tt.mutate(&params)
			_, err := NewGrsaiSettlementRepository(integrationDB).Create(context.Background(), params)
			require.ErrorIs(t, err, ErrGrsaiSettlementInvalidInput)
		})
	}
}

func TestGrsaiSettlementRepository_RejectsNonFiniteSettledAmount(t *testing.T) {
	ctx := context.Background()
	repo := NewGrsaiSettlementRepository(integrationDB)
	now := time.Now().UTC()
	params := grsaiSettlementTestParams(t, "non-finite-settle")
	params.NextAttemptAt = now.Add(-time.Minute)
	cleanupGrsaiSettlements(t, params.BillingIdempotencyKey)
	record, err := repo.Create(ctx, params)
	require.NoError(t, err)
	claimed, err := repo.ClaimDue(ctx, now, 1, now.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	for _, amount := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		called := false
		settled, settleErr := repo.Settle(ctx, record.ID, claimed[0].ClaimVersion, amount,
			func(context.Context, *sql.Tx, *GrsaiSettlement) error {
				called = true
				return nil
			})
		require.False(t, settled)
		require.ErrorIs(t, settleErr, ErrGrsaiSettlementInvalidInput)
		require.False(t, called)
	}
}

func grsaiSettlementTestParams(t *testing.T, suffix string) CreateGrsaiSettlementParams {
	t.Helper()
	return CreateGrsaiSettlementParams{
		AccountID:            3001,
		GroupID:              4001,
		UserID:               1001,
		APIKeyID:             2001,
		Model:                "grsai-image",
		BaseUnitPrice:         0.01,
		GroupRateMultiplier:   1,
		AccountRateMultiplier: 1,
		BillableUnitPrice:     0.01,
		RequestedImageCount:   2,
		Currency:              "USD",
		BillingIdempotencyKey: uniqueTestValue(t, "grsai-billing-"+suffix),
		UpstreamStatus:        "not_submitted",
		NextAttemptAt:         time.Now().UTC().Add(24 * time.Hour),
	}
}

func cleanupGrsaiSettlements(t *testing.T, keys ...string) {
	t.Helper()
	_, err := integrationDB.ExecContext(context.Background(),
		`DELETE FROM grsai_settlements WHERE billing_idempotency_key = ANY($1)`, pq.Array(keys))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, cleanupErr := integrationDB.ExecContext(context.Background(),
			`DELETE FROM grsai_settlements WHERE billing_idempotency_key = ANY($1)`, pq.Array(keys))
		require.NoError(t, cleanupErr)
	})
}

func cleanupGrsaiBillingMarker(t *testing.T, requestID string, apiKeyID int64) {
	t.Helper()
	deleteMarker := func() {
		_, err := integrationDB.ExecContext(context.Background(),
			`DELETE FROM usage_billing_dedup WHERE request_id = $1 AND api_key_id = $2`, requestID, apiKeyID)
		require.NoError(t, err)
	}
	deleteMarker()
	t.Cleanup(deleteMarker)
}

func countGrsaiBillingMarkers(t *testing.T, requestID string, apiKeyID int64) int {
	t.Helper()
	var count int
	err := integrationDB.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id = $1 AND api_key_id = $2`, requestID, apiKeyID).Scan(&count)
	require.NoError(t, err)
	return count
}

func noOpGrsaiSettlementTx(context.Context, *sql.Tx, *GrsaiSettlement) error {
	return nil
}

func requirePostgresUniqueViolation(t *testing.T, err error, constraint string) {
	t.Helper()
	require.Error(t, err)
	var pqErr *pq.Error
	require.ErrorAs(t, err, &pqErr)
	require.Equal(t, pq.ErrorCode("23505"), pqErr.Code)
	require.Equal(t, constraint, pqErr.Constraint)
}
