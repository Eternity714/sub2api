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

type grsaiTaskPayloadMemory struct{ values map[int64][]byte }

func (p *grsaiTaskPayloadMemory) PutEncrypted(_ context.Context, id int64, body []byte, _ time.Time) error {
	if p.values == nil {
		p.values = map[int64][]byte{}
	}
	p.values[id] = append([]byte("ciphertext:"), body...)
	return nil
}
func (p *grsaiTaskPayloadMemory) GetEncrypted(_ context.Context, id int64) ([]byte, error) {
	v, ok := p.values[id]
	if !ok {
		return nil, ErrGrsaiTaskPayloadNotFound
	}
	return append([]byte(nil), v[len("ciphertext:"):]...), nil
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

type grsaiTaskAccountMemory struct{ account *Account }

func (a grsaiTaskAccountMemory) GetByID(context.Context, int64) (*Account, error) {
	return a.account, nil
}

type grsaiTaskRepoMemory struct {
	*grsaiSettlementMemoryRepo
	updateErr error
	statuses  []string
	claimMode GrsaiDeliveryMode
}

func (r *grsaiTaskRepoMemory) GetOwnedByPublicOrUpstreamID(_ context.Context, userID, apiKeyID int64, id string) (*GrsaiSettlement, error) {
	if r.record.UserID != userID || r.record.APIKeyID != apiKeyID || (id != r.record.PublicTaskID && (r.record.UpstreamTaskID == nil || id != *r.record.UpstreamTaskID)) {
		return nil, ErrGrsaiSettlementNotFound
	}
	return r.GetByID(context.Background(), r.record.ID)
}
func (r *grsaiTaskRepoMemory) ClaimDue(_ context.Context, _ time.Time, _ int, lease time.Time) ([]*GrsaiSettlement, error) {
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
	service := &GrsaiTaskService{Settlement: settlement, Repo: r, Payloads: payloads, Accounts: grsaiTaskAccountMemory{account: account}, Upstream: stream, Options: GrsaiTaskOptions{PayloadTTL: time.Hour}}
	group := &Group{ID: 2, Platform: PlatformGrsai, RateMultiplier: 1}
	serviceTestAccount := account
	_ = serviceTestAccount
	_ = group
	return service, r, payloads, stream
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
	repo.updateErr = errors.New("fence write failed")
	_, err := tasks.RunGrsaiTask(context.Background(), repo.record, nil)
	require.Error(t, err)
	require.Equal(t, 0, upstream.posts)
	require.Equal(t, "manual_review", repo.record.InternalStatus)
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
