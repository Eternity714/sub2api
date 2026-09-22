package service

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// This fake replaces PostgreSQL only; the real service builds snapshots,
// commands, transitions and retry decisions. Transactional effects are covered
// by the repository integration tests using the real billing repository.
type grsaiSettlementMemoryRepo struct {
	GrsaiSettlementRepository
	record     *GrsaiSettlement
	failSettle bool
	createErr  error
	claimErr   error
}

func (r *grsaiSettlementMemoryRepo) Create(_ context.Context, p CreateGrsaiSettlementParams) (*GrsaiSettlement, error) {
	if r.createErr != nil {
		return nil, r.createErr
	}
	r.record = &GrsaiSettlement{ID: 17, AccountID: p.AccountID, GroupID: p.GroupID, UserID: p.UserID,
		APIKeyID: p.APIKeyID, Model: p.Model, BaseUnitPrice: p.BaseUnitPrice, GroupRateMultiplier: p.GroupRateMultiplier,
		AccountRateMultiplier: p.AccountRateMultiplier, BillableUnitPrice: p.BillableUnitPrice,
		RequestedImageCount: p.RequestedImageCount, ImageSize: p.ImageSize, Currency: p.Currency, BillingIdempotencyKey: GrsaiSettlementRequestID(17),
		HoldAmount: p.HoldAmount, HoldState: p.HoldState, DeliveryMode: p.DeliveryMode,
		PayloadDeleteAfter: p.PayloadDeleteAfter, ExpiresAt: p.ExpiresAt,
		UpstreamStatus: "not_submitted", InternalStatus: "pending_upstream", NextAttemptAt: p.NextAttemptAt}
	return r.GetByID(context.Background(), 17)
}

func (r *grsaiSettlementMemoryRepo) GetByID(ctx context.Context, _ int64) (*GrsaiSettlement, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	copy := *r.record
	return &copy, nil
}

func (r *grsaiSettlementMemoryRepo) ClaimByID(_ context.Context, _ int64, now, until time.Time) (*GrsaiSettlement, error) {
	if r.claimErr != nil {
		return nil, r.claimErr
	}
	if r.record.InternalStatus == "settled" || r.record.InternalStatus == "closed_no_charge" ||
		(r.record.InternalStatus == "processing" && r.record.NextAttemptAt.After(now)) {
		return nil, ErrGrsaiSettlementClaimLost
	}
	r.record.InternalStatus = "processing"
	r.record.ClaimVersion++
	r.record.NextAttemptAt = until
	return r.GetByID(context.Background(), 17)
}

func (r *grsaiSettlementMemoryRepo) check(v int64) error {
	if r.record.ClaimVersion != v || r.record.InternalStatus != "processing" {
		return ErrGrsaiSettlementClaimLost
	}
	return nil
}

func (r *grsaiSettlementMemoryRepo) BindUpstreamTask(_ context.Context, _, v int64, id, status string) (bool, error) {
	if err := r.check(v); err != nil {
		return false, err
	}
	r.record.UpstreamTaskID = &id
	r.record.UpstreamStatus = status
	return true, nil
}

func (r *grsaiSettlementMemoryRepo) UpdateResult(_ context.Context, _, v int64, status, summary string, next time.Time) (bool, error) {
	if err := r.check(v); err != nil {
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

func (r *grsaiSettlementMemoryRepo) MarkPendingSettlement(_ context.Context, _, v int64, next time.Time) error {
	if err := r.check(v); err != nil {
		return err
	}
	r.record.InternalStatus = "pending_settlement"
	r.record.SettlementRetryCount++
	r.record.NextAttemptAt = next
	return nil
}

func (r *grsaiSettlementMemoryRepo) MarkPendingUpstream(_ context.Context, _, v int64, next time.Time) error {
	if err := r.check(v); err != nil {
		return err
	}
	r.record.InternalStatus = "pending_upstream"
	r.record.NextAttemptAt = next
	return nil
}

func (r *grsaiSettlementMemoryRepo) CloseNoCharge(_ context.Context, _, v int64, _ string) error {
	if err := r.check(v); err != nil {
		return err
	}
	r.record.InternalStatus = "closed_no_charge"
	return nil
}

func (r *grsaiSettlementMemoryRepo) MarkManualReview(_ context.Context, _, v int64, _ string) error {
	if err := r.check(v); err != nil {
		return err
	}
	r.record.InternalStatus = "manual_review"
	return nil
}

func (r *grsaiSettlementMemoryRepo) Settle(ctx context.Context, _, v int64, amount float64, apply GrsaiSettlementTxFunc) (bool, error) {
	if r.record.InternalStatus == "settled" && r.record.ClaimVersion == v {
		return false, nil
	}
	if err := r.check(v); err != nil {
		return false, err
	}
	if r.failSettle {
		return false, errors.New("database unavailable")
	}
	if err := apply(ctx, new(sql.Tx), r.record); err != nil {
		return false, err
	}
	r.record.InternalStatus = "settled"
	r.record.SettledAmount = &amount
	return true, nil
}

type grsaiBillingSpy struct {
	commands     []*UsageBillingCommand
	err          error
	releaseErr   error
	holdReserved int
	holdReleased int
}

func (b *grsaiBillingSpy) ReserveGrsaiBalance(_ context.Context, _ *GrsaiBalanceHoldCommand) (*GrsaiBalanceHoldResult, error) {
	b.holdReserved++
	return &GrsaiBalanceHoldResult{Applied: true}, nil
}
func (b *grsaiBillingSpy) ReleaseGrsaiBalance(_ context.Context, _ *GrsaiBalanceHoldCommand) (*GrsaiBalanceHoldResult, error) {
	b.holdReleased++
	if b.releaseErr != nil {
		return nil, b.releaseErr
	}
	return &GrsaiBalanceHoldResult{Applied: true}, nil
}
func (b *grsaiBillingSpy) ReleaseGrsaiBalanceTx(_ context.Context, _ *sql.Tx, _ *GrsaiBalanceHoldCommand) (*GrsaiBalanceHoldResult, error) {
	b.holdReleased++
	return &GrsaiBalanceHoldResult{Applied: true}, nil
}
func (b *grsaiBillingSpy) CaptureGrsaiBalanceTx(_ context.Context, _ *sql.Tx, _ *GrsaiBalanceHoldCommand) (*GrsaiBalanceHoldResult, error) {
	return &GrsaiBalanceHoldResult{Applied: true}, nil
}

func (b *grsaiBillingSpy) ApplyTx(ctx context.Context, tx *sql.Tx, cmd *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if tx == nil {
		return nil, errors.New("missing settlement transaction")
	}
	if b.err != nil {
		return nil, b.err
	}
	b.commands = append(b.commands, cmd)
	return &UsageBillingApplyResult{Applied: true}, nil
}

type grsaiPriceStub struct{ price float64 }

func (p *grsaiPriceStub) GrsaiUnitPrice(context.Context, string, *Group) (float64, error) {
	return p.price, nil
}

type grsaiUsageSpy struct {
	UsageLogRepository
	logs []*UsageLog
}

func (u *grsaiUsageSpy) Create(_ context.Context, log *UsageLog) (bool, error) {
	u.logs = append(u.logs, log)
	return true, nil
}

func grsaiSettlementFixture(t *testing.T) (*GrsaiSettlementService, *grsaiSettlementMemoryRepo, *grsaiBillingSpy, *GrsaiSettlement, *grsaiPriceStub) {
	t.Helper()
	r := &grsaiSettlementMemoryRepo{}
	b := &grsaiBillingSpy{}
	p := &grsaiPriceStub{price: 0.1}
	s := &GrsaiSettlementService{Repo: r, Billing: b, Pricing: p}
	rate := 3.0
	group := &Group{ID: 2, Platform: PlatformGrsai, RateMultiplier: 2}
	record, err := s.Prepare(context.Background(), GrsaiPrepareInput{Account: &Account{ID: 3, Platform: PlatformGrsai,
		Type: AccountTypeAPIKey, RateMultiplier: &rate}, APIKey: &APIKey{ID: 4, UserID: 1, GroupID: &group.ID, Group: group},
		Model: "grsai-image", ImageCount: 2, ImageSize: "1K"})
	require.NoError(t, err)
	return s, r, b, record, p
}

// Removing the snapshot or repricing during settlement changes the literal $1.20 charge.
func TestGrsaiSettlementSnapshotSurvivesRepricingAndDuplicateSuccess(t *testing.T) {
	s, r, billing, record, price := grsaiSettlementFixture(t)
	usage := &grsaiUsageSpy{}
	s.UsageLogRepo = usage
	require.Equal(t, GrsaiStateSubmissionPending, record.State())
	require.Equal(t, GrsaiSettlementRequestID(record.ID), record.BillingIdempotencyKey)
	price.price = 900
	upstream := &GrsaiUpstreamResult{HTTPStatus: 200, TaskID: "task-17", Status: GrsaiUpstreamStatusSucceeded, RawBody: []byte(`{"status":"succeeded"}`)}
	out := s.Finish(context.Background(), record, upstream, nil)
	require.NoError(t, out.SettlementError)
	require.Same(t, upstream, out.Upstream)
	require.Equal(t, GrsaiStateSettled, out.State)
	require.InDelta(t, 1.2, *r.record.SettledAmount, 1e-10)
	out = s.Finish(context.Background(), record, upstream, nil)
	require.NoError(t, out.SettlementError)
	require.Len(t, billing.commands, 1)
	cmd := billing.commands[0]
	require.Equal(t, "grsai_settlement:17", cmd.RequestID)
	// The hold already debited users.balance; ApplyTx must only apply quota and
	// usage effects, so its balance component is zero.
	require.Zero(t, cmd.BalanceCost)
	require.Equal(t, 1.2, cmd.APIKeyQuotaCost)
	require.Equal(t, 1.2, cmd.APIKeyRateLimitCost)
	require.Equal(t, 0.6, cmd.AccountQuotaCost)
	require.Len(t, usage.logs, 1)
	require.Equal(t, "grsai_settlement:17", usage.logs[0].RequestID)
	require.Equal(t, 1.2, usage.logs[0].ActualCost)
	require.Equal(t, 2, usage.logs[0].ImageCount)
	require.NotNil(t, usage.logs[0].ImageSize)
	require.Equal(t, ImageBillingSize1K, *usage.logs[0].ImageSize)
}

func TestGrsaiSettlementPrepareNormalizesImageSizeSnapshot(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  string
	}{
		{input: "1k", want: ImageBillingSize1K},
		{input: "2048x2048", want: ImageBillingSize2K},
		{input: "4096x4096", want: ImageBillingSize4K},
		{input: "unknown", want: ImageBillingSize2K},
	} {
		t.Run(tt.input, func(t *testing.T) {
			s, repo, _, _, _ := grsaiSettlementFixture(t)
			repo.record = nil
			group := &Group{ID: 2, Platform: PlatformGrsai, RateMultiplier: 1}
			record, err := s.Prepare(context.Background(), GrsaiPrepareInput{Account: &Account{ID: 3, Platform: PlatformGrsai, Type: AccountTypeAPIKey},
				APIKey: &APIKey{ID: 4, UserID: 1, GroupID: &group.ID, Group: group}, Model: "image", ImageCount: 1, ImageSize: tt.input})
			require.NoError(t, err)
			require.Equal(t, tt.want, record.ImageSize)
		})
	}
}

func TestGrsaiSettlementNonSuccessNeverBills(t *testing.T) {
	for _, tt := range []struct {
		name, status, task string
		httpStatus         int
		upstreamErr        error
		state              GrsaiSettlementState
	}{
		{"failed", "failed", "task", 0, nil, GrsaiStateClosedNoCharge},
		{"violation", "violation", "task", 0, nil, GrsaiStateClosedNoCharge},
		{"running", "running", "task", 0, nil, GrsaiStateAwaitingResult},
		{"malformed", "", "", 0, ErrGrsaiInvalidResponse, GrsaiStateUpstreamUnknown},
		{"HTTP status error with task", "succeeded", "task", 500, nil, GrsaiStateClosedNoCharge},
		{"HTTP error with task", "succeeded", "task", 0, &GrsaiHTTPError{StatusCode: 500}, GrsaiStateClosedNoCharge},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, repo, b, record, _ := grsaiSettlementFixture(t)
			out := s.Finish(context.Background(), record, &GrsaiUpstreamResult{HTTPStatus: tt.httpStatus, TaskID: tt.task, Status: tt.status}, tt.upstreamErr)
			require.NoError(t, out.SettlementError)
			require.Equal(t, tt.state, out.State)
			require.Equal(t, tt.upstreamErr, out.UpstreamError)
			require.Empty(t, b.commands)
			if (tt.state == GrsaiStateClosedNoCharge && tt.upstreamErr != nil) || tt.httpStatus >= 400 {
				require.Nil(t, repo.record.UpstreamTaskID)
				require.Equal(t, "not_submitted", repo.record.UpstreamStatus)
			}
		})
	}
}

func TestGrsaiSettlementKnownResultPollTemporaryFailuresRetry(t *testing.T) {
	for _, tt := range []struct {
		name     string
		upstream *GrsaiUpstreamResult
		err      error
		want     string
	}{
		{name: "rate limited", upstream: &GrsaiUpstreamResult{HTTPStatus: 429}, err: &GrsaiHTTPError{Operation: "result", StatusCode: 429}, want: "HTTP 429"},
		{name: "server error", upstream: &GrsaiUpstreamResult{HTTPStatus: 502}, err: &GrsaiHTTPError{Operation: "result", StatusCode: 502}, want: "HTTP 502"},
		{name: "transport", err: errors.New("connection reset by peer"), want: "transport failure"},
		{name: "malformed", upstream: &GrsaiUpstreamResult{HTTPStatus: 200}, err: ErrGrsaiInvalidResponse, want: "invalid response"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, repo, billing, claim, _ := grsaiSettlementFixture(t)
			knownTaskID := "already-submitted-task"
			repo.record.UpstreamTaskID = &knownTaskID
			repo.record.UpstreamStatus = GrsaiUpstreamStatusRunning
			claim.UpstreamTaskID = &knownTaskID
			claim.UpstreamStatus = GrsaiUpstreamStatusRunning
			retryAt := time.Now().Add(2 * time.Minute)

			out := s.FinishAt(context.Background(), claim, tt.upstream, tt.err, retryAt)

			require.NoError(t, out.SettlementError)
			require.Equal(t, "pending_upstream", repo.record.InternalStatus)
			require.Equal(t, "unknown", repo.record.UpstreamStatus)
			require.WithinDuration(t, retryAt, repo.record.NextAttemptAt, time.Millisecond)
			require.NotNil(t, repo.record.LastErrorSummary)
			require.Contains(t, *repo.record.LastErrorSummary, tt.want)
			require.Empty(t, billing.commands)
		})
	}
}

func TestGrsaiSettlementBillingFailurePreservesSuccessAndRetries(t *testing.T) {
	s, r, b, record, _ := grsaiSettlementFixture(t)
	b.err = errors.New("billing temporarily unavailable")
	upstream := &GrsaiUpstreamResult{Status: "succeeded", RawBody: []byte(`{"ok":true}`)}
	out := s.Finish(context.Background(), record, upstream, nil)
	require.Same(t, upstream, out.Upstream)
	require.NoError(t, out.UpstreamError)
	require.Error(t, out.SettlementError)
	require.Equal(t, GrsaiStateSettlementPending, out.State)
	require.Equal(t, "succeeded", r.record.UpstreamStatus)
	b.err = nil
	claimed, err := r.ClaimByID(context.Background(), 17, time.Now(), time.Now().Add(time.Minute))
	require.NoError(t, err)
	_, err = s.Settle(context.Background(), claimed.ID, claimed.ClaimVersion)
	require.NoError(t, err)
	require.Len(t, b.commands, 1)
	require.Equal(t, "settled", r.record.InternalStatus)
}

func TestGrsaiSettlementRejectsStaleClaimAndNonSuccess(t *testing.T) {
	s, r, b, record, _ := grsaiSettlementFixture(t)
	_, err := s.Settle(context.Background(), record.ID, record.ClaimVersion)
	require.ErrorIs(t, err, ErrGrsaiSettlementInvalidState)
	r.record.ClaimVersion++
	out := s.Finish(context.Background(), record, &GrsaiUpstreamResult{Status: "succeeded"}, nil)
	require.ErrorIs(t, out.SettlementError, ErrGrsaiSettlementClaimLost)
	require.Equal(t, "not_submitted", r.record.UpstreamStatus)
	require.Empty(t, b.commands)
}

func TestGrsaiSettlementRejectsInvalidPriceBeforeCreate(t *testing.T) {
	for _, price := range []float64{-1, math.NaN(), math.Inf(1)} {
		s, r, _, _, p := grsaiSettlementFixture(t)
		p.price = price
		r.record = nil
		group := &Group{ID: 2, Platform: PlatformGrsai, RateMultiplier: 1}
		_, err := s.Prepare(context.Background(), GrsaiPrepareInput{Account: &Account{ID: 3, Platform: PlatformGrsai, Type: AccountTypeAPIKey},
			APIKey: &APIKey{ID: 4, UserID: 1, GroupID: &group.ID, Group: group}, Model: "image", ImageCount: 1})
		require.Error(t, err)
		require.Nil(t, r.record)
	}
}

func TestGrsaiSettlementExplicitZeroPriceAndMissingPriceDiffer(t *testing.T) {
	zero := 0.0
	resolver := &GrsaiModelPricingResolver{Resolver: NewModelPricingResolver(nil, nil)}
	group := &Group{ID: 1, ModelPricing: []ChannelModelPricing{{Models: []string{"grsai-free"}, BillingMode: BillingModeImage, PerRequestPrice: &zero}}}
	price, err := resolver.GrsaiUnitPrice(context.Background(), "grsai-free", group)
	require.NoError(t, err)
	require.Zero(t, price)
	group.ModelPricing[0].PerRequestPrice = nil
	_, err = resolver.GrsaiUnitPrice(context.Background(), "grsai-free", group)
	require.ErrorIs(t, err, ErrGrsaiSettlementPricingMissing)
}

func TestGrsaiSettlementCancelledClientStillRecordsSuccess(t *testing.T) {
	s, _, billing, record, _ := grsaiSettlementFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out := s.Finish(ctx, record, &GrsaiUpstreamResult{Status: "succeeded"}, nil)
	require.NoError(t, out.SettlementError)
	require.Equal(t, GrsaiStateSettled, out.State)
	require.Len(t, billing.commands, 1)
}

func TestGrsaiSettlementConflictingTaskGoesToManualReview(t *testing.T) {
	s, repo, billing, record, _ := grsaiSettlementFixture(t)
	known := "known-task"
	repo.record.UpstreamTaskID = &known
	out := s.Finish(context.Background(), record, &GrsaiUpstreamResult{TaskID: "different-task", Status: "succeeded"}, nil)
	require.ErrorIs(t, out.SettlementError, ErrGrsaiHoldReleaseUnavailable)
	require.NotEqual(t, GrsaiStateManualReview, out.State)
	require.Empty(t, billing.commands)
}

func TestGrsaiSettlementPrepareRequiresDurableSnapshot(t *testing.T) {
	s, repo, _, _, _ := grsaiSettlementFixture(t)
	repo.createErr = errors.New("persist unavailable")
	group := &Group{ID: 2, Platform: PlatformGrsai, RateMultiplier: 1}
	record, err := s.Prepare(context.Background(), GrsaiPrepareInput{Account: &Account{ID: 3, Platform: PlatformGrsai, Type: AccountTypeAPIKey},
		APIKey: &APIKey{ID: 4, UserID: 1, GroupID: &group.ID, Group: group}, Model: "image", ImageCount: 1})
	require.ErrorIs(t, err, repo.createErr)
	require.Nil(t, record)
}

func TestGrsaiPrepareReleasesHoldWhenSubmissionClaimFails(t *testing.T) {
	s, repo, billing, _, _ := grsaiSettlementFixture(t)
	s.HoldBilling = billing
	billing.holdReserved, billing.holdReleased = 0, 0
	repo.claimErr = errors.New("claim failed")
	group := &Group{ID: 2, Platform: PlatformGrsai, RateMultiplier: 1}
	_, err := s.Prepare(context.Background(), GrsaiPrepareInput{Account: &Account{ID: 3, Platform: PlatformGrsai, Type: AccountTypeAPIKey}, APIKey: &APIKey{ID: 4, UserID: 1, GroupID: &group.ID, Group: group}, Model: "image", ImageCount: 1})
	require.ErrorIs(t, err, repo.claimErr)
	require.Equal(t, 1, billing.holdReserved)
	require.Equal(t, 1, billing.holdReleased)
}

func TestGrsaiPrepareReturnsClaimAndReleaseErrors(t *testing.T) {
	s, repo, billing, _, _ := grsaiSettlementFixture(t)
	s.HoldBilling = billing
	repo.claimErr = errors.New("claim failed")
	billing.releaseErr = errors.New("release failed")
	group := &Group{ID: 2, Platform: PlatformGrsai, RateMultiplier: 1}
	_, err := s.Prepare(context.Background(), GrsaiPrepareInput{Account: &Account{ID: 3, Platform: PlatformGrsai, Type: AccountTypeAPIKey}, APIKey: &APIKey{ID: 4, UserID: 1, GroupID: &group.ID, Group: group}, Model: "image", ImageCount: 1})
	require.ErrorIs(t, err, repo.claimErr)
	require.ErrorIs(t, err, billing.releaseErr)
}
