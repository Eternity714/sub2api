//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
	"github.com/stretchr/testify/require"
)

// The only substituted boundary is durable account reservation. It preserves
// its relevant behavior: reserved units occupy a shared account slot until
// consumed/released. No real DB, account, upstream, S3, or credential is used.
type mediaAuditConcurrencyRepo struct {
	unifiedMediaMemoryRepo
	capacity      int
	active        int
	busyResponses int
	unavailable   bool
	busyAccounts  map[int64]bool
	reservations  map[string]bool
}

func (r *mediaAuditConcurrencyRepo) ReserveAccount(_ context.Context, task *UnifiedMediaTask, index int, account *Account, _ float64) error {
	if r.unavailable {
		return ErrUnifiedMediaAccountUnavailable
	}
	if r.busyAccounts[account.ID] {
		return ErrUnifiedMediaAccountBusy
	}
	if r.busyResponses > 0 {
		r.busyResponses--
		return ErrUnifiedMediaAccountBusy
	}
	if r.active >= r.capacity {
		return ErrUnifiedMediaAccountBusy
	}
	if r.reservations == nil {
		r.reservations = map[string]bool{}
	}
	key := fmt.Sprintf("%s:%d:%d", task.ID, index, task.Units[index].Attempt)
	r.reservations[key] = true
	r.active++
	return nil
}

func (r *mediaAuditConcurrencyRepo) SettleAccount(_ context.Context, task *UnifiedMediaTask, index int, _ bool) error {
	key := fmt.Sprintf("%s:%d:%d", task.ID, index, task.Units[index].Attempt)
	if r.reservations[key] {
		delete(r.reservations, key)
		r.active--
	}
	return nil
}

type mediaAuditDelayedUpstream struct {
	submits    int
	polls      map[string]int
	reject     bool
	httpStatus int
}

func (u *mediaAuditDelayedUpstream) Submit(context.Context, *Account, string, string, *mediacapability.NormalizedRequest, int) (*UnifiedMediaUpstreamResult, error) {
	u.submits++
	if u.reject {
		return &UnifiedMediaUpstreamResult{Disposition: "rejected", HTTPStatus: u.httpStatus}, nil
	}
	return &UnifiedMediaUpstreamResult{Disposition: "accepted", Status: "in_progress", TaskID: fmt.Sprintf("audit-up-%d", u.submits), HTTPStatus: u.httpStatus}, nil
}
func (u *mediaAuditDelayedUpstream) Poll(_ context.Context, _ *Account, _ string, _ string, id string) (*UnifiedMediaUpstreamResult, error) {
	if u.polls == nil {
		u.polls = map[string]int{}
	}
	u.polls[id]++
	if u.polls[id] == 1 {
		return &UnifiedMediaUpstreamResult{Disposition: "accepted", Status: "in_progress", TaskID: id}, nil
	}
	return &UnifiedMediaUpstreamResult{Disposition: "accepted", Status: "completed", TaskID: id, URLs: []string{"https://example.test/" + id}}, nil
}

func mediaAuditTask(count int, now time.Time) *UnifiedMediaTask {
	units := make([]UnifiedMediaUnit, count)
	for i := range units {
		units[i] = UnifiedMediaUnit{State: "queued", Count: 1}
	}
	return &UnifiedMediaTask{ID: "media_concurrency_audit", Status: "queued", MediaType: "image", Model: "audit-model", GroupID: 1,
		RequestedCount: count, UnitPrice: 0.01, AccountUnitPrice: 0.01, HeldAmount: float64(count) * 0.01,
		CreatedAt: now, Deadline: now.Add(time.Hour), Units: units,
		Payload: UnifiedMediaTaskPayload{
			Normalized: &mediacapability.NormalizedRequest{Values: map[string]any{"model": "audit-model", "prompt": "cat", "n": json.Number(fmt.Sprint(count))}, RequestedCount: count},
			Candidates: []UnifiedMediaCandidateSnapshot{{RouteID: 1, Platform: PlatformGrsai, Model: "audit-model", NativeMaxResults: 1, AllowSplit: true,
				Capability: MediaCapabilitySnapshot{MediaType: "image", Schema: json.RawMessage(`{"version":1,"fields":{"prompt":{"type":"string","required":true},"n":{"type":"integer","minimum":1,"maximum":8}},"batch":{"native_max_results":1,"allow_split":true}}`)}}}}}
}

func mediaAuditAccounts() *unifiedMediaAccounts {
	return &unifiedMediaAccounts{rows: []Account{{ID: 1, Platform: PlatformGrsai, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 2}}}
}

// RED: a temporary occupied slot must not produce a completed failure before
// the account is available again. A change that terminalizes busy work breaks
// this externally visible result even though no generation was attempted.
func TestUnifiedMediaAccountBusySlotRemainsRetryable(t *testing.T) {
	now := time.Now()
	repo := &mediaAuditConcurrencyRepo{capacity: 1, busyResponses: 1}
	upstream := &mediaAuditDelayedUpstream{}
	svc := &UnifiedMediaTaskService{Repo: repo, Accounts: mediaAuditAccounts(), Upstream: upstream, Persister: &unifiedMediaPersister{}, Now: func() time.Time { return now }, PollInterval: time.Second}
	task := mediaAuditTask(1, now)
	require.NoError(t, svc.process(context.Background(), task))
	t.Logf("after transient rejection: parent=%s unit=%s attempts=%d tried=%v error=%s finalizations=%d submissions=%d", task.Status, task.Units[0].State, task.Units[0].Attempt, task.Units[0].TriedCandidates, task.Units[0].Error, repo.finalized, upstream.submits)
	require.Equal(t, "in_progress", task.Status, "temporary account contention must stay pending rather than refund and fail")
	require.Equal(t, "queued", task.Units[0].State)
	require.Empty(t, task.Units[0].TriedCandidates, "reservation denied before POST must remain an eligible candidate")
	require.Zero(t, task.Units[0].Attempt)
	require.Zero(t, task.Units[0].AccountID)
	require.Nil(t, task.Units[0].AccountRateMultiplier)
	require.Empty(t, task.Units[0].Platform)
	require.Empty(t, task.Units[0].UpstreamModel)
	require.Zero(t, task.Units[0].Failures)
	require.True(t, task.Units[0].NextAttemptAt.After(now))
	now = now.Add(2 * time.Second)
	require.NoError(t, svc.process(context.Background(), task))
	now = now.Add(2 * time.Second)
	require.NoError(t, svc.process(context.Background(), task))
	require.Equal(t, "completed", task.Status)
	require.Len(t, task.PublicView().Result.Data, 1)
	require.Empty(t, task.PublicView().Error)
}

// RED: n=3 with one account of capacity 2 must wait for a slot and eventually
// produce all 3 results; capacity is not an upstream generation failure.
func TestUnifiedMediaAccountBusySplitThreeWaitsForTwoSlots(t *testing.T) {
	now := time.Now()
	repo := &mediaAuditConcurrencyRepo{capacity: 2}
	upstream := &mediaAuditDelayedUpstream{}
	svc := &UnifiedMediaTaskService{Repo: repo, Accounts: mediaAuditAccounts(), Upstream: upstream, Persister: &unifiedMediaPersister{}, Now: func() time.Time { return now }, PollInterval: time.Second}
	task := mediaAuditTask(3, now)
	require.NoError(t, svc.process(context.Background(), task))
	t.Logf("round1: states=%s/%s/%s active=%d third_attempts=%d third_tried=%v third_error=%s", task.Units[0].State, task.Units[1].State, task.Units[2].State, repo.active, task.Units[2].Attempt, task.Units[2].TriedCandidates, task.Units[2].Error)
	if task.Units[2].State != "queued" {
		t.Errorf("temporary capacity exhaustion must keep third unit queued; got %s", task.Units[2].State)
	}
	for i := 0; i < 4 && task.Status != "completed"; i++ {
		now = now.Add(2 * time.Second)
		require.NoError(t, svc.process(context.Background(), task))
	}
	t.Logf("final: parent=%s outputs=%d charged=%.2f error=%s submissions=%d active=%d", task.Status, len(task.PublicView().Result.Data), UnifiedMediaTaskCharge(task), task.PublicView().Error, upstream.submits, repo.active)
	require.Equal(t, "completed", task.Status)
	require.Len(t, task.PublicView().Result.Data, 3, "released slots must allow the third unit to run")
	require.Equal(t, 0.03, UnifiedMediaTaskCharge(task))
	require.Empty(t, task.PublicView().Error)
}

// Existing user decision remains a control: truly no configured account is a
// terminal failure with no charge; adding a busy retry must not change it.
func TestUnifiedMediaAccountBusyNoConfiguredAccountIsTerminal(t *testing.T) {
	now := time.Now()
	repo := &mediaAuditConcurrencyRepo{capacity: 2}
	upstream := &mediaAuditDelayedUpstream{}
	svc := &UnifiedMediaTaskService{Repo: repo, Accounts: &unifiedMediaAccounts{}, Upstream: upstream, Persister: &unifiedMediaPersister{}, Now: func() time.Time { return now }}
	task := mediaAuditTask(1, now)
	require.NoError(t, svc.process(context.Background(), task))
	require.Equal(t, "completed", task.Status)
	require.Equal(t, "no_available_media_account", task.PublicView().Error)
	require.Zero(t, UnifiedMediaTaskCharge(task))
	require.Zero(t, upstream.submits)
}

func TestUnifiedMediaAccountBusyCanUseAnotherAccountWithoutPoisoningAttempt(t *testing.T) {
	now := time.Now()
	repo := &mediaAuditConcurrencyRepo{capacity: 2, busyAccounts: map[int64]bool{1: true}}
	upstream := &mediaAuditDelayedUpstream{}
	accounts := mediaAuditAccounts()
	accounts.rows = append(accounts.rows, Account{ID: 2, Platform: PlatformGrsai, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true})
	svc := &UnifiedMediaTaskService{Repo: repo, Accounts: accounts, Upstream: upstream, Persister: &unifiedMediaPersister{}, Now: func() time.Time { return now }}
	task := mediaAuditTask(1, now)
	require.NoError(t, svc.submit(context.Background(), task, 0))
	require.Equal(t, "polling", task.Units[0].State)
	require.EqualValues(t, 2, task.Units[0].AccountID)
	require.Equal(t, 1, task.Units[0].Attempt)
	require.Equal(t, []string{"1:grsai:audit-model:2"}, task.Units[0].TriedCandidates)
}

func TestUnifiedMediaAccountBusyQuotaUnavailableRemainsTerminal(t *testing.T) {
	now := time.Now()
	repo := &mediaAuditConcurrencyRepo{capacity: 2, unavailable: true}
	upstream := &mediaAuditDelayedUpstream{}
	svc := &UnifiedMediaTaskService{Repo: repo, Accounts: mediaAuditAccounts(), Upstream: upstream, Persister: &unifiedMediaPersister{}, Now: func() time.Time { return now }}
	task := mediaAuditTask(1, now)
	require.NoError(t, svc.process(context.Background(), task))
	require.Equal(t, "completed", task.Status)
	require.Equal(t, "no_available_media_account", task.PublicView().Error)
	require.Zero(t, UnifiedMediaTaskCharge(task))
	require.Zero(t, upstream.submits)
}

func TestUnifiedMediaAccountBusyWaitStillExpiresAtDeadline(t *testing.T) {
	now := time.Now()
	repo := &mediaAuditConcurrencyRepo{capacity: 2, busyAccounts: map[int64]bool{1: true}}
	upstream := &mediaAuditDelayedUpstream{}
	svc := &UnifiedMediaTaskService{Repo: repo, Accounts: mediaAuditAccounts(), Upstream: upstream, Persister: &unifiedMediaPersister{}, Now: func() time.Time { return now }}
	task := mediaAuditTask(1, now)
	for i := 0; i < 8; i++ {
		require.NoError(t, svc.process(context.Background(), task))
		require.Equal(t, "in_progress", task.Status)
		require.Zero(t, task.Units[0].Failures, "waiting for a slot must not exhaust generation retry allowance")
		require.Zero(t, task.Units[0].Attempt)
		require.Empty(t, task.Units[0].TriedCandidates)
		now = now.Add(time.Minute)
	}
	now = task.Deadline.Add(time.Second)
	require.NoError(t, svc.process(context.Background(), task))
	require.Equal(t, "completed", task.Status)
	require.Equal(t, "media_task_expired", task.PublicView().Error)
	require.Zero(t, UnifiedMediaTaskCharge(task))
	require.Zero(t, upstream.submits)
}

func TestUnifiedMediaSubmissionRejectionPreservesErrorAndHTTPStatus(t *testing.T) {
	now := time.Now()
	repo := &mediaAuditConcurrencyRepo{capacity: 2}
	upstream := &mediaAuditDelayedUpstream{reject: true, httpStatus: 429}
	svc := &UnifiedMediaTaskService{Repo: repo, Accounts: mediaAuditAccounts(), Upstream: upstream, Persister: &unifiedMediaPersister{}, Now: func() time.Time { return now }}
	task := mediaAuditTask(1, now)
	require.NoError(t, svc.process(context.Background(), task))
	require.Equal(t, "completed", task.Status)
	require.Equal(t, "upstream_submission_rejected", task.PublicView().Error)
	require.Equal(t, 429, task.Units[0].LastHTTPStatus)
	require.Equal(t, []string{"1:grsai:audit-model:1"}, task.Units[0].TriedCandidates)
	require.Equal(t, 1, task.Units[0].Attempt)
	require.Zero(t, repo.active)
	require.Zero(t, UnifiedMediaTaskCharge(task))
	require.Equal(t, 1, upstream.submits)
}

func TestUnifiedMediaSubmissionHTTPStatusRemainsInternal(t *testing.T) {
	now := time.Now()
	repo := &mediaAuditConcurrencyRepo{capacity: 2}
	upstream := &mediaAuditDelayedUpstream{httpStatus: 202}
	svc := &UnifiedMediaTaskService{Repo: repo, Accounts: mediaAuditAccounts(), Upstream: upstream, Persister: &unifiedMediaPersister{}, Now: func() time.Time { return now }}
	task := mediaAuditTask(1, now)
	require.NoError(t, svc.submit(context.Background(), task, 0))
	require.Equal(t, 202, task.Units[0].LastHTTPStatus)
	public, err := json.Marshal(task.PublicView())
	require.NoError(t, err)
	require.NotContains(t, string(public), "http_status")
	require.NotContains(t, string(public), "HTTPStatus")
}
