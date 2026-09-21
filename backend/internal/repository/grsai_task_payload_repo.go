package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type grsaiTaskPayloadRepository struct {
	db        *sql.DB
	encryptor service.SecretEncryptor
}

var _ service.GrsaiTaskPayloadRepository = (*grsaiTaskPayloadRepository)(nil)

func NewGrsaiTaskPayloadRepository(db *sql.DB, encryptor service.SecretEncryptor) *grsaiTaskPayloadRepository {
	return &grsaiTaskPayloadRepository{db: db, encryptor: encryptor}
}

func (r *grsaiTaskPayloadRepository) PutEncrypted(ctx context.Context, settlementID int64, plaintext []byte, expiresAt time.Time) error {
	if r == nil || r.db == nil || r.encryptor == nil || settlementID <= 0 || expiresAt.IsZero() {
		return service.ErrGrsaiSettlementInvalidInput
	}
	ciphertext, err := r.encryptor.Encrypt(string(plaintext))
	if err != nil {
		return fmt.Errorf("encrypt grsai task payload: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO grsai_task_payloads (settlement_id, ciphertext, expires_at, created_at, updated_at)
VALUES ($1, $2, $3, NOW(), NOW())
ON CONFLICT (settlement_id) DO UPDATE
SET ciphertext = EXCLUDED.ciphertext,
    expires_at = EXCLUDED.expires_at,
    updated_at = NOW()`, settlementID, ciphertext, expiresAt)
	return err
}

func (r *grsaiTaskPayloadRepository) GetEncrypted(ctx context.Context, settlementID int64) ([]byte, error) {
	if r == nil || r.db == nil || r.encryptor == nil || settlementID <= 0 {
		return nil, service.ErrGrsaiSettlementInvalidInput
	}
	var ciphertext string
	err := r.db.QueryRowContext(ctx, `
SELECT ciphertext
FROM grsai_task_payloads
WHERE settlement_id = $1
  AND expires_at > NOW()`, settlementID).Scan(&ciphertext)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrGrsaiTaskPayloadNotFound
	}
	if err != nil {
		return nil, err
	}
	plaintext, err := r.encryptor.Decrypt(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decrypt grsai task payload: %w", err)
	}
	return []byte(plaintext), nil
}

func (r *grsaiTaskPayloadRepository) DeleteBySettlementID(ctx context.Context, settlementID int64) error {
	if r == nil || r.db == nil || settlementID <= 0 {
		return service.ErrGrsaiSettlementInvalidInput
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM grsai_task_payloads WHERE settlement_id = $1`, settlementID)
	return err
}

func (r *grsaiTaskPayloadRepository) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	if r == nil || r.db == nil || now.IsZero() {
		return 0, service.ErrGrsaiSettlementInvalidInput
	}
	result, err := r.db.ExecContext(ctx, `DELETE FROM grsai_task_payloads WHERE expires_at <= $1`, now)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
