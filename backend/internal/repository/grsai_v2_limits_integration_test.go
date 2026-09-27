//go:build integration

package repository

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGrsaiV2WaitingAndRunningLimitsPersistAcrossWorkers(t *testing.T) {
	ctx := context.Background()
	repo := grsaiV2TestRepo(t)
	var tasks []*GrsaiSettlement
	for range 20 {
		params := grsaiV2TestParams(t)
		params.MaxWaiting = 20
		record, err := repo.CreateV2GrsaiTask(ctx, params, grsaiV2NoopBalance)
		require.NoError(t, err)
		tasks = append(tasks, record)
		t.Cleanup(func() {
			_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM grsai_task_payloads WHERE local_task_id = $1`, params.LocalTaskID)
		})
	}
	cleanupCreatedGrsaiSettlements(t, tasks...)
	params := grsaiV2TestParams(t)
	params.MaxWaiting = 20
	_, err := repo.CreateV2GrsaiTask(ctx, params, grsaiV2NoopBalance)
	require.True(t, errors.Is(err, ErrGrsaiTaskWaitingLimit), "%v", err)
	now := time.Now().UTC()
	first, err := repo.ClaimDueV2(ctx, now, 20, now.Add(time.Minute), 3)
	require.NoError(t, err)
	require.Len(t, first, 3)
	second, err := repo.ClaimDueV2(ctx, now, 20, now.Add(time.Minute), 3)
	require.NoError(t, err)
	require.Empty(t, second)
	failed, err := repo.FailV2(ctx, first[0].ID, first[0].ClaimVersion, "test_failure", "", grsaiV2NoopBalance)
	require.NoError(t, err)
	require.True(t, failed)
	third, err := repo.ClaimDueV2(ctx, now, 20, now.Add(time.Minute), 3)
	require.NoError(t, err)
	require.Len(t, third, 1)
	require.NotEqual(t, first[0].ID, third[0].ID)
	newTask := grsaiV2TestParams(t)
	newTask.MaxWaiting = 20
	created, err := repo.CreateV2GrsaiTask(ctx, newTask, grsaiV2NoopBalance)
	require.NoError(t, err, "running claims should free waiting capacity")
	cleanupCreatedGrsaiSettlements(t, created)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM grsai_task_payloads WHERE local_task_id = $1`, newTask.LocalTaskID)
	})
}

func TestGrsaiV2WaitingLimitIsAtomicAcrossInstances(t *testing.T) {
	ctx := context.Background()
	params := []service.CreateV2GrsaiTaskParams{grsaiV2TestParams(t), grsaiV2TestParams(t)}
	t.Cleanup(func() {
		for _, input := range params {
			_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM grsai_task_payloads WHERE local_task_id = $1`, input.LocalTaskID)
		}
	})
	repos := []*grsaiSettlementRepository{grsaiV2TestRepo(t), grsaiV2TestRepo(t)}
	start := make(chan struct{})
	type outcome struct {
		record *GrsaiSettlement
		err    error
	}
	results := make(chan outcome, len(params))
	var wg sync.WaitGroup
	for i, input := range params {
		wg.Add(1)
		go func(input service.CreateV2GrsaiTaskParams, repo *grsaiSettlementRepository) {
			defer wg.Done()
			<-start
			input.MaxWaiting = 1
			record, err := repo.CreateV2GrsaiTask(ctx, input, grsaiV2NoopBalance)
			results <- outcome{record: record, err: err}
		}(input, repos[i])
	}
	close(start)
	wg.Wait()
	close(results)
	var created, rejected int
	for result := range results {
		if result.err == nil {
			created++
			cleanupCreatedGrsaiSettlements(t, result.record)
			continue
		}
		require.ErrorIs(t, result.err, ErrGrsaiTaskWaitingLimit)
		rejected++
	}
	require.Equal(t, 1, created)
	require.Equal(t, 1, rejected)
}
