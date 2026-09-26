//go:build integration

package repository

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func grsaiV2TestParams(t *testing.T) service.CreateV2GrsaiTaskParams {
	t.Helper()
	params := grsaiSettlementTestParams(t, "v2")
	params.LocalTaskID = service.NewGrsaiLocalTaskID()
	params.DeliveryMode = "async"
	params.NextAttemptAt = time.Now().UTC().Add(-time.Minute)
	return service.CreateV2GrsaiTaskParams{
		CreateGrsaiSettlementParams: params,
		PayloadExpiresAt:            time.Now().UTC().Add(time.Hour),
		UpstreamPayload:             []byte(`{"prompt":"private"}`),
	}
}

func grsaiV2TestRepo(t *testing.T) *grsaiSettlementRepository {
	t.Helper()
	encryptor, err := NewAESEncryptor(&config.Config{Totp: config.TotpConfig{EncryptionKey: hex.EncodeToString(make([]byte, 32))}})
	require.NoError(t, err)
	return NewGrsaiV2TaskRepository(integrationDB, encryptor)
}

func grsaiV2NoopBalance(context.Context, *sql.Tx, *GrsaiSettlement) error { return nil }

func TestGrsaiV2CreateIsAtomicAndOwnedByOriginalKey(t *testing.T) {
	ctx := context.Background()
	repo := grsaiV2TestRepo(t)
	params := grsaiV2TestParams(t)
	var createdID int64
	failed := errors.New("reserve failed")
	_, err := repo.CreateV2GrsaiTask(ctx, params, func(_ context.Context, _ *sql.Tx, record *GrsaiSettlement) error {
		createdID = record.ID
		return failed
	})
	require.ErrorIs(t, err, failed)
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM grsai_settlements WHERE local_task_id = $1`, params.LocalTaskID).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM grsai_task_payloads WHERE local_task_id = $1`, params.LocalTaskID).Scan(&count))
	require.Zero(t, count)
	require.Positive(t, createdID)

	record, err := repo.CreateV2GrsaiTask(ctx, params, grsaiV2NoopBalance)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, record)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM grsai_task_payloads WHERE local_task_id = $1`, params.LocalTaskID)
	})
	require.Equal(t, params.LocalTaskID, *record.LocalTaskID)
	require.Equal(t, 2, record.TaskVersion)
	require.Equal(t, "v2_queued", record.InternalStatus)
	got, err := repo.GetOwnedV2(ctx, params.UserID, params.APIKeyID, params.LocalTaskID)
	require.NoError(t, err)
	require.Equal(t, record.ID, got.ID)
	_, err = repo.GetOwnedV2(ctx, params.UserID, params.APIKeyID+1, params.LocalTaskID)
	require.ErrorIs(t, err, ErrGrsaiSettlementNotFound)
	_, err = repo.GetOwnedV2(ctx, params.UserID+1, params.APIKeyID, params.LocalTaskID)
	require.ErrorIs(t, err, ErrGrsaiSettlementNotFound)
	items, err := repo.ListOwnedV2(ctx, params.UserID, params.APIKeyID, 20, 0)
	require.NoError(t, err)
	require.NotEmpty(t, items)
	items, err = repo.ListOwnedV2(ctx, params.UserID, params.APIKeyID+1, 20, 0)
	require.NoError(t, err)
	require.Empty(t, items)
	var ciphertext string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT payload_ciphertext FROM grsai_task_payloads WHERE local_task_id = $1`, params.LocalTaskID).Scan(&ciphertext))
	require.NotContains(t, ciphertext, `"prompt"`)
	payloadRepo := NewGrsaiTaskPayloadRepository(integrationDB, repo.encryptor)
	_, err = payloadRepo.GetDecrypted(ctx, params.LocalTaskID, 1)
	require.ErrorIs(t, err, ErrGrsaiTaskPayloadNotFound, "unclaimed payload must not be readable")
	_, err = repo.CreateV2GrsaiTask(ctx, params, grsaiV2NoopBalance)
	requirePostgresUniqueViolation(t, err, "grsai_settlements_local_task_id_uq")
}

func TestGrsaiV2CreateRejectsExpiredPayload(t *testing.T) {
	ctx := context.Background()
	repo := grsaiV2TestRepo(t)
	params := grsaiV2TestParams(t)
	params.PayloadExpiresAt = time.Now().UTC().Add(-time.Minute)
	_, err := repo.CreateV2GrsaiTask(ctx, params, grsaiV2NoopBalance)
	require.ErrorIs(t, err, ErrGrsaiSettlementInvalidInput)
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM grsai_settlements WHERE local_task_id = $1`, params.LocalTaskID).Scan(&count))
	require.Zero(t, count)
}

func TestGrsaiV2ClaimIsSingleWinnerAndDoesNotRetrySubmission(t *testing.T) {
	ctx := context.Background()
	repo := grsaiV2TestRepo(t)
	params := grsaiV2TestParams(t)
	record, err := repo.CreateV2GrsaiTask(ctx, params, grsaiV2NoopBalance)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, record)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM grsai_task_payloads WHERE local_task_id = $1`, params.LocalTaskID)
	})
	now := time.Now().UTC()
	start := make(chan struct{})
	results := make(chan []*GrsaiSettlement, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			claimed, claimErr := repo.ClaimDueV2(ctx, now, 1, now.Add(time.Minute), 3)
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
	var first *GrsaiSettlement
	for claimed := range results {
		for _, item := range claimed {
			if item.ID == record.ID {
				require.Nil(t, first)
				first = item
			}
		}
	}
	require.NotNil(t, first)
	require.Equal(t, "v2_submitting", first.InternalStatus)
	require.Equal(t, 1, first.SubmissionAttempt)
	payloadRepo := NewGrsaiTaskPayloadRepository(integrationDB, repo.encryptor)
	_, err = payloadRepo.GetDecrypted(ctx, params.LocalTaskID, first.ClaimVersion+1)
	require.ErrorIs(t, err, ErrGrsaiTaskPayloadNotFound)
	plaintext, err := payloadRepo.GetDecrypted(ctx, params.LocalTaskID, first.ClaimVersion)
	require.NoError(t, err)
	require.Equal(t, params.UpstreamPayload, plaintext)
	legacy, err := repo.ClaimDue(ctx, now, 10, now.Add(time.Minute))
	require.NoError(t, err)
	for _, item := range legacy {
		require.NotEqual(t, record.ID, item.ID)
	}
	second, err := repo.ClaimDueV2(ctx, now.Add(2*time.Minute), 1, now.Add(3*time.Minute), 3)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Equal(t, record.ID, second[0].ID)
	require.Equal(t, 2, second[0].SubmissionAttempt)
	bound, err := repo.BindV2UpstreamTask(ctx, record.ID, second[0].ClaimVersion, "provider-id")
	require.ErrorIs(t, err, ErrGrsaiSettlementClaimLost)
	require.False(t, bound)
	failed, err := repo.FailV2(ctx, record.ID, second[0].ClaimVersion, "uncertain_submit", "", grsaiV2NoopBalance)
	require.NoError(t, err)
	require.True(t, failed)
}

func TestGrsaiV2BindingDeletesEncryptedPayloadAtomically(t *testing.T) {
	ctx := context.Background()
	repo := grsaiV2TestRepo(t)
	params := grsaiV2TestParams(t)
	record, err := repo.CreateV2GrsaiTask(ctx, params, grsaiV2NoopBalance)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, record)
	now := time.Now().UTC()
	claimed, err := repo.ClaimDueV2(ctx, now, 1, now.Add(time.Minute), 3)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	bound, err := repo.BindV2UpstreamTask(ctx, record.ID, claimed[0].ClaimVersion, "provider-id")
	require.NoError(t, err)
	require.True(t, bound)
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM grsai_task_payloads WHERE local_task_id = $1`, params.LocalTaskID).Scan(&count))
	require.Zero(t, count)
}

func TestGrsaiV2TerminalBillingFailureRollsBackStatus(t *testing.T) {
	ctx := context.Background()
	repo := grsaiV2TestRepo(t)
	params := grsaiV2TestParams(t)
	record, err := repo.CreateV2GrsaiTask(ctx, params, grsaiV2NoopBalance)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, record)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM grsai_task_payloads WHERE local_task_id = $1`, params.LocalTaskID)
	})
	now := time.Now().UTC()
	claimed, err := repo.ClaimDueV2(ctx, now, 1, now.Add(time.Minute), 3)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	bound, err := repo.BindV2UpstreamTask(ctx, record.ID, claimed[0].ClaimVersion, "provider-id")
	require.NoError(t, err)
	require.True(t, bound)
	claimed, err = repo.ClaimDueV2(ctx, now.Add(2*time.Minute), 1, now.Add(3*time.Minute), 3)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	failure := errors.New("billing unavailable")
	stop := func(context.Context, *sql.Tx, *GrsaiSettlement) error { return failure }
	completed, err := repo.CompleteV2(ctx, record.ID, claimed[0].ClaimVersion, []byte(`{"images":[]}`), []byte(`[]`), nil, 0.02, stop)
	require.False(t, completed)
	require.ErrorIs(t, err, failure)
	got, err := repo.GetByID(ctx, record.ID)
	require.NoError(t, err)
	require.NotEqual(t, "succeeded", got.PublicStatus)
	completed, err = repo.CompleteV2(ctx, record.ID, claimed[0].ClaimVersion, []byte(`{"images":[]}`), []byte(`[]`), nil, 0.02, grsaiV2NoopBalance)
	require.NoError(t, err)
	require.True(t, completed)
	got, err = repo.GetByID(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, "succeeded", got.PublicStatus)
	require.NotNil(t, got.SettledAmount)
	require.InDelta(t, 0.02, *got.SettledAmount, 0.000001)
}

func TestGrsaiV2ReleaseFailureRollsBackFailureStatus(t *testing.T) {
	ctx := context.Background()
	repo := grsaiV2TestRepo(t)
	params := grsaiV2TestParams(t)
	record, err := repo.CreateV2GrsaiTask(ctx, params, grsaiV2NoopBalance)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, record)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM grsai_task_payloads WHERE local_task_id = $1`, params.LocalTaskID)
	})
	now := time.Now().UTC()
	claimed, err := repo.ClaimDueV2(ctx, now, 1, now.Add(time.Minute), 3)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	failure := errors.New("release unavailable")
	stop := func(context.Context, *sql.Tx, *GrsaiSettlement) error { return failure }
	failed, err := repo.FailV2(ctx, record.ID, claimed[0].ClaimVersion, "upstream_failed", "safe summary", stop)
	require.False(t, failed)
	require.ErrorIs(t, err, failure)
	got, err := repo.GetByID(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, "v2_submitting", got.InternalStatus)
	require.NotEqual(t, "failed", got.PublicStatus)
	failed, err = repo.FailV2(ctx, record.ID, claimed[0].ClaimVersion, "upstream_failed", "safe summary", grsaiV2NoopBalance)
	require.NoError(t, err)
	require.True(t, failed)
	got, err = repo.GetByID(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, "v2_failed", got.InternalStatus)
	require.Equal(t, "failed", got.PublicStatus)
}

func TestGrsaiV2PersistenceFailureCanEnterManualReview(t *testing.T) {
	ctx := context.Background()
	repo := grsaiV2TestRepo(t)
	params := grsaiV2TestParams(t)
	record, err := repo.CreateV2GrsaiTask(ctx, params, grsaiV2NoopBalance)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, record)
	now := time.Now().UTC()
	claims, err := repo.ClaimDueV2(ctx, now, 1, now.Add(time.Minute), 3)
	require.NoError(t, err)
	require.Len(t, claims, 1)
	bound, err := repo.BindV2UpstreamTask(ctx, record.ID, claims[0].ClaimVersion, "private-upstream-id")
	require.NoError(t, err)
	require.True(t, bound)
	deferred, err := repo.DeferV2Failure(ctx, record.ID, claims[0].ClaimVersion, now.Add(2*time.Minute))
	require.NoError(t, err)
	require.True(t, deferred)
	got, err := repo.GetByID(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, 1, got.SettlementRetryCount)
	claims, err = repo.ClaimDueV2(ctx, now.Add(3*time.Minute), 1, now.Add(4*time.Minute), 3)
	require.NoError(t, err)
	require.Len(t, claims, 1)
	marked, err := repo.MarkV2ManualReview(ctx, record.ID, claims[0].ClaimVersion, "result_persistence_failed")
	require.NoError(t, err)
	require.True(t, marked)
	got, err = repo.GetByID(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, "manual_review", got.PublicStatus)
	claims, err = repo.ClaimDueV2(ctx, now.Add(5*time.Minute), 1, now.Add(6*time.Minute), 3)
	require.NoError(t, err)
	require.Empty(t, claims)
}
