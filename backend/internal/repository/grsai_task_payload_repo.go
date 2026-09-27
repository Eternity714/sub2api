package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

var ErrGrsaiTaskPayloadNotFound = errors.New("grsai task payload not found")

type grsaiTaskPayloadSQLExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type grsaiTaskPayloadRepository struct {
	sql       grsaiTaskPayloadSQLExecutor
	encryptor service.SecretEncryptor
}

var _ service.GrsaiTaskPayloadRepository = (*grsaiTaskPayloadRepository)(nil)

func NewGrsaiTaskPayloadRepository(db *sql.DB, encryptor service.SecretEncryptor) *grsaiTaskPayloadRepository {
	return &grsaiTaskPayloadRepository{sql: db, encryptor: encryptor}
}

func (r *grsaiTaskPayloadRepository) PutEncrypted(ctx context.Context, localTaskID string, payload []byte, expiresAt time.Time) error {
	localTaskID = strings.TrimSpace(localTaskID)
	if r == nil || r.sql == nil || r.encryptor == nil || localTaskID == "" || len(payload) == 0 || expiresAt.IsZero() {
		return service.ErrGrsaiSettlementInvalidInput
	}
	ciphertext, err := r.encryptor.Encrypt(string(payload))
	if err != nil {
		return fmt.Errorf("encrypt grsai task payload: %w", err)
	}
	if _, err := r.sql.ExecContext(ctx, `
INSERT INTO grsai_task_payloads (local_task_id, payload_ciphertext, expires_at)
VALUES ($1, $2, $3)`, localTaskID, ciphertext, expiresAt); err != nil {
		return fmt.Errorf("persist grsai task payload: %w", err)
	}
	return nil
}

func (r *grsaiTaskPayloadRepository) GetDecrypted(ctx context.Context, localTaskID string, claimVersion int64) ([]byte, error) {
	localTaskID = strings.TrimSpace(localTaskID)
	if r == nil || r.sql == nil || r.encryptor == nil || localTaskID == "" || claimVersion <= 0 {
		return nil, service.ErrGrsaiSettlementInvalidInput
	}
	var ciphertext string
	err := r.sql.QueryRowContext(ctx, `
SELECT payloads.payload_ciphertext
FROM grsai_task_payloads AS payloads
JOIN grsai_settlements AS tasks ON tasks.local_task_id = payloads.local_task_id
WHERE payloads.local_task_id = $1 AND payloads.expires_at > NOW()
  AND tasks.claim_version = $2 AND tasks.task_version = 2
  AND tasks.internal_status = 'v2_submitting' AND tasks.submission_attempt = 1
  AND tasks.upstream_task_id IS NULL AND tasks.next_attempt_at > NOW()`, localTaskID, claimVersion).Scan(&ciphertext)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGrsaiTaskPayloadNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load grsai task payload: %w", err)
	}
	plaintext, err := r.encryptor.Decrypt(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decrypt grsai task payload: %w", err)
	}
	return []byte(plaintext), nil
}

func (r *grsaiTaskPayloadRepository) DeleteByTaskID(ctx context.Context, localTaskID string) error {
	localTaskID = strings.TrimSpace(localTaskID)
	if r == nil || r.sql == nil || localTaskID == "" {
		return service.ErrGrsaiSettlementInvalidInput
	}
	_, err := r.sql.ExecContext(ctx, `DELETE FROM grsai_task_payloads WHERE local_task_id = $1`, localTaskID)
	return err
}

func (r *grsaiTaskPayloadRepository) DeleteExpired(ctx context.Context, now time.Time) error {
	if r == nil || r.sql == nil || now.IsZero() {
		return service.ErrGrsaiSettlementInvalidInput
	}
	_, err := r.sql.ExecContext(ctx, `DELETE FROM grsai_task_payloads WHERE expires_at <= $1`, now)
	return err
}
