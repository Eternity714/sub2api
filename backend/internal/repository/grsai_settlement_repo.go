package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	GrsaiSettlementStatusPendingUpstream   = "pending_upstream"
	GrsaiSettlementStatusPendingSettlement = "pending_settlement"
	GrsaiSettlementStatusProcessing        = "processing"
	GrsaiSettlementStatusSettled           = "settled"
	GrsaiSettlementStatusClosedNoCharge     = "closed_no_charge"
	GrsaiSettlementStatusManualReview       = "manual_review"
)

var (
	ErrGrsaiSettlementNotFound     = errors.New("grsai settlement not found")
	ErrGrsaiSettlementInvalidInput = errors.New("invalid grsai settlement input")
	ErrGrsaiSettlementInvalidState = errors.New("invalid grsai settlement state")
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

type CreateGrsaiSettlementParams struct {
	AccountID            int64
	GroupID              int64
	UserID               int64
	APIKeyID             int64
	Model                string
	BaseUnitPrice         float64
	GroupRateMultiplier   float64
	AccountRateMultiplier float64
	BillableUnitPrice     float64
	RequestedImageCount   int
	Currency              string
	BillingIdempotencyKey string
	UpstreamTaskID        *string
	UpstreamStatus        string
	NextAttemptAt         time.Time
}

type GrsaiSettlement struct {
	ID                    int64
	AccountID             int64
	GroupID               int64
	UserID                int64
	APIKeyID              int64
	Model                 string
	BaseUnitPrice         float64
	GroupRateMultiplier   float64
	AccountRateMultiplier float64
	BillableUnitPrice     float64
	RequestedImageCount   int
	Currency              string
	BillingIdempotencyKey string
	UpstreamTaskID        *string
	UpstreamStatus        string
	InternalStatus        string
	RetryCount            int
	NextAttemptAt         time.Time
	LastErrorSummary      *string
	SettledAmount         *float64
	CreatedAt             time.Time
	UpdatedAt             time.Time
	UpstreamBoundAt       *time.Time
	ResultUpdatedAt       *time.Time
	SettledAt             *time.Time
	ClosedAt              *time.Time
}

func NewGrsaiSettlementRepository(db *sql.DB) *grsaiSettlementRepository {
	return &grsaiSettlementRepository{db: db, sql: db}
}

func (r *grsaiSettlementRepository) Create(ctx context.Context, params CreateGrsaiSettlementParams) (*GrsaiSettlement, error) {
	params.Model = strings.TrimSpace(params.Model)
	params.BillingIdempotencyKey = strings.TrimSpace(params.BillingIdempotencyKey)
	if params.AccountID <= 0 || params.GroupID <= 0 || params.UserID <= 0 || params.APIKeyID <= 0 ||
		params.Model == "" || params.BillingIdempotencyKey == "" || params.RequestedImageCount <= 0 {
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
		params.Currency,
		params.BillingIdempotencyKey,
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

func (r *grsaiSettlementRepository) BindUpstreamTask(ctx context.Context, id int64, taskID, upstreamStatus string) (bool, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return false, ErrGrsaiSettlementInvalidInput
	}
	if upstreamStatus == "" {
		upstreamStatus = "queued"
	}
	result, err := r.sql.ExecContext(ctx, `
UPDATE grsai_settlements
SET upstream_task_id = $2,
    upstream_status = $3,
    upstream_bound_at = COALESCE(upstream_bound_at, NOW()),
    updated_at = NOW()
WHERE id = $1
  AND upstream_task_id IS NULL
  AND internal_status NOT IN ('settled', 'closed_no_charge', 'manual_review')`, id, taskID, upstreamStatus)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

func (r *grsaiSettlementRepository) UpdateResult(ctx context.Context, id int64, upstreamStatus, errorSummary string, nextAttemptAt time.Time) (bool, error) {
	upstreamStatus = strings.TrimSpace(upstreamStatus)
	if upstreamStatus == "" || nextAttemptAt.IsZero() {
		return false, ErrGrsaiSettlementInvalidInput
	}
	result, err := r.sql.ExecContext(ctx, `
UPDATE grsai_settlements
SET upstream_status = $2,
    last_error_summary = NULLIF($3, ''),
    next_attempt_at = $4,
    result_updated_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND internal_status NOT IN ('settled', 'closed_no_charge', 'manual_review')`, id, upstreamStatus, errorSummary, nextAttemptAt)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
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

func (r *grsaiSettlementRepository) MarkPendingSettlement(ctx context.Context, id int64, nextAttemptAt time.Time) error {
	if nextAttemptAt.IsZero() {
		return ErrGrsaiSettlementInvalidInput
	}
	return r.transition(ctx, id, `
UPDATE grsai_settlements
SET internal_status = 'pending_settlement',
    next_attempt_at = $2,
    last_error_summary = NULL,
    updated_at = NOW()
WHERE id = $1
  AND internal_status IN ('pending_upstream', 'processing')`, nextAttemptAt)
}

// Settle locks the record and performs the terminal transition in one
// transaction. A second worker observes the terminal state and returns false.
func (r *grsaiSettlementRepository) Settle(ctx context.Context, id int64, settledAmount float64) (bool, error) {
	if r.db == nil || settledAmount < 0 {
		return false, ErrGrsaiSettlementInvalidInput
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	var status string
	if err := tx.QueryRowContext(ctx,
		`SELECT internal_status FROM grsai_settlements WHERE id = $1 FOR UPDATE`, id).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, ErrGrsaiSettlementNotFound
		}
		return false, err
	}
	if status == GrsaiSettlementStatusSettled {
		return false, nil
	}
	if status != GrsaiSettlementStatusProcessing {
		return false, fmt.Errorf("%w: cannot settle from %s", ErrGrsaiSettlementInvalidState, status)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE grsai_settlements
SET internal_status = 'settled',
    settled_amount = $2,
    settled_at = NOW(),
    closed_at = NOW(),
    last_error_summary = NULL,
    updated_at = NOW()
WHERE id = $1`, id, settledAmount); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (r *grsaiSettlementRepository) CloseNoCharge(ctx context.Context, id int64, summary string) error {
	return r.transition(ctx, id, `
UPDATE grsai_settlements
SET internal_status = 'closed_no_charge',
    settled_amount = 0,
    last_error_summary = NULLIF($2, ''),
    closed_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND internal_status NOT IN ('settled', 'closed_no_charge', 'manual_review')`, summary)
}

func (r *grsaiSettlementRepository) MarkManualReview(ctx context.Context, id int64, summary string) error {
	return r.transition(ctx, id, `
UPDATE grsai_settlements
SET internal_status = 'manual_review',
    last_error_summary = NULLIF($2, ''),
    closed_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND internal_status NOT IN ('settled', 'closed_no_charge', 'manual_review')`, summary)
}

func (r *grsaiSettlementRepository) transition(ctx context.Context, id int64, query string, arg any) error {
	result, err := r.sql.ExecContext(ctx, query, id, arg)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrGrsaiSettlementInvalidState
	}
	return nil
}

const grsaiSettlementSelectSQL = `
SELECT id, account_id, group_id, user_id, api_key_id, model,
       base_unit_price, group_rate_multiplier, account_rate_multiplier, billable_unit_price,
       requested_image_count, currency, billing_idempotency_key, upstream_task_id,
       upstream_status, internal_status, retry_count, next_attempt_at, last_error_summary,
       settled_amount, created_at, updated_at, upstream_bound_at, result_updated_at,
       settled_at, closed_at
FROM grsai_settlements`

const grsaiSettlementInsertSQL = `
INSERT INTO grsai_settlements (
    account_id, group_id, user_id, api_key_id, model,
    base_unit_price, group_rate_multiplier, account_rate_multiplier, billable_unit_price,
    requested_image_count, currency, billing_idempotency_key, upstream_task_id,
    upstream_status, next_attempt_at, upstream_bound_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9,
    $10, $11, $12, $13,
    $14, $15, CASE WHEN $13::varchar IS NULL THEN NULL ELSE NOW() END
)
RETURNING id, account_id, group_id, user_id, api_key_id, model,
          base_unit_price, group_rate_multiplier, account_rate_multiplier, billable_unit_price,
          requested_image_count, currency, billing_idempotency_key, upstream_task_id,
          upstream_status, internal_status, retry_count, next_attempt_at, last_error_summary,
          settled_amount, created_at, updated_at, upstream_bound_at, result_updated_at,
          settled_at, closed_at`

func grsaiSettlementReturningColumns(alias string) string {
	columns := []string{
		"id", "account_id", "group_id", "user_id", "api_key_id", "model",
		"base_unit_price", "group_rate_multiplier", "account_rate_multiplier", "billable_unit_price",
		"requested_image_count", "currency", "billing_idempotency_key", "upstream_task_id",
		"upstream_status", "internal_status", "retry_count", "next_attempt_at", "last_error_summary",
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
		&record.Currency,
		&record.BillingIdempotencyKey,
		&upstreamTaskID,
		&record.UpstreamStatus,
		&record.InternalStatus,
		&record.RetryCount,
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
