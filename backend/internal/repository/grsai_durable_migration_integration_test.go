//go:build integration

package repository

import (
	"context"
	"testing"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestGrsaiDurableDeliveryMigrationAddsV2ColumnsAndPayloadTable(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	useIsolatedMigrationSchema(ctx, t, tx, "grsai_durable_migration")
	for _, name := range []string{
		"239_grsai_native_images.sql",
		"240_grsai_settlement_claim_version.sql",
		"241_grsai_settlement_retry_count.sql",
		"241a_grsai_settlement_image_size_precheck.sql",
		"242_grsai_settlement_image_size.sql",
		"243_grsai_durable_delivery.sql",
	} {
		migration, err := dbmigrations.FS.ReadFile(name)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, string(migration))
		require.NoError(t, err)
	}
	for _, column := range []string{"local_task_id", "delivery_mode", "public_status", "progress", "result_json", "link_expires_at", "image_object_metadata", "task_version", "submission_attempt"} {
		var count int
		require.NoError(t, tx.QueryRowContext(ctx, `
SELECT COUNT(*) FROM information_schema.columns
WHERE table_schema = current_schema() AND table_name = 'grsai_settlements' AND column_name = $1`, column).Scan(&count))
		require.Equal(t, 1, count, column)
	}
	var tableCount int
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT COUNT(*) FROM information_schema.tables
WHERE table_schema = current_schema() AND table_name = 'grsai_task_payloads'`).Scan(&tableCount))
	require.Equal(t, 1, tableCount)

	_, err := tx.ExecContext(ctx, `
INSERT INTO grsai_settlements (account_id, group_id, user_id, api_key_id, model,
    base_unit_price, group_rate_multiplier, account_rate_multiplier, billable_unit_price,
    requested_image_count, billing_idempotency_key, next_attempt_at)
VALUES (1, 2, 3, 4, 'legacy', 0.01, 1, 1, 0.01, 1, 'legacy-v2-defaults', NOW())`)
	require.NoError(t, err)
	var deliveryMode, publicStatus string
	var taskVersion, progress int
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT delivery_mode, public_status, progress, task_version
FROM grsai_settlements WHERE billing_idempotency_key = 'legacy-v2-defaults'`).
		Scan(&deliveryMode, &publicStatus, &progress, &taskVersion))
	require.Equal(t, "json", deliveryMode)
	require.Equal(t, "queued", publicStatus)
	require.Zero(t, progress)
	require.Equal(t, 1, taskVersion)
}
