package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGrsaiDeliveryDeleteTerminalUsesClosedAtAndSafeStatuses(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	cutoff := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	mock.ExpectExec(`(?s)DELETE FROM grsai_settlements.*closed_at.*expires_at.*created_at.*internal_status IN \('settled', 'closed_no_charge'\).*hold_state.*LIMIT \$3`).
		WithArgs(cutoff, int64(86400), 20).WillReturnResult(sqlmock.NewResult(0, 2))
	deleted, err := NewGrsaiSettlementRepository(db).DeleteTerminal(context.Background(), cutoff, 24*time.Hour, 20)
	require.NoError(t, err)
	require.EqualValues(t, 2, deleted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGrsaiSettlementReserveHoldRollsBackWhenStateWriteFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewGrsaiSettlementRepository(db)
	record := grsaiSettlementUnitRecord()
	record.HoldAmount = 1.2
	record.UpstreamTaskID = nil
	stateWriteErr := errors.New("state write failed")
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)FROM grsai_settlements WHERE id = \$1 FOR UPDATE`).WithArgs(record.ID).WillReturnRows(grsaiSettlementUnitRows(record))
	mock.ExpectExec(`UPDATE users SET balance = balance -`).WithArgs(record.HoldAmount, record.UserID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE grsai_settlements SET hold_state = 'held'`).WithArgs(record.ID, int64(0)).WillReturnError(stateWriteErr)
	mock.ExpectRollback()
	err = repo.ReserveHold(context.Background(), record.ID, 0, func(ctx context.Context, tx *sql.Tx, locked *service.GrsaiSettlement) error {
		_, err := tx.ExecContext(ctx, `UPDATE users SET balance = balance - $1 WHERE id = $2`, locked.HoldAmount, locked.UserID)
		return err
	})
	require.ErrorIs(t, err, stateWriteErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGrsaiSettlementRepositoryFindsOnlyExactOwner(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewGrsaiSettlementRepository(db)
	record := grsaiSettlementUnitRecord()
	ownerQuery := `(?s)WHERE user_id = \$1 AND api_key_id = \$2\s+AND \(public_task_id = \$3 OR upstream_task_id = \$3\)`
	mock.ExpectQuery(ownerQuery).
		WithArgs(record.UserID, record.APIKeyID, record.PublicTaskID).
		WillReturnRows(grsaiSettlementUnitRows(record))

	got, err := repo.GetOwnedByPublicOrUpstreamID(context.Background(), record.UserID, record.APIKeyID, record.PublicTaskID)
	require.NoError(t, err)
	require.Equal(t, record.ID, got.ID)
	require.Equal(t, record.PublicTaskID, got.PublicTaskID)

	mock.ExpectQuery(ownerQuery).
		WithArgs(record.UserID, record.APIKeyID+1, record.PublicTaskID).
		WillReturnRows(sqlmock.NewRows(grsaiSettlementUnitColumns))
	_, err = repo.GetOwnedByPublicOrUpstreamID(context.Background(), record.UserID, record.APIKeyID+1, record.PublicTaskID)
	require.ErrorIs(t, err, service.ErrGrsaiSettlementNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGrsaiSettlementRepositoryLegacyClaimExcludesAsync(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewGrsaiSettlementRepository(db)
	now := time.Now().UTC()
	mock.ExpectQuery(`(?s)FROM grsai_settlements\s+WHERE.*delivery_mode <> 'async'`).
		WithArgs(now, 1, now.Add(time.Minute)).
		WillReturnRows(sqlmock.NewRows(grsaiSettlementUnitColumns))
	claims, err := repo.ClaimDue(context.Background(), now, 1, now.Add(time.Minute))
	require.NoError(t, err)
	require.Empty(t, claims)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGrsaiSettlementRepositoryAsyncClaimExcludesOtherModes(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewGrsaiSettlementRepository(db)
	now := time.Now().UTC()
	mock.ExpectQuery(`SELECT user_id FROM grsai_settlements AS candidate`).
		WithArgs(now).WillReturnRows(sqlmock.NewRows([]string{"user_id"}))
	claims, err := repo.ClaimDueForDeliveryMode(context.Background(), now, 1, now.Add(time.Minute), service.GrsaiDeliveryAsync)
	require.NoError(t, err)
	require.Empty(t, claims)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGrsaiSettlementRepositoryAsyncAdmissionRejectsFullQueue(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewGrsaiSettlementRepository(db)
	params := CreateGrsaiSettlementParams{AccountID: 1, GroupID: 2, UserID: 3, APIKeyID: 4, Model: "m", RequestedImageCount: 1, ImageSize: service.ImageBillingSize1K, DeliveryMode: service.GrsaiDeliveryAsync, AsyncWaitingLimit: 20, NextAttemptAt: time.Now().Add(time.Hour)}
	mock.ExpectBegin()
	mock.ExpectExec(`pg_advisory_xact_lock`).WithArgs(params.UserID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM grsai_settlements`).WithArgs(params.UserID).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(20))
	mock.ExpectRollback()
	_, err = repo.Create(context.Background(), params)
	require.ErrorIs(t, err, service.ErrGrsaiAsyncQueueFull)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGrsaiSettlementRepositoryAsyncClaimBlocksFourthNewWorker(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewGrsaiSettlementRepository(db)
	now := time.Now().UTC()
	mock.ExpectQuery(`SELECT user_id FROM grsai_settlements`).WithArgs(now).WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(int64(3)))
	mock.ExpectBegin()
	mock.ExpectExec(`pg_advisory_xact_lock`).WithArgs(int64(3)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM grsai_settlements`).WithArgs(int64(3)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	mock.ExpectQuery(`(?s)UPDATE grsai_settlements AS settlements.*async_started_at = COALESCE`).
		WithArgs(now, 1, now.Add(time.Minute), int64(3), 3).
		WillReturnRows(sqlmock.NewRows(grsaiSettlementUnitColumns))
	mock.ExpectCommit()
	claims, err := repo.ClaimDueForDeliveryMode(context.Background(), now, 1, now.Add(time.Minute), service.GrsaiDeliveryAsync)
	require.NoError(t, err)
	require.Empty(t, claims)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGrsaiSettlementRepositoryMarkSubmittingRequiresActiveUnboundClaim(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewGrsaiSettlementRepository(db)
	query := `(?s)UPDATE grsai_settlements\s+SET upstream_status = 'submitting'.*WHERE id = \$1\s+AND claim_version = \$2\s+AND internal_status = 'processing'\s+AND upstream_task_id IS NULL\s+AND upstream_status = 'not_submitted'`
	for _, rows := range []int64{1, 0} {
		mock.ExpectExec(query).WithArgs(int64(42), int64(3)).WillReturnResult(sqlmock.NewResult(0, rows))
		marked, err := repo.MarkSubmitting(context.Background(), 42, 3)
		if rows == 0 {
			require.ErrorIs(t, err, ErrGrsaiSettlementClaimLost)
		} else {
			require.NoError(t, err)
		}
		require.Equal(t, rows == 1, marked)
	}
	marked, err := repo.MarkSubmitting(context.Background(), 42, 0)
	require.ErrorIs(t, err, ErrGrsaiSettlementInvalidInput)
	require.False(t, marked)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGrsaiSettlementRepositoryDeferUnsentSubmissionFencesSameClaimAndUnboundID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewGrsaiSettlementRepository(db)
	next := time.Now().UTC().Add(time.Minute)
	query := `(?s)UPDATE grsai_settlements\s+SET upstream_status = 'not_submitted'.*claim_version = \$2 AND internal_status = 'processing'\s+AND upstream_task_id IS NULL AND upstream_status IN \('not_submitted', 'submitting'\)`
	for _, rows := range []int64{1, 0} {
		mock.ExpectExec(query).WithArgs(int64(42), int64(3), next).WillReturnResult(sqlmock.NewResult(0, rows))
		err := repo.DeferUnsentSubmission(context.Background(), 42, 3, next)
		if rows == 0 {
			require.ErrorIs(t, err, service.ErrGrsaiSettlementClaimLost)
		} else {
			require.NoError(t, err)
		}
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGrsaiSettlementRepositoryOwnerLookupTrimsAndRejectsBlankID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewGrsaiSettlementRepository(db)
	record := grsaiSettlementUnitRecord()
	mock.ExpectQuery(`(?s)WHERE user_id = \$1 AND api_key_id = \$2`).
		WithArgs(record.UserID, record.APIKeyID, record.PublicTaskID).
		WillReturnRows(grsaiSettlementUnitRows(record))

	got, err := repo.GetOwnedByPublicOrUpstreamID(context.Background(), record.UserID, record.APIKeyID, "  "+record.PublicTaskID+"  ")
	require.NoError(t, err)
	require.Equal(t, record.ID, got.ID)

	_, err = repo.GetOwnedByPublicOrUpstreamID(context.Background(), record.UserID, record.APIKeyID, " \t ")
	require.ErrorIs(t, err, service.ErrGrsaiSettlementNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGrsaiSettlementRepositoryCreateOverridesPredictablePublicTaskIDWithUUID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewGrsaiSettlementRepository(db)
	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	deleteAfter := expiresAt.Add(-time.Hour)
	params := CreateGrsaiSettlementParams{
		AccountID:             11,
		GroupID:               12,
		UserID:                13,
		APIKeyID:              14,
		Model:                 "grsai-image",
		BaseUnitPrice:         0.01,
		GroupRateMultiplier:   1,
		AccountRateMultiplier: 1,
		BillableUnitPrice:     0.01,
		RequestedImageCount:   1,
		ImageSize:             service.ImageBillingSize1K,
		Currency:              "USD",
		PublicTaskID:          "42",
		DeliveryMode:          service.GrsaiDeliveryAsync,
		Progress:              25,
		ResultURLs:            []string{"https://example.test/result.png"},
		HoldAmount:            0.01,
		HoldState:             "held",
		PayloadDeleteAfter:    &deleteAfter,
		ExpiresAt:             &expiresAt,
		UpstreamStatus:        "queued",
		NextAttemptAt:         time.Now().UTC().Add(time.Minute),
	}
	record := grsaiSettlementUnitRecord()
	record.DeliveryMode = params.DeliveryMode
	record.Progress = params.Progress
	record.ResultURLs = append([]string(nil), params.ResultURLs...)
	record.HoldAmount = params.HoldAmount
	record.HoldState = params.HoldState
	record.PayloadDeleteAfter = params.PayloadDeleteAfter
	record.ExpiresAt = params.ExpiresAt

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO grsai_settlements")).
		WithArgs(
			params.AccountID, params.GroupID, params.UserID, params.APIKeyID, params.Model,
			params.BaseUnitPrice, params.GroupRateMultiplier, params.AccountRateMultiplier, params.BillableUnitPrice,
			params.RequestedImageCount, params.ImageSize, params.Currency, validUUIDArgument{}, params.DeliveryMode,
			params.Progress, `["https://example.test/result.png"]`, params.HoldAmount, params.HoldState,
			params.PayloadDeleteAfter, params.ExpiresAt, params.UpstreamTaskID, params.UpstreamStatus, params.NextAttemptAt,
		).
		WillReturnRows(grsaiSettlementUnitRows(record))

	got, err := repo.Create(context.Background(), params)
	require.NoError(t, err)
	require.NotEmpty(t, got.PublicTaskID)
	require.NotEqual(t, params.PublicTaskID, got.PublicTaskID)
	require.Equal(t, params.DeliveryMode, got.DeliveryMode)
	require.Equal(t, params.ResultURLs, got.ResultURLs)
	require.NoError(t, mock.ExpectationsWereMet())
}

type validUUIDArgument struct{}

func (validUUIDArgument) Match(value driver.Value) bool {
	publicTaskID, ok := value.(string)
	if !ok {
		return false
	}
	_, err := uuid.Parse(publicTaskID)
	return err == nil
}

var grsaiSettlementUnitColumns = []string{
	"id", "account_id", "group_id", "user_id", "api_key_id", "model",
	"base_unit_price", "group_rate_multiplier", "account_rate_multiplier", "billable_unit_price",
	"requested_image_count", "image_size", "currency", "billing_idempotency_key",
	"public_task_id", "delivery_mode", "progress", "result_urls", "hold_amount", "hold_state",
	"payload_delete_after", "expires_at", "upstream_task_id", "upstream_status", "internal_status",
	"retry_count", "settlement_retry_count", "claim_version", "next_attempt_at", "last_error_summary",
	"settled_amount", "created_at", "updated_at", "upstream_bound_at", "result_updated_at", "settled_at", "closed_at",
}

func grsaiSettlementUnitRecord() *GrsaiSettlement {
	now := time.Now().UTC()
	upstreamTaskID := "upstream-task-42"
	return &GrsaiSettlement{
		ID:                    42,
		AccountID:             11,
		GroupID:               12,
		UserID:                13,
		APIKeyID:              14,
		Model:                 "grsai-image",
		BaseUnitPrice:         0.01,
		GroupRateMultiplier:   1,
		AccountRateMultiplier: 1,
		BillableUnitPrice:     0.01,
		RequestedImageCount:   1,
		ImageSize:             service.ImageBillingSize1K,
		Currency:              "USD",
		BillingIdempotencyKey: "grsai_settlement:42",
		PublicTaskID:          "9804ca78-9c54-4b26-9f0e-d0d4b0d5260d",
		DeliveryMode:          service.GrsaiDeliveryAsync,
		Progress:              0,
		ResultURLs:            []string{},
		HoldAmount:            0,
		HoldState:             "none",
		UpstreamTaskID:        &upstreamTaskID,
		UpstreamStatus:        "queued",
		InternalStatus:        GrsaiSettlementStatusPendingUpstream,
		NextAttemptAt:         now.Add(time.Minute),
		CreatedAt:             now,
		UpdatedAt:             now,
	}
}

func grsaiSettlementUnitRows(record *GrsaiSettlement) *sqlmock.Rows {
	var upstreamTaskID any
	if record.UpstreamTaskID != nil {
		upstreamTaskID = *record.UpstreamTaskID
	}
	resultURLsJSON, _ := json.Marshal(record.ResultURLs)
	return sqlmock.NewRows(grsaiSettlementUnitColumns).AddRow(
		record.ID, record.AccountID, record.GroupID, record.UserID, record.APIKeyID, record.Model,
		record.BaseUnitPrice, record.GroupRateMultiplier, record.AccountRateMultiplier, record.BillableUnitPrice,
		record.RequestedImageCount, record.ImageSize, record.Currency, record.BillingIdempotencyKey,
		record.PublicTaskID, record.DeliveryMode, record.Progress, resultURLsJSON, record.HoldAmount, record.HoldState,
		record.PayloadDeleteAfter, record.ExpiresAt, upstreamTaskID, record.UpstreamStatus, record.InternalStatus,
		record.RetryCount, record.SettlementRetryCount, record.ClaimVersion, record.NextAttemptAt, nil,
		nil, record.CreatedAt, record.UpdatedAt, nil, nil, nil, nil,
	)
}
