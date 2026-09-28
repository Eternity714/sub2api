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
	ErrGrsaiTaskWaitingLimit       = service.ErrGrsaiTaskWaitingLimit
)

const grsaiV2CapacityLock int64 = 74319284011

const grsaiTaskVersionVideo = 3

type grsaiSettlementSQLExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type grsaiSettlementRepository struct {
	db        *sql.DB
	sql       grsaiSettlementSQLExecutor
	encryptor service.SecretEncryptor
}

type CreateGrsaiSettlementParams = service.CreateGrsaiSettlementParams
type GrsaiSettlement = service.GrsaiSettlement

// GrsaiSettlementTxFunc applies the idempotent billing side effect inside the
// same SQL transaction that marks the settlement complete. The callback must
// not commit or roll back tx.
type GrsaiSettlementTxFunc = service.GrsaiSettlementTxFunc

var _ service.GrsaiSettlementRepository = (*grsaiSettlementRepository)(nil)
var _ service.GrsaiV2TaskRepository = (*grsaiSettlementRepository)(nil)

func NewGrsaiSettlementRepository(db *sql.DB) *grsaiSettlementRepository {
	return &grsaiSettlementRepository{db: db, sql: db}
}

func NewGrsaiV2TaskRepository(db *sql.DB, encryptor service.SecretEncryptor) *grsaiSettlementRepository {
	return &grsaiSettlementRepository{db: db, sql: db, encryptor: encryptor}
}

func (r *grsaiSettlementRepository) Create(ctx context.Context, params CreateGrsaiSettlementParams) (*GrsaiSettlement, error) {
	params.Model = strings.TrimSpace(params.Model)
	params.ImageSize = service.NormalizeImageBillingTierOrDefault(params.ImageSize)
	if params.AccountID <= 0 || params.GroupID <= 0 || params.UserID <= 0 || params.APIKeyID <= 0 ||
		params.Model == "" || params.RequestedImageCount <= 0 ||
		!isFiniteNonNegative(params.BaseUnitPrice) || !isFiniteNonNegative(params.GroupRateMultiplier) ||
		!isFiniteNonNegative(params.AccountRateMultiplier) || !isFiniteNonNegative(params.BillableUnitPrice) {
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
	if params.DeliveryMode == "" {
		params.DeliveryMode = string(service.GrsaiDeliveryJSON)
	}
	if params.PublicStatus == "" {
		params.PublicStatus = "queued"
	}
	if params.TaskVersion == 0 {
		params.TaskVersion = 1
	}
	if params.TaskVersion != 1 || !validGrsaiDeliveryMode(params.DeliveryMode) {
		return nil, ErrGrsaiSettlementInvalidInput
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
		params.ImageSize,
		params.Currency,
		params.MediaKind,
		params.VideoDurationSeconds,
		params.VideoResolution,
		params.UpstreamTaskID,
		params.UpstreamStatus,
		params.NextAttemptAt,
		optionalStringArg(params.LocalTaskID),
		params.DeliveryMode,
		params.PublicStatus,
		params.Progress,
		optionalBytesArg(params.ResultJSON),
		params.LinkExpiresAt,
		optionalBytesArg(params.ImageObjectMetadata),
		params.TaskVersion,
	)
	return scanGrsaiSettlement(row)
}

func (r *grsaiSettlementRepository) CreateV2GrsaiTask(ctx context.Context, params service.CreateV2GrsaiTaskParams, reserve GrsaiSettlementTxFunc) (*GrsaiSettlement, error) {
	params.LocalTaskID = strings.TrimSpace(params.LocalTaskID)
	if r.db == nil || r.encryptor == nil || reserve == nil || params.LocalTaskID == "" ||
		!params.PayloadExpiresAt.After(time.Now()) || len(params.UpstreamPayload) == 0 {
		return nil, ErrGrsaiSettlementInvalidInput
	}
	ciphertext, err := r.encryptor.Encrypt(string(params.UpstreamPayload))
	if err != nil {
		return nil, fmt.Errorf("encrypt grsai task payload: %w", err)
	}
	if params.TaskVersion == 0 {
		params.TaskVersion = 2
	}
	if params.MediaKind == "" {
		params.MediaKind = "image"
	}
	if params.MediaKind == "video" {
		if params.TaskVersion != grsaiTaskVersionVideo || params.VideoDurationSeconds <= 0 {
			return nil, ErrGrsaiSettlementInvalidInput
		}
	} else if params.MediaKind != "image" || params.TaskVersion != 2 {
		return nil, ErrGrsaiSettlementInvalidInput
	}
	params.PublicStatus = "queued"
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, grsaiV2CapacityLock); err != nil {
		return nil, err
	}
	if params.DeliveryMode == string(service.GrsaiDeliveryAsync) && params.MaxWaiting > 0 {
		var waiting int
		err = tx.QueryRowContext(ctx, `SELECT count(*) FROM grsai_settlements
WHERE user_id = $1 AND task_version IN (2, 3) AND delivery_mode = 'async' AND internal_status = 'v2_queued'`, params.UserID).Scan(&waiting)
		if err != nil {
			return nil, err
		}
		if waiting >= params.MaxWaiting {
			return nil, ErrGrsaiTaskWaitingLimit
		}
	}
	record, err := r.createV2(ctx, tx, params)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO grsai_task_payloads (local_task_id, payload_ciphertext, expires_at) VALUES ($1, $2, $3)`,
		params.LocalTaskID, ciphertext, params.PayloadExpiresAt); err != nil {
		return nil, err
	}
	if err := reserve(ctx, tx, record); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return record, nil
}

func (r *grsaiSettlementRepository) createV2(ctx context.Context, tx *sql.Tx, params service.CreateV2GrsaiTaskParams) (*GrsaiSettlement, error) {
	params.Model = strings.TrimSpace(params.Model)
	params.ImageSize = service.NormalizeImageBillingTierOrDefault(params.ImageSize)
	if params.MediaKind == "" {
		params.MediaKind = "image"
	}
	if params.MediaKind == "video" {
		if params.TaskVersion != grsaiTaskVersionVideo || params.VideoDurationSeconds <= 0 {
			return nil, ErrGrsaiSettlementInvalidInput
		}
		params.RequestedImageCount = 1
	} else if params.MediaKind != "image" || params.TaskVersion != 2 {
		return nil, ErrGrsaiSettlementInvalidInput
	}
	if params.AccountID <= 0 || params.GroupID <= 0 || params.UserID <= 0 || params.APIKeyID <= 0 ||
		params.Model == "" || params.RequestedImageCount <= 0 || !validGrsaiDeliveryMode(params.DeliveryMode) ||
		!isFiniteNonNegative(params.BaseUnitPrice) || !isFiniteNonNegative(params.GroupRateMultiplier) ||
		!isFiniteNonNegative(params.AccountRateMultiplier) || !isFiniteNonNegative(params.BillableUnitPrice) {
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
	row := tx.QueryRowContext(ctx, grsaiSettlementV2InsertSQL,
		params.AccountID, params.GroupID, params.UserID, params.APIKeyID, params.Model,
		params.BaseUnitPrice, params.GroupRateMultiplier, params.AccountRateMultiplier, params.BillableUnitPrice,
		params.RequestedImageCount, params.ImageSize, params.Currency, params.MediaKind, params.VideoDurationSeconds, params.VideoResolution, params.UpstreamStatus,
		params.LocalTaskID, params.DeliveryMode,
		params.PublicStatus, params.Progress, params.TaskVersion, params.NextAttemptAt,
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

func (r *grsaiSettlementRepository) ClaimDueV2(ctx context.Context, now time.Time, limit int, leaseUntil time.Time, maxRunning int) ([]*GrsaiSettlement, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	if r.db == nil || now.IsZero() || !leaseUntil.After(now) || maxRunning <= 0 {
		return nil, ErrGrsaiSettlementInvalidInput
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, grsaiV2CapacityLock); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `
WITH candidates AS (
    SELECT id, user_id, internal_status, next_attempt_at,
           count(*) FILTER (WHERE internal_status = 'v2_queued') OVER (
               PARTITION BY user_id ORDER BY next_attempt_at, id) AS queued_rank
    FROM grsai_settlements
    WHERE next_attempt_at <= $1 AND task_version IN (2, 3)
      AND internal_status IN ('v2_queued', 'v2_submitting', 'v2_running', 'v2_persisting_images', 'v2_pending_settlement', 'v2_processing')
), due AS (
    SELECT settlements.id FROM grsai_settlements AS settlements
    JOIN candidates ON candidates.id = settlements.id
    WHERE candidates.internal_status <> 'v2_queued'
       OR candidates.queued_rank <= $4 - (
           SELECT count(*) FROM grsai_settlements AS active
           WHERE active.user_id = candidates.user_id AND active.task_version IN (2, 3)
             AND active.internal_status IN ('v2_submitting', 'v2_running', 'v2_persisting_images', 'v2_pending_settlement', 'v2_processing'))
    ORDER BY candidates.next_attempt_at ASC, candidates.id ASC
    LIMIT $2 FOR UPDATE OF settlements SKIP LOCKED
)
UPDATE grsai_settlements AS settlements
SET internal_status = CASE WHEN settlements.internal_status IN ('v2_queued', 'v2_submitting') THEN 'v2_submitting' ELSE 'v2_processing' END,
    submission_attempt = CASE WHEN settlements.internal_status IN ('v2_queued', 'v2_submitting') THEN settlements.submission_attempt + 1 ELSE settlements.submission_attempt END,
    retry_count = settlements.retry_count + 1,
    claim_version = settlements.claim_version + 1,
    next_attempt_at = $3,
    updated_at = $1
FROM due
WHERE settlements.id = due.id AND settlements.task_version IN (2, 3)
  AND settlements.next_attempt_at <= $1
  AND settlements.internal_status IN ('v2_queued', 'v2_submitting', 'v2_running', 'v2_persisting_images', 'v2_pending_settlement', 'v2_processing')
RETURNING `+grsaiSettlementReturningColumns("settlements"), now, limit, leaseUntil, maxRunning)
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
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return records, nil
}

func (r *grsaiSettlementRepository) BindV2UpstreamTask(ctx context.Context, id, claimVersion int64, taskID string) (bool, error) {
	taskID = strings.TrimSpace(taskID)
	if id <= 0 || claimVersion <= 0 || taskID == "" {
		return false, ErrGrsaiSettlementInvalidInput
	}
	return r.withClaimedV2Tx(ctx, id, claimVersion, func(tx *sql.Tx, record *GrsaiSettlement) error {
		if record.InternalStatus != "v2_submitting" || record.SubmissionAttempt != 1 || record.UpstreamTaskID != nil || !record.NextAttemptAt.After(time.Now()) {
			return ErrGrsaiSettlementClaimLost
		}
		result, err := tx.ExecContext(ctx, `
UPDATE grsai_settlements
SET upstream_task_id = $3,
    upstream_status = 'running',
    internal_status = 'v2_running',
    public_status = 'running',
    progress = GREATEST(progress, 5),
    upstream_bound_at = COALESCE(upstream_bound_at, NOW()),
    updated_at = NOW()
WHERE id = $1 AND claim_version = $2 AND task_version IN (2, 3)
  AND upstream_task_id IS NULL AND internal_status = 'v2_submitting' AND submission_attempt = 1
  AND next_attempt_at > NOW()`, id, claimVersion, taskID)
		if err != nil {
			return err
		}
		if _, err := claimedUpdateResult(result); err != nil {
			return err
		}
		if record.LocalTaskID == nil {
			return ErrGrsaiSettlementInvalidState
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM grsai_task_payloads WHERE local_task_id = $1`, *record.LocalTaskID)
		return err
	})
}

func (r *grsaiSettlementRepository) UpdateV2Progress(ctx context.Context, id, claimVersion int64, publicStatus string, progress int, nextAttemptAt time.Time) (bool, error) {
	publicStatus = strings.TrimSpace(publicStatus)
	if id <= 0 || claimVersion <= 0 || publicStatus != "running" || progress != 5 || nextAttemptAt.IsZero() {
		return false, ErrGrsaiSettlementInvalidInput
	}
	result, err := r.sql.ExecContext(ctx, `
UPDATE grsai_settlements
SET public_status = $3::text,
    progress = $4,
    internal_status = CASE WHEN $3::text = 'running' THEN 'v2_running' ELSE internal_status END,
    settlement_retry_count = 0,
    next_attempt_at = $5,
    result_updated_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND claim_version = $2 AND task_version IN (2, 3)
  AND upstream_task_id IS NOT NULL AND next_attempt_at > NOW()
  AND internal_status IN ('v2_processing', 'v2_running')`, id, claimVersion, publicStatus, progress, nextAttemptAt)
	if err != nil {
		return false, err
	}
	return claimedUpdateResult(result)
}

func (r *grsaiSettlementRepository) DeferV2Failure(ctx context.Context, id, claimVersion int64, nextAttemptAt time.Time) (bool, error) {
	if id <= 0 || claimVersion <= 0 || nextAttemptAt.IsZero() {
		return false, ErrGrsaiSettlementInvalidInput
	}
	result, err := r.sql.ExecContext(ctx, `UPDATE grsai_settlements SET
internal_status = 'v2_running', public_status = 'running', progress = 5,
settlement_retry_count = settlement_retry_count + 1,
next_attempt_at = $3, updated_at = NOW()
WHERE id = $1 AND claim_version = $2 AND task_version IN (2, 3)
AND upstream_task_id IS NOT NULL AND next_attempt_at > NOW()
AND internal_status IN ('v2_running', 'v2_processing')`, id, claimVersion, nextAttemptAt)
	if err != nil {
		return false, err
	}
	return claimedUpdateResult(result)
}

func (r *grsaiSettlementRepository) MarkV2ManualReview(ctx context.Context, id, claimVersion int64, reason string) (bool, error) {
	if id <= 0 || claimVersion <= 0 || strings.TrimSpace(reason) == "" {
		return false, ErrGrsaiSettlementInvalidInput
	}
	return r.withClaimedV2Tx(ctx, id, claimVersion, func(tx *sql.Tx, record *GrsaiSettlement) error {
		if record.UpstreamTaskID == nil || (record.InternalStatus != "v2_running" && record.InternalStatus != "v2_processing") {
			return ErrGrsaiSettlementInvalidState
		}
		result, err := tx.ExecContext(ctx, `UPDATE grsai_settlements SET
internal_status = 'v2_manual_review', public_status = 'manual_review',
last_error_summary = $3, updated_at = NOW()
WHERE id = $1 AND claim_version = $2 AND task_version IN (2, 3)
AND internal_status IN ('v2_running', 'v2_processing')`, id, claimVersion, reason)
		if err != nil {
			return err
		}
		_, err = claimedUpdateResult(result)
		return err
	})
}

func (r *grsaiSettlementRepository) CompleteV2(ctx context.Context, id, claimVersion int64, resultJSON, objectMetadata []byte, linkExpiresAt *time.Time, settledAmount float64, capture GrsaiSettlementTxFunc) (bool, error) {
	if id <= 0 || claimVersion <= 0 || !json.Valid(resultJSON) || !json.Valid(objectMetadata) || !isFiniteNonNegative(settledAmount) || capture == nil {
		return false, ErrGrsaiSettlementInvalidInput
	}
	return r.withClaimedV2Tx(ctx, id, claimVersion, func(tx *sql.Tx, record *GrsaiSettlement) error {
		if record.LocalTaskID == nil || record.UpstreamTaskID == nil || (record.InternalStatus != "v2_processing" && record.InternalStatus != "v2_running") {
			return ErrGrsaiSettlementInvalidState
		}
		if err := capture(ctx, tx, record); err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `
UPDATE grsai_settlements
SET public_status = 'succeeded',
    progress = 100,
    internal_status = 'v2_succeeded',
    upstream_status = 'succeeded',
    result_json = $3,
    image_object_metadata = $4,
    link_expires_at = $5,
    result_updated_at = NOW(),
    settled_amount = $6,
    settled_at = NOW(),
    closed_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND claim_version = $2 AND task_version IN (2, 3)
  AND internal_status IN ('v2_processing', 'v2_running')`, id, claimVersion, resultJSON, objectMetadata, linkExpiresAt, settledAmount)
		if err != nil {
			return err
		}
		if _, err := claimedUpdateResult(result); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM grsai_task_payloads WHERE local_task_id = $1`, *record.LocalTaskID)
		return err
	})
}

func (r *grsaiSettlementRepository) FailV2(ctx context.Context, id, claimVersion int64, code, summary string, release GrsaiSettlementTxFunc) (bool, error) {
	code = strings.TrimSpace(code)
	summary = strings.TrimSpace(summary)
	if id <= 0 || claimVersion <= 0 || code == "" || release == nil {
		return false, ErrGrsaiSettlementInvalidInput
	}
	return r.withClaimedV2Tx(ctx, id, claimVersion, func(tx *sql.Tx, record *GrsaiSettlement) error {
		if record.LocalTaskID == nil || (record.InternalStatus != "v2_processing" && record.InternalStatus != "v2_running" && record.InternalStatus != "v2_submitting") {
			return ErrGrsaiSettlementInvalidState
		}
		if err := release(ctx, tx, record); err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `
UPDATE grsai_settlements
SET public_status = 'failed',
    internal_status = 'v2_failed',
    upstream_status = 'failed',
    last_error_summary = NULLIF($3, ''),
    settled_amount = 0,
    closed_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND claim_version = $2 AND task_version IN (2, 3)
  AND internal_status IN ('v2_processing', 'v2_running', 'v2_submitting')`, id, claimVersion, code+": "+summary)
		if err != nil {
			return err
		}
		if _, err := claimedUpdateResult(result); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM grsai_task_payloads WHERE local_task_id = $1`, *record.LocalTaskID)
		return err
	})
}

func (r *grsaiSettlementRepository) withClaimedV2Tx(ctx context.Context, id, claimVersion int64, apply func(*sql.Tx, *GrsaiSettlement) error) (bool, error) {
	if r.db == nil {
		return false, ErrGrsaiSettlementInvalidInput
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	record, err := scanGrsaiSettlement(tx.QueryRowContext(ctx, grsaiSettlementSelectSQL+" WHERE id = $1 AND task_version IN (2, 3) FOR UPDATE", id))
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrGrsaiSettlementNotFound
	}
	if err != nil {
		return false, err
	}
	if record.ClaimVersion != claimVersion || !record.NextAttemptAt.After(time.Now()) {
		return false, ErrGrsaiSettlementClaimLost
	}
	if err := apply(tx, record); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (r *grsaiSettlementRepository) GetOwnedV2(ctx context.Context, userID, apiKeyID int64, localTaskID string) (*GrsaiSettlement, error) {
	localTaskID = strings.TrimSpace(localTaskID)
	if userID <= 0 || apiKeyID <= 0 || localTaskID == "" {
		return nil, ErrGrsaiSettlementNotFound
	}
	record, err := scanGrsaiSettlement(r.sql.QueryRowContext(ctx, grsaiSettlementSelectSQL+`
 WHERE local_task_id = $1 AND user_id = $2 AND api_key_id = $3 AND task_version IN (2, 3)`, localTaskID, userID, apiKeyID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGrsaiSettlementNotFound
	}
	return record, err
}

func (r *grsaiSettlementRepository) ListOwnedV2(ctx context.Context, userID, apiKeyID int64, limit, offset int) ([]*GrsaiSettlement, error) {
	if userID <= 0 || apiKeyID <= 0 || limit <= 0 || offset < 0 {
		return nil, ErrGrsaiSettlementInvalidInput
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := r.sql.QueryContext(ctx, grsaiSettlementSelectSQL+`
 WHERE user_id = $1 AND api_key_id = $2 AND task_version IN (2, 3)
 ORDER BY created_at DESC, id DESC LIMIT $3 OFFSET $4`, userID, apiKeyID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := make([]*GrsaiSettlement, 0, limit)
	for rows.Next() {
		record, scanErr := scanGrsaiSettlement(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, record)
	}
	return result, rows.Err()
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
  AND internal_status = 'processing'`, summary)
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
	       requested_image_count, image_size, currency, media_kind, video_duration_seconds, video_resolution, billing_idempotency_key, upstream_task_id,
	       upstream_status, internal_status, local_task_id, delivery_mode, public_status, progress,
	       result_json, link_expires_at, image_object_metadata, task_version, submission_attempt,
	       retry_count, settlement_retry_count, claim_version, next_attempt_at, last_error_summary,
       settled_amount, created_at, updated_at, upstream_bound_at, result_updated_at,
       settled_at, closed_at
FROM grsai_settlements`

var grsaiSettlementInsertSQL = `
WITH new_id AS (SELECT nextval(pg_get_serial_sequence('grsai_settlements', 'id')) AS id)
INSERT INTO grsai_settlements (
 id, account_id, group_id, user_id, api_key_id, model, base_unit_price, group_rate_multiplier, account_rate_multiplier, billable_unit_price,
 requested_image_count, image_size, currency, media_kind, video_duration_seconds, video_resolution, billing_idempotency_key, upstream_task_id,
 upstream_status, next_attempt_at, upstream_bound_at, local_task_id, delivery_mode, public_status, progress, result_json, link_expires_at, image_object_metadata, task_version
) VALUES (
 (SELECT id FROM new_id), $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15, CONCAT('grsai_settlement:', (SELECT id FROM new_id)), $16,
 $17,$18, CASE WHEN $16::varchar IS NULL THEN NULL ELSE NOW() END, $19,$20,$21,$22,$23,$24,$25,$26
)
RETURNING ` + grsaiSettlementReturningColumns("grsai_settlements")

var grsaiSettlementV2InsertSQL = `
WITH new_id AS (SELECT nextval(pg_get_serial_sequence('grsai_settlements', 'id')) AS id)
INSERT INTO grsai_settlements (
 id, account_id, group_id, user_id, api_key_id, model, base_unit_price, group_rate_multiplier, account_rate_multiplier, billable_unit_price,
 requested_image_count, image_size, currency, media_kind, video_duration_seconds, video_resolution, billing_idempotency_key,
 upstream_status, internal_status, next_attempt_at, local_task_id, delivery_mode, public_status, progress, task_version
) VALUES (
 (SELECT id FROM new_id), $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12, $13,$14,$15, CONCAT('grsai_settlement:', (SELECT id FROM new_id)),
 $16, 'v2_queued', $22, $17,$18,$19,$20,$21
)
RETURNING ` + grsaiSettlementReturningColumns("grsai_settlements")

func grsaiSettlementReturningColumns(alias string) string {
	columns := []string{
		"id", "account_id", "group_id", "user_id", "api_key_id", "model",
		"base_unit_price", "group_rate_multiplier", "account_rate_multiplier", "billable_unit_price",
		"requested_image_count", "image_size", "currency", "media_kind", "video_duration_seconds", "video_resolution", "billing_idempotency_key", "upstream_task_id",
		"upstream_status", "internal_status", "local_task_id", "delivery_mode", "public_status", "progress",
		"result_json", "link_expires_at", "image_object_metadata", "task_version", "submission_attempt",
		"retry_count", "settlement_retry_count", "claim_version", "next_attempt_at", "last_error_summary",
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
	var localTaskID sql.NullString
	var resultJSON []byte
	var linkExpiresAt sql.NullTime
	var imageObjectMetadata []byte
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
		&record.MediaKind,
		&record.VideoDurationSeconds,
		&record.VideoResolution,
		&record.BillingIdempotencyKey,
		&upstreamTaskID,
		&record.UpstreamStatus,
		&record.InternalStatus,
		&localTaskID,
		&record.DeliveryMode,
		&record.PublicStatus,
		&record.Progress,
		&resultJSON,
		&linkExpiresAt,
		&imageObjectMetadata,
		&record.TaskVersion,
		&record.SubmissionAttempt,
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
	if localTaskID.Valid {
		record.LocalTaskID = &localTaskID.String
	}
	if resultJSON != nil {
		record.ResultJSON = append([]byte(nil), resultJSON...)
	}
	if linkExpiresAt.Valid {
		record.LinkExpiresAt = &linkExpiresAt.Time
	}
	if imageObjectMetadata != nil {
		record.ImageObjectMetadata = append([]byte(nil), imageObjectMetadata...)
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

func optionalStringArg(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func optionalBytesArg(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

func validGrsaiDeliveryMode(value string) bool {
	switch service.GrsaiDeliveryMode(value) {
	case service.GrsaiDeliveryJSON, service.GrsaiDeliveryStream, service.GrsaiDeliveryAsync:
		return true
	default:
		return false
	}
}
