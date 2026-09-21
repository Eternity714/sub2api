package repository

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type testSecretEncryptor struct{}

func (testSecretEncryptor) Encrypt(plaintext string) (string, error) {
	return "cipher:" + base64.StdEncoding.EncodeToString([]byte(plaintext)), nil
}

func (testSecretEncryptor) Decrypt(ciphertext string) (string, error) {
	const prefix = "cipher:"
	if len(ciphertext) < len(prefix) || ciphertext[:len(prefix)] != prefix {
		return "", errors.New("invalid ciphertext")
	}
	plaintext, err := base64.StdEncoding.DecodeString(ciphertext[len(prefix):])
	return string(plaintext), err
}

func TestGrsaiTaskPayloadRepositoryNeverPersistsPlaintext(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewGrsaiTaskPayloadRepository(db, testSecretEncryptor{})
	payload := []byte(`{"prompt":"private"}`)
	expiresAt := time.Now().UTC().Add(time.Hour)
	ciphertext, err := (testSecretEncryptor{}).Encrypt(string(payload))
	require.NoError(t, err)

	mock.ExpectExec(`(?s)INSERT INTO grsai_task_payloads.*ON CONFLICT \(settlement_id\) DO UPDATE`).
		WithArgs(int64(18), ciphertext, expiresAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.PutEncrypted(context.Background(), 18, payload, expiresAt))

	mock.ExpectQuery(`(?s)SELECT ciphertext.*expires_at > NOW\(\)`).
		WithArgs(int64(18)).
		WillReturnRows(sqlmock.NewRows([]string{"ciphertext"}).AddRow(ciphertext))
	got, err := repo.GetEncrypted(context.Background(), 18)
	require.NoError(t, err)
	require.JSONEq(t, `{"prompt":"private"}`, string(got))
	require.NotContains(t, ciphertext, `"prompt":"private"`)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGrsaiTaskPayloadRepositoryExpiredPayloadIsNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewGrsaiTaskPayloadRepository(db, testSecretEncryptor{})
	mock.ExpectQuery(`(?s)SELECT ciphertext.*expires_at > NOW\(\)`).
		WithArgs(int64(18)).
		WillReturnRows(sqlmock.NewRows([]string{"ciphertext"}))

	_, err = repo.GetEncrypted(context.Background(), 18)
	require.ErrorIs(t, err, service.ErrGrsaiTaskPayloadNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGrsaiTaskPayloadRepositoryDeletesIdempotentlyAndOnlyWhenDue(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewGrsaiTaskPayloadRepository(db, testSecretEncryptor{})
	mock.ExpectExec("DELETE FROM grsai_task_payloads WHERE settlement_id = \\$1").
		WithArgs(int64(18)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	require.NoError(t, repo.DeleteBySettlementID(context.Background(), 18))

	now := time.Now().UTC()
	mock.ExpectExec("DELETE FROM grsai_task_payloads WHERE expires_at <= \\$1").
		WithArgs(now).
		WillReturnResult(sqlmock.NewResult(0, 3))
	deleted, err := repo.DeleteExpired(context.Background(), now)
	require.NoError(t, err)
	require.EqualValues(t, 3, deleted)
	require.NoError(t, mock.ExpectationsWereMet())
}
