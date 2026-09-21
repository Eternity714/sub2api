package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type GrsaiSettlementState string

const (
	GrsaiStateSubmissionPending GrsaiSettlementState = "submission_pending"
	GrsaiStateAwaitingResult    GrsaiSettlementState = "awaiting_result"
	GrsaiStateSettlementPending GrsaiSettlementState = "settlement_pending"
	GrsaiStateSettled           GrsaiSettlementState = "settled"
	GrsaiStateClosedNoCharge    GrsaiSettlementState = "closed_no_charge"
	GrsaiStateUpstreamUnknown   GrsaiSettlementState = "upstream_unknown"
	GrsaiStateManualReview      GrsaiSettlementState = "manual_review"

	grsaiSettlementRetryDelay = time.Minute
	grsaiSettlementLease      = grsaiRequestTimeout + time.Minute
)

var (
	ErrGrsaiSettlementNotFound       = errors.New("grsai settlement not found")
	ErrGrsaiSettlementInvalidInput   = errors.New("invalid grsai settlement input")
	ErrGrsaiSettlementInvalidState   = errors.New("invalid grsai settlement state")
	ErrGrsaiSettlementClaimLost      = errors.New("grsai settlement claim lost")
	ErrGrsaiSettlementPricingMissing = errors.New("grsai requires explicit flat image or per-request model pricing")
	ErrGrsaiTaskPayloadNotFound      = errors.New("grsai task payload not found")
)

// These contracts live in service so the SQL repository can implement them
// without introducing a service -> repository -> service import cycle.
type CreateGrsaiSettlementParams struct {
	AccountID             int64
	GroupID               int64
	UserID                int64
	APIKeyID              int64
	Model                 string
	BaseUnitPrice         float64
	GroupRateMultiplier   float64
	AccountRateMultiplier float64
	BillableUnitPrice     float64
	RequestedImageCount   int
	ImageSize             string
	Currency              string
	BillingIdempotencyKey string
	PublicTaskID          string
	DeliveryMode          GrsaiDeliveryMode
	Progress              int
	ResultURLs            []string
	HoldAmount            float64
	HoldState             string
	PayloadDeleteAfter    *time.Time
	ExpiresAt             *time.Time
	UpstreamTaskID        *string
	UpstreamStatus        string
	NextAttemptAt         time.Time
}

type GrsaiSettlement struct {
	ID                    int64
	AccountID             int64
	GroupID               int64
	UserID                int64
	APIKeyID              int64
	Model                 string
	BaseUnitPrice         float64
	GroupRateMultiplier   float64
	AccountRateMultiplier float64
	BillableUnitPrice     float64
	RequestedImageCount   int
	ImageSize             string
	Currency              string
	BillingIdempotencyKey string
	PublicTaskID          string
	DeliveryMode          GrsaiDeliveryMode
	Progress              int
	ResultURLs            []string
	HoldAmount            float64
	HoldState             string
	PayloadDeleteAfter    *time.Time
	ExpiresAt             *time.Time
	UpstreamTaskID        *string
	UpstreamStatus        string
	InternalStatus        string
	// RetryCount records worker claims, while SettlementRetryCount only records
	// failed billing attempts. They must remain independent: a long-running
	// upstream task can be polled many times before its first settlement retry.
	RetryCount           int
	SettlementRetryCount int
	ClaimVersion         int64
	NextAttemptAt        time.Time
	LastErrorSummary     *string
	SettledAmount        *float64
	CreatedAt            time.Time
	UpdatedAt            time.Time
	UpstreamBoundAt      *time.Time
	ResultUpdatedAt      *time.Time
	SettledAt            *time.Time
	ClosedAt             *time.Time
}

// State projects the business state from the existing durable status pair.
// "processing" is a leased ownership marker, never a business outcome.
func (r *GrsaiSettlement) State() GrsaiSettlementState {
	if r == nil {
		return GrsaiStateUpstreamUnknown
	}
	switch r.InternalStatus {
	case "settled":
		return GrsaiStateSettled
	case "closed_no_charge":
		return GrsaiStateClosedNoCharge
	case "manual_review":
		return GrsaiStateManualReview
	case "pending_settlement":
		return GrsaiStateSettlementPending
	}
	switch r.UpstreamStatus {
	case "not_submitted":
		return GrsaiStateSubmissionPending
	case GrsaiUpstreamStatusSucceeded:
		return GrsaiStateSettlementPending
	case GrsaiUpstreamStatusRunning, "queued":
		return GrsaiStateAwaitingResult
	default:
		return GrsaiStateUpstreamUnknown
	}
}

// The callback must use tx for every billing side effect, without committing it.
type GrsaiSettlementTxFunc func(context.Context, *sql.Tx, *GrsaiSettlement) error

type GrsaiSettlementRepository interface {
	Create(context.Context, CreateGrsaiSettlementParams) (*GrsaiSettlement, error)
	GetByID(context.Context, int64) (*GrsaiSettlement, error)
	GetOwnedByPublicOrUpstreamID(context.Context, int64, int64, string) (*GrsaiSettlement, error)
	ClaimByID(context.Context, int64, time.Time, time.Time) (*GrsaiSettlement, error)
	ClaimDue(context.Context, time.Time, int, time.Time) ([]*GrsaiSettlement, error)
	BindUpstreamTask(context.Context, int64, int64, string, string) (bool, error)
	UpdateResult(context.Context, int64, int64, string, string, time.Time) (bool, error)
	MarkPendingSettlement(context.Context, int64, int64, time.Time) error
	MarkPendingUpstream(context.Context, int64, int64, time.Time) error
	Settle(context.Context, int64, int64, float64, GrsaiSettlementTxFunc) (bool, error)
	CloseNoCharge(context.Context, int64, int64, string) error
	MarkManualReview(context.Context, int64, int64, string) error
}

// GrsaiTaskPayloadRepository stores the original async request only in
// encrypted form and removes it independently from the public task record.
type GrsaiTaskPayloadRepository interface {
	PutEncrypted(context.Context, int64, []byte, time.Time) error
	GetEncrypted(context.Context, int64) ([]byte, error)
	DeleteBySettlementID(context.Context, int64) error
	DeleteExpired(context.Context, time.Time) (int64, error)
}

// UsageBillingTransactionalRepository is an optional extension implemented by
// the existing billing repository. Ordinary Apply callers keep their API.
type UsageBillingTransactionalRepository interface {
	ApplyTx(context.Context, *sql.Tx, *UsageBillingCommand) (*UsageBillingApplyResult, error)
}

type GrsaiPricingResolver interface {
	GrsaiUnitPrice(context.Context, string, *Group) (float64, error)
}

type GrsaiModelPricingResolver struct{ Resolver *ModelPricingResolver }

func (r *GrsaiModelPricingResolver) GrsaiUnitPrice(ctx context.Context, model string, group *Group) (float64, error) {
	if r == nil || r.Resolver == nil || group == nil {
		return 0, ErrGrsaiSettlementPricingMissing
	}
	resolved := r.Resolver.Resolve(ctx, PricingInput{Model: model, GroupID: &group.ID, Group: group})
	if resolved == nil || (resolved.Source != PricingSourceGroup && resolved.Source != PricingSourceChannel) ||
		(resolved.Mode != BillingModeImage && resolved.Mode != BillingModePerRequest) {
		return 0, ErrGrsaiSettlementPricingMissing
	}
	// Native model-specific options are opaque. Reject tiered prices instead of
	// guessing a size/quality tier or treating a token price as an image price.
	if len(resolved.RequestTiers) > 0 || resolved.channelPricing == nil || resolved.channelPricing.PerRequestPrice == nil {
		return 0, ErrGrsaiSettlementPricingMissing
	}
	if !grsaiFiniteNonNegative(resolved.DefaultPerRequestPrice) {
		return 0, ErrGrsaiSettlementPricingMissing
	}
	return resolved.DefaultPerRequestPrice, nil
}

type GrsaiSettlementService struct {
	Repo         GrsaiSettlementRepository
	Billing      UsageBillingTransactionalRepository
	HoldBilling  GrsaiHoldBillingRepository
	Pricing      GrsaiPricingResolver
	UsageLogRepo UsageLogRepository
	AuthCache    APIKeyAuthCacheInvalidator
}

type GrsaiPrepareInput struct {
	Account    *Account
	APIKey     *APIKey
	Model      string
	ImageCount int
	ImageSize  string
	// Pass the existing user/group rate resolver's result when it overrides the
	// group default. Independent image rates still take precedence.
	EffectiveGroupMultiplier *float64
}

// Prepare persists and claims the immutable snapshot. The caller may send
// Generate only after this succeeds, and must never resend it on Finish errors.
func (s *GrsaiSettlementService) Prepare(ctx context.Context, input GrsaiPrepareInput) (*GrsaiSettlement, error) {
	if s == nil || s.Repo == nil || s.Pricing == nil || s.Billing == nil || input.Account == nil || input.APIKey == nil || input.APIKey.Group == nil {
		return nil, ErrGrsaiSettlementInvalidInput
	}
	group := input.APIKey.Group
	if input.Account.Platform != PlatformGrsai || input.Account.Type != AccountTypeAPIKey || group.Platform != PlatformGrsai ||
		input.Account.ID <= 0 || input.APIKey.ID <= 0 || input.APIKey.UserID <= 0 || group.ID <= 0 ||
		input.APIKey.GroupID == nil || *input.APIKey.GroupID != group.ID || input.ImageCount <= 0 ||
		strings.TrimSpace(input.Model) == "" || group.SubscriptionType == SubscriptionTypeSubscription {
		return nil, ErrGrsaiSettlementInvalidInput
	}
	base, err := s.Pricing.GrsaiUnitPrice(ctx, strings.TrimSpace(input.Model), group)
	if err != nil {
		return nil, err
	}
	groupRate := group.RateMultiplier
	if input.EffectiveGroupMultiplier != nil {
		groupRate = *input.EffectiveGroupMultiplier
	}
	groupRate = resolveImageRateMultiplier(input.APIKey, groupRate)
	accountRate := input.Account.BillingRateMultiplier()
	if !grsaiFiniteNonNegative(base) || !grsaiFiniteNonNegative(groupRate) || !grsaiFiniteNonNegative(accountRate) {
		return nil, ErrGrsaiSettlementPricingMissing
	}
	// Match the persisted DECIMAL scales before deriving the billable price.
	base, _ = decimal.NewFromFloat(base).Round(10).Float64()
	groupRate, _ = decimal.NewFromFloat(groupRate).Round(4).Float64()
	accountRate, _ = decimal.NewFromFloat(accountRate).Round(4).Float64()
	billable, _ := decimal.NewFromFloat(base).Mul(decimal.NewFromFloat(groupRate)).Mul(decimal.NewFromFloat(accountRate)).Round(10).Float64()
	if !grsaiFiniteNonNegative(billable) {
		return nil, ErrGrsaiSettlementPricingMissing
	}
	now := time.Now()
	record, err := s.Repo.Create(ctx, CreateGrsaiSettlementParams{
		AccountID: input.Account.ID, GroupID: group.ID, UserID: input.APIKey.UserID, APIKeyID: input.APIKey.ID,
		Model: strings.TrimSpace(input.Model), BaseUnitPrice: base, GroupRateMultiplier: groupRate,
		AccountRateMultiplier: accountRate, BillableUnitPrice: billable, RequestedImageCount: input.ImageCount,
		HoldAmount: billable * float64(input.ImageCount), HoldState: "none",
		ImageSize: NormalizeImageBillingTierOrDefault(input.ImageSize), Currency: "USD", UpstreamStatus: "not_submitted",
		// Keep the scanner out of the create -> initial claim window.
		NextAttemptAt: now.Add(grsaiSettlementLease),
	})
	if err != nil {
		return nil, err
	}
	if err := s.reserveGrsaiBalance(ctx, record); err != nil {
		if errors.Is(err, ErrGrsaiInsufficientBalance) {
			_ = s.Repo.CloseNoCharge(ctx, record.ID, 0, "insufficient balance")
		}
		return nil, err
	}
	if hs, ok := s.Repo.(GrsaiHoldStateRepository); ok {
		if err := hs.MarkHoldHeld(ctx, record.ID, 0); err != nil {
			return nil, err
		}
	}
	return s.Repo.ClaimByID(ctx, record.ID, now, now.Add(grsaiSettlementLease))
}

type GrsaiSettlementOutcome struct {
	Upstream        *GrsaiUpstreamResult
	UpstreamError   error
	SettlementError error
	State           GrsaiSettlementState
}

// Finish keeps protocol errors separate from local accounting errors. In
// particular SettlementError must never replace a successful upstream body.
// A disconnected client must not cancel recording the already obtained result.
func (s *GrsaiSettlementService) Finish(ctx context.Context, claim *GrsaiSettlement, upstream *GrsaiUpstreamResult, upstreamErr error) *GrsaiSettlementOutcome {
	return s.FinishAt(ctx, claim, upstream, upstreamErr, time.Now().Add(grsaiSettlementRetryDelay))
}

// FinishAt records an upstream outcome with a caller-supplied durable retry
// time. The request path uses Finish's one-minute default; recovery workers
// use this variant so restarts only depend on next_attempt_at.
func (s *GrsaiSettlementService) FinishAt(ctx context.Context, claim *GrsaiSettlement, upstream *GrsaiUpstreamResult, upstreamErr error, retryAt time.Time) *GrsaiSettlementOutcome {
	out := &GrsaiSettlementOutcome{Upstream: upstream, UpstreamError: upstreamErr, State: GrsaiStateUpstreamUnknown}
	if s == nil || s.Repo == nil || claim == nil || retryAt.IsZero() {
		out.SettlementError = ErrGrsaiSettlementInvalidInput
		return out
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	record, err := s.Repo.GetByID(ctx, claim.ID)
	if err != nil {
		out.SettlementError = err
		return out
	}
	out.State = record.State()
	if record.InternalStatus == "settled" || record.InternalStatus == "closed_no_charge" || record.InternalStatus == "manual_review" {
		return out
	}
	if record.ClaimVersion != claim.ClaimVersion || record.InternalStatus != "processing" {
		out.SettlementError = ErrGrsaiSettlementClaimLost
		return out
	}
	if record.UpstreamStatus == GrsaiUpstreamStatusSucceeded {
		_, out.SettlementError = s.SettleAt(ctx, record.ID, record.ClaimVersion, retryAt)
	} else {
		out.SettlementError = s.recordResult(ctx, record, upstream, upstreamErr, retryAt)
	}
	if updated, readErr := s.Repo.GetByID(ctx, record.ID); readErr == nil {
		out.State = updated.State()
	}
	return out
}

func (s *GrsaiSettlementService) recordResult(ctx context.Context, record *GrsaiSettlement, upstream *GrsaiUpstreamResult, upstreamErr error, retryAt time.Time) error {
	// Once a task ID is durable, a result query is never an initial submission.
	// Retry temporary query failures rather than converting a potentially
	// successful upstream task into a free terminal record. Initial Generate
	// HTTP failures deliberately retain the no-charge behavior below.
	if record.UpstreamTaskID != nil && *record.UpstreamTaskID != "" {
		if grsaiResultPollRetryable(upstream, upstreamErr) {
			return s.deferResultPoll(ctx, record, grsaiResultPollFailureSummary(upstream, upstreamErr), retryAt)
		}
		if grsaiUpstreamHTTPFailed(upstream, upstreamErr) {
			return s.markManualReviewWithRelease(ctx, record.ID, record.ClaimVersion, "upstream result polling failed permanently; do not resubmit")
		}
	}
	// A received HTTP failure is a final failed submission, even if an error
	// payload happens to include a task-like identifier. Never poll or bind it.
	if grsaiUpstreamHTTPFailed(upstream, upstreamErr) {
		return s.closeNoChargeWithRelease(ctx, record.ID, record.ClaimVersion, "upstream HTTP request failed; no charge")
	}
	status := "unknown"
	if upstream != nil && upstreamErr == nil && (upstream.HTTPStatus == 0 || (upstream.HTTPStatus >= 200 && upstream.HTTPStatus < 300)) {
		switch upstream.Status {
		case GrsaiUpstreamStatusSucceeded, GrsaiUpstreamStatusFailed, GrsaiUpstreamStatusViolation, GrsaiUpstreamStatusRunning:
			status = upstream.Status
		}
	}
	if upstream != nil && strings.TrimSpace(upstream.TaskID) != "" {
		taskID := strings.TrimSpace(upstream.TaskID)
		if record.UpstreamTaskID != nil && *record.UpstreamTaskID != taskID {
			return s.markManualReviewWithRelease(ctx, record.ID, record.ClaimVersion, "upstream task identity conflict")
		}
		if record.UpstreamTaskID == nil {
			if _, err := s.Repo.BindUpstreamTask(ctx, record.ID, record.ClaimVersion, taskID, status); err != nil {
				return err
			}
			record.UpstreamTaskID = &taskID
		}
	}
	if status == GrsaiUpstreamStatusRunning && record.UpstreamTaskID == nil {
		status = "unknown"
	}
	next := retryAt
	// Persist only fixed summaries; upstream errors can contain prompts or keys.
	summary := ""
	if status == "unknown" {
		summary = "upstream outcome unknown; do not resubmit"
	}
	if _, err := s.Repo.UpdateResult(ctx, record.ID, record.ClaimVersion, status, summary, next); err != nil {
		return err
	}
	switch status {
	case GrsaiUpstreamStatusSucceeded:
		_, err := s.SettleAt(ctx, record.ID, record.ClaimVersion, retryAt)
		return err
	case GrsaiUpstreamStatusFailed, GrsaiUpstreamStatusViolation:
		return s.closeNoChargeWithRelease(ctx, record.ID, record.ClaimVersion, "upstream "+status)
	default:
		return s.Repo.MarkPendingUpstream(ctx, record.ID, record.ClaimVersion, next)
	}
}

func (s *GrsaiSettlementService) closeNoChargeWithRelease(ctx context.Context, id, claimVersion int64, summary string) error {
	if ext, ok := s.Repo.(GrsaiHoldStateRepository); ok {
		return ext.CloseNoChargeWithRelease(ctx, id, claimVersion, summary, func(txCtx context.Context, tx *sql.Tx, record *GrsaiSettlement) error {
			return s.releaseGrsaiBalanceTx(txCtx, tx, record)
		})
	}
	return s.Repo.CloseNoCharge(ctx, id, claimVersion, summary)
}

func (s *GrsaiSettlementService) markManualReviewWithRelease(ctx context.Context, id, claimVersion int64, summary string) error {
	if ext, ok := s.Repo.(GrsaiHoldStateRepository); ok {
		return ext.MarkManualReviewWithRelease(ctx, id, claimVersion, summary, func(txCtx context.Context, tx *sql.Tx, record *GrsaiSettlement) error {
			return s.releaseGrsaiBalanceTx(txCtx, tx, record)
		})
	}
	return s.Repo.MarkManualReview(ctx, id, claimVersion, summary)
}

func (s *GrsaiSettlementService) deferResultPoll(ctx context.Context, record *GrsaiSettlement, summary string, retryAt time.Time) error {
	if _, err := s.Repo.UpdateResult(ctx, record.ID, record.ClaimVersion, "unknown", summary, retryAt); err != nil {
		return err
	}
	return s.Repo.MarkPendingUpstream(ctx, record.ID, record.ClaimVersion, retryAt)
}

func grsaiResultPollRetryable(upstream *GrsaiUpstreamResult, upstreamErr error) bool {
	if errors.Is(upstreamErr, ErrGrsaiInvalidResponse) {
		return true
	}
	statusCode := 0
	if upstream != nil {
		statusCode = upstream.HTTPStatus
	}
	var httpErr *GrsaiHTTPError
	if errors.As(upstreamErr, &httpErr) && httpErr != nil && statusCode == 0 {
		statusCode = httpErr.StatusCode
	}
	if statusCode == 429 || statusCode >= 500 {
		return true
	}
	// A transport error does not establish a terminal provider outcome. HTTP
	// errors handled above are the only errors with a known response class.
	return upstreamErr != nil && !errors.As(upstreamErr, &httpErr)
}

func grsaiResultPollFailureSummary(upstream *GrsaiUpstreamResult, upstreamErr error) string {
	statusCode := 0
	if upstream != nil {
		statusCode = upstream.HTTPStatus
	}
	var httpErr *GrsaiHTTPError
	if errors.As(upstreamErr, &httpErr) && httpErr != nil && statusCode == 0 {
		statusCode = httpErr.StatusCode
	}
	switch {
	case statusCode == 429:
		return "upstream result polling temporarily rate limited (HTTP 429)"
	case statusCode >= 500:
		return fmt.Sprintf("upstream result polling temporarily failed (HTTP %d)", statusCode)
	case errors.Is(upstreamErr, ErrGrsaiInvalidResponse):
		return "upstream result polling returned an invalid response"
	default:
		return "upstream result polling transport failure"
	}
}

func grsaiUpstreamHTTPFailed(upstream *GrsaiUpstreamResult, upstreamErr error) bool {
	if upstream != nil && upstream.HTTPStatus >= 400 && upstream.HTTPStatus < 600 {
		return true
	}
	var httpErr *GrsaiHTTPError
	return errors.As(upstreamErr, &httpErr)
}

// Settle operates on an existing claim (from Prepare or ClaimDue). It never
// reads live pricing, reissues Generate, or owns a second balance transaction.
func (s *GrsaiSettlementService) Settle(ctx context.Context, id, claimVersion int64) (bool, error) {
	return s.SettleAt(ctx, id, claimVersion, time.Now().Add(grsaiSettlementRetryDelay))
}

// SettleAt is the recovery-worker variant of Settle. It keeps the same
// transaction and fencing protocol while allowing the durable worker to store
// its retry schedule rather than relying on an in-memory timer.
func (s *GrsaiSettlementService) SettleAt(ctx context.Context, id, claimVersion int64, retryAt time.Time) (bool, error) {
	if s == nil || s.Repo == nil || s.Billing == nil {
		return false, ErrGrsaiSettlementInvalidInput
	}
	if retryAt.IsZero() {
		return false, ErrGrsaiSettlementInvalidInput
	}
	record, err := s.Repo.GetByID(ctx, id)
	if err != nil {
		return false, err
	}
	if record.ClaimVersion != claimVersion {
		return false, ErrGrsaiSettlementClaimLost
	}
	if record.InternalStatus == "settled" {
		return false, nil
	}
	if record.InternalStatus != "processing" || record.UpstreamStatus != GrsaiUpstreamStatusSucceeded {
		return false, ErrGrsaiSettlementInvalidState
	}
	cmd, err := grsaiBillingCommand(record)
	if err != nil {
		return false, err
	}
	applied, err := s.Repo.Settle(ctx, id, claimVersion, cmd.BalanceCost, func(txCtx context.Context, tx *sql.Tx, locked *GrsaiSettlement) error {
		if locked.UpstreamStatus != GrsaiUpstreamStatusSucceeded {
			return ErrGrsaiSettlementInvalidState
		}
		lockedCommand, buildErr := grsaiBillingCommand(locked)
		if buildErr != nil {
			return buildErr
		}
		if lockedCommand.RequestFingerprint != cmd.RequestFingerprint {
			return ErrUsageBillingRequestConflict
		}
		if err := s.captureGrsaiBalanceTx(txCtx, tx, locked); err != nil {
			return err
		}
		_, applyErr := s.Billing.ApplyTx(txCtx, tx, lockedCommand)
		return applyErr
	})
	if err != nil {
		if !errors.Is(err, ErrGrsaiSettlementClaimLost) {
			if retryErr := s.Repo.MarkPendingSettlement(ctx, id, claimVersion, retryAt); retryErr != nil {
				return false, errors.Join(err, retryErr)
			}
		}
		return false, err
	}
	if applied {
		if s.AuthCache != nil {
			s.AuthCache.InvalidateAuthCacheByUserID(ctx, record.UserID)
		}
		s.recordUsage(ctx, record, cmd)
	}
	return applied, nil
}

func grsaiBillingCommand(record *GrsaiSettlement) (*UsageBillingCommand, error) {
	if record == nil || record.ID <= 0 || record.RequestedImageCount <= 0 || record.Currency != "USD" ||
		!grsaiFiniteNonNegative(record.BillableUnitPrice) || !grsaiFiniteNonNegative(record.BaseUnitPrice) ||
		!grsaiFiniteNonNegative(record.AccountRateMultiplier) || record.BillingIdempotencyKey == "" {
		return nil, ErrGrsaiSettlementInvalidInput
	}
	count := decimal.NewFromInt(int64(record.RequestedImageCount))
	amount, _ := decimal.NewFromFloat(record.BillableUnitPrice).Mul(count).Float64()
	accountCost, _ := decimal.NewFromFloat(record.BaseUnitPrice).Mul(count).Mul(decimal.NewFromFloat(record.AccountRateMultiplier)).Float64()
	if !grsaiFiniteNonNegative(amount) || !grsaiFiniteNonNegative(accountCost) {
		return nil, ErrGrsaiSettlementInvalidInput
	}
	cmd := &UsageBillingCommand{RequestID: record.BillingIdempotencyKey, APIKeyID: record.APIKeyID,
		UserID: record.UserID, AccountID: record.AccountID, AccountType: AccountTypeAPIKey, Model: record.Model,
		BillingType: BillingTypeBalance, ImageCount: record.RequestedImageCount, MediaType: "image",
		BalanceCost: amount, APIKeyQuotaCost: amount, APIKeyRateLimitCost: amount, AccountQuotaCost: accountCost}
	cmd.Normalize()
	return cmd, nil
}

func (s *GrsaiSettlementService) recordUsage(ctx context.Context, record *GrsaiSettlement, cmd *UsageBillingCommand) {
	if s.UsageLogRepo == nil {
		return
	}
	mode, endpoint := string(BillingModeImage), "/v1/api/generate"
	baseCost := record.BaseUnitPrice * float64(record.RequestedImageCount)
	usage := &UsageLog{UserID: record.UserID, APIKeyID: record.APIKeyID, AccountID: record.AccountID,
		GroupID: &record.GroupID, RequestID: cmd.RequestID, Model: record.Model, RequestedModel: record.Model,
		ImageCount: record.RequestedImageCount, ImageSize: &record.ImageSize, ImageOutputCost: baseCost, TotalCost: baseCost, ActualCost: cmd.BalanceCost,
		RateMultiplier: record.GroupRateMultiplier, AccountRateMultiplier: &record.AccountRateMultiplier,
		BillingType: BillingTypeBalance, RequestType: RequestTypeSync, BillingMode: &mode,
		InboundEndpoint: &endpoint, UpstreamEndpoint: &endpoint, CreatedAt: time.Now()}
	writeUsageLogBestEffort(ctx, s.UsageLogRepo, usage, "service.grsai_settlement")
}

func GrsaiSettlementRequestID(id int64) string { return fmt.Sprintf("grsai_settlement:%d", id) }

func grsaiFiniteNonNegative(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
