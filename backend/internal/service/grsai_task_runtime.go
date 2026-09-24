package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

type GrsaiTaskRuntimeOptions struct {
	Enabled                  bool
	ScanInterval             time.Duration
	BatchLimit               int
	SubmissionUnknownTimeout time.Duration
	ClaimLease               time.Duration
	ResultRetention          time.Duration
	Now                      func() time.Time
}

// GrsaiTaskRuntime is the durable async worker. It claims with the repository
// fence and therefore remains safe when two process instances scan together.
type GrsaiTaskRuntime struct {
	Tasks    *GrsaiTaskService
	Repo     GrsaiSettlementRepository
	Upstream GrsaiNativeClient
	Options  GrsaiTaskRuntimeOptions

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func NewGrsaiTaskRuntime(tasks *GrsaiTaskService, repo GrsaiSettlementRepository, upstream GrsaiNativeClient, options GrsaiTaskRuntimeOptions) *GrsaiTaskRuntime {
	if repo == nil && tasks != nil {
		repo = tasks.Repo
	}
	return &GrsaiTaskRuntime{Tasks: tasks, Repo: repo, Upstream: upstream, Options: options}
}

func (r *GrsaiTaskRuntime) now() time.Time {
	if r != nil && r.Options.Now != nil {
		return r.Options.Now()
	}
	return time.Now()
}

func (r *GrsaiTaskRuntime) normalizeOptions() {
	if r.Options.ScanInterval <= 0 {
		r.Options.ScanInterval = time.Minute
	}
	if r.Options.BatchLimit <= 0 {
		r.Options.BatchLimit = 20
	}
	if r.Options.BatchLimit > 1000 {
		r.Options.BatchLimit = 1000
	}
	if r.Options.SubmissionUnknownTimeout <= 0 {
		r.Options.SubmissionUnknownTimeout = 10 * time.Minute
	}
	if r.Options.ClaimLease <= 0 {
		r.Options.ClaimLease = grsaiSettlementLease
	}
	if r.Options.ResultRetention <= 0 {
		r.Options.ResultRetention = defaultGrsaiTaskResultRetention
	}
}

func (r *GrsaiTaskRuntime) Start() {
	if r == nil || !r.Options.Enabled || r.Repo == nil || r.Tasks == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		return
	}
	r.normalizeOptions()
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.done = make(chan struct{})
	done := r.done
	go func() {
		defer close(done)
		ticker := time.NewTicker(r.Options.ScanInterval)
		defer ticker.Stop()
		r.runOnce(ctx, context.WithoutCancel(ctx))
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.runOnce(ctx, context.WithoutCancel(ctx))
			}
		}
	}()
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

func (r *GrsaiTaskRuntime) Running() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cancel != nil
}

func (r *GrsaiTaskRuntime) RunOnce(ctx context.Context) {
	r.runOnce(ctx, ctx)
}

func (r *GrsaiTaskRuntime) runOnce(ctx, claimContext context.Context) {
	if r == nil || r.Repo == nil || r.Tasks == nil {
		return
	}
	r.normalizeOptions()
	r.cleanup(ctx)
	for processed := 0; processed < r.Options.BatchLimit; processed++ {
		if ctx.Err() != nil {
			return
		}
		now := r.now()
		var claims []*GrsaiSettlement
		var err error
		if modeRepo, ok := r.Repo.(GrsaiDeliveryModeRepository); ok {
			// Claim exactly one record at a time. A task can spend longer than the
			// lease in the upstream stream, and pre-claiming later records would let
			// another process re-claim one before this worker reaches its first POST.
			claims, err = modeRepo.ClaimDueForDeliveryMode(ctx, now, 1, now.Add(r.Options.ClaimLease), GrsaiDeliveryAsync)
		} else {
			// Compatibility for in-memory/test repositories. Production SQL uses the
			// mode-fenced method above, so legacy rows are never leased by this worker.
			claims, err = r.Repo.ClaimDue(ctx, now, 1, now.Add(r.Options.ClaimLease))
		}
		if err != nil || len(claims) == 0 {
			return
		}
		if ctx.Err() != nil {
			return
		}
		claim := claims[0]
		// A shutdown stops future claims, but an already-started provider POST
		// must drain so cancellation cannot turn it into an unknown submission.
		r.runClaim(claimContext, claim)
	}
}

type grsaiTerminalCleanupRepository interface {
	DeleteTerminal(context.Context, time.Time, time.Duration, int) (int64, error)
}

func (r *GrsaiTaskRuntime) cleanup(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	now := r.now()
	if r.Tasks.Payloads != nil {
		if _, err := r.Tasks.Payloads.DeleteExpired(ctx, now); err != nil && ctx.Err() == nil {
			logger.L().Warn("grsai async payload cleanup failed", zap.Error(err))
		}
	}
	if repo, ok := r.Repo.(grsaiTerminalCleanupRepository); ok && ctx.Err() == nil {
		if _, err := repo.DeleteTerminal(ctx, now, r.Options.ResultRetention, r.Options.BatchLimit); err != nil && ctx.Err() == nil {
			logger.L().Warn("grsai terminal task cleanup failed", zap.Error(err))
		}
	}
}

func (r *GrsaiTaskRuntime) runClaim(ctx context.Context, claim *GrsaiSettlement) {
	if claim == nil {
		return
	}
	if claim.DeliveryMode != GrsaiDeliveryAsync {
		// The mode-fenced ClaimDueForDeliveryMode path should make this
		// unreachable in production. Never mutate a legacy JSON/stream record if
		// an older repository implementation returns one anyway.
		return
	}
	if (claim.InternalStatus == "pending_settlement" || claim.UpstreamStatus == GrsaiUpstreamStatusSucceeded) && !grsaiNeedsResultURLs(claim) {
		_, _ = r.Tasks.Settlement.SettleAt(ctx, claim.ID, claim.ClaimVersion, r.now().Add(time.Minute))
		return
	}
	if claim.UpstreamTaskID != nil && grsaiTaskTrim(*claim.UpstreamTaskID) != "" {
		r.pollBound(ctx, claim)
		return
	}
	if grsaiNeedsResultURLs(claim) {
		_ = r.Tasks.markManualReview(ctx, claim, "upstream success has no result URL or task ID")
		return
	}
	if claim.UpstreamStatus != "not_submitted" && claim.UpstreamStatus != "unknown" && claim.UpstreamStatus != "submitting" {
		return
	}
	if claim.UpstreamStatus == "unknown" || claim.UpstreamStatus == "submitting" {
		deadline := claim.CreatedAt.Add(r.Options.SubmissionUnknownTimeout)
		if !claim.CreatedAt.IsZero() && !r.now().Before(deadline) {
			_ = r.Tasks.markManualReview(ctx, claim, "submission outcome unknown without upstream task ID; upstream request was not resent")
			return
		}
		next := r.now().Add(r.Options.ScanInterval)
		if !claim.CreatedAt.IsZero() && deadline.Before(next) {
			next = deadline
		}
		_, _ = r.Repo.UpdateResult(ctx, claim.ID, claim.ClaimVersion, "unknown", "upstream outcome unknown; do not resubmit", next)
		_ = r.Repo.MarkPendingUpstream(ctx, claim.ID, claim.ClaimVersion, next)
		return
	}
	_, _ = r.Tasks.RunGrsaiTask(ctx, claim, nil)
}

func (r *GrsaiTaskRuntime) pollBound(ctx context.Context, claim *GrsaiSettlement) {
	if r.Upstream == nil || r.Tasks.Accounts == nil {
		return
	}
	account, err := r.Tasks.Accounts.GetByID(ctx, claim.AccountID)
	if err != nil || account == nil || account.Platform != PlatformGrsai {
		if grsaiMissingResultExpired(claim, r.now()) {
			_ = r.Tasks.markManualReview(ctx, claim, "upstream success remained without result URL beyond deadline")
			return
		}
		if !grsaiNeedsResultURLs(claim) {
			_, _ = r.Repo.UpdateResult(ctx, claim.ID, claim.ClaimVersion, "unknown", "account unavailable for result polling", r.now().Add(r.Options.ScanInterval))
		}
		_ = r.Repo.MarkPendingUpstream(ctx, claim.ID, claim.ClaimVersion, r.now().Add(r.Options.ScanInterval))
		return
	}
	result, resultErr := r.Upstream.Result(ctx, account, grsaiTaskTrim(*claim.UpstreamTaskID))
	next := r.now().Add(r.Options.ScanInterval)
	if resultErr == nil && result != nil && result.Status == GrsaiUpstreamStatusSucceeded {
		next = r.now().Add(time.Minute)
	}
	out := r.Tasks.Settlement.FinishAt(ctx, claim, result, resultErr, next)
	if out != nil && errors.Is(out.SettlementError, ErrGrsaiSettlementClaimLost) {
		return
	}
}

func grsaiTaskTrim(value string) string {
	for len(value) > 0 && (value[0] == ' ' || value[0] == '\t' || value[0] == '\n' || value[0] == '\r') {
		value = value[1:]
	}
	for len(value) > 0 && (value[len(value)-1] == ' ' || value[len(value)-1] == '\t' || value[len(value)-1] == '\n' || value[len(value)-1] == '\r') {
		value = value[:len(value)-1]
	}
	return value
}
