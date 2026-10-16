//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// AC-029: real PostgreSQL facts/histograms merge only for ordinary users, after
// SQL group authorization and before computing percentiles, rates and health.
func TestMediaMonitorRepositoryPublicProjectionIntegration(t *testing.T) {
	for _, bucket := range []time.Duration{time.Minute, 5 * time.Minute} {
		t.Run(bucket.String(), func(t *testing.T) {
			repo, filter, cfg, visibleGroup, privateGroup := mediaMonitorProjectionIntegrationFixture(t, bucket)
			ctx := context.Background()
			models, err := repo.GetModels(ctx, filter, cfg, false)
			require.NoError(t, err)
			require.Len(t, models.Items, 1)
			require.Equal(t, "media", models.Items[0].Platform)
			assertMergedMediaMonitorMetrics(t, models.Items[0].Metrics, 5)
			require.Equal(t, service.ChannelMonitorV2HealthForWithThresholds(models.Items[0].Metrics, cfg.HealthThresholds), models.Items[0].Health)

			for _, grouping := range []service.ChannelMonitorV2GroupBy{
				service.ChannelMonitorV2GroupByPlatform,
				service.ChannelMonitorV2GroupByPlatformModel,
				service.ChannelMonitorV2GroupByPlatformGroup,
				service.ChannelMonitorV2GroupByPlatformGroupModel,
			} {
				matrix, err := repo.GetMatrix(ctx, filter, cfg, grouping, false)
				require.NoError(t, err)
				require.Len(t, matrix.Items, 1, string(grouping))
				row := matrix.Items[0]
				require.Equal(t, "media", row.Platform)
				assertMergedMediaMonitorMetrics(t, row.Metrics, 5)
				require.Len(t, row.Buckets, 1)
				assertMergedMediaMonitorMetrics(t, row.Buckets[0].Metrics, bucket.Minutes())
				if row.GroupID != nil {
					require.Equal(t, visibleGroup, *row.GroupID)
				}
			}

			adminFilter := filter
			adminFilter.RestrictGroups, adminFilter.AllowedGroupIDs = false, nil
			adminModels, err := repo.GetModels(ctx, adminFilter, cfg, true)
			require.NoError(t, err)
			require.Len(t, adminModels.Items, 2)
			require.ElementsMatch(t, []string{"grsai", "toapis"}, []string{adminModels.Items[0].Platform, adminModels.Items[1].Platform})
			require.Equal(t, int64(1110), adminModels.Items[0].Metrics.RequestCount+adminModels.Items[1].Metrics.RequestCount)
			adminMatrix, err := repo.GetMatrix(ctx, adminFilter, cfg, service.ChannelMonitorV2GroupByPlatformGroupModel, true)
			require.NoError(t, err)
			require.Len(t, adminMatrix.Items, 3)
			for _, row := range adminMatrix.Items {
				require.Contains(t, []string{"grsai", "toapis"}, row.Platform)
			}

			// A requested group outside the server-derived scope cannot leak its
			// high-volume/high-latency poison rows, even through a neutral model.
			denied := filter
			denied.GroupIDs = []int64{privateGroup}
			deniedModels, err := repo.GetModels(ctx, denied, cfg, false)
			require.NoError(t, err)
			require.Empty(t, deniedModels.Items)
			deniedMatrix, err := repo.GetMatrix(ctx, denied, cfg, service.ChannelMonitorV2GroupByPlatformGroupModel, false)
			require.NoError(t, err)
			require.Empty(t, deniedMatrix.Items)

			// Read projections never rewrite stored provider identities.
			var platforms pq.StringArray
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT array_agg(DISTINCT platform ORDER BY platform) FROM channel_monitor_v2_metrics_1m WHERE group_id=ANY($1)`, pq.Array([]int64{visibleGroup, privateGroup})).Scan(&platforms))
			require.Equal(t, pq.StringArray{"grsai", "toapis"}, platforms)
		})
	}
}

func mediaMonitorProjectionIntegrationFixture(t *testing.T, bucket time.Duration) (*channelMonitorV2Repository, service.ChannelMonitorV2Filter, service.ChannelMonitorV2Config, int64, int64) {
	t.Helper()
	ctx := context.Background()
	suffix := uuid.NewString()
	var visibleGroup, privateGroup int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO groups(name,platform) VALUES($1,'composite') RETURNING id`, "monitor-visible-"+suffix).Scan(&visibleGroup))
	t.Cleanup(func() { _, _ = integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id=$1`, visibleGroup) })
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO groups(name,platform) VALUES($1,'composite') RETURNING id`, "monitor-private-"+suffix).Scan(&privateGroup))
	t.Cleanup(func() { _, _ = integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id=$1`, privateGroup) })
	groups := []int64{visibleGroup, privateGroup}
	var previous [5]sql.NullTime
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT usage_coverage_start,error_coverage_start,data_through,last_successful_at,backfill_cursor FROM channel_monitor_v2_watermarks WHERE id=1`).Scan(&previous[0], &previous[1], &previous[2], &previous[3], &previous[4]))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, `UPDATE channel_monitor_v2_watermarks SET usage_coverage_start=$1,error_coverage_start=$2,data_through=$3,last_successful_at=$4,backfill_cursor=$5 WHERE id=1`, previous[0], previous[1], previous[2], previous[3], previous[4])
		for _, table := range []string{"channel_monitor_v2_metrics_1m", "channel_monitor_v2_error_metrics_1m", "channel_monitor_v2_latency_histograms_1m", "channel_monitor_v2_metrics_rollup", "channel_monitor_v2_error_metrics_rollup", "channel_monitor_v2_latency_histograms_rollup"} {
			_, _ = integrationDB.ExecContext(ctx, `DELETE FROM `+table+` WHERE group_id=ANY($1)`, pq.Array(groups))
		}
	})
	start := time.Now().UTC().Truncate(5 * time.Minute).Add(-10 * time.Minute)
	filter := service.ChannelMonitorV2Filter{
		Start: start, End: start.Add(5 * time.Minute), Bucket: bucket,
		Platforms: []string{"grsai", "toapis"}, RestrictGroups: true, AllowedGroupIDs: []int64{visibleGroup},
	}
	model := "monitor-media-" + suffix
	filter.Models = []string{"grsai\x00" + model, "toapis\x00" + model}
	cfg := service.ChannelMonitorV2Config{
		Platforms: []service.ChannelMonitorV2PlatformConfig{{Platform: "grsai", Enabled: true, Models: []string{model}}, {Platform: "toapis", Enabled: true, Models: []string{model}}},
		GroupIDs:  groups, IgnoredErrorCategories: []string{"content_policy"}, HealthThresholds: service.DefaultChannelMonitorV2HealthThresholds(),
	}
	_, err := integrationDB.ExecContext(ctx, `UPDATE channel_monitor_v2_watermarks SET usage_coverage_start=$1,error_coverage_start=$1,data_through=$2,last_successful_at=$2,backfill_cursor=$1 WHERE id=1`, start, filter.End)
	require.NoError(t, err)
	for _, fact := range []struct {
		platform                                                                 string
		group, success, errors, input, output, cache, sum, count, bound, ignored int64
	}{
		{"grsai", visibleGroup, 8, 2, 80, 10, 20, 1000, 10, 100, 1},
		{"toapis", visibleGroup, 90, 10, 900, 100, 100, 100000, 100, 1000, 5},
		{"toapis", privateGroup, 500, 500, 50000, 10000, 0, 20000000, 1000, 20000, 500},
	} {
		for _, rollup := range []bool{false, true} {
			suffix, extraColumn, extraValue := "1m", "", ""
			if rollup {
				suffix, extraColumn, extraValue = "rollup", ",bucket_seconds", ",300"
			}
			query := fmt.Sprintf(`INSERT INTO channel_monitor_v2_metrics_%s (bucket_start,platform,group_id,model,success_requests,error_requests,input_tokens,output_tokens,cache_read_tokens,ttft_sum_ms,ttft_count,duration_sum_ms,duration_count%s) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$10,$11%s)`, suffix, extraColumn, extraValue)
			_, err := integrationDB.ExecContext(ctx, query, start, fact.platform, fact.group, model, fact.success, fact.errors, fact.input, fact.output, fact.cache, fact.sum, fact.count)
			require.NoError(t, err)
			query = fmt.Sprintf(`INSERT INTO channel_monitor_v2_error_metrics_%s (bucket_start,platform,group_id,model,error_category,taxonomy_version,error_requests%s) VALUES($1,$2,$3,$4,'content_policy',1,$5%s)`, suffix, extraColumn, extraValue)
			_, err = integrationDB.ExecContext(ctx, query, start, fact.platform, fact.group, model, fact.ignored)
			require.NoError(t, err)
			for _, metric := range []string{"ttft", "duration"} {
				query = fmt.Sprintf(`INSERT INTO channel_monitor_v2_latency_histograms_%s (bucket_start,platform,group_id,model,metric,upper_bound_ms,sample_count%s) VALUES($1,$2,$3,$4,$5,$6,$7%s)`, suffix, extraColumn, extraValue)
				_, err = integrationDB.ExecContext(ctx, query, start, fact.platform, fact.group, model, metric, fact.bound, fact.count)
				require.NoError(t, err)
			}
		}
	}
	return &channelMonitorV2Repository{db: integrationDB}, filter, cfg, visibleGroup, privateGroup
}
