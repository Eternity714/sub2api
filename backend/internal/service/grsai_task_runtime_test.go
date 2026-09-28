package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type grsaiRuntimeRepoStub struct {
	GrsaiV2TaskRepository
	bound, failed, completed, updated, deferred, manual int
	lastNext                                            time.Time
	manualReason                                        string
	amount                                              float64
}

func (r *grsaiRuntimeRepoStub) BindV2UpstreamTask(context.Context, int64, int64, string) (bool, error) {
	r.bound++
	return true, nil
}

func (r *grsaiRuntimeRepoStub) UpdateV2Progress(_ context.Context, _, _ int64, _ string, _ int, next time.Time) (bool, error) {
	r.updated++
	r.lastNext = next
	return true, nil
}

func (r *grsaiRuntimeRepoStub) DeferV2Failure(_ context.Context, _, _ int64, next time.Time) (bool, error) {
	r.deferred++
	r.lastNext = next
	return true, nil
}

func (r *grsaiRuntimeRepoStub) MarkV2ManualReview(_ context.Context, _, _ int64, reason string) (bool, error) {
	r.manual++
	r.manualReason = reason
	return true, nil
}

func (r *grsaiRuntimeRepoStub) CompleteV2(_ context.Context, _ int64, _ int64, _ []byte, _ []byte, _ *time.Time, amount float64, _ GrsaiSettlementTxFunc) (bool, error) {
	r.completed++
	r.amount = amount
	return true, nil
}

func (r *grsaiRuntimeRepoStub) FailV2(context.Context, int64, int64, string, string, GrsaiSettlementTxFunc) (bool, error) {
	r.failed++
	return true, nil
}

type grsaiRuntimePayloadStub struct {
	GrsaiTaskPayloadRepository
	reads int
}

func (p *grsaiRuntimePayloadStub) GetDecrypted(context.Context, string, int64) ([]byte, error) {
	p.reads++
	return []byte(`{"model":"test","replyType":"async"}`), nil
}

type grsaiRuntimeAccountStub struct{}

func (grsaiRuntimeAccountStub) GetByID(context.Context, int64) (*Account, error) {
	return &Account{Platform: PlatformGrsai, Type: AccountTypeAPIKey}, nil
}

type grsaiRuntimeUpstreamStub struct {
	posts, polls int
	result       *GrsaiUpstreamResult
	err          error
}

func (u *grsaiRuntimeUpstreamStub) GenerateAsync(context.Context, *Account, []byte) (*GrsaiUpstreamResult, error) {
	u.posts++
	return &GrsaiUpstreamResult{TaskID: "upstream-private", Status: GrsaiUpstreamStatusRunning}, nil
}

func (u *grsaiRuntimeUpstreamStub) Result(context.Context, *Account, string) (*GrsaiUpstreamResult, error) {
	u.polls++
	return u.result, u.err
}

type grsaiRuntimeImagesStub struct {
	err   error
	calls int
}

func (i *grsaiRuntimeImagesStub) PersistGrsaiImages(context.Context, string, *GrsaiUpstreamResult) (*GrsaiStoredResult, error) {
	i.calls++
	if i.err != nil {
		return nil, i.err
	}
	return &GrsaiStoredResult{ResultJSON: []byte(`{"results":[{"url":"https://local.example/a"}]`), ObjectMetadata: []byte(`[]`)}, nil
}

type grsaiRuntimeBalanceStub struct{}

func (grsaiRuntimeBalanceStub) ReserveGrsaiBalance(context.Context, *sql.Tx, *GrsaiSettlement) error {
	return nil
}
func (grsaiRuntimeBalanceStub) CaptureGrsaiBalanceTx(context.Context, *sql.Tx, *GrsaiSettlement) error {
	return nil
}
func (grsaiRuntimeBalanceStub) ReleaseGrsaiBalanceTx(context.Context, *sql.Tx, *GrsaiSettlement) error {
	return nil
}

func newGrsaiRuntimeTest(repo *grsaiRuntimeRepoStub, payload *grsaiRuntimePayloadStub, upstream *grsaiRuntimeUpstreamStub, images *grsaiRuntimeImagesStub) *GrsaiTaskRuntime {
	return NewGrsaiTaskRuntime(repo, payload, grsaiRuntimeAccountStub{}, upstream, images, grsaiRuntimeBalanceStub{}, GrsaiTaskRuntimeOptions{ScanInterval: time.Second, MaxRunning: 3})
}

func grsaiRuntimeClaim() *GrsaiSettlement {
	id := "grsai-local"
	return &GrsaiSettlement{ID: 1, AccountID: 2, UserID: 3, APIKeyID: 4, LocalTaskID: &id,
		ClaimVersion: 1, SubmissionAttempt: 1, BillableUnitPrice: 0.25, RequestedImageCount: 1}
}

func TestGrsaiTaskRuntimeNeverResubmitsUnboundClaim(t *testing.T) {
	repo, payload, upstream, images := &grsaiRuntimeRepoStub{}, &grsaiRuntimePayloadStub{}, &grsaiRuntimeUpstreamStub{}, &grsaiRuntimeImagesStub{}
	runtime := newGrsaiRuntimeTest(repo, payload, upstream, images)
	claim := grsaiRuntimeClaim()
	require.NoError(t, runtime.processClaim(context.Background(), claim))
	require.Equal(t, 1, upstream.posts)
	require.Equal(t, 1, repo.bound)
	require.Equal(t, 1, payload.reads)
	claim.SubmissionAttempt = 2
	require.NoError(t, runtime.processClaim(context.Background(), claim))
	require.Equal(t, 1, upstream.posts)
	require.Equal(t, 1, repo.failed)
}

func TestGrsaiTaskRuntimeBoundResultPersistsBeforeSuccess(t *testing.T) {
	repo, payload, upstream, images := &grsaiRuntimeRepoStub{}, &grsaiRuntimePayloadStub{}, &grsaiRuntimeUpstreamStub{}, &grsaiRuntimeImagesStub{}
	upstream.result = &GrsaiUpstreamResult{Status: GrsaiUpstreamStatusSucceeded, ImageURLs: []string{"https://upstream.example/a"}}
	runtime := newGrsaiRuntimeTest(repo, payload, upstream, images)
	claim := grsaiRuntimeClaim()
	upstreamID := "upstream-private"
	claim.UpstreamTaskID = &upstreamID
	claim.DeliveryMode = string(GrsaiDeliveryStream)
	usage := &grsaiUsageSpy{}
	runtime.WithUsageLogs(usage)
	images.err = errors.New("s3 unavailable")
	require.Error(t, runtime.processClaim(context.Background(), claim))
	require.Equal(t, 0, repo.completed)
	require.Equal(t, 1, repo.deferred)
	images.err = nil
	require.NoError(t, runtime.processClaim(context.Background(), claim))
	require.Equal(t, 1, repo.completed)
	require.Equal(t, 0, upstream.posts)
	require.Equal(t, 2, upstream.polls)
	require.Zero(t, payload.reads)
	require.Len(t, usage.logs, 1)
	require.Equal(t, "grsai_task:grsai-local", usage.logs[0].RequestID)
	require.Equal(t, RequestTypeStream, usage.logs[0].RequestType)
	require.InDelta(t, 0.25, usage.logs[0].ActualCost, 0.000001)
}

func TestGrsaiTaskRuntimeExhaustedStorageRetriesRetainsHoldForReview(t *testing.T) {
	repo, payload, upstream := &grsaiRuntimeRepoStub{}, &grsaiRuntimePayloadStub{}, &grsaiRuntimeUpstreamStub{}
	images := &grsaiRuntimeImagesStub{err: errors.New("s3 unavailable")}
	upstream.result = &GrsaiUpstreamResult{Status: GrsaiUpstreamStatusSucceeded, ImageURLs: []string{"https://upstream.example/a"}}
	runtime := newGrsaiRuntimeTest(repo, payload, upstream, images)
	claim := grsaiRuntimeClaim()
	upstreamID := "upstream-private"
	claim.UpstreamTaskID = &upstreamID
	claim.SettlementRetryCount = 4
	require.Error(t, runtime.processClaim(context.Background(), claim))
	require.Equal(t, 1, repo.manual)
	require.Zero(t, repo.failed)
	require.Zero(t, repo.completed)
}

func TestGrsaiTaskRuntimeResultErrorsBackOffThenRequireReview(t *testing.T) {
	repo, payload, upstream, images := &grsaiRuntimeRepoStub{}, &grsaiRuntimePayloadStub{}, &grsaiRuntimeUpstreamStub{err: errors.New("temporary upstream failure")}, &grsaiRuntimeImagesStub{}
	runtime := newGrsaiRuntimeTest(repo, payload, upstream, images)
	now := time.Now().UTC()
	runtime.now = func() time.Time { return now }
	claim := grsaiRuntimeClaim()
	upstreamID := "upstream-private"
	claim.UpstreamTaskID = &upstreamID
	for retry := 0; retry < runtime.opts.FailureRetryLimit; retry++ {
		claim.SettlementRetryCount = retry
		require.Error(t, runtime.processClaim(context.Background(), claim))
		if retry+1 < runtime.opts.FailureRetryLimit {
			require.Equal(t, retry+1, repo.deferred)
			require.Equal(t, now.Add(grsaiSettlementRecoveryBackoff[retry]), repo.lastNext)
			require.Zero(t, repo.manual)
		}
	}
	require.Equal(t, runtime.opts.FailureRetryLimit, upstream.polls)
	require.Equal(t, 1, repo.manual)
	require.Equal(t, "result_poll_failed", repo.manualReason)
	require.Zero(t, repo.failed)
	require.Zero(t, repo.completed)
	require.Zero(t, images.calls)
}

// GRSAI-VIDEO-04/05/06: output count is metadata, never a charge multiplier.
type grsaiRuntimeVideosStub struct {
	calls int
	err   error
}

func (v *grsaiRuntimeVideosStub) PersistGrsaiVideo(context.Context, string, *GrsaiUpstreamResult) (*GrsaiStoredResult, error) {
	v.calls++
	if v.err != nil {
		return nil, v.err
	}
	return &GrsaiStoredResult{ResultCount: 3, ResultJSON: []byte(`{"results":[{"url":"https://local/a.mp4"},{"url":"https://local/b.mp4"},{"url":"https://local/c.mp4"}]}`)}, nil
}
func TestGrsaiTaskRuntimeVideoDispatchUsageAndSingleCharge(t *testing.T) {
	for _, mode := range []GrsaiDeliveryMode{GrsaiDeliveryAsync, GrsaiDeliveryJSON, GrsaiDeliveryStream} {
		t.Run(string(mode), func(t *testing.T) {
			repo, payload, upstream, images := &grsaiRuntimeRepoStub{}, &grsaiRuntimePayloadStub{}, &grsaiRuntimeUpstreamStub{}, &grsaiRuntimeImagesStub{}
			videos := &grsaiRuntimeVideosStub{err: errors.New("storage failed")}
			runtime := newGrsaiRuntimeTest(repo, payload, upstream, images).WithVideos(videos)
			claim := grsaiRuntimeClaim()
			id := "bound-task"
			claim.UpstreamTaskID = &id
			claim.MediaKind = "video"
			claim.VideoDurationSeconds = 5
			claim.VideoResolution = "768p"
			claim.BaseUnitPrice = .14
			claim.GroupRateMultiplier = 1.5
			claim.AccountRateMultiplier = 1.2
			claim.BillableUnitPrice = 1.26
			claim.DeliveryMode = string(mode)
			upstream.result = &GrsaiUpstreamResult{Status: GrsaiUpstreamStatusSucceeded, ImageURLs: []string{"a", "b", "c"}}
			usage := &grsaiUsageSpy{}
			runtime.WithUsageLogs(usage)
			require.Error(t, runtime.processClaim(context.Background(), claim))
			require.Zero(t, repo.completed)
			require.Empty(t, usage.logs)
			videos.err = nil
			require.NoError(t, runtime.processClaim(context.Background(), claim))
			require.Zero(t, images.calls)
			require.Zero(t, upstream.posts)
			require.Equal(t, 1, repo.completed)
			require.Len(t, usage.logs, 1)
			log := usage.logs[0]
			require.Equal(t, string(BillingModeVideo), *log.BillingMode)
			require.Zero(t, log.ImageCount)
			require.Nil(t, log.ImageSize)
			require.Equal(t, 3, log.VideoCount)
			require.Equal(t, 5, *log.VideoDurationSeconds)
			require.Equal(t, "768p", *log.VideoResolution)
			require.InDelta(t, .7, log.TotalCost, 1e-9)
			require.InDelta(t, 1.26, log.ActualCost, 1e-9)
			require.InDelta(t, 1.26, repo.amount, 1e-9)
		})
	}
}
