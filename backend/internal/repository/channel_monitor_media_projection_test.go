package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// AC-029: merge raw counts and histograms before calculating public rates or percentiles.
func TestMediaMonitorModelsMergeRawMetricsForUsersOnly(t *testing.T) {
	for _, admin := range []bool{false, true} {
		t.Run(fmt.Sprintf("admin=%t", admin), func(t *testing.T) {
			repo, mock, filter, cfg := mediaMonitorRepositoryFixture(t, false, 7)
			result, err := repo.GetModels(context.Background(), filter, cfg, admin)
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())
			if admin {
				require.Len(t, result.Items, 2)
				require.Equal(t, "toapis", result.Items[0].Platform)
				require.Equal(t, "grsai", result.Items[1].Platform)
				return
			}
			require.Len(t, result.Items, 1)
			require.Equal(t, "media", result.Items[0].Platform)
			require.Equal(t, "shared", result.Items[0].Model)
			assertMergedMediaMonitorMetrics(t, result.Items[0].Metrics, 5)
			require.Equal(t, service.ChannelMonitorV2HealthForWithThresholds(result.Items[0].Metrics, cfg.HealthThresholds), result.Items[0].Health)
		})
	}
}

// AC-029: each matrix grouping merges media providers, including time buckets, but admin data stays separate.
func TestMediaMonitorMatrixMergesRawMetricsForUsersOnly(t *testing.T) {
	for _, grouping := range []service.ChannelMonitorV2GroupBy{
		service.ChannelMonitorV2GroupByPlatform,
		service.ChannelMonitorV2GroupByPlatformModel,
		service.ChannelMonitorV2GroupByPlatformGroup,
		service.ChannelMonitorV2GroupByPlatformGroupModel,
	} {
		for _, admin := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/admin=%t", grouping, admin), func(t *testing.T) {
				repo, mock, filter, cfg := mediaMonitorRepositoryFixture(t, true, 7)
				result, err := repo.GetMatrix(context.Background(), filter, cfg, grouping, admin)
				require.NoError(t, err)
				require.NoError(t, mock.ExpectationsWereMet())
				if admin {
					require.Len(t, result.Items, 2)
					require.Equal(t, "grsai", result.Items[0].Platform)
					require.Equal(t, "toapis", result.Items[1].Platform)
					return
				}
				require.Len(t, result.Items, 1)
				row := result.Items[0]
				require.Equal(t, "media", row.Platform)
				assertMergedMediaMonitorMetrics(t, row.Metrics, 5)
				require.Len(t, row.Buckets, 1)
				assertMergedMediaMonitorMetrics(t, row.Buckets[0].Metrics, 1)
				require.Equal(t, service.ChannelMonitorV2HealthForWithThresholds(row.Metrics, cfg.HealthThresholds), row.Health)
			})
		}
	}
}

// AC-029: group identity survives media provider projection.
func TestMediaMonitorMatrixKeepsDifferentGroupsSeparate(t *testing.T) {
	repo, mock, filter, cfg := mediaMonitorRepositoryFixture(t, true, 8)
	result, err := repo.GetMatrix(context.Background(), filter, cfg, service.ChannelMonitorV2GroupByPlatformGroupModel, false)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	require.Len(t, result.Items, 2)
	require.Equal(t, int64(7), *result.Items[0].GroupID)
	require.Equal(t, int64(8), *result.Items[1].GroupID)
	require.Equal(t, "media", result.Items[0].Platform)
	require.Equal(t, "media", result.Items[1].Platform)
	require.Equal(t, int64(10), result.Items[0].Metrics.RequestCount)
	require.Equal(t, int64(100), result.Items[1].Metrics.RequestCount)
}

func assertMergedMediaMonitorMetrics(t *testing.T, m service.ChannelMonitorV2Metric, minutes float64) {
	t.Helper()
	require.Equal(t, int64(110), m.RequestCount)
	require.Equal(t, int64(98), m.SuccessRequests)
	require.Equal(t, int64(12), m.ErrorRequests)
	require.InDelta(t, 6.0/110, m.ErrorRate, 0.000001)
	require.InDelta(t, 98.0/110, m.SuccessRate, 0.000001)
	require.InDelta(t, 120.0/1100, m.CacheRate, 0.000001)
	require.InDelta(t, 110.0/minutes, m.RPM, 0.000001)
	require.Equal(t, int64(110), m.TTFT.SampleCount)
	require.InDelta(t, 101000.0/110, *m.TTFT.AvgMs, 0.000001)
	require.Equal(t, int64(1000), *m.TTFT.P50Ms)
	require.Equal(t, int64(1000), *m.TTFT.P90Ms)
	require.Equal(t, int64(1000), *m.TTFT.P95Ms)
	require.Equal(t, int64(1000), *m.Duration.P50Ms)
	require.Nil(t, m.UpstreamAffectedRequests)
}

func mediaMonitorRepositoryFixture(t *testing.T, matrix bool, toapisGroup int64) (*channelMonitorV2Repository, sqlmock.Sqlmock, service.ChannelMonitorV2Filter, service.ChannelMonitorV2Config) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	start := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	filter := service.ChannelMonitorV2Filter{Start: start, End: start.Add(5 * time.Minute), Bucket: time.Minute}
	cfg := service.ChannelMonitorV2Config{
		Platforms: []service.ChannelMonitorV2PlatformConfig{{Platform: "grsai", Enabled: true, Models: []string{"shared"}}, {Platform: "toapis", Enabled: true, Models: []string{"shared"}}},
		GroupIDs:  []int64{7, 8}, IgnoredErrorCategories: []string{"content_policy"},
		HealthThresholds: service.DefaultChannelMonitorV2HealthThresholds(),
	}
	mock.ExpectQuery("FROM channel_monitor_v2_watermarks").WillReturnRows(sqlmock.NewRows([]string{"usage", "error", "through", "computed", "backfill"}).AddRow(start, start, filter.End, filter.End, start))
	mock.ExpectQuery("FROM channel_monitor_v2_metrics_1m m").WillReturnRows(sqlmock.NewRows([]string{"bucket", "platform", "group_id", "group_name", "model", "success", "errors", "affected", "attempts", "input", "output", "cache_create", "cache_read", "ttft_sum", "ttft_count", "duration_sum", "duration_count"}).
		AddRow(start, "grsai", 7, "same-group", "shared", 8, 2, 1, 2, 80, 10, 0, 20, 1000, 10, 1000, 10).
		AddRow(start, "toapis", toapisGroup, "same-group", "shared", 90, 10, 3, 4, 900, 100, 0, 100, 100000, 100, 100000, 100))
	hist := sqlmock.NewRows([]string{"bucket", "platform", "group_id", "model", "user_id", "metric", "bound", "count"})
	for _, metric := range []string{"ttft", "duration"} {
		hist.AddRow(start, "grsai", 7, "shared", 0, metric, 100, 10)
		hist.AddRow(start, "toapis", toapisGroup, "shared", 0, metric, 1000, 100)
	}
	mock.ExpectQuery("FROM channel_monitor_v2_latency_histograms_1m h").WillReturnRows(hist)
	if matrix {
		mock.ExpectQuery("FROM groups WHERE id").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "platform"}).AddRow(7, "same-group", "composite").AddRow(8, "same-group", "composite"))
		mock.ExpectQuery("FROM channel_monitor_v2_error_metrics_1m e").WillReturnRows(sqlmock.NewRows([]string{"bucket", "platform", "group_id", "model", "errors"}).
			AddRow(start, "grsai", 7, "shared", 1).AddRow(start, "toapis", toapisGroup, "shared", 5))
	} else {
		mock.ExpectQuery("FROM channel_monitor_v2_error_metrics_1m e").WillReturnRows(sqlmock.NewRows([]string{"platform", "model", "errors"}).AddRow("grsai", "shared", 1).AddRow("toapis", "shared", 5))
	}
	return &channelMonitorV2Repository{db: db}, mock, filter, cfg
}
