//go:build integration

package repository

import (
	"context"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestMediaResolutionPrioritiesMigrationPreservesLegacyRows(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	useIsolatedMigrationSchema(ctx, t, tx, "media_resolution_priorities")
	_, err := tx.ExecContext(ctx, `CREATE TABLE composite_model_routes(id BIGINT PRIMARY KEY,priority INTEGER NOT NULL); INSERT INTO composite_model_routes VALUES(1,20)`)
	require.NoError(t, err)
	migration, err := dbmigrations.FS.ReadFile("249_media_resolution_priorities.sql")
	require.NoError(t, err)
	for range 2 {
		_, err = tx.ExecContext(ctx, string(migration))
		require.NoError(t, err)
	}
	var priority int
	var overrides string
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT priority,resolution_priorities::text FROM composite_model_routes WHERE id=1`).Scan(&priority, &overrides))
	require.Equal(t, 20, priority)
	require.JSONEq(t, `{}`, overrides)
}

func TestMediaResolutionPrioritiesPersistAndLegacyUpdatesRetainOverrides(t *testing.T) {
	tx := testEntTx(t)
	ctx := dbent.NewTxContext(context.Background(), tx)
	client := tx.Client()
	group := mustCreateGroup(t, client, &service.Group{Name: "centos-resolution-priorities", Platform: service.PlatformComposite})
	repo := NewCompositeModelRouteRepository(client)
	route := &service.CompositeModelRoute{GroupID: group.ID, PublicModel: "gpt-image-2.6-flare", MatchType: "exact", Endpoint: "images", TargetPlatform: service.PlatformCentOS, UpstreamModel: "gpt-image-2.6-flare", Priority: 10, ResolutionPriorities: map[string]int{"4K": 20}, Enabled: true}
	require.NoError(t, repo.Create(ctx, route))
	rows, err := repo.ListByGroup(ctx, group.ID, true)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, map[string]int{"4K": 20}, rows[0].ResolutionPriorities)
	require.Equal(t, 20, rows[0].ResolutionPriority("4K"))
	route.ResolutionPriorities = nil
	route.Notes = "legacy client updates notes only"
	require.NoError(t, repo.Update(ctx, route))
	require.Equal(t, map[string]int{"4K": 20}, route.ResolutionPriorities)
	route.ResolutionPriorities = map[string]int{}
	require.NoError(t, repo.Update(ctx, route))
	rows, err = repo.ListByGroup(ctx, group.ID, true)
	require.NoError(t, err)
	require.Empty(t, rows[0].ResolutionPriorities)
	require.Equal(t, 10, rows[0].ResolutionPriority("4K"))
}
