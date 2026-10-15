//go:build unit

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

var mediaCapabilityColumns = []string{"id", "public_model", "media_type", "schema_version", "revision", "enabled", "schema", "template_id", "template_version", "notes", "created_at", "updated_at"}

func mediaCapabilityMockRow(revision int64) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(mediaCapabilityColumns).AddRow(7, "example", "image", 1, revision, false, []byte(`{"version":1,"fields":{}}`), nil, nil, "", now, now)
}
func TestMediaCapabilityRepositoryCASCannotOverwriteStaleRevision(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewMediaCapabilityRepository(db)
	row := &service.MediaCapability{ID: 7, PublicModel: "example", MediaType: "image", SchemaVersion: 1, Schema: json.RawMessage(`{"version":1,"fields":{}}`)}
	mock.ExpectQuery("UPDATE media_model_capabilities").WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT .*media_model_capabilities").WithArgs(int64(7)).WillReturnRows(mediaCapabilityMockRow(2))
	require.ErrorIs(t, repo.Update(context.Background(), row, 1), service.ErrMediaCapabilityConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}
func TestMediaCapabilityRepositoryClassifiesUniqueConstraint(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewMediaCapabilityRepository(db)
	mock.ExpectQuery("INSERT INTO media_model_capabilities").WillReturnError(&pq.Error{Code: "23505", Constraint: "idx_media_model_capabilities_unique_active"})
	err = repo.Create(context.Background(), &service.MediaCapability{PublicModel: "x", MediaType: "image", SchemaVersion: 1, Schema: json.RawMessage(`{}`)})
	require.ErrorIs(t, err, service.ErrMediaCapabilityExists)
	require.NoError(t, mock.ExpectationsWereMet())
}
