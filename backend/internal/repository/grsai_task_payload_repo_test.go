package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

type grsaiPayloadTestEncryptor struct{}

func (grsaiPayloadTestEncryptor) Encrypt(value string) (string, error) {
	return "ciphertext:" + value, nil
}

func (grsaiPayloadTestEncryptor) Decrypt(value string) (string, error) {
	if len(value) < len("ciphertext:") || value[:len("ciphertext:")] != "ciphertext:" {
		return "", errors.New("invalid ciphertext")
	}
	return value[len("ciphertext:"):], nil
}

func TestGrsaiTaskPayloadRepositoryEncryptsBeforePersistingAndDecryptsOnRead(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	expiresAt := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	mock.ExpectExec("INSERT INTO grsai_task_payloads").
		WithArgs("local-task-1", "ciphertext:{\"prompt\":\"private\"}", expiresAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT payloads.payload_ciphertext").
		WithArgs("local-task-1", int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"payload_ciphertext"}).AddRow("ciphertext:{\"prompt\":\"private\"}"))

	repo := NewGrsaiTaskPayloadRepository(db, grsaiPayloadTestEncryptor{})
	require.NoError(t, repo.PutEncrypted(context.Background(), "local-task-1", []byte(`{"prompt":"private"}`), expiresAt))
	got, err := repo.GetDecrypted(context.Background(), "local-task-1", 1)
	require.NoError(t, err)
	require.Equal(t, []byte(`{"prompt":"private"}`), got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGrsaiTaskPayloadRepositoryRejectsInvalidInputWithoutWriting(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewGrsaiTaskPayloadRepository(db, grsaiPayloadTestEncryptor{})

	err = repo.PutEncrypted(context.Background(), "", []byte("payload"), time.Now())
	require.Error(t, err)
	err = repo.PutEncrypted(context.Background(), "local-task-1", nil, time.Now())
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
