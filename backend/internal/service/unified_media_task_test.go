//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

// AC-004, AC-024 (public-view scope): in_progress hides outputs at 50; failed completed is 100 with error and no expiry.
func TestUnifiedMediaPublicCompletedMeansFinished(t *testing.T) {
	task := &UnifiedMediaTask{ID: "media_test", Model: "example", MediaType: "image", Status: "completed", CreatedAt: time.Unix(10, 0), CompletedAt: unifiedMediaTimePtr(time.Unix(20, 0)), Units: []UnifiedMediaUnit{{State: "failed", Error: "upstream failed"}}}
	out := task.PublicView()
	require.Equal(t, 100, out.Progress)
	require.Equal(t, "completed", out.Status)
	require.EqualValues(t, 20, *out.CompletedAt)
	require.Empty(t, out.Result.Data)
	require.NotEmpty(t, out.Error)
	require.Nil(t, out.ExpiresAt)
	task.Status = "in_progress"
	require.Equal(t, 50, task.PublicView().Progress)
	require.Empty(t, task.PublicView().Result.Data)
}
func unifiedMediaTimePtr(v time.Time) *time.Time { return &v }

type unifiedMediaUpstreamStub struct{ submits int }

func (u *unifiedMediaUpstreamStub) Submit(context.Context, *Account, string, string, *mediacapability.NormalizedRequest, int) (*UnifiedMediaUpstreamResult, error) {
	u.submits++
	return &UnifiedMediaUpstreamResult{Disposition: "accepted", Status: "in_progress", TaskID: "upstream"}, nil
}
func (u *unifiedMediaUpstreamStub) Poll(context.Context, *Account, string, string, string) (*UnifiedMediaUpstreamResult, error) {
	return &UnifiedMediaUpstreamResult{Disposition: "accepted", Status: "completed", URLs: []string{"https://upstream/image"}}, nil
}
func TestUnifiedMediaExpiredSubmissionIntentCannotResubmit(t *testing.T) {
	repo := &unifiedMediaMemoryRepo{}
	upstream := &unifiedMediaUpstreamStub{}
	svc := &UnifiedMediaTaskService{Repo: repo, Upstream: upstream, MaxFailures: 3, Now: time.Now}
	task := &UnifiedMediaTask{ID: "media_test", MediaType: "image", Units: []UnifiedMediaUnit{{State: "submitting", AccountID: 1, Attempt: 1}}, Deadline: time.Now().Add(time.Hour)}
	require.NoError(t, svc.process(context.Background(), task))
	require.Zero(t, upstream.submits)
	require.Equal(t, "failed", task.Units[0].State)
	require.Equal(t, 1, repo.finalized)
}

type unifiedMediaRecoveryRepo struct {
	unifiedMediaMemoryRepo
	snapshot  []byte
	claimed   bool
	completed *UnifiedMediaTask
}

func (r *unifiedMediaRecoveryRepo) ClaimDue(context.Context, time.Time, int, time.Time) ([]*UnifiedMediaTask, error) {
	if r.claimed {
		return nil, nil
	}
	var task UnifiedMediaTask
	if err := json.Unmarshal(r.snapshot, &task); err != nil {
		return nil, err
	}
	r.claimed = true
	return []*UnifiedMediaTask{&task}, nil
}
func (r *unifiedMediaRecoveryRepo) Finalize(ctx context.Context, task *UnifiedMediaTask) error {
	r.completed = task
	return r.unifiedMediaMemoryRepo.Finalize(ctx, task)
}

type unifiedMediaRecoveryUpstream struct {
	unifiedMediaUpstreamStub
	accountIDs []int64
	models     []string
	taskIDs    []string
}

func (u *unifiedMediaRecoveryUpstream) Poll(_ context.Context, account *Account, _ string, model, taskID string) (*UnifiedMediaUpstreamResult, error) {
	u.accountIDs = append(u.accountIDs, account.ID)
	u.models = append(u.models, model)
	u.taskIDs = append(u.taskIDs, taskID)
	return &UnifiedMediaUpstreamResult{Disposition: "accepted", Status: "completed", URLs: []string{"recovered-result"}}, nil
}

// AC-010 (worker recovery scope): a new service resumes a persisted accepted task on its original account without submitting again.
func TestUnifiedMediaRestartResumesAcceptedTaskOnOriginalAccount(t *testing.T) {
	now := time.Now()
	accepted := &UnifiedMediaTask{
		ID: "media_recovered", MediaType: "image", Status: "in_progress", RequestedCount: 1, UnitPrice: 0.5, HeldAmount: 0.5,
		Deadline: now.Add(time.Hour),
		Units:    []UnifiedMediaUnit{{State: "polling", Count: 1, AccountID: 7, Platform: PlatformGrsai, UpstreamModel: "accepted-model", UpstreamTaskID: "accepted-task-123", Attempt: 1, NextAttemptAt: now.Add(-time.Second)}},
	}
	snapshot, err := json.Marshal(accepted)
	require.NoError(t, err)
	// Only the persisted snapshot is retained across construction of the replacement worker.
	repo := &unifiedMediaRecoveryRepo{snapshot: snapshot}
	upstream := &unifiedMediaRecoveryUpstream{}
	persister := &unifiedMediaPersister{}
	accounts := &unifiedMediaAccounts{rows: []Account{{ID: 99, Platform: "toapis"}, {ID: 7, Platform: PlatformGrsai}}}
	restarted := NewUnifiedMediaTaskService(repo, nil, nil, accounts, nil, upstream, persister)
	require.NoError(t, restarted.RunOnce(context.Background()))
	require.Zero(t, upstream.submits)
	require.Equal(t, []int64{7}, upstream.accountIDs)
	require.Equal(t, []string{"accepted-model"}, upstream.models)
	require.Equal(t, []string{"accepted-task-123"}, upstream.taskIDs)
	require.Equal(t, []string{"recovered-result"}, persister.calls)
	require.NotNil(t, repo.completed)
	require.Equal(t, "completed", repo.completed.Status)
	require.Equal(t, []string{"https://s3/recovered-result"}, repo.completed.PublicView().Result.Data)
	require.Equal(t, 0.5, UnifiedMediaTaskCharge(repo.completed))
	require.Equal(t, 1, repo.finalized)
	require.NoError(t, restarted.RunOnce(context.Background()))
	require.Zero(t, upstream.submits)
	require.Len(t, upstream.taskIDs, 1)
	require.Len(t, persister.calls, 1)
	require.Equal(t, 1, repo.finalized)
}

type unifiedMediaMemoryRepo struct {
	finalized      int
	reservedAmount float64
}

func (r *unifiedMediaMemoryRepo) Create(context.Context, *UnifiedMediaTask) error { return nil }
func (r *unifiedMediaMemoryRepo) GetOwned(context.Context, int64, int64, string, string) (*UnifiedMediaTask, error) {
	return nil, ErrUnifiedMediaTaskNotFound
}
func (r *unifiedMediaMemoryRepo) ClaimDue(context.Context, time.Time, int, time.Time) ([]*UnifiedMediaTask, error) {
	return nil, nil
}
func (r *unifiedMediaMemoryRepo) Save(context.Context, *UnifiedMediaTask, time.Time) error {
	return nil
}
func (r *unifiedMediaMemoryRepo) ReserveAccount(_ context.Context, _ *UnifiedMediaTask, _ int, _ *Account, amount float64) error {
	r.reservedAmount = amount
	return nil
}
func (r *unifiedMediaMemoryRepo) SettleAccount(context.Context, *UnifiedMediaTask, int, bool) error {
	return nil
}
func (r *unifiedMediaMemoryRepo) Finalize(_ context.Context, t *UnifiedMediaTask) error {
	r.finalized++
	t.Status = "completed"
	return nil
}

func (r *unifiedMediaMemoryRepo) ReleaseClaim(context.Context, *UnifiedMediaTask, time.Time) error {
	return nil
}

// An embedded repository makes unexpected calls fail loudly without a fake implementation of unrelated methods.
type unifiedMediaAccounts struct {
	AccountRepository
	rows []Account
}

func (r *unifiedMediaAccounts) ListSchedulableByGroupID(context.Context, int64) ([]Account, error) {
	return r.rows, nil
}
func (r *unifiedMediaAccounts) GetByID(_ context.Context, id int64) (*Account, error) {
	for _, account := range r.rows {
		if account.ID == id {
			copy := account
			return &copy, nil
		}
	}
	return nil, ErrAccountNotFound
}

type unifiedMediaPersister struct {
	calls   []string
	failURL string
}

func (p *unifiedMediaPersister) Persist(_ context.Context, _ string, _ string, _ int, _ int, url string) (*UnifiedMediaStoredResult, error) {
	p.calls = append(p.calls, url)
	if url == p.failURL {
		return nil, errors.New("temporary storage error")
	}
	return &UnifiedMediaStoredResult{URL: "https://s3/" + url}, nil
}

// AC-009 (worker scope): storage retries keep the task unfinished and hide partial outputs until retries finish.
func TestUnifiedMediaPartialStorageRetriesKeepSuccessfulResult(t *testing.T) {
	now := time.Now()
	repo := &unifiedMediaMemoryRepo{}
	persister := &unifiedMediaPersister{failURL: "second"}
	svc := &UnifiedMediaTaskService{Repo: repo, Persister: persister, MaxFailures: 2, Now: func() time.Time { return now }}
	task := &UnifiedMediaTask{ID: "media_test", MediaType: "image", UnitPrice: 0.5, RequestedCount: 2, Units: []UnifiedMediaUnit{{State: "persisting", Count: 2, URLs: []string{"first", "second"}}}, Deadline: now.Add(time.Hour)}
	require.NoError(t, svc.process(context.Background(), task))
	require.Equal(t, []string{"first", "second"}, persister.calls)
	require.Len(t, task.Units[0].Stored, 1)
	require.Empty(t, task.PublicView().Result.Data)
	require.Equal(t, "in_progress", task.Status)
	now = now.Add(time.Minute)
	require.NoError(t, svc.process(context.Background(), task))
	require.Equal(t, []string{"first", "second", "second"}, persister.calls)
	require.Equal(t, "failed", task.Units[0].State)
	require.Equal(t, "completed", task.Status)
	require.Equal(t, 0.5, UnifiedMediaTaskCharge(task))
	require.Len(t, task.PublicView().Result.Data, 1)
	require.NotEmpty(t, task.PublicView().Error)
}
func TestUnifiedMediaFirstStorageFailureDoesNotBlockLaterResults(t *testing.T) {
	for _, recoverFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("recover_first_%t", recoverFirst), func(t *testing.T) {
			now := time.Now()
			repo := &unifiedMediaMemoryRepo{}
			persister := &unifiedMediaPersister{failURL: "first"}
			svc := &UnifiedMediaTaskService{Repo: repo, Persister: persister, MaxFailures: 2, Now: func() time.Time { return now }}
			task := &UnifiedMediaTask{ID: "media_test", MediaType: "image", UnitPrice: 0.5, RequestedCount: 2, Units: []UnifiedMediaUnit{{State: "persisting", Count: 2, URLs: []string{"first", "second"}}}, Deadline: now.Add(time.Hour)}
			require.NoError(t, svc.process(context.Background(), task))
			require.Equal(t, []string{"first", "second"}, persister.calls)
			require.Equal(t, "https://s3/second", task.Units[0].Stored[0].URL)
			require.Empty(t, task.PublicView().Result.Data)
			if recoverFirst {
				persister.failURL = ""
			}
			now = now.Add(time.Minute)
			require.NoError(t, svc.process(context.Background(), task))
			require.Equal(t, []string{"first", "second", "first"}, persister.calls)
			require.Equal(t, "completed", task.Status)
			if recoverFirst {
				require.Equal(t, []string{"https://s3/first", "https://s3/second"}, task.PublicView().Result.Data)
				require.Equal(t, 1.0, UnifiedMediaTaskCharge(task))
				require.Empty(t, task.PublicView().Error)
			} else {
				require.Equal(t, []string{"https://s3/second"}, task.PublicView().Result.Data)
				require.Equal(t, 0.5, UnifiedMediaTaskCharge(task))
				require.NotEmpty(t, task.PublicView().Error)
			}
		})
	}
}

func TestUnifiedMediaNoAccountTaskCompletesWithoutSubmit(t *testing.T) {
	repo := &unifiedMediaMemoryRepo{}
	upstream := &unifiedMediaUpstreamStub{}
	svc := &UnifiedMediaTaskService{Repo: repo, Upstream: upstream, Accounts: &unifiedMediaAccounts{}, Now: time.Now}
	task := &UnifiedMediaTask{ID: "media_test", GroupID: 3, MediaType: "image", RequestedCount: 1, Units: []UnifiedMediaUnit{{State: "queued", Count: 1}}, Deadline: time.Now().Add(time.Hour)}
	require.NoError(t, svc.process(context.Background(), task))
	require.Equal(t, "completed", task.Status)
	require.Zero(t, upstream.submits)
	require.Equal(t, "no_available_media_account", task.PublicView().Error)
	require.Zero(t, UnifiedMediaTaskCharge(task))
}
func TestUnifiedMediaUnexpectedExtraOutputsFailWithoutTruncation(t *testing.T) {
	repo := &unifiedMediaMemoryRepo{}
	svc := &UnifiedMediaTaskService{Repo: repo, Now: time.Now}
	task := &UnifiedMediaTask{Units: []UnifiedMediaUnit{{State: "polling", Count: 1}}}
	require.NoError(t, svc.acceptResult(context.Background(), task, 0, &UnifiedMediaUpstreamResult{Status: "completed", URLs: []string{"first", "second"}}))
	require.Len(t, task.Units[0].URLs, 2)
	require.Equal(t, "failed", task.Units[0].State)
	require.Equal(t, "unexpected_upstream_result_count", task.Units[0].Error)
}

func TestUnifiedMediaAccountRateOnlyAffectsSharedAccountUsage(t *testing.T) {
	repo := &unifiedMediaMemoryRepo{}
	rate := 1.5
	upstream := &unifiedMediaUpstreamStub{}
	svc := &UnifiedMediaTaskService{Repo: repo, Upstream: upstream, Accounts: &unifiedMediaAccounts{rows: []Account{{ID: 1, Platform: PlatformGrsai, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, RateMultiplier: &rate}}}, Now: time.Now}
	task := &UnifiedMediaTask{ID: "media_rate", MediaType: "image", GroupID: 1, RequestedCount: 1, UnitPrice: 0.4, AccountUnitPrice: 0.2, Units: []UnifiedMediaUnit{{State: "queued", Count: 1}}, Payload: UnifiedMediaTaskPayload{Normalized: &mediacapability.NormalizedRequest{Values: map[string]any{"model": "example", "prompt": "cat"}, RequestedCount: 1}, Candidates: []UnifiedMediaCandidateSnapshot{{Platform: PlatformGrsai, Model: "example", NativeMaxResults: 1, Capability: MediaCapabilitySnapshot{MediaType: "image", Schema: json.RawMessage(`{"version":1,"fields":{"prompt":{"type":"string","required":true}}}`)}}}}}
	require.NoError(t, svc.submit(context.Background(), task, 0))
	require.Equal(t, 0.3, repo.reservedAmount)
	task.Units[0].Stored = []UnifiedMediaStoredResult{{URL: "https://s3/example"}}
	require.Equal(t, 0.4, UnifiedMediaTaskCharge(task))
}

type unifiedMediaCandidateFallbackUpstream struct {
	unifiedMediaUpstreamStub
	models     []string
	accountIDs []int64
}

func (u *unifiedMediaCandidateFallbackUpstream) Submit(_ context.Context, account *Account, _ string, model string, _ *mediacapability.NormalizedRequest, _ int) (*UnifiedMediaUpstreamResult, error) {
	u.models = append(u.models, model)
	u.accountIDs = append(u.accountIDs, account.ID)
	if model == "upstream-a" {
		return &UnifiedMediaUpstreamResult{Disposition: "rejected"}, nil
	}
	return &UnifiedMediaUpstreamResult{Disposition: "accepted", Status: "in_progress", TaskID: "accepted-b"}, nil
}

func TestUnifiedMediaRejectedCandidateCanUseSameAccountForNextModel(t *testing.T) {
	repo := &unifiedMediaMemoryRepo{}
	upstream := &unifiedMediaCandidateFallbackUpstream{}
	svc := &UnifiedMediaTaskService{Repo: repo, Upstream: upstream, Accounts: &unifiedMediaAccounts{rows: []Account{{ID: 7, Platform: PlatformGrsai, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true}}}, Now: time.Now}
	capability := MediaCapabilitySnapshot{MediaType: "image", Schema: json.RawMessage(`{"version":1,"fields":{"prompt":{"type":"string","required":true}}}`)}
	task := &UnifiedMediaTask{
		ID: "media_candidates", MediaType: "image", GroupID: 1, RequestedCount: 1, AccountUnitPrice: 0.2,
		Units: []UnifiedMediaUnit{{State: "queued", Count: 1}},
		Payload: UnifiedMediaTaskPayload{
			Normalized: &mediacapability.NormalizedRequest{Values: map[string]any{"model": "public-model", "prompt": "cat"}, RequestedCount: 1},
			Candidates: []UnifiedMediaCandidateSnapshot{
				{RouteID: 1, Platform: PlatformGrsai, Model: "upstream-a", NativeMaxResults: 1, Capability: capability},
				{RouteID: 2, Platform: PlatformGrsai, Model: "upstream-b", NativeMaxResults: 1, Capability: capability},
			},
		},
	}
	require.NoError(t, svc.submit(context.Background(), task, 0))
	require.Equal(t, []string{"upstream-a", "upstream-b"}, upstream.models)
	require.Equal(t, []int64{7, 7}, upstream.accountIDs)
	require.Equal(t, "polling", task.Units[0].State)
	require.Equal(t, "upstream-b", task.Units[0].UpstreamModel)
	require.Equal(t, "accepted-b", task.Units[0].UpstreamTaskID)
	require.Equal(t, 2, task.Units[0].Attempt)
	require.Len(t, task.Units[0].TriedCandidates, 2)
}
