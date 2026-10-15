//go:build integration

package repository

import (
	"context"
	"testing"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestCompositeMediaRouteMigrationPreservesLegacyUniquenessAndAllowsCandidates(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	useIsolatedMigrationSchema(ctx, t, tx, "composite_media_routes")
	_, err := tx.ExecContext(ctx, `CREATE TABLE groups (id BIGINT PRIMARY KEY); INSERT INTO groups(id) VALUES (7)`)
	require.NoError(t, err)
	base, err := dbmigrations.FS.ReadFile("172_composite_model_routes.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(base))
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `ALTER TABLE composite_model_routes DROP CONSTRAINT composite_model_routes_target_platform_check`)
	require.NoError(t, err)
	migration, err := dbmigrations.FS.ReadFile("247_composite_media_candidates.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO composite_model_routes(group_id,public_model,target_platform,upstream_model,endpoint,capability_model) VALUES
      (7,'shared','grsai','grs-a','images','grs/shared'),
      (7,'shared','toapis','toa-a','images','toa/shared'),
      (7,'shared','grsai','grs-b','images','grs/shared'),
      (7,'shared','toapis','toa-a','videos','toa/video'),
      (7,'shared','openai','chat','responses','')`)
	require.NoError(t, err)
	for _, query := range []string{
		`INSERT INTO composite_model_routes(group_id,public_model,target_platform,upstream_model,endpoint) VALUES(7,'shared','grsai','grs-a','images')`,
		`INSERT INTO composite_model_routes(group_id,public_model,target_platform,upstream_model,endpoint) VALUES(7,'shared','anthropic','other','responses')`,
	} {
		_, err = tx.ExecContext(ctx, "SAVEPOINT duplicate_check")
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, query)
		require.Error(t, err)
		_, rollbackErr := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT duplicate_check")
		require.NoError(t, rollbackErr)
	}
	var count int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM composite_model_routes WHERE public_model='shared'`).Scan(&count))
	require.Equal(t, 5, count)
}
