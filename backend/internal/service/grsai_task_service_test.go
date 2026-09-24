package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type grsaiTaskPayloadMemory struct {
	values  map[int64][]byte
	readErr error
}

func (p *grsaiTaskPayloadMemory) PutEncrypted(_ context.Context, id int64, body []byte, _ time.Time) error {
	if p.values == nil {
		p.values = map[int64][]byte{}
	}
	p.values[id] = append([]byte("ciphertext:"), body...)
	return nil
}
func (p *grsaiTaskPayloadMemory) GetEncrypted(_ context.Context, id int64) ([]byte, error) {
	if p.readErr != nil {
		return nil, p.readErr
	}
	v, ok := p.values[id]
	if !ok {
		return nil, ErrGrsaiTaskPayloadNotFound
	}
	return append([]byte(nil), v[len("ciphertext:"):]...), nil
}

func TestAsyncTaskTransientPayloadReadErrorKeepsTaskForRetry(t *testing.T) {
	tasks, repo, payloads, upstream := grsaiTaskFixture(t)
	repo.record = &GrsaiSettlement{ID: 17, AccountID: 3, UserID: 1, APIKeyID: 4, Model: "m", DeliveryMode: GrsaiDeliveryAsync, InternalStatus: "processing", UpstreamStatus: "not_submitted", ClaimVersion: 1}
	payloads.values = map[int64][]byte{17: append([]byte("ciphertext:"), []byte(`{"model":"m","replyType":"async"}`)...)}
	payloads.readErr = errors.New("temporary database failure")
	_, err := tasks.RunGrsaiTask(context.Background(), repo.record, nil)
	require.ErrorIs(t, err, payloads.readErr)
	require.Equal(t, "pending_upstream", repo.record.InternalStatus)
	require.Contains(t, payloads.values, int64(17))
	require.Zero(t, upstream.posts)
}

func TestAsyncTaskWithoutDurableHoldNeverPosts(t *testing.T) {
	tasks, repo, payloads, upstream := grsaiTaskFixture(t)
	repo.record = &GrsaiSettlement{ID: 17, AccountID: 3, UserID: 1, APIKeyID: 4, HoldAmount: 0.1, HoldState: "none", DeliveryMode: GrsaiDeliveryAsync, InternalStatus: "processing", UpstreamStatus: "not_submitted", ClaimVersion: 1}
	payloads.values = map[int64][]byte{17: append([]byte("ciphertext:"), []byte(`{"model":"m","replyType":"async"}`)...)}
	_, err := tasks.RunGrsaiTask(context.Background(), repo.record, nil)
	require.ErrorIs(t, err, ErrGrsaiSettlementInvalidState)
	require.Equal(t, "manual_review", repo.record.InternalStatus)
	require.Zero(t, upstream.posts)
}
func (p *grsaiTaskPayloadMemory) DeleteBySettlementID(_ context.Context, id int64) error {
	delete(p.values, id)
	return nil
}
func (p *grsaiTaskPayloadMemory) DeleteExpired(context.Context, time.Time) (int64, error) {
	return 0, nil
}

type grsaiTaskStreamMemory struct {
	posts int
	body  string
}

func (u *grsaiTaskStreamMemory) OpenGenerateStream(context.Context, *Account, []byte) (*GrsaiUpstreamStream, error) {
	u.posts++
	body := u.body
	if body == "" {
		body = "data: {\"id\":\"up-1\",\"status\":\"succeeded\",\"progress\":100,\"results\":[{\"url\":\"https://img.invalid/1\"}]}\n\n"
	}
	return &GrsaiUpstreamStream{StatusCode: 200, ContentType: "text/event-stream", Body: io.NopCloser(strings.NewReader(body))}, nil
}

type grsaiTaskAccountMemory struct {
	account *Account
	err     error
}

func (a grsaiTaskAccountMemory) GetByID(context.Context, int64) (*Account, error) {
	return a.account, a.err
}

type grsaiTaskRepoMemory struct {
	*grsaiSettlementMemoryRepo
	updateErr            error
	markErr              error
	markDenied           bool
	markPersisted        bool
	queueErr             error
	queueClaim           bool
	queueFailBeforeWrite bool
	statuses             []string
	claimMode            GrsaiDeliveryMode
	claimLimits          []int
}

func (r *grsaiTaskRepoMemory) MarkSubmitting(_ context.Context, _, version int64) (bool, error) {
	if r.markErr != nil {
		if r.markPersisted {
			r.record.UpstreamStatus = "submitting"
		}
		return false, r.markErr
	}
	if r.markDenied {
		return false, nil
	}
	if err := r.check(version); err != nil {
		return false, err
	}
	r.record.UpstreamStatus = "submitting"
	r.statuses = append(r.statuses, "submitting")
	return true, nil
}

func (r *grsaiTaskRepoMemory) DeferUnsentSubmission(_ context.Context, _, version int64, next time.Time) error {
	if err := r.check(version); err != nil {
		return err
	}
	if r.record.UpstreamStatus != "not_submitted" && r.record.UpstreamStatus != "submitting" {
		return ErrGrsaiSettlementClaimLost
	}
	r.record.UpstreamStatus = "not_submitted"
	r.record.InternalStatus = "pending_upstream"
	r.record.NextAttemptAt = next
	return nil
}

func (r *grsaiTaskRepoMemory) MarkPendingUpstream(ctx context.Context, id, version int64, next time.Time) error {
	if r.queueFailBeforeWrite {
		return r.queueErr
	}
	if err := r.grsaiSettlementMemoryRepo.MarkPendingUpstream(ctx, id, version, next); err != nil {
		return err
	}
	if r.queueErr != nil {
		if r.queueClaim {
			r.record.InternalStatus = "processing"
			r.record.ClaimVersion++
		}
		return r.queueErr
	}
	return nil
}

func (r *grsaiTaskRepoMemory) GetOwnedByPublicOrUpstreamID(_ context.Context, userID, apiKeyID int64, id string) (*GrsaiSettlement, error) {
	if r.record.UserID != userID || r.record.APIKeyID != apiKeyID || (id != r.record.PublicTaskID && (r.record.UpstreamTaskID == nil || id != *r.record.UpstreamTaskID)) {
		return nil, ErrGrsaiSettlementNotFound
	}
	return r.GetByID(context.Background(), r.record.ID)
}
func (r *grsaiTaskRepoMemory) ClaimDue(_ context.Context, _ time.Time, limit int, lease time.Time) ([]*GrsaiSettlement, error) {
	r.claimLimits = append(r.claimLimits, limit)
	if r.record.InternalStatus != "pending_upstream" {
		return nil, nil
	}
	r.record.InternalStatus = "processing"
	r.record.ClaimVersion++
	r.record.NextAttemptAt = lease
	claim, _ := r.GetByID(context.Background(), r.record.ID)
	return []*GrsaiSettlement{claim}, nil
}
func (r *grsaiTaskRepoMemory) ClaimDueForDeliveryMode(ctx context.Context, now time.Time, limit int, lease time.Time, mode GrsaiDeliveryMode) ([]*GrsaiSettlement, error) {
	r.claimMode = mode
	if r.record == nil || r.record.DeliveryMode != mode {
		return nil, nil
	}
	return r.ClaimDue(ctx, now, limit, lease)
}
func (r *grsaiTaskRepoMemory) UpdateResult(ctx context.Context, id, version int64, status, summary string, next time.Time) (bool, error) {
	if r.updateErr != nil {
		return false, r.updateErr
	}
	r.statuses = append(r.statuses, status)
	return r.grsaiSettlementMemoryRepo.UpdateResult(ctx, id, version, status, summary, next)
}
func (r *grsaiTaskRepoMemory) BindAndRecordStreamEvent(_ context.Context, _, version int64, event GrsaiStreamEvent) (bool, error) {
	if err := r.check(version); err != nil {
		return false, err
	}
	id := event.TaskID
	r.record.UpstreamTaskID = &id
	r.record.UpstreamStatus, r.record.Progress = event.Status, event.Progress
	r.record.ResultURLs = append([]string(nil), event.ResultURLs...)
	return true, nil
}
func (r *grsaiTaskRepoMemory) RecordStreamEvent(_ context.Context, _, version int64, event GrsaiStreamEvent) (bool, error) {
	if err := r.check(version); err != nil {
		return false, err
	}
	r.record.UpstreamStatus, r.record.Progress = event.Status, event.Progress
	return true, nil
}

func grsaiTaskFixture(t *testing.T) (*GrsaiTaskService, *grsaiTaskRepoMemory, *grsaiTaskPayloadMemory, *grsaiTaskStreamMemory) {
	t.Helper()
	r := &grsaiTaskRepoMemory{grsaiSettlementMemoryRepo: &grsaiSettlementMemoryRepo{}}
	b := &grsaiBillingSpy{}
	settlement := &GrsaiSettlementService{Repo: r, Billing: b, Pricing: &grsaiPriceStub{price: 0.1}}
	payloads := &grsaiTaskPayloadMemory{}
	stream := &grsaiTaskStreamMemory{}
	account := &Account{ID: 3, Platform: PlatformGrsai, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "secret"}}
	service := &GrsaiTaskService{Settlement: settlement, Repo: r, Payloads: payloads, Accounts: grsaiTaskAccountMemory{account: account}, Upstream: stream, Options: GrsaiTaskOptions{Enabled: true, PayloadTTL: time.Hour}}
	group := &Group{ID: 2, Platform: PlatformGrsai, RateMultiplier: 1}
	serviceTestAccount := account
	_ = serviceTestAccount
	_ = group
	return service, r, payloads, stream
}

func TestGrsaiDeliveryDisabledRejectsAsyncBeforeHold(t *testing.T) {
	tasks, repo, payloads, upstream := grsaiTaskFixture(t)
	tasks.Options.Enabled = false
	group := &Group{ID: 2, Platform: PlatformGrsai, RateMultiplier: 1}
	key := &APIKey{ID: 4, UserID: 1, GroupID: &group.ID, Group: group}
	account := &Account{ID: 3, Platform: PlatformGrsai, Type: AccountTypeAPIKey}
	_, err := tasks.CreateGrsaiTask(context.Background(), GrsaiTaskInput{Account: account, APIKey: key, Body: []byte(`{"model":"nano-banana-2-lite","replyType":"async"}`)})
	require.ErrorIs(t, err, ErrGrsaiAsyncDisabled)
	require.Nil(t, repo.record)
	require.Empty(t, payloads.values)
	require.Zero(t, upstream.posts)
}

func TestAsyncTaskPersistsEncryptedPayloadThenWorkerConsumesOnce(t *testing.T) {
	tasks, repo, payloads, upstream := grsaiTaskFixture(t)
	group := &Group{ID: 2, Platform: PlatformGrsai, RateMultiplier: 1}
	apiKey := &APIKey{ID: 4, UserID: 1, GroupID: &group.ID, Group: group}
	account := &Account{ID: 3, Platform: PlatformGrsai, Type: AccountTypeAPIKey}
	task, err := tasks.CreateGrsaiTask(context.Background(), GrsaiTaskInput{Account: account, APIKey: apiKey, Body: []byte(`{"model":"nano-banana-2-lite","replyType":"async"}`)})
	require.NoError(t, err)
	require.Equal(t, "pending_upstream", task.InternalStatus)
	task.DeliveryMode = GrsaiDeliveryAsync
	repo.record.DeliveryMode = GrsaiDeliveryAsync
	require.NotEmpty(t, payloads.values[task.ID])
	runtime := &GrsaiTaskRuntime{Tasks: tasks, Repo: repo, Options: GrsaiTaskRuntimeOptions{Enabled: true, BatchLimit: 1}}
	runtime.RunOnce(context.Background())
	require.Equal(t, 1, upstream.posts)
	require.Empty(t, payloads.values[task.ID])
	require.Equal(t, "settled", repo.record.InternalStatus)
}

func TestAsyncTaskPersistsPreBindFenceBeforePost(t *testing.T) {
	tasks, repo, payloads, upstream := grsaiTaskFixture(t)
	repo.record = &GrsaiSettlement{ID: 17, AccountID: 3, UserID: 1, APIKeyID: 4, Model: "m", DeliveryMode: GrsaiDeliveryAsync, InternalStatus: "processing", UpstreamStatus: "not_submitted", ClaimVersion: 1, NextAttemptAt: time.Now().Add(time.Minute)}
	payloads.values = map[int64][]byte{17: append([]byte("ciphertext:"), []byte(`{"model":"m","replyType":"async"}`)...)}
	_, err := tasks.RunGrsaiTask(context.Background(), repo.record, nil)
	// The fixture intentionally omits billing snapshot fields; this test only
	// asserts that the pre-bind fence precedes the provider POST.
	require.Error(t, err)
	require.Equal(t, 1, upstream.posts)
	require.NotEmpty(t, repo.statuses)
	require.Equal(t, "submitting", repo.statuses[0])
}

func TestAsyncTaskPreBindFencePersistenceFailureDoesNotPost(t *testing.T) {
	tasks, repo, payloads, upstream := grsaiTaskFixture(t)
	repo.record = &GrsaiSettlement{ID: 17, AccountID: 3, UserID: 1, APIKeyID: 4, Model: "m", DeliveryMode: GrsaiDeliveryAsync, InternalStatus: "processing", UpstreamStatus: "not_submitted", ClaimVersion: 1, NextAttemptAt: time.Now().Add(time.Minute)}
	payloads.values = map[int64][]byte{17: append([]byte("ciphertext:"), []byte(`{"model":"m","replyType":"async"}`)...)}
	repo.markErr = errors.New("fence write failed")
	_, err := tasks.RunGrsaiTask(context.Background(), repo.record, nil)
	require.Error(t, err)
	require.Equal(t, 0, upstream.posts)
	require.Equal(t, "pending_upstream", repo.record.InternalStatus)
	require.Equal(t, "not_submitted", repo.record.UpstreamStatus)
	require.Contains(t, payloads.values, int64(17))
}

func TestAsyncTaskAmbiguousFenceWriteRearamsBeforeAnyPost(t *testing.T) {
	tasks, repo, payloads, upstream := grsaiTaskFixture(t)
	repo.record = &GrsaiSettlement{ID: 17, AccountID: 3, UserID: 1, APIKeyID: 4, HoldAmount: 0.1, HoldState: "held", Model: "m", DeliveryMode: GrsaiDeliveryAsync, InternalStatus: "processing", UpstreamStatus: "not_submitted", ClaimVersion: 1}
	payloads.values = map[int64][]byte{17: append([]byte("ciphertext:"), []byte(`{"model":"m","replyType":"async"}`)...)}
	repo.markErr, repo.markPersisted = errors.New("commit outcome unknown"), true
	_, err := tasks.RunGrsaiTask(context.Background(), repo.record, nil)
	require.ErrorIs(t, err, repo.markErr)
	require.Equal(t, "pending_upstream", repo.record.InternalStatus)
	require.Equal(t, "not_submitted", repo.record.UpstreamStatus)
	require.Equal(t, "held", repo.record.HoldState)
	require.Contains(t, payloads.values, int64(17))
	require.Zero(t, upstream.posts)
	repo.markErr = nil
	(&GrsaiTaskRuntime{Tasks: tasks, Repo: repo, Options: GrsaiTaskRuntimeOptions{BatchLimit: 1}}).RunOnce(context.Background())
	require.Equal(t, 1, upstream.posts)
}

func TestAsyncTaskAmbiguousQueueWriteReturnsExistingTaskAfterWorkerClaim(t *testing.T) {
	tasks, repo, payloads, upstream := grsaiTaskFixture(t)
	repo.queueErr, repo.queueClaim = errors.New("commit response lost"), true
	group := &Group{ID: 2, Platform: PlatformGrsai, RateMultiplier: 1}
	task, err := tasks.CreateGrsaiTask(context.Background(), GrsaiTaskInput{Account: &Account{ID: 3, Platform: PlatformGrsai, Type: AccountTypeAPIKey}, APIKey: &APIKey{ID: 4, UserID: 1, GroupID: &group.ID, Group: group}, Body: []byte(`{"model":"m","replyType":"async"}`)})
	require.NoError(t, err)
	require.Equal(t, int64(17), task.ID)
	require.Equal(t, "held", repo.record.HoldState)
	require.Equal(t, "processing", repo.record.InternalStatus)
	require.Contains(t, payloads.values, task.ID)
	require.Zero(t, upstream.posts)
}

func TestAsyncTaskQueueWriteFailureReturnsRecoverableTask(t *testing.T) {
	tasks, repo, payloads, upstream := grsaiTaskFixture(t)
	repo.queueErr, repo.queueFailBeforeWrite = errors.New("queue write unavailable"), true
	group := &Group{ID: 2, Platform: PlatformGrsai, RateMultiplier: 1}
	task, err := tasks.CreateGrsaiTask(context.Background(), GrsaiTaskInput{Account: &Account{ID: 3, Platform: PlatformGrsai, Type: AccountTypeAPIKey}, APIKey: &APIKey{ID: 4, UserID: 1, GroupID: &group.ID, Group: group}, Body: []byte(`{"model":"m","replyType":"async"}`)})
	require.NoError(t, err)
	require.Equal(t, int64(17), task.ID)
	require.Equal(t, "held", repo.record.HoldState)
	require.Equal(t, "processing", repo.record.InternalStatus)
	require.Contains(t, payloads.values, task.ID)
	require.Zero(t, upstream.posts)
}

func TestAsyncTaskTransientAccountReadErrorRetriesWithoutPosting(t *testing.T) {
	tasks, repo, payloads, upstream := grsaiTaskFixture(t)
	repo.record = &GrsaiSettlement{ID: 17, AccountID: 3, UserID: 1, APIKeyID: 4, HoldAmount: 0.1, HoldState: "held", Model: "m", DeliveryMode: GrsaiDeliveryAsync, InternalStatus: "processing", UpstreamStatus: "not_submitted", ClaimVersion: 1}
	payloads.values = map[int64][]byte{17: append([]byte("ciphertext:"), []byte(`{"model":"m","replyType":"async"}`)...)}
	readErr := errors.New("temporary account database timeout")
	tasks.Accounts = grsaiTaskAccountMemory{err: readErr}
	_, err := tasks.RunGrsaiTask(context.Background(), repo.record, nil)
	require.ErrorIs(t, err, readErr)
	require.Equal(t, "pending_upstream", repo.record.InternalStatus)
	require.Equal(t, "held", repo.record.HoldState)
	require.Contains(t, payloads.values, int64(17))
	require.Zero(t, upstream.posts)
}

func TestAsyncTaskMissingAccountRequiresManualReview(t *testing.T) {
	tasks, repo, payloads, upstream := grsaiTaskFixture(t)
	repo.record = &GrsaiSettlement{ID: 17, AccountID: 3, UserID: 1, APIKeyID: 4, HoldAmount: 0.1, HoldState: "held", Model: "m", DeliveryMode: GrsaiDeliveryAsync, InternalStatus: "processing", UpstreamStatus: "not_submitted", ClaimVersion: 1}
	payloads.values = map[int64][]byte{17: append([]byte("ciphertext:"), []byte(`{"model":"m","replyType":"async"}`)...)}
	tasks.Accounts = grsaiTaskAccountMemory{err: ErrAccountNotFound}
	_, err := tasks.RunGrsaiTask(context.Background(), repo.record, nil)
	require.ErrorIs(t, err, ErrGrsaiSettlementInvalidState)
	require.Equal(t, "manual_review", repo.record.InternalStatus)
	require.Equal(t, "released", repo.record.HoldState)
	require.NotContains(t, payloads.values, int64(17))
	require.Zero(t, upstream.posts)
}

func TestAsyncTaskQueueSuccessNeedsNoFinalRead(t *testing.T) {
	for _, tt := range []struct {
		name   string
		cancel bool
	}{{"read failure", false}, {"request canceled", true}} {
		t.Run(tt.name, func(t *testing.T) {
			tasks, repo, payloads, upstream := grsaiTaskFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.cancel {
				repo.onPending = cancel
			} else {
				repo.readErrAt = 3
			}
			group := &Group{ID: 2, Platform: PlatformGrsai, RateMultiplier: 1}
			task, err := tasks.CreateGrsaiTask(ctx, GrsaiTaskInput{Account: &Account{ID: 3, Platform: PlatformGrsai, Type: AccountTypeAPIKey}, APIKey: &APIKey{ID: 4, UserID: 1, GroupID: &group.ID, Group: group}, Body: []byte(`{"model":"m","replyType":"async"}`)})
			require.NoError(t, err)
			require.Equal(t, "pending_upstream", task.InternalStatus)
			require.Equal(t, "pending_upstream", repo.record.InternalStatus)
			require.Contains(t, payloads.values, task.ID)
			require.Zero(t, upstream.posts)
		})
	}
}

func TestAsyncTaskPreBindManualReviewDeletesPayload(t *testing.T) {
	tasks, repo, payloads, upstream := grsaiTaskFixture(t)
	repo.record = &GrsaiSettlement{ID: 17, AccountID: 3, UserID: 1, APIKeyID: 4, Model: "m", DeliveryMode: GrsaiDeliveryAsync, InternalStatus: "processing", UpstreamStatus: "not_submitted", ClaimVersion: 1, NextAttemptAt: time.Now().Add(time.Minute)}
	payloads.values = map[int64][]byte{17: append([]byte("ciphertext:"), []byte(`{"model":"m","replyType":"async"}`)...)}
	upstream.body = "data: {\"status\":\"running\"}\n\n"
	_, err := tasks.RunGrsaiTask(context.Background(), repo.record, nil)
	require.Error(t, err)
	require.Equal(t, "manual_review", repo.record.InternalStatus)
	require.NotContains(t, payloads.values, int64(17))
}

func TestAsyncTaskResultlessSuccessDeletesBoundPayload(t *testing.T) {
	tasks, repo, payloads, upstream := grsaiTaskFixture(t)
	repo.record = &GrsaiSettlement{ID: 17, AccountID: 3, UserID: 1, APIKeyID: 4, Model: "m", DeliveryMode: GrsaiDeliveryAsync, InternalStatus: "processing", UpstreamStatus: "not_submitted", ClaimVersion: 1}
	payloads.values = map[int64][]byte{17: append([]byte("ciphertext:"), []byte(`{"model":"m","replyType":"async"}`)...)}
	upstream.body = "data: {\"id\":\"up-17\",\"status\":\"succeeded\",\"progress\":100}\n\n"
	_, err := tasks.RunGrsaiTask(context.Background(), repo.record, nil)
	require.ErrorIs(t, err, ErrGrsaiSSEProtocol)
	require.Equal(t, "pending_upstream", repo.record.InternalStatus)
	require.Equal(t, "up-17", *repo.record.UpstreamTaskID)
	require.NotContains(t, payloads.values, int64(17))
	require.Equal(t, 1, upstream.posts)
}

func TestAsyncTaskPreBindFenceRejectedDoesNotPost(t *testing.T) {
	tasks, repo, payloads, upstream := grsaiTaskFixture(t)
	repo.record = &GrsaiSettlement{ID: 17, AccountID: 3, DeliveryMode: GrsaiDeliveryAsync, InternalStatus: "processing", UpstreamStatus: "not_submitted", ClaimVersion: 1}
	payloads.values = map[int64][]byte{17: append([]byte("ciphertext:"), []byte(`{"model":"m","replyType":"async"}`)...)}
	repo.markDenied = true
	_, err := tasks.RunGrsaiTask(context.Background(), repo.record, nil)
	require.Error(t, err)
	require.Zero(t, upstream.posts)
}

func TestAsyncTaskAlreadySubmittingDoesNotPostAgain(t *testing.T) {
	tasks, repo, payloads, upstream := grsaiTaskFixture(t)
	repo.record = &GrsaiSettlement{ID: 17, AccountID: 3, DeliveryMode: GrsaiDeliveryAsync, InternalStatus: "processing", UpstreamStatus: "submitting", ClaimVersion: 1}
	payloads.values = map[int64][]byte{17: append([]byte("ciphertext:"), []byte(`{"model":"m","replyType":"async"}`)...)}
	_, err := tasks.RunGrsaiTask(context.Background(), repo.record, nil)
	require.Error(t, err)
	require.Zero(t, upstream.posts)
}

func TestAsyncTaskSeparatesPayloadTTLFromResultRetention(t *testing.T) {
	tasks, _, _, _ := grsaiTaskFixture(t)
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	tasks.Options.Now = func() time.Time { return now }
	tasks.Options.PayloadTTL = 15 * time.Minute
	tasks.Options.ResultRetention = 48 * time.Hour
	group := &Group{ID: 2, Platform: PlatformGrsai, RateMultiplier: 1}
	apiKey := &APIKey{ID: 4, UserID: 1, GroupID: &group.ID, Group: group}
	account := &Account{ID: 3, Platform: PlatformGrsai, Type: AccountTypeAPIKey}
	task, err := tasks.CreateGrsaiTask(context.Background(), GrsaiTaskInput{Account: account, APIKey: apiKey, Body: []byte(`{"model":"m","replyType":"async"}`)})
	require.NoError(t, err)
	require.Equal(t, now.Add(15*time.Minute), *task.PayloadDeleteAfter)
	require.Equal(t, now.Add(48*time.Hour), *task.ExpiresAt)
}

func TestPublicTaskViewDoesNotExposePrivateFields(t *testing.T) {
	record := &GrsaiSettlement{PublicTaskID: "public-1", AccountID: 9, UserID: 1, APIKeyID: 2, Model: "m", CreatedAt: time.Now(), UpdatedAt: time.Now(), ResultURLs: []string{"https://img.invalid/a"}, BillableUnitPrice: 99, InternalStatus: "settled"}
	view := BuildGrsaiTaskView(record)
	raw, err := json.Marshal(view)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "account_id")
	require.NotContains(t, string(raw), "billable_unit_price")
	require.NotContains(t, string(raw), "ciphertext")
}

func TestPublicTaskViewPreservesDurableIntermediateStatuses(t *testing.T) {
	for _, tt := range []struct {
		name, internal, upstream, want string
	}{
		{"manual review", "manual_review", "unknown", "manual_review"},
		{"pending settlement", "pending_settlement", GrsaiUpstreamStatusSucceeded, "pending_settlement"},
		{"upstream unknown", "processing", "unknown", "upstream_unknown"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			view := BuildGrsaiTaskView(&GrsaiSettlement{PublicTaskID: "public-1", InternalStatus: tt.internal, UpstreamStatus: tt.upstream})
			require.Equal(t, tt.want, view.Status)
		})
	}
}

func TestPublicTaskViewSuccessWithoutResultIsStillRunning(t *testing.T) {
	id := "legacy-provider-no-url"
	view := BuildGrsaiTaskView(&GrsaiSettlement{PublicTaskID: "public-1", DeliveryMode: GrsaiDeliveryAsync,
		InternalStatus: "pending_settlement", UpstreamStatus: GrsaiUpstreamStatusSucceeded, UpstreamTaskID: &id})
	require.Equal(t, "running", view.Status)
	require.Empty(t, view.Results)
}

func TestPublicTaskViewUsesDocumentedTerminalStatuses(t *testing.T) {
	for _, tt := range []struct{ internal, want string }{
		{"settled", "settled"},
		{"closed_no_charge", "closed_no_charge"},
	} {
		t.Run(tt.internal, func(t *testing.T) {
			view := BuildGrsaiTaskView(&GrsaiSettlement{PublicTaskID: "public-1", InternalStatus: tt.internal})
			require.Equal(t, tt.want, view.Status)
		})
	}
	view := BuildGrsaiTaskView(&GrsaiSettlement{PublicTaskID: "public-1", InternalStatus: "processing", UpstreamStatus: "submitting"})
	require.Equal(t, "submitting", view.Status)
	upstreamID := "upstream-1"
	view = BuildGrsaiTaskView(&GrsaiSettlement{PublicTaskID: "public-1", UpstreamTaskID: &upstreamID, InternalStatus: "pending_upstream", UpstreamStatus: "unknown"})
	require.Equal(t, "upstream_unknown", view.Status)
}

func TestPublicTaskViewExpiresOnlyTerminalRecordsAfterRetention(t *testing.T) {
	tasks, repo, _, _ := grsaiTaskFixture(t)
	now := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)
	tasks.Options.Now = func() time.Time { return now }
	created := now.Add(-48 * time.Hour)
	closed := now.Add(-23 * time.Hour)
	expires := created.Add(24 * time.Hour)
	repo.record = &GrsaiSettlement{ID: 42, UserID: 1, APIKeyID: 4, PublicTaskID: "public-42", CreatedAt: created, ClosedAt: &closed, ExpiresAt: &expires, InternalStatus: "settled"}
	_, err := tasks.GetPublicTaskView(context.Background(), 1, 4, "public-42")
	require.NoError(t, err, "terminal retention starts at close, not creation")

	closed = now.Add(-25 * time.Hour)
	_, err = tasks.GetPublicTaskView(context.Background(), 1, 4, "public-42")
	require.ErrorIs(t, err, ErrGrsaiSettlementNotFound)

	repo.record.InternalStatus = "manual_review"
	_, err = tasks.GetPublicTaskView(context.Background(), 1, 4, "public-42")
	require.NoError(t, err, "actionable records remain queryable")
}
