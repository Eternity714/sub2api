package repository

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
)

type unifiedMediaTaskRepository struct {
	db        *sql.DB
	encryptor service.SecretEncryptor
}

var _ service.UnifiedMediaTaskRepository = (*unifiedMediaTaskRepository)(nil)

func NewUnifiedMediaTaskRepository(db *sql.DB, encryptor service.SecretEncryptor) service.UnifiedMediaTaskRepository {
	return &unifiedMediaTaskRepository{db: db, encryptor: encryptor}
}

const unifiedMediaTaskColumns = `id,user_id,api_key_id,group_id,model,media_type,status,requested_count,unit_price,account_unit_price,held_amount,charged_amount,claim_version,created_at,updated_at,completed_at,expires_at,deadline,units,payload_ciphertext`

func (r *unifiedMediaTaskRepository) scan(scanner mediaCapabilityScanner, decrypt bool) (*service.UnifiedMediaTask, error) {
	var task service.UnifiedMediaTask
	var completed, expires sql.NullTime
	var units []byte
	var ciphertext string
	err := scanner.Scan(&task.ID, &task.UserID, &task.APIKeyID, &task.GroupID, &task.Model, &task.MediaType, &task.Status, &task.RequestedCount, &task.UnitPrice, &task.AccountUnitPrice, &task.HeldAmount, &task.ChargedAmount, &task.ClaimVersion, &task.CreatedAt, &task.UpdatedAt, &completed, &expires, &task.Deadline, &units, &ciphertext)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrUnifiedMediaTaskNotFound
	}
	if err != nil {
		return nil, err
	}
	if completed.Valid {
		task.CompletedAt = &completed.Time
	}
	if expires.Valid {
		task.ExpiresAt = &expires.Time
	}
	if err = json.Unmarshal(units, &task.Units); err != nil {
		return nil, err
	}
	if decrypt {
		plain, err := r.encryptor.Decrypt(ciphertext)
		if err != nil {
			task.PayloadError = "decryption_failed"
			return &task, nil
		}
		decoder := json.NewDecoder(bytes.NewReader([]byte(plain)))
		decoder.UseNumber()
		if err = decoder.Decode(&task.Payload); err != nil {
			task.PayloadError = "decoding_failed"
			task.Payload = service.UnifiedMediaTaskPayload{}
			return &task, nil
		}
	}
	return &task, nil
}
func (r *unifiedMediaTaskRepository) Create(ctx context.Context, task *service.UnifiedMediaTask) error {
	if r.encryptor == nil {
		return errors.New("media payload encryption is unavailable")
	}
	raw, err := json.Marshal(task.Payload)
	if err != nil {
		return err
	}
	encrypted, err := r.encryptor.Encrypt(string(raw))
	if err != nil {
		return err
	}
	units, err := json.Marshal(task.Units)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var balance float64
	var userStatus string
	if err = tx.QueryRowContext(ctx, `SELECT balance,status FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, task.UserID).Scan(&balance, &userStatus); err != nil {
		return err
	}
	if userStatus != service.StatusActive {
		return infraerrors.Forbidden("USER_INACTIVE", "user is inactive")
	}
	var quota, used, l5, l1, l7, u5, u1, u7 float64
	var keyStatus string
	var groupID sql.NullInt64
	var expiry sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT quota,quota_used,status,group_id,expires_at,rate_limit_5h,rate_limit_1d,rate_limit_7d,CASE WHEN window_5h_start IS NULL OR window_5h_start+INTERVAL '5 hours'<=NOW() THEN 0 ELSE usage_5h END,CASE WHEN window_1d_start IS NULL OR window_1d_start+INTERVAL '24 hours'<=NOW() THEN 0 ELSE usage_1d END,CASE WHEN window_7d_start IS NULL OR window_7d_start+INTERVAL '7 days'<=NOW() THEN 0 ELSE usage_7d END FROM api_keys WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL FOR UPDATE`, task.APIKeyID, task.UserID).Scan(&quota, &used, &keyStatus, &groupID, &expiry, &l5, &l1, &l7, &u5, &u1, &u7)
	if err != nil {
		return err
	}
	if keyStatus != service.StatusAPIKeyActive || !groupID.Valid || groupID.Int64 != task.GroupID || expiry.Valid && !time.Now().Before(expiry.Time) {
		return infraerrors.Forbidden("API_KEY_UNAVAILABLE", "API Key is inactive, expired, or changed groups")
	}
	waitingLimit := task.WaitingLimit
	if waitingLimit <= 0 {
		waitingLimit = 20
	}
	var waiting int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM unified_media_tasks WHERE api_key_id=$1 AND hold_status='reserved'", task.APIKeyID).Scan(&waiting); err != nil {
		return err
	}
	if waiting >= waitingLimit {
		return infraerrors.New(429, "MEDIA_WAITING_LIMIT", "too many unfinished media tasks for this API Key")
	}
	var held float64
	err = tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT SUM(held_amount) FROM unified_media_tasks WHERE api_key_id=$1 AND hold_status='reserved'),0)+COALESCE((SELECT SUM(amount) FROM grsai_balance_holds WHERE api_key_id=$1 AND status='reserved'),0)`, task.APIKeyID).Scan(&held)
	if err != nil {
		return err
	}
	for _, pair := range [][2]float64{{quota, used}, {l5, u5}, {l1, u1}, {l7, u7}} {
		if unifiedMediaQuotaExceeded(pair[0], pair[1], held, task.HeldAmount) {
			return service.ErrUnifiedMediaKeyQuota
		}
	}
	if decimal.NewFromFloat(balance).LessThan(decimal.NewFromFloat(task.HeldAmount)) {
		return service.ErrUnifiedMediaInsufficientBalance
	}
	result, err := tx.ExecContext(ctx, `UPDATE users SET balance=balance-$1,frozen_balance=COALESCE(frozen_balance,0)+$1,updated_at=NOW() WHERE id=$2 AND deleted_at IS NULL AND balance>=$1`, task.HeldAmount, task.UserID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return service.ErrUnifiedMediaInsufficientBalance
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO unified_media_tasks(id,user_id,api_key_id,group_id,model,media_type,status,requested_count,unit_price,account_unit_price,held_amount,payload_ciphertext,units,deadline,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,'queued',$7,$8,$9,$10,$11,$12::jsonb,$13,$14,$14)`, task.ID, task.UserID, task.APIKeyID, task.GroupID, task.Model, task.MediaType, task.RequestedCount, task.UnitPrice, task.AccountUnitPrice, task.HeldAmount, encrypted, string(units), task.Deadline, task.CreatedAt)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func unifiedMediaQuotaExceeded(limit, used, held, amount float64) bool {
	return limit > 0 && decimal.NewFromFloat(used).Add(decimal.NewFromFloat(held)).Add(decimal.NewFromFloat(amount)).GreaterThan(decimal.NewFromFloat(limit))
}
func (r *unifiedMediaTaskRepository) GetOwned(ctx context.Context, userID, keyID int64, mediaType, id string) (*service.UnifiedMediaTask, error) {
	return r.scan(r.db.QueryRowContext(ctx, `SELECT `+unifiedMediaTaskColumns+` FROM unified_media_tasks WHERE id=$1 AND user_id=$2 AND api_key_id=$3 AND media_type=$4`, id, userID, keyID, mediaType), false)
}
func (r *unifiedMediaTaskRepository) ClaimDue(ctx context.Context, now time.Time, limit int, lease time.Time) ([]*service.UnifiedMediaTask, error) {
	if limit < 1 {
		limit = 4
	}
	columns := "t." + strings.ReplaceAll(unifiedMediaTaskColumns, ",", ",t.")
	rows, err := r.db.QueryContext(ctx, `WITH due AS (SELECT id FROM unified_media_tasks WHERE status<>'completed' AND next_attempt_at<=$1 AND (lease_until IS NULL OR lease_until<$1) ORDER BY next_attempt_at,id FOR UPDATE SKIP LOCKED LIMIT $2) UPDATE unified_media_tasks t SET status='in_progress',claim_version=claim_version+1,lease_until=$3,updated_at=$1 FROM due WHERE t.id=due.id RETURNING `+columns, now, limit, lease)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*service.UnifiedMediaTask, 0)
	for rows.Next() {
		task, err := r.scan(rows, true)
		if err != nil {
			return nil, err
		}
		out = append(out, task)
	}
	return out, rows.Err()
}
func (r *unifiedMediaTaskRepository) Save(ctx context.Context, task *service.UnifiedMediaTask, next time.Time) error {
	raw, err := json.Marshal(task.Units)
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `UPDATE unified_media_tasks SET units=$1::jsonb,next_attempt_at=$2,lease_until=$3,updated_at=NOW() WHERE id=$4 AND claim_version=$5 AND status<>'completed' AND lease_until>NOW()`, string(raw), next, time.Now().Add(30*time.Minute), task.ID, task.ClaimVersion)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return service.ErrUnifiedMediaClaimLost
	}
	return nil
}
func lockUnifiedMediaTask(ctx context.Context, tx *sql.Tx, task *service.UnifiedMediaTask) error {
	var version int64
	var lease sql.NullTime
	var status string
	err := tx.QueryRowContext(ctx, `SELECT claim_version,lease_until,status FROM unified_media_tasks WHERE id=$1 FOR UPDATE`, task.ID).Scan(&version, &lease, &status)
	if err != nil {
		return err
	}
	if version != task.ClaimVersion || status == "completed" || !lease.Valid || !time.Now().Before(lease.Time) {
		return service.ErrUnifiedMediaClaimLost
	}
	return nil
}
func (r *unifiedMediaTaskRepository) ReserveAccount(ctx context.Context, task *service.UnifiedMediaTask, index int, account *service.Account, amount float64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockUnifiedMediaTask(ctx, tx, task); err != nil {
		return err
	}
	var platform, status string
	var concurrency int
	var schedulable, autoPause bool
	var expires sql.NullTime
	var extra []byte
	var totalLimit, totalUsed, dailyLimit, dailyUsed, weeklyLimit, weeklyUsed float64
	err = tx.QueryRowContext(ctx, `SELECT platform,status,schedulable,auto_pause_on_expired,expires_at,extra,concurrency,COALESCE((extra->>'quota_limit')::numeric,0),COALESCE((extra->>'quota_used')::numeric,0),COALESCE((extra->>'quota_daily_limit')::numeric,0),CASE WHEN `+dailyExpiredExpr+` THEN 0 ELSE COALESCE((extra->>'quota_daily_used')::numeric,0) END,COALESCE((extra->>'quota_weekly_limit')::numeric,0),CASE WHEN `+weeklyExpiredExpr+` THEN 0 ELSE COALESCE((extra->>'quota_weekly_used')::numeric,0) END FROM accounts WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, account.ID).Scan(&platform, &status, &schedulable, &autoPause, &expires, &extra, &concurrency, &totalLimit, &totalUsed, &dailyLimit, &dailyUsed, &weeklyLimit, &weeklyUsed)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrUnifiedMediaAccountUnavailable
	}
	if err != nil {
		return err
	}
	if platform != account.Platform || status != service.StatusActive || !schedulable || autoPause && expires.Valid && !time.Now().Before(expires.Time) {
		return service.ErrUnifiedMediaAccountUnavailable
	}
	var member bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM account_groups WHERE account_id=$1 AND group_id=$2)`, account.ID, task.GroupID).Scan(&member); err != nil {
		return err
	}
	if !member {
		return service.ErrUnifiedMediaAccountUnavailable
	}
	var active int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM unified_media_account_holds WHERE account_id=$1 AND status='reserved'", account.ID).Scan(&active); err != nil {
		return err
	}
	if concurrency <= 0 {
		return service.ErrUnifiedMediaAccountUnavailable
	}
	var held float64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount),0) FROM unified_media_account_holds WHERE account_id=$1 AND status='reserved'`, account.ID).Scan(&held); err != nil {
		return err
	}
	for _, pair := range [][2]float64{{totalLimit, totalUsed}, {dailyLimit, dailyUsed}, {weeklyLimit, weeklyUsed}} {
		if unifiedMediaQuotaExceeded(pair[0], pair[1], held, amount) {
			return service.ErrUnifiedMediaAccountUnavailable
		}
	}
	if active >= concurrency {
		return service.ErrUnifiedMediaAccountBusy
	}
	unit := task.Units[index]
	_, err = tx.ExecContext(ctx, `INSERT INTO unified_media_account_holds(task_id,unit_index,attempt,account_id,amount) VALUES($1,$2,$3,$4,$5)`, task.ID, index, unit.Attempt, account.ID, amount)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(task.Units)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE unified_media_tasks SET units=$1::jsonb,lease_until=$2,updated_at=NOW() WHERE id=$3 AND claim_version=$4`, string(raw), time.Now().Add(30*time.Minute), task.ID, task.ClaimVersion)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (r *unifiedMediaTaskRepository) SettleAccount(ctx context.Context, task *service.UnifiedMediaTask, index int, consume bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockUnifiedMediaTask(ctx, tx, task); err != nil {
		return err
	}
	unit := task.Units[index]
	if err = settleUnifiedMediaAccountHold(ctx, tx, task.ID, index, unit.Attempt, consume); err != nil {
		return err
	}
	return tx.Commit()
}
func settleUnifiedMediaAccountHold(ctx context.Context, tx *sql.Tx, taskID string, index, attempt int, consume bool) error {
	var accountID int64
	var amount float64
	var status string
	err := tx.QueryRowContext(ctx, `SELECT account_id,amount,status FROM unified_media_account_holds WHERE task_id=$1 AND unit_index=$2 AND attempt=$3 FOR UPDATE`, taskID, index, attempt).Scan(&accountID, &amount, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if status != "reserved" {
		return nil
	}
	target := "released"
	if consume {
		target = "consumed"
		if amount > 0 {
			if err = incrementUnifiedMediaAccountUsage(ctx, tx, accountID, amount); err != nil {
				return err
			}
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE unified_media_account_holds SET status=$1,updated_at=NOW() WHERE task_id=$2 AND unit_index=$3 AND attempt=$4 AND status='reserved'`, target, taskID, index, attempt)
	return err
}
func (r *unifiedMediaTaskRepository) Finalize(ctx context.Context, task *service.UnifiedMediaTask) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status, holdStatus string
	var fence, userID, keyID int64
	var held float64
	var lease sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT status,hold_status,claim_version,user_id,api_key_id,held_amount,lease_until FROM unified_media_tasks WHERE id=$1 FOR UPDATE`, task.ID).Scan(&status, &holdStatus, &fence, &userID, &keyID, &held, &lease)
	if err != nil {
		return err
	}
	if status == "completed" {
		task.Status = "completed"
		return nil
	}
	if fence != task.ClaimVersion || holdStatus != "reserved" || !lease.Valid || !time.Now().Before(lease.Time) {
		return service.ErrUnifiedMediaClaimLost
	}
	for _, unit := range task.Units {
		if unit.State != "completed" && unit.State != "failed" {
			return errors.New("cannot finalize unfinished media units")
		}
	}
	amount := service.UnifiedMediaTaskCharge(task)
	if amount < 0 || decimal.NewFromFloat(amount).GreaterThan(decimal.NewFromFloat(held)) {
		return errors.New("media charge exceeds reservation")
	}
	var balance float64
	if err = tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&balance); err != nil {
		return err
	}
	var keyExists int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM api_keys WHERE id=$1 FOR UPDATE`, keyID).Scan(&keyExists); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1-$2,frozen_balance=COALESCE(frozen_balance,0)-$1,updated_at=NOW() WHERE id=$3 AND COALESCE(frozen_balance,0)>=$1`, held, amount, userID)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return errors.New("media frozen balance is inconsistent")
	}
	if amount > 0 {
		if err = incrementUnifiedMediaKeyUsage(ctx, tx, keyID, amount); err != nil {
			return err
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT unit_index,attempt FROM unified_media_account_holds WHERE task_id=$1 AND status='reserved' ORDER BY unit_index,attempt`, task.ID)
	if err != nil {
		return err
	}
	type holdKey struct{ index, attempt int }
	var pending []holdKey
	for rows.Next() {
		var key holdKey
		if err = rows.Scan(&key.index, &key.attempt); err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, key)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err = rows.Close(); err != nil {
		return err
	}
	for _, key := range pending {
		if err = settleUnifiedMediaAccountHold(ctx, tx, task.ID, key.index, key.attempt, true); err != nil {
			return err
		}
	}
	now := time.Now().UTC()
	if err = writeUnifiedMediaUsage(ctx, tx, task, now); err != nil {
		return err
	}
	expiry := service.UnifiedMediaTaskResultExpiry(task)
	units, err := json.Marshal(task.Units)
	if err != nil {
		return err
	}
	holdTarget := "released"
	if amount > 0 {
		holdTarget = "captured"
	}
	_, err = tx.ExecContext(ctx, `UPDATE unified_media_tasks SET status='completed',hold_status=$1,charged_amount=$2,units=$3::jsonb,completed_at=$4,expires_at=$5,lease_until=NULL,updated_at=$4 WHERE id=$6 AND claim_version=$7`, holdTarget, amount, string(units), now, expiry, task.ID, task.ClaimVersion)
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	task.Status = "completed"
	task.CompletedAt = &now
	task.ExpiresAt = expiry
	task.ChargedAmount = amount
	return nil
}

func (r *unifiedMediaTaskRepository) ReleaseClaim(ctx context.Context, task *service.UnifiedMediaTask, next time.Time) error {
	result, err := r.db.ExecContext(ctx, `UPDATE unified_media_tasks SET next_attempt_at=$1,lease_until=NULL,updated_at=NOW() WHERE id=$2 AND claim_version=$3 AND status<>'completed'`, next, task.ID, task.ClaimVersion)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return service.ErrUnifiedMediaClaimLost
	}
	return nil
}

func writeUnifiedMediaUsage(ctx context.Context, tx *sql.Tx, task *service.UnifiedMediaTask, now time.Time) error {
	billing := &usageBillingRepository{}
	command := &service.UsageBillingCommand{RequestID: "media_settle:" + task.ID, APIKeyID: task.APIKeyID, UserID: task.UserID, Model: task.Model, MediaType: task.MediaType, APIKeyQuotaCost: service.UnifiedMediaTaskCharge(task), APIKeyRateLimitCost: service.UnifiedMediaTaskCharge(task), RequestPayloadHash: service.HashUsageRequestPayload([]byte(task.ID))}
	command.Normalize()
	applied, err := billing.claimUsageBillingKey(ctx, tx, command)
	if err != nil {
		return err
	}
	if !applied {
		return service.ErrUsageBillingRequestConflict
	}
	var resolution any
	var duration any
	if task.Payload.Normalized != nil {
		if metadata, ok := task.Payload.Normalized.Values["metadata"].(map[string]any); ok {
			if text, ok := metadata["resolution"].(string); ok && len(text) <= 10 {
				resolution = text
			}
		}
		if value, ok := task.Payload.Normalized.Values["duration"].(json.Number); ok {
			if n, err := value.Int64(); err == nil && n > 0 && n <= 2147483647 {
				duration = n
			}
		}
	}
	endpoint := "/v1/" + task.MediaType + "s/generations"
	for index, unit := range task.Units {
		count := len(unit.Stored)
		if count == 0 {
			continue
		}
		if unit.AccountID <= 0 {
			return errors.New("successful media result has no account attribution")
		}
		actual, _ := decimal.NewFromFloat(task.UnitPrice).Mul(decimal.NewFromInt(int64(count))).Round(8).Float64()
		base, _ := decimal.NewFromFloat(task.AccountUnitPrice).Mul(decimal.NewFromInt(int64(count))).Round(8).Float64()
		rate := 1.0
		if task.AccountUnitPrice > 0 {
			rate = task.UnitPrice / task.AccountUnitPrice
		}
		accountRate := 1.0
		if unit.AccountRateMultiplier != nil {
			accountRate = *unit.AccountRateMultiplier
		}
		imageCount, videoCount := count, 0
		var imageSize, videoResolution, videoDuration any
		if task.MediaType == "video" {
			imageCount = 0
			videoCount = count
			videoResolution = resolution
			videoDuration = duration
		} else {
			imageSize = service.NormalizeImageBillingTierOrDefault(fmt.Sprint(resolution))
		}
		requestID := fmt.Sprintf("media_task:%s:%d", task.ID, index)
		_, err = tx.ExecContext(ctx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,group_id,request_id,model,requested_model,upstream_model,upstream_request_id,billing_mode,billing_type,request_type,image_count,image_size,image_size_source,video_count,video_resolution,video_duration_seconds,total_cost,actual_cost,rate_multiplier,account_rate_multiplier,inbound_endpoint,upstream_endpoint,created_at) VALUES($1,$2,$3,$4,$5,$6,$6,$7,$8,$9,0,1,$10,$11,CASE WHEN $10>0 THEN 'default' ELSE NULL END,$12,$13,$14,$15,$16,$17,$18,$19,$19,$20)`, task.UserID, task.APIKeyID, unit.AccountID, task.GroupID, requestID, task.Model, unit.UpstreamModel, unit.UpstreamTaskID, task.MediaType, imageCount, imageSize, videoCount, videoResolution, videoDuration, base, actual, rate, accountRate, endpoint, now)
		if err != nil {
			return err
		}
	}
	return nil
}

type unifiedMediaAdminQuerier interface {
	AdminList(context.Context, int, int, string) ([]*service.UnifiedMediaTask, int64, error)
	GetAdmin(context.Context, string) (*service.UnifiedMediaTask, error)
}

func (r *unifiedMediaTaskRepository) GetAdmin(ctx context.Context, id string) (*service.UnifiedMediaTask, error) {
	return r.scan(r.db.QueryRowContext(ctx, `SELECT `+unifiedMediaTaskColumns+` FROM unified_media_tasks WHERE id=$1`, id), false)
}
func (r *unifiedMediaTaskRepository) AdminList(ctx context.Context, page, size int, status string) ([]*service.UnifiedMediaTask, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	args := []any{}
	where := "TRUE"
	if status != "" {
		where = "status=$1"
		args = append(args, status)
	}
	var count int64
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM unified_media_tasks WHERE `+where, args...).Scan(&count); err != nil {
		return nil, 0, err
	}
	args = append(args, size, (int64(page)-1)*int64(size))
	rows, err := r.db.QueryContext(ctx, `SELECT `+unifiedMediaTaskColumns+` FROM unified_media_tasks WHERE `+where+fmt.Sprintf(" ORDER BY created_at DESC,id LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	tasks := make([]*service.UnifiedMediaTask, 0)
	for rows.Next() {
		task, err := r.scan(rows, false)
		if err != nil {
			return nil, 0, err
		}
		tasks = append(tasks, task)
	}
	return tasks, count, rows.Err()
}

// Existing reservations must remain settleable after administrators or users revoke the
// associated credential. The task and hold rows fix ownership before these updates run.
func incrementUnifiedMediaKeyUsage(ctx context.Context, tx *sql.Tx, keyID int64, cost float64) error {
	result, err := tx.ExecContext(ctx, `UPDATE api_keys SET
 quota_used=quota_used+$1,
 status=CASE WHEN deleted_at IS NULL AND quota>0 AND status=$3 AND quota_used<quota AND quota_used+$1>=quota THEN $4 ELSE status END,
 usage_5h=CASE WHEN window_5h_start IS NULL OR window_5h_start+INTERVAL '5 hours'<=NOW() THEN $1 ELSE usage_5h+$1 END,
 usage_1d=CASE WHEN window_1d_start IS NULL OR window_1d_start+INTERVAL '24 hours'<=NOW() THEN $1 ELSE usage_1d+$1 END,
 usage_7d=CASE WHEN window_7d_start IS NULL OR window_7d_start+INTERVAL '7 days'<=NOW() THEN $1 ELSE usage_7d+$1 END,
 window_5h_start=CASE WHEN window_5h_start IS NULL OR window_5h_start+INTERVAL '5 hours'<=NOW() THEN NOW() ELSE window_5h_start END,
 window_1d_start=CASE WHEN window_1d_start IS NULL OR window_1d_start+INTERVAL '24 hours'<=NOW() THEN date_trunc('day',NOW()) ELSE window_1d_start END,
 window_7d_start=CASE WHEN window_7d_start IS NULL OR window_7d_start+INTERVAL '7 days'<=NOW() THEN date_trunc('day',NOW()) ELSE window_7d_start END,
 updated_at=NOW() WHERE id=$2`, cost, keyID, service.StatusAPIKeyActive, service.StatusAPIKeyQuotaExhausted)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return service.ErrAPIKeyNotFound
	}
	return nil
}
func incrementUnifiedMediaAccountUsage(ctx context.Context, tx *sql.Tx, accountID int64, amount float64) error {
	result, err := tx.ExecContext(ctx, `UPDATE accounts SET extra=(COALESCE(extra,'{}'::jsonb)
 ||jsonb_build_object('quota_used',COALESCE((extra->>'quota_used')::numeric,0)+$1)
 ||CASE WHEN COALESCE((extra->>'quota_daily_limit')::numeric,0)>0 THEN jsonb_build_object(
 'quota_daily_used',CASE WHEN `+dailyExpiredExpr+` THEN $1 ELSE COALESCE((extra->>'quota_daily_used')::numeric,0)+$1 END,
 'quota_daily_start',CASE WHEN `+dailyExpiredExpr+` THEN `+nowUTC+` ELSE COALESCE(extra->>'quota_daily_start',`+nowUTC+`) END)
 ||CASE WHEN `+dailyExpiredExpr+` AND `+nextDailyResetAtExpr+` IS NOT NULL THEN jsonb_build_object('quota_daily_reset_at',`+nextDailyResetAtExpr+`) ELSE '{}'::jsonb END ELSE '{}'::jsonb END
 ||CASE WHEN COALESCE((extra->>'quota_weekly_limit')::numeric,0)>0 THEN jsonb_build_object(
 'quota_weekly_used',CASE WHEN `+weeklyExpiredExpr+` THEN $1 ELSE COALESCE((extra->>'quota_weekly_used')::numeric,0)+$1 END,
 'quota_weekly_start',CASE WHEN `+weeklyExpiredExpr+` THEN `+nowUTC+` ELSE COALESCE(extra->>'quota_weekly_start',`+nowUTC+`) END)
 ||CASE WHEN `+weeklyExpiredExpr+` AND `+nextWeeklyResetAtExpr+` IS NOT NULL THEN jsonb_build_object('quota_weekly_reset_at',`+nextWeeklyResetAtExpr+`) ELSE '{}'::jsonb END ELSE '{}'::jsonb END),updated_at=NOW() WHERE id=$2`, amount, accountID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return service.ErrAccountNotFound
	}
	return nil
}
