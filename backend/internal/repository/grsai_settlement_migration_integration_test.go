//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

const (
	grsaiSettlementBaseMigration              = "239_grsai_native_images.sql"
	grsaiSettlementClaimVersionMigration      = "240_grsai_settlement_claim_version.sql"
	grsaiSettlementRetryCountMigration        = "241_grsai_settlement_retry_count.sql"
	grsaiSettlementImageSizePrecheckMigration = "241a_grsai_settlement_image_size_precheck.sql"
	grsaiSettlementImageSizeMigration         = "242_grsai_settlement_image_size.sql"
)

func TestGrsaiSettlementRepository_MigrationsNewDatabaseIncludeClaimVersion(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	useIsolatedMigrationSchema(ctx, t, tx, "grsai_migration_new")

	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementBaseMigration)
	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementClaimVersionMigration)
	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementRetryCountMigration)
	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementImageSizePrecheckMigration)
	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementImageSizeMigration)

	requireGrsaiClaimVersionColumn(ctx, t, tx)
	requireGrsaiSettlementRetryCountColumn(ctx, t, tx)
	requireGrsaiSettlementImageSizeColumn(ctx, t, tx)
}

func TestGrsaiSettlementRepository_Migration240UpgradesApplied239WithoutDataLoss(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	useIsolatedMigrationSchema(ctx, t, tx, "grsai_migration_upgrade")

	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementBaseMigration)

	var claimVersionColumns int
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM information_schema.columns
WHERE table_schema = current_schema()
  AND table_name = 'grsai_settlements'
  AND column_name = 'claim_version'
`).Scan(&claimVersionColumns))
	require.Zero(t, claimVersionColumns, "the immutable 239 migration must represent the already-applied schema")

	var settlementID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO grsai_settlements (
    account_id, group_id, user_id, api_key_id, model,
    base_unit_price, group_rate_multiplier, account_rate_multiplier,
    billable_unit_price, requested_image_count, billing_idempotency_key,
    next_attempt_at
) VALUES (1, 2, 3, 4, 'grsai-test', 0.01, 1, 1, 0.01, 1, 'migration-upgrade-row', NOW())
RETURNING id
`).Scan(&settlementID))

	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementClaimVersionMigration)
	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementClaimVersionMigration)

	var claimVersion int64
	require.NoError(t, tx.QueryRowContext(ctx,
		"SELECT claim_version FROM grsai_settlements WHERE id = $1", settlementID).Scan(&claimVersion))
	require.Zero(t, claimVersion, "existing rows must receive the initial fencing version")
	requireGrsaiClaimVersionColumn(ctx, t, tx)
}

func TestGrsaiSettlementRepository_Migration241AddsIndependentSettlementRetryCount(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	useIsolatedMigrationSchema(ctx, t, tx, "grsai_migration_retry_count")

	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementBaseMigration)
	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementClaimVersionMigration)
	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementRetryCountMigration)
	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementImageSizePrecheckMigration)
	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementRetryCountMigration)

	var retryCount int
	require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO grsai_settlements (
    account_id, group_id, user_id, api_key_id, model,
    base_unit_price, group_rate_multiplier, account_rate_multiplier,
    billable_unit_price, requested_image_count, billing_idempotency_key,
    next_attempt_at
) VALUES (1, 2, 3, 4, 'grsai-test', 0.01, 1, 1, 0.01, 1, 'migration-retry-count-row', NOW())
RETURNING settlement_retry_count
`).Scan(&retryCount))
	require.Zero(t, retryCount)
	requireGrsaiSettlementRetryCountColumn(ctx, t, tx)
}

func TestGrsaiSettlementRepository_Migration242AddsImageSizeSnapshot(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	useIsolatedMigrationSchema(ctx, t, tx, "grsai_migration_image_size")

	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementBaseMigration)
	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementClaimVersionMigration)
	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementRetryCountMigration)
	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementImageSizePrecheckMigration)

	var settlementID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO grsai_settlements (
    account_id, group_id, user_id, api_key_id, model,
    base_unit_price, group_rate_multiplier, account_rate_multiplier,
    billable_unit_price, requested_image_count, billing_idempotency_key,
    next_attempt_at
) VALUES (1, 2, 3, 4, 'grsai-test', 0.01, 1, 1, 0.01, 1, 'migration-image-size-row', NOW())
RETURNING id
`).Scan(&settlementID))

	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementImageSizeMigration)

	var imageSize string
	require.NoError(t, tx.QueryRowContext(ctx,
		"SELECT image_size FROM grsai_settlements WHERE id = $1", settlementID).Scan(&imageSize))
	require.Equal(t, "2K", imageSize)
	requireGrsaiSettlementImageSizeColumn(ctx, t, tx)
}

func TestGrsaiSettlementRepository_Migration241aRepairsExistingNullableImageSize(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	useIsolatedMigrationSchema(ctx, t, tx, "grsai_migration_image_size_nullable")

	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementBaseMigration)
	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementClaimVersionMigration)
	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementRetryCountMigration)
	_, err := tx.ExecContext(ctx, `
ALTER TABLE grsai_settlements ADD COLUMN image_size VARCHAR(10)
`)
	require.NoError(t, err)

	insertGrsaiSettlementForMigration(t, ctx, tx, "migration-image-size-null-row", nil)
	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementImageSizePrecheckMigration)
	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementImageSizeMigration)

	var imageSize string
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT image_size FROM grsai_settlements WHERE billing_idempotency_key = 'migration-image-size-null-row'
`).Scan(&imageSize))
	require.Equal(t, "2K", imageSize)
	requireGrsaiSettlementImageSizeColumn(ctx, t, tx)
}

func TestGrsaiSettlementRepository_Migration241aRepairsExistingInvalidImageSize(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	useIsolatedMigrationSchema(ctx, t, tx, "grsai_migration_image_size_invalid")

	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementBaseMigration)
	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementClaimVersionMigration)
	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementRetryCountMigration)
	_, err := tx.ExecContext(ctx, `
ALTER TABLE grsai_settlements ADD COLUMN image_size VARCHAR(10)
`)
	require.NoError(t, err)

	invalidImageSize := "invalid"
	insertGrsaiSettlementForMigration(t, ctx, tx, "migration-image-size-invalid-row", &invalidImageSize)
	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementImageSizePrecheckMigration)
	applyGrsaiSettlementMigration(ctx, t, tx, grsaiSettlementImageSizeMigration)

	var imageSize string
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT image_size FROM grsai_settlements WHERE billing_idempotency_key = 'migration-image-size-invalid-row'
`).Scan(&imageSize))
	require.Equal(t, "2K", imageSize)
	requireGrsaiSettlementImageSizeColumn(ctx, t, tx)
}

func insertGrsaiSettlementForMigration(t *testing.T, ctx context.Context, tx *sql.Tx, idempotencyKey string, imageSize *string) {
	t.Helper()

	_, err := tx.ExecContext(ctx, `
INSERT INTO grsai_settlements (
    account_id, group_id, user_id, api_key_id, model,
    base_unit_price, group_rate_multiplier, account_rate_multiplier,
    billable_unit_price, requested_image_count, billing_idempotency_key,
    next_attempt_at, image_size
) VALUES (1, 2, 3, 4, 'grsai-test', 0.01, 1, 1, 0.01, 1, $1, NOW(), $2)
`, idempotencyKey, imageSize)
	require.NoError(t, err)
}

func useIsolatedMigrationSchema(ctx context.Context, t *testing.T, tx *sql.Tx, schemaName string) {
	t.Helper()

	quotedSchema := pq.QuoteIdentifier(schemaName)
	_, err := tx.ExecContext(ctx, "CREATE SCHEMA "+quotedSchema)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "SET LOCAL search_path TO "+quotedSchema)
	require.NoError(t, err)
}

func applyGrsaiSettlementMigration(ctx context.Context, t *testing.T, tx *sql.Tx, name string) {
	t.Helper()

	migrationSQL, err := dbmigrations.FS.ReadFile(name)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migrationSQL))
	require.NoError(t, err)
}

func requireGrsaiClaimVersionColumn(ctx context.Context, t *testing.T, tx *sql.Tx) {
	t.Helper()

	var dataType, isNullable, columnDefault string
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT data_type, is_nullable, COALESCE(column_default, '')
FROM information_schema.columns
WHERE table_schema = current_schema()
  AND table_name = 'grsai_settlements'
  AND column_name = 'claim_version'
`).Scan(&dataType, &isNullable, &columnDefault))
	require.Equal(t, "bigint", dataType)
	require.Equal(t, "NO", isNullable)
	require.Contains(t, columnDefault, "0")
}

func requireGrsaiSettlementRetryCountColumn(ctx context.Context, t *testing.T, tx *sql.Tx) {
	t.Helper()
	var dataType, isNullable, columnDefault string
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT data_type, is_nullable, COALESCE(column_default, '')
FROM information_schema.columns
WHERE table_schema = current_schema()
  AND table_name = 'grsai_settlements'
  AND column_name = 'settlement_retry_count'
`).Scan(&dataType, &isNullable, &columnDefault))
	require.Equal(t, "integer", dataType)
	require.Equal(t, "NO", isNullable)
	require.Contains(t, columnDefault, "0")
}

func requireGrsaiSettlementImageSizeColumn(ctx context.Context, t *testing.T, tx *sql.Tx) {
	t.Helper()

	var dataType, isNullable, columnDefault string
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT data_type, is_nullable, COALESCE(column_default, '')
FROM information_schema.columns
WHERE table_schema = current_schema()
  AND table_name = 'grsai_settlements'
  AND column_name = 'image_size'
`).Scan(&dataType, &isNullable, &columnDefault))
	require.Equal(t, "character varying", dataType)
	require.Equal(t, "NO", isNullable)
	require.Contains(t, columnDefault, "2K")
}
