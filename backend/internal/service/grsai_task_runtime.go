package service

import (
	"context"
	"errors"
	"sync"
	"time"
)

type GrsaiTaskRuntimeOptions struct {
	Enabled                  bool
	ScanInterval             time.Duration
	BatchLimit               int
	SubmissionUnknownTimeout time.Duration
	ClaimLease               time.Duration
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
		r.RunOnce(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.RunOnce(ctx)
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
	if r == nil || r.Repo == nil || r.Tasks == nil {
		return
	}
	r.normalizeOptions()
	now := r.now()
	claims, err := r.Repo.ClaimDue(ctx, now, r.Options.BatchLimit, now.Add(r.Options.ClaimLease))
	if err != nil {
		return
	}
	for _, claim := range claims {
		if ctx.Err() != nil {
			return
		}
		r.runClaim(ctx, claim)
	}
}

func (r *GrsaiTaskRuntime) runClaim(ctx context.Context, claim *GrsaiSettlement) {
	if claim == nil {
		return
	}
	if claim.DeliveryMode != GrsaiDeliveryAsync {
		// The legacy settlement recovery runtime owns JSON/stream records. Keep
		// their lease durable without ever treating them as async payloads.
		next := r.now().Add(r.Options.ScanInterval)
		_, _ = r.Repo.UpdateResult(ctx, claim.ID, claim.ClaimVersion, claim.UpstreamStatus, "", next)
		_ = r.Repo.MarkPendingUpstream(ctx, claim.ID, claim.ClaimVersion, next)
		return
	}
	if claim.InternalStatus == "pending_settlement" || claim.UpstreamStatus == GrsaiUpstreamStatusSucceeded {
		_, _ = r.Tasks.Settlement.SettleAt(ctx, claim.ID, claim.ClaimVersion, r.now().Add(time.Minute))
		return
	}
	if claim.UpstreamTaskID != nil && grsaiTaskTrim(*claim.UpstreamTaskID) != "" {
		r.pollBound(ctx, claim)
		return
	}
	if claim.UpstreamStatus != "not_submitted" && claim.UpstreamStatus != "unknown" {
		return
	}
	if claim.UpstreamStatus == "unknown" {
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
		_, _ = r.Repo.UpdateResult(ctx, claim.ID, claim.ClaimVersion, "unknown", "account unavailable for result polling", r.now().Add(r.Options.ScanInterval))
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
