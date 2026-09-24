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

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestGrsaiSettlementRepository_DerivesUniqueBillingIdempotencyKeyFromSettlementID(t *testing.T) {
	ctx := context.Background()
	repo := NewGrsaiSettlementRepository(integrationDB)
	first := grsaiSettlementTestParams(t, "billing-key")
	second := grsaiSettlementTestParams(t, "billing-key-duplicate")
	second.BillingIdempotencyKey = first.BillingIdempotencyKey

	firstRecord, err := repo.Create(ctx, first)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, firstRecord)
	secondRecord, err := repo.Create(ctx, second)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, secondRecord)
	require.Equal(t, service.GrsaiSettlementRequestID(firstRecord.ID), firstRecord.BillingIdempotencyKey)
	require.Equal(t, service.GrsaiSettlementRequestID(secondRecord.ID), secondRecord.BillingIdempotencyKey)
	require.NotEqual(t, firstRecord.BillingIdempotencyKey, secondRecord.BillingIdempotencyKey)
}

func TestGrsaiDeliveryCleanupOnlyExpiredSafeTerminalRows(t *testing.T) {
	ctx := context.Background()
	repo := NewGrsaiSettlementRepository(integrationDB)
	now := time.Now().UTC()
	cutoff := now.Add(-24 * time.Hour)
	type row struct {
		mode, state, hold string
		closed            time.Time
		created, expires  time.Time
		wantDeleted       bool
	}
	cases := []row{
		{mode: "async", state: "settled", hold: "captured", closed: cutoff.Add(-time.Second), wantDeleted: true},
		{mode: "json", state: "closed_no_charge", hold: "released", closed: cutoff.Add(-time.Second), wantDeleted: true},
		{mode: "stream", state: "settled", hold: "none", closed: cutoff.Add(-time.Second), wantDeleted: true},
		{mode: "async", state: "settled", hold: "captured", closed: cutoff.Add(time.Second)},
		{mode: "async", state: "pending_settlement", hold: "held", closed: cutoff.Add(-time.Second)},
		{mode: "async", state: "manual_review", hold: "released", closed: cutoff.Add(-time.Second)},
		{mode: "async", state: "closed_no_charge", hold: "held", closed: cutoff.Add(-time.Second)},
		{mode: "async", state: "settled", hold: "captured", closed: cutoff.Add(-time.Second), created: now.Add(-48 * time.Hour), expires: now.Add(120 * time.Hour)},
		{mode: "async", state: "settled", hold: "captured", closed: now.Add(-2 * time.Hour), created: now.Add(-48 * time.Hour), expires: now.Add(-47 * time.Hour), wantDeleted: true},
	}
	ids := make([]int64, 0, len(cases))
	for i, tc := range cases {
		params := grsaiSettlementTestParams(t, "cleanup")
		params.DeliveryMode = service.GrsaiDeliveryMode(tc.mode)
		record, err := repo.Create(ctx, params)
		require.NoError(t, err)
		cleanupCreatedGrsaiSettlements(t, record)
		ids = append(ids, record.ID)
		if tc.created.IsZero() {
			_, err = integrationDB.ExecContext(ctx, `UPDATE grsai_settlements SET internal_status = $2, hold_state = $3, closed_at = $4 WHERE id = $1`, record.ID, tc.state, tc.hold, tc.closed)
		} else {
			_, err = integrationDB.ExecContext(ctx, `UPDATE grsai_settlements SET internal_status = $2, hold_state = $3, closed_at = $4, created_at = $5, expires_at = $6 WHERE id = $1`, record.ID, tc.state, tc.hold, tc.closed, tc.created, tc.expires)
		}
		require.NoError(t, err, "case %d", i)
	}
	deleted, err := repo.DeleteTerminal(ctx, now, 24*time.Hour, 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, deleted, int64(4))
	for i, id := range ids {
		var count int
		require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM grsai_settlements WHERE id = $1`, id).Scan(&count))
		if cases[i].wantDeleted {
			require.Zero(t, count, "case %d", i)
		} else {
			require.Equal(t, 1, count, "case %d", i)
		}
	}
}

func TestGrsaiDeliveryCleanupRemovesBoundPayloadBeforeTTLButKeepsQueue(t *testing.T) {
	ctx := context.Background()
	repo := NewGrsaiSettlementRepository(integrationDB)
	payloads := NewGrsaiTaskPayloadRepository(integrationDB, testSecretEncryptor{})
	now := time.Now().UTC()
	ids := make([]int64, 0, 2)
	for i := 0; i < 2; i++ {
		params := grsaiSettlementTestParams(t, "cleanup-payload")
		params.DeliveryMode = service.GrsaiDeliveryAsync
		record, err := repo.Create(ctx, params)
		require.NoError(t, err)
		cleanupCreatedGrsaiSettlements(t, record)
		ids = append(ids, record.ID)
		require.NoError(t, payloads.PutEncrypted(ctx, record.ID, []byte(`{"prompt":"private"}`), now.Add(time.Hour)))
	}
	_, err := integrationDB.ExecContext(ctx, `UPDATE grsai_settlements SET upstream_task_id = $2 WHERE id = $1`, ids[1], "bound-cleanup-task")
	require.NoError(t, err)
	_, err = payloads.DeleteExpired(ctx, now)
	require.NoError(t, err)
	_, err = payloads.GetEncrypted(ctx, ids[0])
	require.NoError(t, err)
	_, err = payloads.GetEncrypted(ctx, ids[1])
	require.ErrorIs(t, err, service.ErrGrsaiTaskPayloadNotFound)
}

func TestGrsaiDeliveryCleanupKeepsExpiredSubmittingPayloadForUnsentFenceRollback(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	params := grsaiSettlementTestParams(t, "submitting-payload")
	params.DeliveryMode = service.GrsaiDeliveryAsync
	repo := NewGrsaiSettlementRepository(integrationDB)
	record, err := repo.Create(ctx, params)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, record)
	payloads := NewGrsaiTaskPayloadRepository(integrationDB, testSecretEncryptor{})
	require.NoError(t, payloads.PutEncrypted(ctx, record.ID, []byte(`{"model":"m","replyType":"async"}`), now.Add(-time.Hour)))
	_, err = integrationDB.ExecContext(ctx, `UPDATE grsai_settlements SET internal_status = 'processing', upstream_status = 'submitting', claim_version = 1 WHERE id = $1`, record.ID)
	require.NoError(t, err)
	_, err = payloads.DeleteExpired(ctx, now)
	require.NoError(t, err)
	require.NoError(t, repo.DeferUnsentSubmission(ctx, record.ID, 1, now.Add(time.Minute)))
	got, err := payloads.GetEncrypted(ctx, record.ID)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"m","replyType":"async"}`, string(got))
}

// @covers AC-009 AC-011
func TestGrsaiSettlementRepository_AsyncWaitingLimitAcrossConcurrentKeys(t *testing.T) {
	ctx := context.Background()
	repo := NewGrsaiSettlementRepository(integrationDB)
	userID := time.Now().UnixNano()
	start := make(chan struct{})
	results := make(chan *GrsaiSettlement, 25)
	errs := make(chan error, 25)
	var wg sync.WaitGroup
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			params := grsaiSettlementTestParams(t, "async-admission")
			params.UserID = userID
			params.APIKeyID += int64(i % 2)
			params.DeliveryMode = service.GrsaiDeliveryAsync
			params.AsyncWaitingLimit = 20
			record, err := repo.Create(ctx, params)
			if err == nil {
				results <- record
			} else {
				errs <- err
			}
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	accepted := 0
	for record := range results {
		cleanupCreatedGrsaiSettlements(t, record)
		accepted++
	}
	require.Equal(t, 20, accepted)
	rejected := 0
	for err := range errs {
		require.ErrorIs(t, err, service.ErrGrsaiAsyncQueueFull)
		rejected++
	}
	require.Equal(t, 5, rejected)
}

// @covers AC-010 AC-011
func TestGrsaiSettlementRepository_AsyncRunningLimitAndLeaseRecovery(t *testing.T) {
	ctx := context.Background()
	repo := NewGrsaiSettlementRepository(integrationDB)
	userID := time.Now().UnixNano()
	now := time.Now().UTC()
	for i := 0; i < 4; i++ {
		params := grsaiSettlementTestParams(t, "async-running")
		params.UserID = userID
		params.APIKeyID += int64(i % 2)
		params.DeliveryMode = service.GrsaiDeliveryAsync
		params.AsyncWaitingLimit = 20
		params.NextAttemptAt = now.Add(-time.Minute)
		record, err := repo.Create(ctx, params)
		require.NoError(t, err)
		cleanupCreatedGrsaiSettlements(t, record)
	}
	start := make(chan struct{})
	results := make(chan []*GrsaiSettlement, 4)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			claims, err := repo.ClaimDueForDeliveryMode(ctx, now, 1, now.Add(time.Hour), service.GrsaiDeliveryAsync)
			if err != nil {
				results <- nil
				return
			}
			results <- claims
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	active := make([]*GrsaiSettlement, 0, 3)
	for claims := range results {
		active = append(active, claims...)
	}
	require.Len(t, active, 3)
	var waiting, running int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FILTER (WHERE async_started_at IS NULL), COUNT(*) FILTER (WHERE async_started_at IS NOT NULL) FROM grsai_settlements WHERE user_id = $1 AND delivery_mode = 'async'`, userID).Scan(&waiting, &running))
	require.Equal(t, 1, waiting)
	require.Equal(t, 3, running)
	// An expired lease may be reclaimed without consuming a fourth slot.
	recovered, err := repo.ClaimDueForDeliveryMode(ctx, now.Add(2*time.Hour), 1, now.Add(3*time.Hour), service.GrsaiDeliveryAsync)
	require.NoError(t, err)
	require.Len(t, recovered, 1)
	require.Contains(t, []int64{active[0].ID, active[1].ID, active[2].ID}, recovered[0].ID)
	require.NoError(t, repo.CloseNoCharge(ctx, recovered[0].ID, recovered[0].ClaimVersion, "test closed"))
	claimed, err := repo.ClaimDueForDeliveryMode(ctx, now.Add(time.Second), 1, now.Add(time.Hour), service.GrsaiDeliveryAsync)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.NotContains(t, []int64{active[0].ID, active[1].ID, active[2].ID}, claimed[0].ID)
}

func TestGrsaiSettlementRepository_UnsentSubmissionFenceCanBeRearmedByOwner(t *testing.T) {
	ctx := context.Background()
	repo := NewGrsaiSettlementRepository(integrationDB)
	params := grsaiSettlementTestParams(t, "unsent-fence-rearm")
	params.UserID = time.Now().UnixNano()
	params.DeliveryMode = service.GrsaiDeliveryAsync
	params.AsyncWaitingLimit = 20
	record, err := repo.Create(ctx, params)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, record)
	claim, err := repo.ClaimByID(ctx, record.ID, time.Now(), time.Now().Add(time.Hour))
	require.NoError(t, err)
	marked, err := repo.MarkSubmitting(ctx, claim.ID, claim.ClaimVersion)
	require.NoError(t, err)
	require.True(t, marked)
	require.NoError(t, repo.DeferUnsentSubmission(ctx, claim.ID, claim.ClaimVersion, time.Now().Add(-time.Second)))
	stored, err := repo.GetByID(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, "pending_upstream", stored.InternalStatus)
	require.Equal(t, "not_submitted", stored.UpstreamStatus)
	next, err := repo.ClaimByID(ctx, record.ID, time.Now(), time.Now().Add(time.Hour))
	require.NoError(t, err)
	require.Greater(t, next.ClaimVersion, claim.ClaimVersion)
	require.ErrorIs(t, repo.DeferUnsentSubmission(ctx, claim.ID, claim.ClaimVersion, time.Now().Add(time.Minute)), service.ErrGrsaiSettlementClaimLost)
}

func TestGrsaiSettlementRepository_PreBindTerminalRejectsCommittedBinding(t *testing.T) {
	ctx := context.Background()
	repo := NewGrsaiSettlementRepository(integrationDB)
	params := grsaiSettlementTestParams(t, "prebind-bound-guard")
	record, err := repo.Create(ctx, params)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, record)
	claim, err := repo.ClaimByID(ctx, record.ID, time.Now(), time.Now().Add(time.Hour))
	require.NoError(t, err)
	bound, err := repo.BindUpstreamTask(ctx, claim.ID, claim.ClaimVersion, uniqueTestValue(t, "first-frame"), "running")
	require.NoError(t, err)
	require.True(t, bound)
	released := false
	err = repo.MarkPreBindManualReviewWithRelease(ctx, claim.ID, claim.ClaimVersion, "first frame failed", func(context.Context, *sql.Tx, *GrsaiSettlement) error {
		released = true
		return nil
	})
	require.ErrorIs(t, err, service.ErrGrsaiSettlementClaimLost)
	require.False(t, released)
	stored, err := repo.GetByID(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, "processing", stored.InternalStatus)
	require.NotNil(t, stored.UpstreamTaskID)
}

func TestGrsaiTaskPayloadRepository_QueuedTaskSurvivesPayloadTTL(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	params := grsaiSettlementTestParams(t, "queued-payload-ttl")
	params.UserID = now.UnixNano()
	params.DeliveryMode = service.GrsaiDeliveryAsync
	params.AsyncWaitingLimit = 20
	record, err := NewGrsaiSettlementRepository(integrationDB).Create(ctx, params)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, record)
	payloads := NewGrsaiTaskPayloadRepository(integrationDB, testSecretEncryptor{})
	original := []byte(`{"model":"grsai-image","replyType":"async"}`)
	require.NoError(t, payloads.PutEncrypted(ctx, record.ID, original, now.Add(-time.Minute)))
	deleted, err := payloads.DeleteExpired(ctx, now)
	require.NoError(t, err)
	require.Zero(t, deleted)
	got, err := payloads.GetEncrypted(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, original, got)
	claim, err := NewGrsaiSettlementRepository(integrationDB).ClaimByID(ctx, record.ID, now, now.Add(time.Hour))
	require.NoError(t, err)
	got, err = payloads.GetEncrypted(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, original, got)
	marked, err := NewGrsaiSettlementRepository(integrationDB).MarkSubmitting(ctx, claim.ID, claim.ClaimVersion)
	require.NoError(t, err)
	require.True(t, marked)
	_, err = payloads.GetEncrypted(ctx, record.ID)
	require.ErrorIs(t, err, service.ErrGrsaiTaskPayloadNotFound)
	deleted, err = payloads.DeleteExpired(ctx, now)
	require.NoError(t, err)
	require.Zero(t, deleted)
	bound, err := NewGrsaiSettlementRepository(integrationDB).BindUpstreamTask(ctx, claim.ID, claim.ClaimVersion, "bound-queued-payload", service.GrsaiUpstreamStatusRunning)
	require.NoError(t, err)
	require.True(t, bound)
	deleted, err = payloads.DeleteExpired(ctx, now)
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)
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
	firstRecord, err := repo.Create(ctx, first)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, firstRecord)
	_, err = repo.Create(ctx, second)
	requirePostgresUniqueViolation(t, err, "grsai_settlements_account_upstream_task_uq")
	otherAccountRecord, err := repo.Create(ctx, otherAccount)
	require.NoError(t, err, "different accounts may receive the same provider task identifier")
	cleanupCreatedGrsaiSettlements(t, otherAccountRecord)
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
	firstRecord, err := repo.Create(ctx, first)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, firstRecord)
	secondRecord, err := repo.Create(ctx, second)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, secondRecord)
	thirdRecord, err := repo.Create(ctx, third)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, thirdRecord)
}

func TestGrsaiSettlementRepository_BindAndUpdateSanitizedResult(t *testing.T) {
	ctx := context.Background()
	repo := NewGrsaiSettlementRepository(integrationDB)
	params := grsaiSettlementTestParams(t, "bind-result")
	params.NextAttemptAt = time.Now().UTC().Add(-time.Minute)
	record, err := repo.Create(ctx, params)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, record)
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

func TestGrsaiSettlementRepository_ResultPollSnapshotIsDurableAndFenced(t *testing.T) {
	ctx := context.Background()
	repo := NewGrsaiSettlementRepository(integrationDB)
	params := grsaiSettlementTestParams(t, "result-snapshot")
	params.NextAttemptAt = time.Now().UTC().Add(-time.Minute)
	params.DeliveryMode = service.GrsaiDeliveryAsync
	record, err := repo.Create(ctx, params)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, record)
	claim, err := repo.ClaimByID(ctx, record.ID, time.Now().UTC(), time.Now().UTC().Add(time.Minute))
	require.NoError(t, err)
	next := time.Now().UTC().Add(time.Minute)
	updated, err := repo.RecordResultSnapshot(ctx, record.ID, claim.ClaimVersion, "running", "", next, 45, nil)
	require.NoError(t, err)
	require.True(t, updated)
	updated, err = repo.RecordResultSnapshot(ctx, record.ID, claim.ClaimVersion, "succeeded", "", next, 100, []string{"https://img.invalid/recovered.png"})
	require.NoError(t, err)
	require.True(t, updated)
	_, err = repo.RecordResultSnapshot(ctx, record.ID, claim.ClaimVersion-1, "running", "", next, 10, nil)
	require.ErrorIs(t, err, service.ErrGrsaiSettlementClaimLost)
	stored, err := repo.GetByID(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, "succeeded", stored.UpstreamStatus)
	require.Equal(t, 100, stored.Progress)
	require.Equal(t, []string{"https://img.invalid/recovered.png"}, stored.ResultURLs)
}

func TestGrsaiSettlementRepository_SettlementRetryCountIgnoresPollingClaims(t *testing.T) {
	ctx := context.Background()
	repo := NewGrsaiSettlementRepository(integrationDB)
	now := time.Now().UTC()
	params := grsaiSettlementTestParams(t, "settlement-retry-count")
	params.NextAttemptAt = now.Add(-time.Minute)
	record, err := repo.Create(ctx, params)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, record)

	first, err := repo.ClaimDue(ctx, now, 1, now.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Equal(t, 1, first[0].RetryCount)
	require.Zero(t, first[0].SettlementRetryCount)
	require.NoError(t, repo.MarkPendingSettlement(ctx, record.ID, first[0].ClaimVersion, now.Add(-time.Second)))

	second, err := repo.ClaimDue(ctx, now, 1, now.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Equal(t, 2, second[0].RetryCount)
	require.Equal(t, 1, second[0].SettlementRetryCount)
}

func TestGrsaiSettlementRepository_ConcurrentClaimHasSingleWinner(t *testing.T) {
	ctx := context.Background()
	repo := NewGrsaiSettlementRepository(integrationDB)
	params := grsaiSettlementTestParams(t, "claim-single-winner")
	params.NextAttemptAt = time.Now().UTC().Add(-time.Minute)
	created, err := repo.Create(ctx, params)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, created)

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
	settled, err := repo.Create(ctx, settledParams)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, settled)
	free, err := repo.Create(ctx, freeParams)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, free)
	review, err := repo.Create(ctx, reviewParams)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, review)

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
	record, err := repo.Create(ctx, params)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, record)
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
			record, err := repo.Create(ctx, params)
			require.NoError(t, err)
			cleanupCreatedGrsaiSettlements(t, record)

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
	record, err := repo.Create(ctx, params)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, record)
	cleanupGrsaiBillingMarker(t, record.BillingIdempotencyKey, record.APIKeyID)
	claimed, err := repo.ClaimDue(ctx, now, 1, now.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	fingerprint := hashedTestValue(t, "grsai-atomic-settle")

	billingErr := errors.New("billing failed")
	settled, err := repo.Settle(ctx, record.ID, claimed[0].ClaimVersion, 0.02,
		func(ctx context.Context, tx *sql.Tx, locked *GrsaiSettlement) error {
			require.Equal(t, service.GrsaiSettlementRequestID(record.ID), locked.BillingIdempotencyKey)
			_, insertErr := tx.ExecContext(ctx, `
INSERT INTO usage_billing_dedup (request_id, api_key_id, request_fingerprint)
VALUES ($1, $2, $3)`, locked.BillingIdempotencyKey, locked.APIKeyID, fingerprint)
			require.NoError(t, insertErr)
			return billingErr
		})
	require.False(t, settled)
	require.ErrorIs(t, err, billingErr)
	require.Equal(t, 0, countGrsaiBillingMarkers(t, record.BillingIdempotencyKey, record.APIKeyID))
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
	require.Equal(t, 1, countGrsaiBillingMarkers(t, record.BillingIdempotencyKey, record.APIKeyID))
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
	record, err := repo.Create(ctx, params)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, record)
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
		AccountID:             3001,
		GroupID:               4001,
		UserID:                1001,
		APIKeyID:              2001,
		Model:                 "grsai-image",
		BaseUnitPrice:         0.01,
		GroupRateMultiplier:   1,
		AccountRateMultiplier: 1,
		BillableUnitPrice:     0.01,
		RequestedImageCount:   2,
		ImageSize:             service.ImageBillingSize1K,
		Currency:              "USD",
		BillingIdempotencyKey: uniqueTestValue(t, "grsai-billing-"+suffix),
		UpstreamStatus:        "not_submitted",
		NextAttemptAt:         time.Now().UTC().Add(24 * time.Hour),
	}
}

func cleanupCreatedGrsaiSettlements(t *testing.T, records ...*GrsaiSettlement) {
	t.Helper()
	ids := make([]int64, 0, len(records))
	for _, record := range records {
		require.NotNil(t, record)
		ids = append(ids, record.ID)
	}
	t.Cleanup(func() {
		_, cleanupErr := integrationDB.ExecContext(context.Background(),
			`DELETE FROM grsai_settlements WHERE id = ANY($1)`, pq.Array(ids))
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
