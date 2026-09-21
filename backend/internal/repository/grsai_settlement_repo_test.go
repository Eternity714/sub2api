package repository

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

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

func TestGrsaiSettlementRepositoryCreateGeneratesPublicTaskIDAndPersistsTaskFields(t *testing.T) {
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
