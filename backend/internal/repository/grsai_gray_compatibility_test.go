//go:build integration

package repository

import (
	"context"
	"database/sql"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type grsaiGrayAccountReader struct{}

func (grsaiGrayAccountReader) GetByID(context.Context, int64) (*service.Account, error) {
	return &service.Account{Platform: service.PlatformGrsai, Type: service.AccountTypeAPIKey}, nil
}

type grsaiGrayUpstream struct {
	db      *sql.DB
	localID string
	posts   atomic.Int32
	polls   atomic.Int32
}

func (u *grsaiGrayUpstream) GenerateAsync(ctx context.Context, _ *service.Account, _ []byte) (*service.GrsaiUpstreamResult, error) {
	var count int
	if err := u.db.QueryRowContext(ctx, `SELECT count(*) FROM grsai_settlements WHERE local_task_id = $1 AND task_version = 2`, u.localID).Scan(&count); err != nil || count != 1 {
		return nil, service.ErrGrsaiSettlementInvalidState
	}
	u.posts.Add(1)
	return &service.GrsaiUpstreamResult{TaskID: "private-upstream-id", Status: service.GrsaiUpstreamStatusRunning}, nil
}

func (u *grsaiGrayUpstream) Result(context.Context, *service.Account, string) (*service.GrsaiUpstreamResult, error) {
	u.polls.Add(1)
	return &service.GrsaiUpstreamResult{Status: service.GrsaiUpstreamStatusRunning}, nil
}

type grsaiGrayImages struct{}

func (grsaiGrayImages) PersistGrsaiImages(context.Context, string, *service.GrsaiUpstreamResult) (*service.GrsaiStoredResult, error) {
	return nil, service.ErrGrsaiSettlementInvalidState
}

type grsaiGrayBalance struct{}

func (grsaiGrayBalance) ReserveGrsaiBalance(context.Context, *sql.Tx, *service.GrsaiSettlement) error {
	return nil
}
func (grsaiGrayBalance) CaptureGrsaiBalanceTx(context.Context, *sql.Tx, *service.GrsaiSettlement) error {
	return nil
}
func (grsaiGrayBalance) ReleaseGrsaiBalanceTx(context.Context, *sql.Tx, *service.GrsaiSettlement) error {
	return nil
}

func TestGrsaiGrayCandidateHandoffUsesBoundResultOnly(t *testing.T) {
	ctx := context.Background()
	repo := grsaiV2TestRepo(t)
	params := grsaiV2TestParams(t)
	v2, err := repo.CreateV2GrsaiTask(ctx, params, grsaiV2NoopBalance)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, v2)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM grsai_task_payloads WHERE local_task_id = $1`, params.LocalTaskID)
	})
	legacyParams := grsaiSettlementTestParams(t, "gray-old")
	legacyParams.NextAttemptAt = time.Now().UTC().Add(-time.Minute)
	legacy, err := repo.Create(ctx, legacyParams)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, legacy)

	oldClaims, err := repo.ClaimDue(ctx, time.Now().UTC(), 10, time.Now().UTC().Add(time.Minute))
	require.NoError(t, err)
	require.Contains(t, settlementIDs(oldClaims), legacy.ID)
	require.NotContains(t, settlementIDs(oldClaims), v2.ID)

	upstream := &grsaiGrayUpstream{db: integrationDB, localID: params.LocalTaskID}
	makeRuntime := func() *service.GrsaiTaskRuntime {
		return service.NewGrsaiTaskRuntime(grsaiV2TestRepo(t), NewGrsaiTaskPayloadRepository(integrationDB, repo.encryptor),
			grsaiGrayAccountReader{}, upstream, grsaiGrayImages{}, grsaiGrayBalance{},
			service.GrsaiTaskRuntimeOptions{ScanInterval: time.Millisecond, BatchLimit: 1, MaxRunning: 3})
	}
	require.NoError(t, makeRuntime().RunOnce(ctx))
	require.EqualValues(t, 1, upstream.posts.Load())
	bound, err := repo.GetByID(ctx, v2.ID)
	require.NoError(t, err)
	require.NotNil(t, bound.UpstreamTaskID)
	time.Sleep(20 * time.Millisecond)
	require.NoError(t, makeRuntime().RunOnce(ctx))
	require.EqualValues(t, 1, upstream.posts.Load())
	require.EqualValues(t, 1, upstream.polls.Load())
}

func settlementIDs(records []*GrsaiSettlement) []int64 {
	ids := make([]int64, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	return ids
}
