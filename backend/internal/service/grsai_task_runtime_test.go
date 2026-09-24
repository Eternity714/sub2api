package service

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type grsaiDrainStream struct {
	entered chan struct{}
	release chan struct{}
}

type grsaiClaimAfterStopRepo struct {
	*grsaiTaskRepoMemory
	entered chan context.Context
	release chan struct{}
}

func (r *grsaiClaimAfterStopRepo) ClaimDueForDeliveryMode(ctx context.Context, now time.Time, limit int, lease time.Time, mode GrsaiDeliveryMode) ([]*GrsaiSettlement, error) {
	r.entered <- ctx
	<-r.release
	return r.grsaiTaskRepoMemory.ClaimDueForDeliveryMode(context.Background(), now, limit, lease, mode)
}

func TestGrsaiDeliveryStopAfterClaimDoesNotStartNewPost(t *testing.T) {
	tasks, base, payloads, upstream := grsaiTaskFixture(t)
	base.record = &GrsaiSettlement{ID: 17, AccountID: 3, UserID: 1, APIKeyID: 4, Model: "m", RequestedImageCount: 1, DeliveryMode: GrsaiDeliveryAsync, InternalStatus: "pending_upstream", UpstreamStatus: "not_submitted", ClaimVersion: 1}
	payloads.values = map[int64][]byte{17: append([]byte("ciphertext:"), []byte(`{"model":"m","replyType":"async"}`)...)}
	repo := &grsaiClaimAfterStopRepo{grsaiTaskRepoMemory: base, entered: make(chan context.Context, 1), release: make(chan struct{})}
	tasks.Repo = repo
	runtime := NewGrsaiTaskRuntime(tasks, repo, nil, GrsaiTaskRuntimeOptions{Enabled: true, ScanInterval: time.Hour, BatchLimit: 1})
	runtime.Start()
	var claimCtx context.Context
	select {
	case claimCtx = <-repo.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("claim did not begin")
	}
	stopped := make(chan struct{})
	go func() { runtime.Stop(); close(stopped) }()
	select {
	case <-claimCtx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not cancel the claim scan")
	}
	close(repo.release)
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not finish")
	}
	require.Zero(t, upstream.posts)
	require.Contains(t, payloads.values, int64(17))
}

func (s *grsaiDrainStream) OpenGenerateStream(ctx context.Context, _ *Account, _ []byte) (*GrsaiUpstreamStream, error) {
	close(s.entered)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.release:
		return &GrsaiUpstreamStream{StatusCode: 200, ContentType: "text/event-stream", Body: io.NopCloser(strings.NewReader("data: {\"id\":\"drain-1\",\"status\":\"succeeded\",\"progress\":100,\"results\":[{\"url\":\"https://img.invalid/drain\"}]}\n\n"))}, nil
	}
}

func TestGrsaiDeliveryStopDrainsStartedSubmissionWithoutCancel(t *testing.T) {
	tasks, repo, payloads, _ := grsaiTaskFixture(t)
	repo.record = &GrsaiSettlement{ID: 17, AccountID: 3, UserID: 1, APIKeyID: 4, Model: "m", RequestedImageCount: 1, DeliveryMode: GrsaiDeliveryAsync, InternalStatus: "pending_upstream", UpstreamStatus: "not_submitted", ClaimVersion: 1}
	payloads.values = map[int64][]byte{17: append([]byte("ciphertext:"), []byte(`{"model":"m","replyType":"async"}`)...)}
	stream := &grsaiDrainStream{entered: make(chan struct{}), release: make(chan struct{})}
	tasks.Upstream = stream
	runtime := NewGrsaiTaskRuntime(tasks, repo, nil, GrsaiTaskRuntimeOptions{Enabled: true, ScanInterval: time.Hour, BatchLimit: 1})
	runtime.Start()
	select {
	case <-stream.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not start the upstream request")
	}
	stopped := make(chan struct{})
	go func() { runtime.Stop(); close(stopped) }()
	select {
	case <-stopped:
		t.Fatal("Stop returned while a submission was still in flight")
	case <-time.After(20 * time.Millisecond):
	}
	close(stream.release)
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not finish after the submission drained")
	}
	require.NotEqual(t, "manual_review", repo.record.InternalStatus)
}

func TestGrsaiDeliveryRuntimeDrainsAfterNewAcceptanceDisabledAndStop(t *testing.T) {
	tasks, repo, _, _ := grsaiTaskFixture(t)
	native := &grsaiTaskNativeMemory{}
	cfg := &config.Config{}
	runtime := ProvideGrsaiTaskRuntime(tasks, repo, native, cfg)
	require.True(t, runtime.Running(), "existing async tasks need a worker after new acceptance is disabled")
	runtime.Stop()

	cfg.GrsaiDelivery.Enabled = true
	cfg.GrsaiDelivery.ScanIntervalSeconds = 1
	cfg.GrsaiDelivery.BatchLimit = 1
	runtime = ProvideGrsaiTaskRuntime(tasks, repo, native, cfg)
	require.True(t, runtime.Running())
	runtime.Stop()
	require.False(t, runtime.Running())
}

type grsaiCleanupRepo struct {
	*grsaiTaskRepoMemory
	at        time.Time
	retention time.Duration
	limit     int
}

func (r *grsaiCleanupRepo) DeleteTerminal(_ context.Context, at time.Time, retention time.Duration, limit int) (int64, error) {
	r.at, r.retention, r.limit = at, retention, limit
	return 0, nil
}

type grsaiCleanupPayload struct {
	*grsaiTaskPayloadMemory
	at time.Time
}

func (p *grsaiCleanupPayload) DeleteExpired(_ context.Context, at time.Time) (int64, error) {
	p.at = at
	return 0, nil
}

func TestGrsaiDeliveryRuntimeCleansEvenWhenNoClaims(t *testing.T) {
	tasks, repo, payloads, _ := grsaiTaskFixture(t)
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	cleanRepo := &grsaiCleanupRepo{grsaiTaskRepoMemory: repo}
	cleanPayload := &grsaiCleanupPayload{grsaiTaskPayloadMemory: payloads}
	tasks.Repo = cleanRepo
	tasks.Payloads = cleanPayload
	runtime := &GrsaiTaskRuntime{Tasks: tasks, Repo: cleanRepo, Options: GrsaiTaskRuntimeOptions{
		Enabled: true, BatchLimit: 7, ResultRetention: 24 * time.Hour, Now: func() time.Time { return now },
	}}
	runtime.RunOnce(context.Background())
	require.Equal(t, now, cleanPayload.at)
	require.Equal(t, now, cleanRepo.at)
	require.Equal(t, 24*time.Hour, cleanRepo.retention)
	require.Equal(t, 7, cleanRepo.limit)
}

type grsaiTaskNativeMemory struct {
	posts   int
	results int
	result  *GrsaiUpstreamResult
}

func (u *grsaiTaskNativeMemory) Generate(context.Context, *Account, []byte) (*GrsaiUpstreamResult, error) {
	u.posts++
	return u.result, nil
}
func (u *grsaiTaskNativeMemory) Result(context.Context, *Account, string) (*GrsaiUpstreamResult, error) {
	u.results++
	return u.result, nil
}

func TestBoundDisconnectRecoveryPollsResultWithoutSecondPost(t *testing.T) {
	tasks, repo, payloads, stream := grsaiTaskFixture(t)
	_ = payloads
	claim := &GrsaiSettlement{ID: 17, AccountID: 3, UserID: 1, APIKeyID: 4, Model: "m", DeliveryMode: GrsaiDeliveryAsync, InternalStatus: "processing", UpstreamStatus: GrsaiUpstreamStatusRunning, ClaimVersion: 1, CreatedAt: time.Now().Add(-time.Minute)}
	id := "upstream-17"
	claim.UpstreamTaskID = &id
	repo.record = claim
	native := &grsaiTaskNativeMemory{result: &GrsaiUpstreamResult{HTTPStatus: 200, TaskID: id, Status: GrsaiUpstreamStatusRunning}}
	runtime := &GrsaiTaskRuntime{Tasks: tasks, Repo: repo, Upstream: native, Options: GrsaiTaskRuntimeOptions{Enabled: true}}
	runtime.runClaim(context.Background(), claim)
	require.Equal(t, 0, stream.posts)
	require.Equal(t, 1, native.results)
}

func TestLegacyAsyncSuccessWithoutResultQueriesProviderInsteadOfSettling(t *testing.T) {
	tasks, repo, _, stream := grsaiTaskFixture(t)
	id := "upstream-17"
	claim := &GrsaiSettlement{ID: 17, AccountID: 3, UserID: 1, APIKeyID: 4, Model: "m",
		DeliveryMode: GrsaiDeliveryAsync, InternalStatus: "processing", UpstreamStatus: GrsaiUpstreamStatusSucceeded,
		ClaimVersion: 1, UpstreamTaskID: &id}
	repo.record = claim
	native := &grsaiTaskNativeMemory{result: &GrsaiUpstreamResult{HTTPStatus: 200, TaskID: id,
		Status: GrsaiUpstreamStatusSucceeded, ResultURLs: []string{"https://img.invalid/recovered.png"}}}
	runtime := &GrsaiTaskRuntime{Tasks: tasks, Repo: repo, Upstream: native, Options: GrsaiTaskRuntimeOptions{Enabled: true}}
	runtime.runClaim(context.Background(), claim)
	require.Zero(t, stream.posts)
	require.Equal(t, 1, native.results)
}

func TestQueuedTaskMissingPayloadMovesToManualReview(t *testing.T) {
	tasks, repo, _, upstream := grsaiTaskFixture(t)
	claim := &GrsaiSettlement{ID: 17, AccountID: 3, UserID: 1, APIKeyID: 4, Model: "m", DeliveryMode: GrsaiDeliveryAsync, InternalStatus: "processing", UpstreamStatus: "not_submitted", ClaimVersion: 1}
	repo.record = claim
	runtime := &GrsaiTaskRuntime{Tasks: tasks, Repo: repo, Options: GrsaiTaskRuntimeOptions{Enabled: true}}
	runtime.runClaim(context.Background(), claim)
	require.Equal(t, "manual_review", repo.record.InternalStatus)
	require.Equal(t, 0, upstream.posts)
}

func TestSubmittingTaskTimeoutMovesToManualReviewWithoutPost(t *testing.T) {
	tasks, repo, _, upstream := grsaiTaskFixture(t)
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	claim := &GrsaiSettlement{ID: 17, AccountID: 3, UserID: 1, APIKeyID: 4, Model: "m", DeliveryMode: GrsaiDeliveryAsync, InternalStatus: "processing", UpstreamStatus: "submitting", ClaimVersion: 1, CreatedAt: now.Add(-time.Hour)}
	repo.record = claim
	runtime := &GrsaiTaskRuntime{Tasks: tasks, Repo: repo, Options: GrsaiTaskRuntimeOptions{Enabled: true, SubmissionUnknownTimeout: 10 * time.Minute, Now: func() time.Time { return now }}}
	runtime.runClaim(context.Background(), claim)
	require.Equal(t, "manual_review", repo.record.InternalStatus)
	require.Equal(t, 0, upstream.posts)
}

func TestRuntimeClaimsOneTaskAtATimeEvenWhenBatchLimitIsLarger(t *testing.T) {
	tasks, repo, _, _ := grsaiTaskFixture(t)
	repo.record = &GrsaiSettlement{
		ID: 17, AccountID: 3, UserID: 1, APIKeyID: 4, Model: "m",
		DeliveryMode: GrsaiDeliveryAsync, InternalStatus: "pending_upstream",
		UpstreamStatus: "unknown", ClaimVersion: 1, CreatedAt: time.Now(),
	}
	runtime := &GrsaiTaskRuntime{
		Tasks: tasks,
		Repo:  repo,
		Options: GrsaiTaskRuntimeOptions{
			Enabled: true, BatchLimit: 20, SubmissionUnknownTimeout: time.Hour,
		},
	}

	runtime.RunOnce(context.Background())

	require.NotEmpty(t, repo.claimLimits)
	for _, limit := range repo.claimLimits {
		require.Equal(t, 1, limit)
	}
}
