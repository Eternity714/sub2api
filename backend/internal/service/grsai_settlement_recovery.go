package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

const (
	defaultGrsaiSettlementRecoveryScanInterval = time.Minute
	defaultGrsaiSettlementRecoveryBatchLimit   = 100
	defaultGrsaiSubmissionUnknownTimeout       = 10 * time.Minute
	defaultGrsaiSettlementRecoveryRetryLimit   = 5
)

var grsaiSettlementRecoveryBackoff = []time.Duration{
	time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	time.Hour,
	6 * time.Hour,
}

// GrsaiSettlementAccountReader deliberately depends on the one account lookup
// needed for a persisted upstream task. This keeps recovery tests and the
// worker's authority surface narrow.
type GrsaiSettlementAccountReader interface {
	GetByID(context.Context, int64) (*Account, error)
}

type GrsaiSettlementRecoveryRuntime struct {
	repo       GrsaiSettlementRepository
	accounts   GrsaiSettlementAccountReader
	upstream   GrsaiNativeClient
	settlement *GrsaiSettlementService
	opts       GrsaiSettlementRecoveryOptions
	now        func() time.Time

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

type GrsaiSettlementRecoveryOptions struct {
	Enabled                  bool
	ScanInterval             time.Duration
	BatchLimit               int
	SubmissionUnknownTimeout time.Duration
	SettlementRetryLimit     int
}

func NewGrsaiSettlementRecoveryOptionsFromConfig(cfg *config.Config) GrsaiSettlementRecoveryOptions {
	options := GrsaiSettlementRecoveryOptions{
		ScanInterval:             defaultGrsaiSettlementRecoveryScanInterval,
		BatchLimit:               defaultGrsaiSettlementRecoveryBatchLimit,
		SubmissionUnknownTimeout: defaultGrsaiSubmissionUnknownTimeout,
		SettlementRetryLimit:     defaultGrsaiSettlementRecoveryRetryLimit,
	}
	if cfg == nil {
		return options
	}
	options.Enabled = cfg.GrsaiSettlementRecovery.Enabled
	if cfg.GrsaiSettlementRecovery.ScanIntervalSeconds > 0 {
		options.ScanInterval = time.Duration(cfg.GrsaiSettlementRecovery.ScanIntervalSeconds) * time.Second
	}
	if cfg.GrsaiSettlementRecovery.BatchLimit > 0 {
		options.BatchLimit = cfg.GrsaiSettlementRecovery.BatchLimit
	}
	if cfg.GrsaiSettlementRecovery.SubmissionUnknownTimeoutSeconds > 0 {
		options.SubmissionUnknownTimeout = time.Duration(cfg.GrsaiSettlementRecovery.SubmissionUnknownTimeoutSeconds) * time.Second
	}
	if cfg.GrsaiSettlementRecovery.SettlementRetryLimit > 0 {
		options.SettlementRetryLimit = cfg.GrsaiSettlementRecovery.SettlementRetryLimit
	}
	return normalizeGrsaiSettlementRecoveryOptions(options)
}

func normalizeGrsaiSettlementRecoveryOptions(options GrsaiSettlementRecoveryOptions) GrsaiSettlementRecoveryOptions {
	if options.ScanInterval <= 0 {
		options.ScanInterval = defaultGrsaiSettlementRecoveryScanInterval
	}
	if options.BatchLimit <= 0 {
		options.BatchLimit = defaultGrsaiSettlementRecoveryBatchLimit
	}
	if options.BatchLimit > 1000 {
		options.BatchLimit = 1000
	}
	if options.SubmissionUnknownTimeout <= 0 {
		options.SubmissionUnknownTimeout = defaultGrsaiSubmissionUnknownTimeout
	}
	if options.SettlementRetryLimit <= 0 {
		options.SettlementRetryLimit = defaultGrsaiSettlementRecoveryRetryLimit
	}
	return options
}

func NewGrsaiSettlementRecoveryRuntime(
	repo GrsaiSettlementRepository,
	accounts GrsaiSettlementAccountReader,
	upstream GrsaiNativeClient,
	settlement *GrsaiSettlementService,
	cfg *config.Config,
) *GrsaiSettlementRecoveryRuntime {
	return &GrsaiSettlementRecoveryRuntime{
		repo:       repo,
		accounts:   accounts,
		upstream:   upstream,
		settlement: settlement,
		opts:       NewGrsaiSettlementRecoveryOptionsFromConfig(cfg),
		now:        time.Now,
	}
}

// Start is intentionally a no-op when recovery is disabled. Its state is
// entirely durable: every restart asks ClaimDue for next_attempt_at records.
func (r *GrsaiSettlementRecoveryRuntime) Start() {
	if r == nil || !r.opts.Enabled || r.repo == nil || r.accounts == nil || r.upstream == nil || r.settlement == nil {
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
		r.RunOnce(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.RunOnce(ctx)
			}
		}
	}(r.done)
}

func (r *GrsaiSettlementRecoveryRuntime) Stop() {
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

func (r *GrsaiSettlementRecoveryRuntime) Running() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cancel != nil
}

// RunOnce claims a bounded durable batch. A claim lease is the only ownership
// mechanism; no in-memory queue or upstream re-submission is involved.
func (r *GrsaiSettlementRecoveryRuntime) RunOnce(ctx context.Context) {
	if r == nil || r.repo == nil || r.settlement == nil {
		return
	}
	now := r.now()
	// A provider call can consume its full 30-minute timeout. Claim one record
	// per run so no later record in a sequential batch starts after its 31-minute
	// lease has already expired. Fencing prevents a second worker from replaying
	// the same task while this call owns the lease.
	claims, err := r.repo.ClaimDue(ctx, now, 1, now.Add(grsaiSettlementLease))
	if err != nil {
		if ctx.Err() == nil {
			logger.L().Warn("grsai settlement recovery claim failed", zap.Error(err))
		}
		return
	}
	for _, claim := range claims {
		if ctx.Err() != nil {
			return
		}
		r.recoverClaim(ctx, claim)
	}
}

func (r *GrsaiSettlementRecoveryRuntime) recoverClaim(ctx context.Context, claim *GrsaiSettlement) {
	if claim == nil {
		return
	}
	if claim.UpstreamStatus == GrsaiUpstreamStatusSucceeded || claim.InternalStatus == "pending_settlement" {
		r.recoverSettlement(ctx, claim)
		return
	}
	if claim.UpstreamTaskID == nil || *claim.UpstreamTaskID == "" {
		r.recoverUnknownSubmission(ctx, claim)
		return
	}
	account, err := r.accounts.GetByID(ctx, claim.AccountID)
	if err != nil || account == nil || account.Platform != PlatformGrsai {
		r.deferUpstream(ctx, claim, "account unavailable for result polling")
		return
	}
	result, resultErr := r.upstream.Result(ctx, account, *claim.UpstreamTaskID)
	nextAttempt := r.now().Add(r.opts.ScanInterval)
	if resultErr == nil && result != nil && result.Status == GrsaiUpstreamStatusSucceeded {
		nextAttempt = r.now().Add(r.settlementRetryDelay(claim.SettlementRetryCount + 1))
	}
	outcome := r.settlement.FinishAt(ctx, claim, result, resultErr, nextAttempt)
	if outcome.SettlementError != nil && !errors.Is(outcome.SettlementError, ErrGrsaiSettlementClaimLost) {
		logger.L().Warn("grsai settlement recovery result handling failed", zap.Int64("settlement_id", claim.ID), zap.Error(outcome.SettlementError))
	}
}

func (r *GrsaiSettlementRecoveryRuntime) recoverSettlement(ctx context.Context, claim *GrsaiSettlement) {
	if claim.SettlementRetryCount >= r.opts.SettlementRetryLimit {
		r.manualReview(ctx, claim, "settlement retry limit exceeded; no upstream request was retried")
		return
	}
	_, err := r.settlement.SettleAt(ctx, claim.ID, claim.ClaimVersion, r.now().Add(r.settlementRetryDelay(claim.SettlementRetryCount+1)))
	if err != nil && !errors.Is(err, ErrGrsaiSettlementClaimLost) {
		logger.L().Warn("grsai settlement recovery billing failed", zap.Int64("settlement_id", claim.ID), zap.Error(err))
	}
}

func (r *GrsaiSettlementRecoveryRuntime) recoverUnknownSubmission(ctx context.Context, claim *GrsaiSettlement) {
	now := r.now()
	deadline := claim.CreatedAt.Add(r.opts.SubmissionUnknownTimeout)
	if !claim.CreatedAt.IsZero() && !now.Before(deadline) {
		r.manualReview(ctx, claim, "submission outcome unknown without upstream task ID; upstream request was not resent")
		return
	}
	next := now.Add(r.opts.ScanInterval)
	if !claim.CreatedAt.IsZero() && deadline.Before(next) {
		next = deadline
	}
	if err := r.repo.MarkPendingUpstream(ctx, claim.ID, claim.ClaimVersion, next); err != nil && !errors.Is(err, ErrGrsaiSettlementClaimLost) {
		logger.L().Warn("grsai settlement recovery deferred unknown submission failed", zap.Int64("settlement_id", claim.ID), zap.Error(err))
	}
}

func (r *GrsaiSettlementRecoveryRuntime) deferUpstream(ctx context.Context, claim *GrsaiSettlement, summary string) {
	next := r.now().Add(r.opts.ScanInterval)
	if _, err := r.repo.UpdateResult(ctx, claim.ID, claim.ClaimVersion, "unknown", summary, next); err != nil {
		if !errors.Is(err, ErrGrsaiSettlementClaimLost) {
			logger.L().Warn("grsai settlement recovery could not record poll failure", zap.Int64("settlement_id", claim.ID), zap.Error(err))
		}
		return
	}
	if err := r.repo.MarkPendingUpstream(ctx, claim.ID, claim.ClaimVersion, next); err != nil && !errors.Is(err, ErrGrsaiSettlementClaimLost) {
		logger.L().Warn("grsai settlement recovery could not defer poll failure", zap.Int64("settlement_id", claim.ID), zap.Error(err))
	}
}

func (r *GrsaiSettlementRecoveryRuntime) manualReview(ctx context.Context, claim *GrsaiSettlement, summary string) {
	if err := r.repo.MarkManualReview(ctx, claim.ID, claim.ClaimVersion, summary); err != nil && !errors.Is(err, ErrGrsaiSettlementClaimLost) {
		logger.L().Error("grsai settlement requires manual review", zap.String("priority", "high"), zap.Int64("settlement_id", claim.ID), zap.Error(err))
		return
	}
	logger.L().Error("grsai settlement requires manual review", zap.String("priority", "high"), zap.Int64("settlement_id", claim.ID), zap.String("reason", summary))
}

func (r *GrsaiSettlementRecoveryRuntime) settlementRetryDelay(failureCount int) time.Duration {
	if failureCount <= 1 {
		return grsaiSettlementRecoveryBackoff[0]
	}
	index := failureCount - 1
	if index >= len(grsaiSettlementRecoveryBackoff) {
		return grsaiSettlementRecoveryBackoff[len(grsaiSettlementRecoveryBackoff)-1]
	}
	return grsaiSettlementRecoveryBackoff[index]
}
