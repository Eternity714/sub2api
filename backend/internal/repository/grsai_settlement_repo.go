package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

const (
	GrsaiSettlementStatusPendingUpstream   = "pending_upstream"
	GrsaiSettlementStatusPendingSettlement = "pending_settlement"
	GrsaiSettlementStatusProcessing        = "processing"
	GrsaiSettlementStatusSettled           = "settled"
	GrsaiSettlementStatusClosedNoCharge    = "closed_no_charge"
	GrsaiSettlementStatusManualReview      = "manual_review"
)

var (
	ErrGrsaiSettlementNotFound     = service.ErrGrsaiSettlementNotFound
	ErrGrsaiSettlementInvalidInput = service.ErrGrsaiSettlementInvalidInput
	ErrGrsaiSettlementInvalidState = service.ErrGrsaiSettlementInvalidState
	ErrGrsaiSettlementClaimLost    = service.ErrGrsaiSettlementClaimLost
)

type grsaiSettlementSQLExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type grsaiSettlementRepository struct {
	db  *sql.DB
	sql grsaiSettlementSQLExecutor
}

type CreateGrsaiSettlementParams = service.CreateGrsaiSettlementParams
type GrsaiSettlement = service.GrsaiSettlement

// GrsaiSettlementTxFunc applies the idempotent billing side effect inside the
// same SQL transaction that marks the settlement complete. The callback must
// not commit or roll back tx.
type GrsaiSettlementTxFunc = service.GrsaiSettlementTxFunc

var _ service.GrsaiSettlementRepository = (*grsaiSettlementRepository)(nil)

func NewGrsaiSettlementRepository(db *sql.DB) *grsaiSettlementRepository {
	return &grsaiSettlementRepository{db: db, sql: db}
}

func (r *grsaiSettlementRepository) Create(ctx context.Context, params CreateGrsaiSettlementParams) (*GrsaiSettlement, error) {
	params.Model = strings.TrimSpace(params.Model)
	params.ImageSize = service.NormalizeImageBillingTierOrDefault(params.ImageSize)
	params.PublicTaskID = uuid.NewString()
	if params.DeliveryMode == "" {
		params.DeliveryMode = service.GrsaiDeliveryJSON
	}
	params.HoldState = strings.TrimSpace(params.HoldState)
	if params.HoldState == "" {
		params.HoldState = "none"
	}
	if params.ResultURLs == nil {
		params.ResultURLs = []string{}
	}
	if params.AccountID <= 0 || params.GroupID <= 0 || params.UserID <= 0 || params.APIKeyID <= 0 ||
		params.Model == "" || params.RequestedImageCount <= 0 || len(params.PublicTaskID) > 64 ||
		(params.DeliveryMode != service.GrsaiDeliveryJSON && params.DeliveryMode != service.GrsaiDeliveryStream && params.DeliveryMode != service.GrsaiDeliveryAsync) ||
		params.Progress < 0 || params.Progress > 100 ||
		!isFiniteNonNegative(params.BaseUnitPrice) || !isFiniteNonNegative(params.GroupRateMultiplier) ||
		!isFiniteNonNegative(params.AccountRateMultiplier) || !isFiniteNonNegative(params.BillableUnitPrice) ||
		!isFiniteNonNegative(params.HoldAmount) ||
		(params.PayloadDeleteAfter != nil && params.PayloadDeleteAfter.IsZero()) ||
		(params.ExpiresAt != nil && params.ExpiresAt.IsZero()) {
		return nil, ErrGrsaiSettlementInvalidInput
	}
	if params.Currency == "" {
		params.Currency = "USD"
	}
	if params.UpstreamStatus == "" {
		params.UpstreamStatus = "not_submitted"
	}
	if params.NextAttemptAt.IsZero() {
		params.NextAttemptAt = time.Now()
	}
	if params.UpstreamTaskID != nil {
		trimmed := strings.TrimSpace(*params.UpstreamTaskID)
		if trimmed == "" {
			params.UpstreamTaskID = nil
		} else {
			params.UpstreamTaskID = &trimmed
		}
	}
	resultURLsJSON, err := json.Marshal(params.ResultURLs)
	if err != nil {
		return nil, fmt.Errorf("marshal grsai result URLs: %w", err)
	}

	row := r.sql.QueryRowContext(ctx, grsaiSettlementInsertSQL,
		params.AccountID,
		params.GroupID,
		params.UserID,
		params.APIKeyID,
		params.Model,
		params.BaseUnitPrice,
		params.GroupRateMultiplier,
		params.AccountRateMultiplier,
		params.BillableUnitPrice,
		params.RequestedImageCount,
		params.ImageSize,
		params.Currency,
		params.PublicTaskID,
		params.DeliveryMode,
		params.Progress,
		string(resultURLsJSON),
		params.HoldAmount,
		params.HoldState,
		params.PayloadDeleteAfter,
		params.ExpiresAt,
		params.UpstreamTaskID,
		params.UpstreamStatus,
		params.NextAttemptAt,
	)
	return scanGrsaiSettlement(row)
}

func (r *grsaiSettlementRepository) GetByID(ctx context.Context, id int64) (*GrsaiSettlement, error) {
	record, err := scanGrsaiSettlement(r.sql.QueryRowContext(ctx, grsaiSettlementSelectSQL+" WHERE id = $1", id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGrsaiSettlementNotFound
	}
	return record, err
}

func (r *grsaiSettlementRepository) GetOwnedByPublicOrUpstreamID(ctx context.Context, userID, apiKeyID int64, id string) (*GrsaiSettlement, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrGrsaiSettlementNotFound
	}
	record, err := scanGrsaiSettlement(r.sql.QueryRowContext(ctx, grsaiSettlementSelectSQL+`
WHERE user_id = $1 AND api_key_id = $2
  AND (public_task_id = $3 OR upstream_task_id = $3)`, userID, apiKeyID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGrsaiSettlementNotFound
	}
	return record, err
}

// ClaimByID is for an immediate request or an explicit retry. Active leases
// cannot be stolen; expired leases use the same increasing fence as ClaimDue.
func (r *grsaiSettlementRepository) ClaimByID(ctx context.Context, id int64, now, leaseUntil time.Time) (*GrsaiSettlement, error) {
	if id <= 0 || now.IsZero() || !leaseUntil.After(now) {
		return nil, ErrGrsaiSettlementInvalidInput
	}
	record, err := scanGrsaiSettlement(r.sql.QueryRowContext(ctx, `
UPDATE grsai_settlements
SET internal_status = 'processing',
    claim_version = claim_version + 1,
    next_attempt_at = $3,
    updated_at = $2
WHERE id = $1
  AND (internal_status IN ('pending_upstream', 'pending_settlement')
       OR (internal_status = 'processing' AND next_attempt_at <= $2))
RETURNING `+grsaiSettlementReturningColumns("grsai_settlements"), id, now, leaseUntil))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGrsaiSettlementClaimLost
	}
	return record, err
}

func (r *grsaiSettlementRepository) MarkPendingUpstream(ctx context.Context, id, claimVersion int64, nextAttemptAt time.Time) error {
	if nextAttemptAt.IsZero() {
		return ErrGrsaiSettlementInvalidInput
	}
	return r.transition(ctx, id, claimVersion, `
UPDATE grsai_settlements
SET internal_status = 'pending_upstream',
    next_attempt_at = $3,
    updated_at = NOW()
WHERE id = $1
  AND claim_version = $2
  AND internal_status = 'processing'`, nextAttemptAt)
}

func (r *grsaiSettlementRepository) BindUpstreamTask(ctx context.Context, id, claimVersion int64, taskID, upstreamStatus string) (bool, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return false, ErrGrsaiSettlementInvalidInput
	}
	if upstreamStatus == "" {
		upstreamStatus = "queued"
	}
	result, err := r.sql.ExecContext(ctx, `
UPDATE grsai_settlements
SET upstream_task_id = $3,
    upstream_status = $4,
    upstream_bound_at = COALESCE(upstream_bound_at, NOW()),
    updated_at = NOW()
WHERE id = $1
  AND claim_version = $2
  AND upstream_task_id IS NULL
  AND internal_status = 'processing'`, id, claimVersion, taskID, upstreamStatus)
	if err != nil {
		return false, err
	}
	return claimedUpdateResult(result)
}

func (r *grsaiSettlementRepository) UpdateResult(ctx context.Context, id, claimVersion int64, upstreamStatus, errorSummary string, nextAttemptAt time.Time) (bool, error) {
	upstreamStatus = strings.TrimSpace(upstreamStatus)
	if upstreamStatus == "" || nextAttemptAt.IsZero() {
		return false, ErrGrsaiSettlementInvalidInput
	}
	result, err := r.sql.ExecContext(ctx, `
UPDATE grsai_settlements
SET upstream_status = $3,
    last_error_summary = NULLIF($4, ''),
    next_attempt_at = $5,
    result_updated_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND claim_version = $2
  AND internal_status = 'processing'
  AND (upstream_status NOT IN ('succeeded', 'failed', 'violation') OR upstream_status = $3)`, id, claimVersion, upstreamStatus, errorSummary, nextAttemptAt)
	if err != nil {
		return false, err
	}
	return claimedUpdateResult(result)
}

func (r *grsaiSettlementRepository) ClaimDue(ctx context.Context, now time.Time, limit int, leaseUntil time.Time) ([]*GrsaiSettlement, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	if now.IsZero() || !leaseUntil.After(now) {
		return nil, ErrGrsaiSettlementInvalidInput
	}
	rows, err := r.sql.QueryContext(ctx, `
WITH due AS (
    SELECT id
    FROM grsai_settlements
    WHERE next_attempt_at <= $1
      AND internal_status IN ('pending_upstream', 'pending_settlement', 'processing')
    ORDER BY next_attempt_at ASC, id ASC
    LIMIT $2
    FOR UPDATE SKIP LOCKED
)
UPDATE grsai_settlements AS settlements
SET internal_status = 'processing',
    retry_count = settlements.retry_count + 1,
    claim_version = settlements.claim_version + 1,
    next_attempt_at = $3,
    updated_at = $1
FROM due
WHERE settlements.id = due.id
RETURNING `+grsaiSettlementReturningColumns("settlements"), now, limit, leaseUntil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	records := make([]*GrsaiSettlement, 0, limit)
	for rows.Next() {
		record, scanErr := scanGrsaiSettlement(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (r *grsaiSettlementRepository) MarkPendingSettlement(ctx context.Context, id, claimVersion int64, nextAttemptAt time.Time) error {
	if nextAttemptAt.IsZero() {
		return ErrGrsaiSettlementInvalidInput
	}
	return r.transition(ctx, id, claimVersion, `
UPDATE grsai_settlements
SET internal_status = 'pending_settlement',
	settlement_retry_count = settlement_retry_count + 1,
    next_attempt_at = $3,
    last_error_summary = NULL,
    updated_at = NOW()
WHERE id = $1
  AND claim_version = $2
  AND internal_status = 'processing'`, nextAttemptAt)
}

func (r *grsaiSettlementRepository) MarkHoldHeld(ctx context.Context, id, claimVersion int64) error {
	result, err := r.sql.ExecContext(ctx, `UPDATE grsai_settlements SET hold_state = 'held', updated_at = NOW() WHERE id = $1 AND claim_version = $2 AND internal_status = 'pending_upstream' AND hold_state = 'none'`, id, claimVersion)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrGrsaiSettlementClaimLost
	}
	return nil
}

// Settle locks the claimed record, applies idempotent billing through apply,
// and performs the terminal transition in the same transaction. A stale claim
// is rejected before apply is called.
func (r *grsaiSettlementRepository) Settle(
	ctx context.Context,
	id, claimVersion int64,
	settledAmount float64,
	apply GrsaiSettlementTxFunc,
) (bool, error) {
	if r.db == nil || apply == nil || !isFiniteNonNegative(settledAmount) {
		return false, ErrGrsaiSettlementInvalidInput
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	record, err := scanGrsaiSettlement(tx.QueryRowContext(ctx, grsaiSettlementSelectSQL+" WHERE id = $1 FOR UPDATE", id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, ErrGrsaiSettlementNotFound
		}
		return false, err
	}
	if record.ClaimVersion != claimVersion {
		return false, ErrGrsaiSettlementClaimLost
	}
	if record.InternalStatus == GrsaiSettlementStatusSettled {
		return false, nil
	}
	if record.InternalStatus != GrsaiSettlementStatusProcessing {
		return false, fmt.Errorf("%w: cannot settle from %s", ErrGrsaiSettlementInvalidState, record.InternalStatus)
	}
	if err := apply(ctx, tx, record); err != nil {
		return false, err
	}
	result, err := tx.ExecContext(ctx, `
UPDATE grsai_settlements
SET internal_status = 'settled',
    hold_state = CASE WHEN hold_state = 'held' THEN 'captured' ELSE hold_state END,
    settled_amount = $3,
    settled_at = NOW(),
    closed_at = NOW(),
    last_error_summary = NULL,
    updated_at = NOW()
WHERE id = $1
  AND claim_version = $2
  AND internal_status = 'processing'`, id, claimVersion, settledAmount)
	if err != nil {
		return false, err
	}
	if _, err := claimedUpdateResult(result); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (r *grsaiSettlementRepository) CloseNoChargeWithRelease(ctx context.Context, id, claimVersion int64, summary string, release service.GrsaiSettlementTxFunc) error {
	return r.terminalWithHold(ctx, id, claimVersion, summary, release, "closed_no_charge")
}

func (r *grsaiSettlementRepository) MarkManualReviewWithRelease(ctx context.Context, id, claimVersion int64, summary string, release service.GrsaiSettlementTxFunc) error {
	return r.terminalWithHold(ctx, id, claimVersion, summary, release, "manual_review")
}

func (r *grsaiSettlementRepository) terminalWithHold(ctx context.Context, id, claimVersion int64, summary string, release service.GrsaiSettlementTxFunc, status string) error {
	if r.db == nil || release == nil {
		return service.ErrGrsaiSettlementInvalidInput
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	record, err := scanGrsaiSettlement(tx.QueryRowContext(ctx, grsaiSettlementSelectSQL+" WHERE id = $1 FOR UPDATE", id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrGrsaiSettlementNotFound
		}
		return err
	}
	if record.ClaimVersion != claimVersion || record.InternalStatus != GrsaiSettlementStatusProcessing {
		return service.ErrGrsaiSettlementClaimLost
	}
	if record.HoldState == "held" {
		if err := release(ctx, tx, record); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE grsai_settlements SET internal_status = $3, hold_state = CASE WHEN hold_state = 'held' THEN 'released' ELSE hold_state END, settled_amount = 0, last_error_summary = NULLIF($4,''), closed_at = NOW(), updated_at = NOW() WHERE id = $1 AND claim_version = $2 AND internal_status = 'processing'`, id, claimVersion, status, summary)
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *grsaiSettlementRepository) CloseNoCharge(ctx context.Context, id, claimVersion int64, summary string) error {
	return r.transition(ctx, id, claimVersion, `
UPDATE grsai_settlements
SET internal_status = 'closed_no_charge',
    settled_amount = 0,
    last_error_summary = NULLIF($3, ''),
    closed_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND claim_version = $2
  AND (internal_status = 'processing' OR (internal_status = 'pending_upstream' AND $2 = 0))`, summary)
}

func (r *grsaiSettlementRepository) MarkManualReview(ctx context.Context, id, claimVersion int64, summary string) error {
	return r.transition(ctx, id, claimVersion, `
UPDATE grsai_settlements
SET internal_status = 'manual_review',
    last_error_summary = NULLIF($3, ''),
    closed_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND claim_version = $2
  AND internal_status = 'processing'`, summary)
}

func (r *grsaiSettlementRepository) transition(ctx context.Context, id, claimVersion int64, query string, arg any) error {
	result, err := r.sql.ExecContext(ctx, query, id, claimVersion, arg)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrGrsaiSettlementClaimLost
	}
	return nil
}

const grsaiSettlementSelectSQL = `
SELECT id, account_id, group_id, user_id, api_key_id, model,
       base_unit_price, group_rate_multiplier, account_rate_multiplier, billable_unit_price,
	       requested_image_count, image_size, currency, billing_idempotency_key,
	       public_task_id, delivery_mode, progress, result_urls, hold_amount, hold_state,
	       payload_delete_after, expires_at, upstream_task_id,
	       upstream_status, internal_status, retry_count, settlement_retry_count, claim_version, next_attempt_at, last_error_summary,
       settled_amount, created_at, updated_at, upstream_bound_at, result_updated_at,
       settled_at, closed_at
FROM grsai_settlements`

const grsaiSettlementInsertSQL = `
WITH new_id AS (
    SELECT nextval(pg_get_serial_sequence('grsai_settlements', 'id')) AS id
)
INSERT INTO grsai_settlements (
    id, account_id, group_id, user_id, api_key_id, model,
    base_unit_price, group_rate_multiplier, account_rate_multiplier, billable_unit_price,
	    requested_image_count, image_size, currency, billing_idempotency_key,
	    public_task_id, delivery_mode, progress, result_urls, hold_amount, hold_state,
	    payload_delete_after, expires_at, upstream_task_id,
    upstream_status, next_attempt_at, upstream_bound_at
) VALUES (
    (SELECT id FROM new_id), $1, $2, $3, $4, $5,
    $6, $7, $8, $9,
	    $10, $11, $12, CONCAT('grsai_settlement:', (SELECT id FROM new_id)),
	    $13, $14, $15, $16::jsonb, $17, $18,
	    $19, $20, $21,
	    $22, $23, CASE WHEN $21::varchar IS NULL THEN NULL ELSE NOW() END
)
RETURNING id, account_id, group_id, user_id, api_key_id, model,
          base_unit_price, group_rate_multiplier, account_rate_multiplier, billable_unit_price,
	      requested_image_count, image_size, currency, billing_idempotency_key,
	      public_task_id, delivery_mode, progress, result_urls, hold_amount, hold_state,
	      payload_delete_after, expires_at, upstream_task_id,
	          upstream_status, internal_status, retry_count, settlement_retry_count, claim_version, next_attempt_at, last_error_summary,
          settled_amount, created_at, updated_at, upstream_bound_at, result_updated_at,
          settled_at, closed_at`

func grsaiSettlementReturningColumns(alias string) string {
	columns := []string{
		"id", "account_id", "group_id", "user_id", "api_key_id", "model",
		"base_unit_price", "group_rate_multiplier", "account_rate_multiplier", "billable_unit_price",
		"requested_image_count", "image_size", "currency", "billing_idempotency_key",
		"public_task_id", "delivery_mode", "progress", "result_urls", "hold_amount", "hold_state",
		"payload_delete_after", "expires_at", "upstream_task_id",
		"upstream_status", "internal_status", "retry_count", "settlement_retry_count", "claim_version", "next_attempt_at", "last_error_summary",
		"settled_amount", "created_at", "updated_at", "upstream_bound_at", "result_updated_at",
		"settled_at", "closed_at",
	}
	for i := range columns {
		columns[i] = alias + "." + columns[i]
	}
	return strings.Join(columns, ", ")
}

type grsaiSettlementScanner interface {
	Scan(dest ...any) error
}

func scanGrsaiSettlement(scanner grsaiSettlementScanner) (*GrsaiSettlement, error) {
	record := &GrsaiSettlement{}
	var publicTaskID sql.NullString
	var deliveryMode string
	var resultURLsJSON []byte
	var payloadDeleteAfter sql.NullTime
	var expiresAt sql.NullTime
	var upstreamTaskID sql.NullString
	var lastErrorSummary sql.NullString
	var settledAmount sql.NullFloat64
	var upstreamBoundAt sql.NullTime
	var resultUpdatedAt sql.NullTime
	var settledAt sql.NullTime
	var closedAt sql.NullTime
	err := scanner.Scan(
		&record.ID,
		&record.AccountID,
		&record.GroupID,
		&record.UserID,
		&record.APIKeyID,
		&record.Model,
		&record.BaseUnitPrice,
		&record.GroupRateMultiplier,
		&record.AccountRateMultiplier,
		&record.BillableUnitPrice,
		&record.RequestedImageCount,
		&record.ImageSize,
		&record.Currency,
		&record.BillingIdempotencyKey,
		&publicTaskID,
		&deliveryMode,
		&record.Progress,
		&resultURLsJSON,
		&record.HoldAmount,
		&record.HoldState,
		&payloadDeleteAfter,
		&expiresAt,
		&upstreamTaskID,
		&record.UpstreamStatus,
		&record.InternalStatus,
		&record.RetryCount,
		&record.SettlementRetryCount,
		&record.ClaimVersion,
		&record.NextAttemptAt,
		&lastErrorSummary,
		&settledAmount,
		&record.CreatedAt,
		&record.UpdatedAt,
		&upstreamBoundAt,
		&resultUpdatedAt,
		&settledAt,
		&closedAt,
	)
	if err != nil {
		return nil, err
	}
	if upstreamTaskID.Valid {
		record.UpstreamTaskID = &upstreamTaskID.String
	}
	if publicTaskID.Valid {
		record.PublicTaskID = publicTaskID.String
	}
	record.DeliveryMode = service.GrsaiDeliveryMode(deliveryMode)
	if len(resultURLsJSON) == 0 {
		record.ResultURLs = []string{}
	} else if err := json.Unmarshal(resultURLsJSON, &record.ResultURLs); err != nil {
		return nil, fmt.Errorf("decode grsai result URLs: %w", err)
	}
	if payloadDeleteAfter.Valid {
		record.PayloadDeleteAfter = &payloadDeleteAfter.Time
	}
	if expiresAt.Valid {
		record.ExpiresAt = &expiresAt.Time
	}
	if lastErrorSummary.Valid {
		record.LastErrorSummary = &lastErrorSummary.String
	}
	if settledAmount.Valid {
		record.SettledAmount = &settledAmount.Float64
	}
	if upstreamBoundAt.Valid {
		record.UpstreamBoundAt = &upstreamBoundAt.Time
	}
	if resultUpdatedAt.Valid {
		record.ResultUpdatedAt = &resultUpdatedAt.Time
	}
	if settledAt.Valid {
		record.SettledAt = &settledAt.Time
	}
	if closedAt.Valid {
		record.ClosedAt = &closedAt.Time
	}
	return record, nil
}

func claimedUpdateResult(result sql.Result) (bool, error) {
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected == 0 {
		return false, ErrGrsaiSettlementClaimLost
	}
	return true, nil
}

func isFiniteNonNegative(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
