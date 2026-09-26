package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

type GrsaiTaskUpstream interface {
	GrsaiAsyncNativeClient
	Result(context.Context, *Account, string) (*GrsaiUpstreamResult, error)
}

type GrsaiTaskImagePersister interface {
	PersistGrsaiImages(context.Context, string, *GrsaiUpstreamResult) (*GrsaiStoredResult, error)
}

type GrsaiTaskRuntimeOptions struct {
	Enabled           bool
	ScanInterval      time.Duration
	BatchLimit        int
	MaxRunning        int
	FailureRetryLimit int
}

type GrsaiTaskRuntime struct {
	repo      GrsaiV2TaskRepository
	payloads  GrsaiTaskPayloadRepository
	accounts  GrsaiSettlementAccountReader
	upstream  GrsaiTaskUpstream
	images    GrsaiTaskImagePersister
	balance   GrsaiBalanceHoldRepository
	usageLogs UsageLogRepository
	opts      GrsaiTaskRuntimeOptions
	now       func() time.Time
	mu        sync.Mutex
	cancel    context.CancelFunc
	done      chan struct{}
}

func (r *GrsaiTaskRuntime) WithUsageLogs(repo UsageLogRepository) *GrsaiTaskRuntime {
	if r != nil {
		r.usageLogs = repo
	}
	return r
}

func NewGrsaiTaskRuntime(repo GrsaiV2TaskRepository, payloads GrsaiTaskPayloadRepository,
	accounts GrsaiSettlementAccountReader, upstream GrsaiTaskUpstream,
	images GrsaiTaskImagePersister, balance GrsaiBalanceHoldRepository,
	opts GrsaiTaskRuntimeOptions) *GrsaiTaskRuntime {
	if opts.ScanInterval <= 0 {
		opts.ScanInterval = time.Minute
	}
	if opts.BatchLimit <= 0 {
		opts.BatchLimit = 20
	}
	if opts.MaxRunning <= 0 {
		opts.MaxRunning = 3
	}
	if opts.FailureRetryLimit <= 0 {
		opts.FailureRetryLimit = 5
	}
	return &GrsaiTaskRuntime{repo: repo, payloads: payloads, accounts: accounts,
		upstream: upstream, images: images, balance: balance, opts: opts, now: time.Now}
}

func (r *GrsaiTaskRuntime) Start() {
	if r == nil || !r.opts.Enabled || r.repo == nil || r.payloads == nil || r.accounts == nil ||
		r.upstream == nil || r.images == nil || r.balance == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.done = make(chan struct{})
	go func(done chan struct{}) {
		defer close(done)
		ticker := time.NewTicker(r.opts.ScanInterval)
		defer ticker.Stop()
		for {
			if err := r.RunOnce(ctx); err != nil && ctx.Err() == nil {
				logger.L().Warn("grsai durable task worker failed", zap.Error(err))
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}(r.done)
}

func (r *GrsaiTaskRuntime) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	cancel, done := r.cancel, r.done
	r.cancel, r.done = nil, nil
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (r *GrsaiTaskRuntime) RunOnce(ctx context.Context) error {
	if r == nil || r.repo == nil || r.payloads == nil || r.accounts == nil || r.upstream == nil || r.images == nil || r.balance == nil {
		return ErrGrsaiSettlementInvalidInput
	}
	now := r.now()
	claims, err := r.repo.ClaimDueV2(ctx, now, r.opts.BatchLimit, now.Add(grsaiSettlementLease), r.opts.MaxRunning)
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	errs := make(chan error, len(claims))
	for _, claim := range claims {
		wg.Add(1)
		go func(claim *GrsaiSettlement) {
			defer wg.Done()
			if err := r.processClaim(ctx, claim); err != nil {
				errs <- err
			}
		}(claim)
	}
	wg.Wait()
	close(errs)
	var joined error
	for err := range errs {
		joined = errors.Join(joined, err)
	}
	return joined
}

func (r *GrsaiTaskRuntime) processClaim(ctx context.Context, claim *GrsaiSettlement) error {
	if claim == nil || claim.LocalTaskID == nil || *claim.LocalTaskID == "" {
		return ErrGrsaiSettlementInvalidInput
	}
	if claim.UpstreamTaskID == nil || *claim.UpstreamTaskID == "" {
		if claim.SubmissionAttempt != 1 {
			return r.fail(ctx, claim, "uncertain_submit")
		}
		payload, err := r.payloads.GetDecrypted(ctx, *claim.LocalTaskID, claim.ClaimVersion)
		if err != nil {
			return r.fail(ctx, claim, "payload_unavailable")
		}
		account, err := r.accounts.GetByID(ctx, claim.AccountID)
		if err != nil || account == nil || account.Platform != PlatformGrsai {
			return r.fail(ctx, claim, "account_unavailable")
		}
		result, err := r.upstream.GenerateAsync(ctx, account, payload)
		if err != nil || result == nil || result.TaskID == "" {
			return r.fail(ctx, claim, "uncertain_submit")
		}
		bound, err := r.repo.BindV2UpstreamTask(ctx, claim.ID, claim.ClaimVersion, result.TaskID)
		if err != nil {
			return err
		}
		if !bound {
			return ErrGrsaiSettlementClaimLost
		}
		return r.deferPoll(ctx, claim)
	}
	if claim.SettlementRetryCount >= r.opts.FailureRetryLimit {
		return r.manualReview(ctx, claim, "result_retries_exhausted")
	}
	account, err := r.accounts.GetByID(ctx, claim.AccountID)
	if err != nil || account == nil || account.Platform != PlatformGrsai {
		return r.deferFailure(ctx, claim, "account_unavailable")
	}
	result, err := r.upstream.Result(ctx, account, *claim.UpstreamTaskID)
	if err != nil || result == nil {
		if retryErr := r.deferFailure(ctx, claim, "result_poll_failed"); retryErr != nil {
			return retryErr
		}
		return fmt.Errorf("grsai result polling failed")
	}
	switch result.Status {
	case GrsaiUpstreamStatusSucceeded:
		stored, err := r.images.PersistGrsaiImages(ctx, *claim.LocalTaskID, result)
		if err != nil {
			if retryErr := r.deferFailure(ctx, claim, "result_persistence_failed"); retryErr != nil {
				return retryErr
			}
			return err
		}
		amount, err := GrsaiTaskHoldAmount(claim)
		if err != nil {
			return err
		}
		completed, err := r.repo.CompleteV2(ctx, claim.ID, claim.ClaimVersion, stored.ResultJSON,
			stored.ObjectMetadata, stored.LinkExpiresAt, amount, r.balance.CaptureGrsaiBalanceTx)
		if err != nil {
			if retryErr := r.deferFailure(ctx, claim, "result_persistence_failed"); retryErr != nil {
				return retryErr
			}
			return err
		}
		if !completed {
			return ErrGrsaiSettlementClaimLost
		}
		r.recordUsage(claim, amount)
		return nil
	case GrsaiUpstreamStatusFailed, GrsaiUpstreamStatusViolation:
		return r.fail(ctx, claim, "upstream_failed")
	default:
		return r.deferPoll(ctx, claim)
	}
}

func (r *GrsaiTaskRuntime) recordUsage(claim *GrsaiSettlement, amount float64) {
	if r.usageLogs == nil || claim == nil || claim.LocalTaskID == nil {
		return
	}
	mode, endpoint := string(BillingModeImage), "/v1/api/generate"
	baseCost := claim.BaseUnitPrice * float64(claim.RequestedImageCount)
	requestType := RequestTypeSync
	if claim.DeliveryMode == string(GrsaiDeliveryStream) {
		requestType = RequestTypeStream
	}
	usage := &UsageLog{UserID: claim.UserID, APIKeyID: claim.APIKeyID, AccountID: claim.AccountID,
		GroupID: &claim.GroupID, RequestID: "grsai_task:" + *claim.LocalTaskID,
		Model: claim.Model, RequestedModel: claim.Model, ImageCount: claim.RequestedImageCount,
		ImageSize: &claim.ImageSize, ImageOutputCost: baseCost, TotalCost: baseCost,
		ActualCost: amount, RateMultiplier: claim.GroupRateMultiplier,
		AccountRateMultiplier: &claim.AccountRateMultiplier,
		BillingType:           BillingTypeBalance, RequestType: requestType, BillingMode: &mode,
		InboundEndpoint: &endpoint, UpstreamEndpoint: &endpoint, CreatedAt: r.now()}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	writeUsageLogBestEffort(ctx, r.usageLogs, usage, "service.grsai_task_runtime")
}

func (r *GrsaiTaskRuntime) deferPoll(ctx context.Context, claim *GrsaiSettlement) error {
	updated, err := r.repo.UpdateV2Progress(ctx, claim.ID, claim.ClaimVersion, "running", 5, r.now().Add(r.opts.ScanInterval))
	if err != nil {
		return err
	}
	if !updated {
		return ErrGrsaiSettlementClaimLost
	}
	return nil
}

func (r *GrsaiTaskRuntime) fail(ctx context.Context, claim *GrsaiSettlement, code string) error {
	failed, err := r.repo.FailV2(ctx, claim.ID, claim.ClaimVersion, code, "", r.balance.ReleaseGrsaiBalanceTx)
	if err != nil {
		return err
	}
	if !failed {
		return ErrGrsaiSettlementClaimLost
	}
	return nil
}

func (r *GrsaiTaskRuntime) deferFailure(ctx context.Context, claim *GrsaiSettlement, reason string) error {
	if claim.SettlementRetryCount+1 >= r.opts.FailureRetryLimit {
		return r.manualReview(ctx, claim, reason)
	}
	index := claim.SettlementRetryCount
	if index >= len(grsaiSettlementRecoveryBackoff) {
		index = len(grsaiSettlementRecoveryBackoff) - 1
	}
	updated, err := r.repo.DeferV2Failure(ctx, claim.ID, claim.ClaimVersion,
		r.now().Add(grsaiSettlementRecoveryBackoff[index]))
	if err != nil {
		return err
	}
	if !updated {
		return ErrGrsaiSettlementClaimLost
	}
	return nil
}

func (r *GrsaiTaskRuntime) manualReview(ctx context.Context, claim *GrsaiSettlement, reason string) error {
	marked, err := r.repo.MarkV2ManualReview(ctx, claim.ID, claim.ClaimVersion, reason)
	if err != nil {
		return err
	}
	if !marked {
		return ErrGrsaiSettlementClaimLost
	}
	return nil
}
