//go:build integration

package repository

import (
	"context"
	"sync"
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
	require.Error(t, err)
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
	require.Error(t, err, "the same account must not persist the same non-empty upstream task twice")
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
	cleanupGrsaiSettlements(t, params.BillingIdempotencyKey)
	record, err := repo.Create(ctx, params)
	require.NoError(t, err)

	taskID := uniqueTestValue(t, "grsai-task-bind")
	bound, err := repo.BindUpstreamTask(ctx, record.ID, taskID, "queued")
	require.NoError(t, err)
	require.True(t, bound)

	nextAttempt := time.Now().UTC().Add(time.Minute)
	updated, err := repo.UpdateResult(ctx, record.ID, "failed", "provider timeout", nextAttempt)
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

	require.NoError(t, repo.MarkPendingSettlement(ctx, settled.ID, now.Add(-time.Minute)))
	claimed, err := repo.ClaimDue(ctx, now, 1, now.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, settled.ID, claimed[0].ID)
	didSettle, err := repo.Settle(ctx, settled.ID, 0.02)
	require.NoError(t, err)
	require.True(t, didSettle)
	didSettle, err = repo.Settle(ctx, settled.ID, 0.02)
	require.NoError(t, err)
	require.False(t, didSettle, "settlement is idempotent after the terminal transition")

	require.NoError(t, repo.CloseNoCharge(ctx, free.ID, "no billable output"))
	require.NoError(t, repo.MarkManualReview(ctx, review.ID, "ambiguous provider result"))
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
	require.NoError(t, repo.MarkPendingSettlement(ctx, record.ID, now.Add(-time.Minute)))
	claimed, err := repo.ClaimDue(ctx, now, 1, now.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, record.ID, claimed[0].ID)

	start := make(chan struct{})
	results := make(chan bool, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			settled, settleErr := repo.Settle(ctx, record.ID, 0.02)
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
