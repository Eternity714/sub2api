package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type grsaiRecoveryRepo struct {
	mu             sync.Mutex
	record         *GrsaiSettlement
	lastClaimLimit int
}

type grsaiRecoveryHoldRepo struct {
	*grsaiRecoveryRepo
	releaseCalled bool
}

func (r *grsaiRecoveryHoldRepo) MarkHoldHeld(context.Context, int64, int64) error { return nil }
func (r *grsaiRecoveryHoldRepo) CloseNoChargeWithRelease(context.Context, int64, int64, string, GrsaiSettlementTxFunc) error {
	return errors.New("not used by recovery")
}
func (r *grsaiRecoveryHoldRepo) MarkManualReviewWithRelease(ctx context.Context, id, version int64, summary string, release GrsaiSettlementTxFunc) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.check(version); err != nil {
		return err
	}
	if err := release(ctx, new(sql.Tx), r.record); err != nil {
		return err
	}
	r.releaseCalled = true
	r.record.HoldState = "released"
	r.record.InternalStatus = "manual_review"
	r.record.LastErrorSummary = &summary
	return nil
}

func (r *grsaiRecoveryRepo) Create(context.Context, CreateGrsaiSettlementParams) (*GrsaiSettlement, error) {
	return nil, errors.New("not used by recovery")
}

func (r *grsaiRecoveryRepo) GetByID(_ context.Context, _ int64) (*GrsaiSettlement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.copy(), nil
}

func (r *grsaiRecoveryRepo) GetOwnedByPublicOrUpstreamID(context.Context, int64, int64, string) (*GrsaiSettlement, error) {
	return nil, errors.New("not used by recovery")
}

func (r *grsaiRecoveryRepo) ClaimByID(context.Context, int64, time.Time, time.Time) (*GrsaiSettlement, error) {
	return nil, errors.New("not used by recovery")
}

func (r *grsaiRecoveryRepo) ClaimDue(_ context.Context, now time.Time, limit int, leaseUntil time.Time) ([]*GrsaiSettlement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastClaimLimit = limit
	if r.record == nil || r.record.NextAttemptAt.After(now) || r.record.InternalStatus == "settled" ||
		r.record.InternalStatus == "closed_no_charge" || r.record.InternalStatus == "manual_review" {
		return nil, nil
	}
	r.record.InternalStatus = "processing"
	r.record.ClaimVersion++
	r.record.RetryCount++
	r.record.NextAttemptAt = leaseUntil
	return []*GrsaiSettlement{r.copy()}, nil
}

func (r *grsaiRecoveryRepo) BindUpstreamTask(_ context.Context, _, version int64, taskID, status string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.check(version); err != nil {
		return false, err
	}
	r.record.UpstreamTaskID = &taskID
	r.record.UpstreamStatus = status
	return true, nil
}

func (r *grsaiRecoveryRepo) UpdateResult(_ context.Context, _, version int64, status, summary string, next time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.check(version); err != nil {
		return false, err
	}
	r.record.UpstreamStatus = status
	r.record.LastErrorSummary = nil
	if summary != "" {
		r.record.LastErrorSummary = &summary
	}
	r.record.NextAttemptAt = next
	return true, nil
}

func (r *grsaiRecoveryRepo) RecordResultSnapshot(ctx context.Context, id, version int64, status, summary string, next time.Time, progress int, urls []string) (bool, error) {
	updated, err := r.UpdateResult(ctx, id, version, status, summary, next)
	if updated && err == nil {
		r.mu.Lock()
		r.record.Progress = progress
		r.record.ResultURLs = append([]string(nil), urls...)
		r.mu.Unlock()
	}
	return updated, err
}

func (r *grsaiRecoveryRepo) MarkPendingSettlement(_ context.Context, _, version int64, next time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.check(version); err != nil {
		return err
	}
	r.record.InternalStatus = "pending_settlement"
	r.record.SettlementRetryCount++
	r.record.NextAttemptAt = next
	return nil
}

func (r *grsaiRecoveryRepo) MarkPendingUpstream(_ context.Context, _, version int64, next time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.check(version); err != nil {
		return err
	}
	r.record.InternalStatus = "pending_upstream"
	r.record.NextAttemptAt = next
	return nil
}

func (r *grsaiRecoveryRepo) Settle(ctx context.Context, _ int64, version int64, amount float64, apply GrsaiSettlementTxFunc) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.check(version); err != nil {
		return false, err
	}
	if err := apply(ctx, new(sql.Tx), r.record); err != nil {
		return false, err
	}
	r.record.InternalStatus = "settled"
	r.record.SettledAmount = &amount
	return true, nil
}

func (r *grsaiRecoveryRepo) CloseNoCharge(_ context.Context, _, version int64, summary string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.check(version); err != nil {
		return err
	}
	r.record.InternalStatus = "closed_no_charge"
	r.record.LastErrorSummary = &summary
	return nil
}

func (r *grsaiRecoveryRepo) MarkManualReview(_ context.Context, _, version int64, summary string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.check(version); err != nil {
		return err
	}
	r.record.InternalStatus = "manual_review"
	r.record.LastErrorSummary = &summary
	return nil
}

func (r *grsaiRecoveryRepo) check(version int64) error {
	if r.record == nil || r.record.InternalStatus != "processing" || r.record.ClaimVersion != version {
		return ErrGrsaiSettlementClaimLost
	}
	return nil
}

func (r *grsaiRecoveryRepo) copy() *GrsaiSettlement {
	copy := *r.record
	return &copy
}

type grsaiRecoveryBilling struct {
	mu       sync.Mutex
	commands []*UsageBillingCommand
	err      error
}

func (b *grsaiRecoveryBilling) ApplyTx(_ context.Context, _ *sql.Tx, command *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err != nil {
		return nil, b.err
	}
	b.commands = append(b.commands, command)
	return &UsageBillingApplyResult{Applied: true}, nil
}

type grsaiRecoveryAccounts struct{ account *Account }

func (a grsaiRecoveryAccounts) GetByID(context.Context, int64) (*Account, error) {
	return a.account, nil
}

type grsaiRecoveryClient struct {
	mu            sync.Mutex
	result        *GrsaiUpstreamResult
	err           error
	resultCalls   int
	generateCalls int
}

func (c *grsaiRecoveryClient) Generate(context.Context, *Account, []byte) (*GrsaiUpstreamResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generateCalls++
	return nil, errors.New("Generate must not be called by recovery")
}

func (c *grsaiRecoveryClient) Result(context.Context, *Account, string) (*GrsaiUpstreamResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resultCalls++
	return c.result, c.err
}

func newGrsaiRecoveryRuntime(now time.Time, record *GrsaiSettlement, client *grsaiRecoveryClient, billing *grsaiRecoveryBilling) (*GrsaiSettlementRecoveryRuntime, *grsaiRecoveryRepo) {
	repo := &grsaiRecoveryRepo{record: record}
	settlement := &GrsaiSettlementService{Repo: repo, Billing: billing}
	runtime := NewGrsaiSettlementRecoveryRuntime(repo, grsaiRecoveryAccounts{account: &Account{ID: record.AccountID, Platform: PlatformGrsai}}, client, settlement, &config.Config{
		GrsaiSettlementRecovery: config.GrsaiSettlementRecoveryConfig{
			Enabled: true, ScanIntervalSeconds: 60, BatchLimit: 10, SubmissionUnknownTimeoutSeconds: 600, SettlementRetryLimit: 5,
		},
	})
	runtime.now = func() time.Time { return now }
	return runtime, repo
}

func TestGrsaiLegacyRecoveryContinuesAfterNewDeliveryDisabled(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	id := "legacy-bound"
	client := &grsaiRecoveryClient{result: &GrsaiUpstreamResult{TaskID: id, Status: GrsaiUpstreamStatusRunning}}
	runtime, _ := newGrsaiRecoveryRuntime(now, grsaiRecoveryRecord(now, &id, GrsaiUpstreamStatusRunning, "pending_upstream"), client, &grsaiRecoveryBilling{})
	runtime.opts = NewGrsaiSettlementRecoveryOptionsFromConfig(&config.Config{})
	runtime.RunOnce(context.Background())
	require.Equal(t, 1, client.resultCalls)
	runtime.Start()
	require.True(t, runtime.Running(), "existing JSON/stream records still need recovery when new delivery is disabled")
	runtime.Stop()
}

func TestGrsaiLegacyStreamSuccessWithoutResultRepollsBeforeBilling(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	id := "legacy-stream-task"
	client := &grsaiRecoveryClient{err: errors.New("temporary result lookup failure")}
	billing := &grsaiRecoveryBilling{}
	record := grsaiRecoveryRecord(now, &id, GrsaiUpstreamStatusSucceeded, "pending_settlement")
	record.DeliveryMode = GrsaiDeliveryStream
	runtime, repo := newGrsaiRecoveryRuntime(now, record, client, billing)
	runtime.now = func() time.Time { return now }
	runtime.RunOnce(context.Background())
	require.Equal(t, 1, client.resultCalls)
	require.Empty(t, billing.commands)
	require.Equal(t, "pending_upstream", repo.record.InternalStatus)
	require.Equal(t, GrsaiUpstreamStatusSucceeded, repo.record.UpstreamStatus)

	now = now.Add(time.Minute)
	client.err = nil
	client.result = &GrsaiUpstreamResult{HTTPStatus: 200, TaskID: id, Status: GrsaiUpstreamStatusSucceeded,
		ResultURLs: []string{"https://img.invalid/recovered-stream.png"}}
	runtime.RunOnce(context.Background())
	require.Equal(t, 2, client.resultCalls)
	require.Equal(t, "settled", repo.record.InternalStatus)
	require.Equal(t, []string{"https://img.invalid/recovered-stream.png"}, repo.record.ResultURLs)
	require.Len(t, billing.commands, 1)
}

func grsaiRecoveryRecord(now time.Time, taskID *string, upstreamStatus, internalStatus string) *GrsaiSettlement {
	return &GrsaiSettlement{
		ID: 19, AccountID: 8, GroupID: 3, UserID: 2, APIKeyID: 4, Model: "nano-banana-2",
		BaseUnitPrice: 0.2, GroupRateMultiplier: 1, AccountRateMultiplier: 1, BillableUnitPrice: 0.2,
		RequestedImageCount: 1, Currency: "USD", BillingIdempotencyKey: "grsai_settlement:19",
		HoldState:      "none",
		UpstreamTaskID: taskID, UpstreamStatus: upstreamStatus, InternalStatus: internalStatus,
		CreatedAt: now.Add(-time.Minute), NextAttemptAt: now.Add(-time.Second),
	}
}

func TestGrsaiSettlementRecoveryRunningAndClaimCompetition(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	taskID := "task-running"
	client := &grsaiRecoveryClient{result: &GrsaiUpstreamResult{TaskID: taskID, Status: GrsaiUpstreamStatusRunning}}
	billing := &grsaiRecoveryBilling{}
	runtime, repo := newGrsaiRecoveryRuntime(now, grsaiRecoveryRecord(now, &taskID, GrsaiUpstreamStatusRunning, "pending_upstream"), client, billing)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); runtime.RunOnce(context.Background()) }()
	go func() { defer wg.Done(); runtime.RunOnce(context.Background()) }()
	wg.Wait()

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Equal(t, "pending_upstream", repo.record.InternalStatus)
	require.Equal(t, GrsaiUpstreamStatusRunning, repo.record.UpstreamStatus)
	require.Equal(t, now.Add(time.Minute), repo.record.NextAttemptAt)
	require.Equal(t, 1, client.resultCalls)
	require.Zero(t, client.generateCalls)
	require.Empty(t, billing.commands)
}

func TestGrsaiSettlementRecoverySuccessFailureAndViolation(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name, status, want string
		wantBills          int
	}{
		{name: "succeeded bills", status: GrsaiUpstreamStatusSucceeded, want: "settled", wantBills: 1},
		{name: "failed closes", status: GrsaiUpstreamStatusFailed, want: "closed_no_charge"},
		{name: "violation closes", status: GrsaiUpstreamStatusViolation, want: "closed_no_charge"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			taskID := "task-terminal"
			client := &grsaiRecoveryClient{result: &GrsaiUpstreamResult{TaskID: taskID, Status: tt.status}}
			billing := &grsaiRecoveryBilling{}
			runtime, repo := newGrsaiRecoveryRuntime(now, grsaiRecoveryRecord(now, &taskID, GrsaiUpstreamStatusRunning, "pending_upstream"), client, billing)
			runtime.RunOnce(context.Background())

			repo.mu.Lock()
			defer repo.mu.Unlock()
			require.Equal(t, tt.want, repo.record.InternalStatus)
			require.Len(t, billing.commands, tt.wantBills)
			require.Equal(t, 1, client.resultCalls)
			require.Zero(t, client.generateCalls)
		})
	}
}

func TestGrsaiSettlementRecoverySettlementBackoffAndManualReview(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	client := &grsaiRecoveryClient{}
	billing := &grsaiRecoveryBilling{err: errors.New("billing unavailable")}
	record := grsaiRecoveryRecord(now, nil, GrsaiUpstreamStatusSucceeded, "pending_settlement")
	// Long-running result polling can raise RetryCount many times. It must not
	// affect the first durable billing retry delay.
	record.RetryCount = 27
	runtime, repo := newGrsaiRecoveryRuntime(now, record, client, billing)
	runtime.RunOnce(context.Background())

	repo.mu.Lock()
	require.Equal(t, "pending_settlement", repo.record.InternalStatus)
	require.Equal(t, now.Add(time.Minute), repo.record.NextAttemptAt)
	require.Equal(t, 1, repo.record.SettlementRetryCount)
	require.Empty(t, billing.commands)
	repo.mu.Unlock()
	require.Zero(t, client.generateCalls)

	repo.mu.Lock()
	repo.record.NextAttemptAt = now.Add(-time.Second)
	repo.record.SettlementRetryCount = 1
	repo.mu.Unlock()
	runtime.RunOnce(context.Background())
	repo.mu.Lock()
	require.Equal(t, now.Add(5*time.Minute), repo.record.NextAttemptAt)
	require.Equal(t, 2, repo.record.SettlementRetryCount)
	repo.record.NextAttemptAt = now.Add(-time.Second)
	repo.record.SettlementRetryCount = 5
	repo.mu.Unlock()
	runtime.RunOnce(context.Background())
	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Equal(t, "manual_review", repo.record.InternalStatus)
	require.Contains(t, *repo.record.LastErrorSummary, "no upstream request was retried")
	require.Zero(t, client.generateCalls)
}

// @covers AC-006
func TestGrsaiSettlementRecoveryManualReviewReleasesHeldBalance(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	record := grsaiRecoveryRecord(now, nil, "submitting", "pending_upstream")
	record.CreatedAt = now.Add(-time.Hour)
	record.HoldAmount, record.HoldState = 0.2, "held"
	runtime, base := newGrsaiRecoveryRuntime(now, record, &grsaiRecoveryClient{}, &grsaiRecoveryBilling{})
	repo := &grsaiRecoveryHoldRepo{grsaiRecoveryRepo: base}
	billing := &grsaiBillingSpy{}
	runtime.repo = repo
	runtime.settlement.Repo = repo
	runtime.settlement.HoldBilling = billing

	runtime.RunOnce(context.Background())

	require.True(t, repo.releaseCalled)
	require.Equal(t, 1, billing.holdReleased)
	require.Equal(t, "released", record.HoldState)
	require.Equal(t, "manual_review", record.InternalStatus)
}

func TestGrsaiSettlementRecoveryLimitsClaimsToOneLongRequestLease(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	taskID := "long-running-task"
	client := &grsaiRecoveryClient{result: &GrsaiUpstreamResult{TaskID: taskID, Status: GrsaiUpstreamStatusRunning}}
	billing := &grsaiRecoveryBilling{}
	runtime, repo := newGrsaiRecoveryRuntime(now, grsaiRecoveryRecord(now, &taskID, GrsaiUpstreamStatusRunning, "pending_upstream"), client, billing)
	runtime.opts.BatchLimit = 100

	runtime.RunOnce(context.Background())

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Equal(t, 1, repo.lastClaimLimit)
	require.Greater(t, grsaiSettlementLease, grsaiRequestTimeout)
	require.Equal(t, 1, client.resultCalls)
	require.Zero(t, client.generateCalls)
}

func TestGrsaiSettlementRecoveryUnknownSubmissionNeverResendsAndSurvivesRestart(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	client := &grsaiRecoveryClient{}
	billing := &grsaiRecoveryBilling{}
	record := grsaiRecoveryRecord(now, nil, "not_submitted", "pending_upstream")
	record.CreatedAt = now.Add(-time.Hour)
	runtime, repo := newGrsaiRecoveryRuntime(now, record, client, billing)
	runtime.RunOnce(context.Background())

	repo.mu.Lock()
	require.Equal(t, "manual_review", repo.record.InternalStatus)
	require.Contains(t, *repo.record.LastErrorSummary, "not resent")
	repo.mu.Unlock()
	require.Zero(t, client.resultCalls)
	require.Zero(t, client.generateCalls)

	// A fresh runtime observes only next_attempt_at state. There is no process
	// local queue to recover, and terminal manual-review records remain inert.
	restarted, _ := newGrsaiRecoveryRuntime(now.Add(time.Hour), repo.record, client, billing)
	restarted.RunOnce(context.Background())
	require.Zero(t, client.resultCalls)
	require.Zero(t, client.generateCalls)
}

func TestGrsaiSettlementRecoveryRestartClaimsExistingDueTask(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	taskID := "task-after-restart"
	client := &grsaiRecoveryClient{result: &GrsaiUpstreamResult{TaskID: taskID, Status: GrsaiUpstreamStatusRunning}}
	billing := &grsaiRecoveryBilling{}
	record := grsaiRecoveryRecord(now, &taskID, GrsaiUpstreamStatusRunning, "pending_upstream")

	// This runtime did not create the record: it models a process that starts
	// after a previous process has persisted next_attempt_at and exited.
	restarted, repo := newGrsaiRecoveryRuntime(now, record, client, billing)
	restarted.RunOnce(context.Background())

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Equal(t, "pending_upstream", repo.record.InternalStatus)
	require.Equal(t, now.Add(time.Minute), repo.record.NextAttemptAt)
	require.Equal(t, 1, client.resultCalls)
	require.Zero(t, client.generateCalls)
}

func TestGrsaiSettlementRecoveryMalformedResultIsDeferredWithoutResubmission(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	taskID := "task-malformed"
	client := &grsaiRecoveryClient{err: ErrGrsaiInvalidResponse}
	billing := &grsaiRecoveryBilling{}
	runtime, repo := newGrsaiRecoveryRuntime(now, grsaiRecoveryRecord(now, &taskID, GrsaiUpstreamStatusRunning, "pending_upstream"), client, billing)
	runtime.RunOnce(context.Background())

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Equal(t, "pending_upstream", repo.record.InternalStatus)
	require.Equal(t, "unknown", repo.record.UpstreamStatus)
	require.Equal(t, now.Add(time.Minute), repo.record.NextAttemptAt)
	require.Zero(t, client.generateCalls)
	require.Empty(t, billing.commands)
}

func TestGrsaiSettlementRecoveryTemporaryResultHTTPFailuresAreDeferred(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	for _, statusCode := range []int{429, 502} {
		t.Run(fmt.Sprintf("HTTP_%d", statusCode), func(t *testing.T) {
			taskID := "task-temporary-http"
			client := &grsaiRecoveryClient{
				result: &GrsaiUpstreamResult{HTTPStatus: statusCode, TaskID: taskID},
				err:    &GrsaiHTTPError{Operation: "result", StatusCode: statusCode},
			}
			billing := &grsaiRecoveryBilling{}
			runtime, repo := newGrsaiRecoveryRuntime(now, grsaiRecoveryRecord(now, &taskID, GrsaiUpstreamStatusRunning, "pending_upstream"), client, billing)

			runtime.RunOnce(context.Background())

			repo.mu.Lock()
			defer repo.mu.Unlock()
			require.Equal(t, "pending_upstream", repo.record.InternalStatus)
			require.Equal(t, "unknown", repo.record.UpstreamStatus)
			require.NotNil(t, repo.record.LastErrorSummary)
			require.Contains(t, *repo.record.LastErrorSummary, fmt.Sprintf("HTTP %d", statusCode))
			require.Equal(t, now.Add(time.Minute), repo.record.NextAttemptAt)
			require.Zero(t, client.generateCalls)
		})
	}
}
