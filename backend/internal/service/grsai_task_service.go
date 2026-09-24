package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

const (
	defaultGrsaiTaskPayloadTTL      = 15 * time.Minute
	defaultGrsaiTaskResultRetention = 24 * time.Hour
	maxGrsaiPublicErrorSummary      = 256
)

// GrsaiTaskInput is the request-scoped input for an async task. Body and
// OriginalBody are aliases for callers that already parsed the delivery
// request; Request is accepted to avoid parsing the same object twice.
type GrsaiTaskInput struct {
	Account                  *Account
	APIKey                   *APIKey
	Body                     []byte
	OriginalBody             []byte
	Request                  *GrsaiDeliveryRequest
	EffectiveGroupMultiplier *float64
}

type GrsaiTaskOptions struct {
	Enabled         bool
	StreamEnabled   bool
	PayloadTTL      time.Duration
	ResultRetention time.Duration
	Now             func() time.Time
}

// GrsaiTaskService coordinates the durable settlement state machine and the
// encrypted async payload. It deliberately depends on the validated Task 1
// parser and Task 4 stream boundary rather than constructing provider JSON.
type GrsaiTaskService struct {
	Settlement *GrsaiSettlementService
	Repo       GrsaiSettlementRepository
	Payloads   GrsaiTaskPayloadRepository
	Accounts   GrsaiSettlementAccountReader
	Upstream   GrsaiStreamClient
	Options    GrsaiTaskOptions
}

func (s *GrsaiTaskService) now() time.Time {
	if s != nil && s.Options.Now != nil {
		return s.Options.Now()
	}
	return time.Now()
}

func (s *GrsaiTaskService) payloadTTL() time.Duration {
	if s == nil || s.Options.PayloadTTL <= 0 {
		return defaultGrsaiTaskPayloadTTL
	}
	return s.Options.PayloadTTL
}

func (s *GrsaiTaskService) resultRetention() time.Duration {
	if s == nil || s.Options.ResultRetention <= 0 {
		return defaultGrsaiTaskResultRetention
	}
	return s.Options.ResultRetention
}

// CreateGrsaiTask validates, snapshots pricing, freezes balance, and stores
// only the encrypted original request. The returned record is queued (its
// internal durable status is pending_upstream) and has a public UUID.
func (s *GrsaiTaskService) CreateGrsaiTask(ctx context.Context, input GrsaiTaskInput) (*GrsaiSettlement, error) {
	if s == nil || !s.Options.Enabled {
		return nil, ErrGrsaiAsyncDisabled
	}
	if s == nil || s.Settlement == nil || s.Repo == nil || s.Payloads == nil || input.Account == nil || input.APIKey == nil {
		return nil, ErrGrsaiSettlementInvalidInput
	}
	raw := input.Body
	if len(raw) == 0 {
		raw = input.OriginalBody
	}
	request := input.Request
	var err error
	if request == nil {
		request, err = ParseGrsaiDeliveryRequest(raw)
		if err != nil {
			return nil, err
		}
	}
	if request.Mode != GrsaiDeliveryAsync || len(request.OriginalBody) == 0 || len(request.UpstreamBody) == 0 {
		return nil, fmt.Errorf("%w: async delivery is required", ErrGrsaiInvalidRequest)
	}
	if input.Account.Platform != PlatformGrsai || input.APIKey.Group == nil {
		return nil, ErrGrsaiSettlementInvalidInput
	}
	now := s.now()
	payloadDeleteAfter := now.Add(s.payloadTTL())
	resultExpiresAt := now.Add(s.resultRetention())
	claim, err := s.Settlement.Prepare(ctx, GrsaiPrepareInput{
		Account: input.Account, APIKey: input.APIKey, Model: request.Model,
		ImageCount: request.ImageCount, ImageSize: request.ImageSize,
		EffectiveGroupMultiplier: input.EffectiveGroupMultiplier,
		DeliveryMode:             GrsaiDeliveryAsync, PayloadDeleteAfter: &payloadDeleteAfter, ExpiresAt: &resultExpiresAt,
	})
	if err != nil {
		return nil, err
	}
	claim.OriginalBody = append([]byte(nil), request.OriginalBody...)
	claim.UpstreamBody = append([]byte(nil), request.UpstreamBody...)
	if err := s.Payloads.PutEncrypted(ctx, claim.ID, request.OriginalBody, payloadDeleteAfter); err != nil {
		releaseErr := s.Settlement.markManualReviewWithRelease(context.WithoutCancel(ctx), claim.ID, claim.ClaimVersion, "async payload could not be durably stored")
		return nil, errors.Join(err, releaseErr)
	}
	// This update may have committed and been claimed by a worker before
	// an error is observed. The payload and hold already exist; returning
	// the task ID prevents the caller from creating a second one.
	// If it did not commit, the processing lease makes it recoverable.
	_ = s.Repo.MarkPendingUpstream(context.WithoutCancel(ctx), claim.ID, claim.ClaimVersion, now)
	// MarkPendingUpstream is the last required durable write. Reading again
	// through a canceled request could report failure for an accepted task.
	claim.InternalStatus = "pending_upstream"
	claim.NextAttemptAt = now
	claim.OriginalBody = nil
	claim.UpstreamBody = nil
	return claim, nil
}

// RunGrsaiTask consumes one active claim. It only submits claims without an
// upstream ID; once the first validated event binds an ID, all later recovery
// uses GET result polling and never POSTs again.
func (s *GrsaiTaskService) RunGrsaiTask(ctx context.Context, claim *GrsaiSettlement, onPersistedEvent func(GrsaiStreamEvent) error) (*GrsaiUpstreamResult, error) {
	if s == nil || s.Settlement == nil || s.Repo == nil || s.Upstream == nil || claim == nil || claim.ID <= 0 {
		return nil, ErrGrsaiSettlementInvalidInput
	}
	if claim.InternalStatus != "processing" || (claim.UpstreamTaskID != nil && strings.TrimSpace(*claim.UpstreamTaskID) != "") {
		return nil, ErrGrsaiSettlementInvalidState
	}
	if claim.UpstreamStatus != "not_submitted" {
		return nil, ErrGrsaiSettlementInvalidState
	}
	if claim.HoldAmount > 0 && claim.HoldState != "held" {
		return nil, s.markManualReview(ctx, claim, "async task has no durable balance hold")
	}
	body := append([]byte(nil), claim.UpstreamBody...)
	if len(body) == 0 {
		if s.Payloads == nil {
			return nil, s.markManualReview(ctx, claim, "async payload repository unavailable")
		}
		original, err := s.Payloads.GetEncrypted(ctx, claim.ID)
		if err != nil {
			if errors.Is(err, ErrGrsaiTaskPayloadNotFound) || errors.Is(err, ErrGrsaiTaskPayloadCorrupt) {
				return nil, errors.Join(err, s.markManualReview(ctx, claim, "async payload missing or corrupt"))
			}
			// A temporary storage failure is not evidence that the queued request
			// is gone. Preserve the ciphertext and the frozen balance for retry.
			next := s.now().Add(grsaiSettlementRetryDelay)
			pendingErr := s.Repo.MarkPendingUpstream(context.WithoutCancel(ctx), claim.ID, claim.ClaimVersion, next)
			return nil, errors.Join(err, pendingErr)
		}
		parsed, err := ParseGrsaiDeliveryRequest(original)
		if err != nil || parsed.Mode != GrsaiDeliveryAsync {
			return nil, s.markManualReview(ctx, claim, "async payload failed validation")
		}
		claim.OriginalBody = original
		body = parsed.UpstreamBody
		claim.UpstreamBody = append([]byte(nil), body...)
	} else {
		parsed, err := ParseGrsaiDeliveryRequest(body)
		if err != nil {
			return nil, s.markManualReview(ctx, claim, "task request failed validation")
		}
		body = parsed.UpstreamBody
	}
	if s.Accounts == nil {
		return nil, s.markManualReview(ctx, claim, "account unavailable for async task")
	}
	account, err := s.Accounts.GetByID(ctx, claim.AccountID)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			return nil, s.markManualReview(ctx, claim, "account unavailable for async task")
		}
		return nil, s.retryUnsubmitted(ctx, claim, err)
	}
	if account == nil || account.Platform != PlatformGrsai {
		return nil, s.markManualReview(ctx, claim, "account unavailable for async task")
	}
	// Persist a pre-bind fence before POST. If the process dies after the
	// provider accepts the request but before the first SSE event binds its ID,
	// recovery sees "submitting" and must never issue a second POST.
	marker, ok := s.Repo.(GrsaiSubmissionRepository)
	if !ok {
		return nil, s.markManualReview(ctx, claim, "submission fencing unavailable")
	}
	marked, err := marker.MarkSubmitting(context.WithoutCancel(ctx), claim.ID, claim.ClaimVersion)
	if err != nil {
		if errors.Is(err, ErrGrsaiSettlementClaimLost) {
			return nil, err
		}
		// No provider POST occurred. If the write committed despite the error,
		// undo it only under this same claim before retrying. If that write
		// fails, retain the fence and let recovery avoid a duplicate POST.
		if unsent, ok := s.Repo.(GrsaiUnsentSubmissionRepository); ok {
			resetErr := unsent.DeferUnsentSubmission(context.WithoutCancel(ctx), claim.ID, claim.ClaimVersion, s.now().Add(grsaiSettlementRetryDelay))
			return nil, errors.Join(err, resetErr)
		}
		return nil, s.retryUnsubmitted(ctx, claim, err)
	}
	if !marked {
		return nil, ErrGrsaiSettlementClaimLost
	}
	claim.UpstreamStatus = "submitting"
	stream, err := s.Upstream.OpenGenerateStream(ctx, account, body)
	if err != nil {
		// No provider ID was durably bound. The POST outcome is uncertain, so
		// never retry it and release via manual review.
		return nil, errors.Join(err, s.markManualReview(ctx, claim, "upstream submission outcome unknown before task ID binding"))
	}
	if stream == nil || stream.Body == nil {
		return nil, s.markManualReview(ctx, claim, "upstream stream unavailable")
	}
	defer func() { _ = stream.Body.Close() }()
	final, consumeErr := s.Settlement.ConsumeGrsaiSSE(ctx, claim, stream.Body, func(event GrsaiStreamEvent) error {
		// RecordStreamEvent has already durably bound the ID at this point.
		if claim.UpstreamTaskID != nil && strings.TrimSpace(*claim.UpstreamTaskID) != "" {
			_ = s.deletePayload(claim.ID)
		}
		if onPersistedEvent != nil {
			return onPersistedEvent(event)
		}
		return nil
	})
	if consumeErr != nil {
		if claim.UpstreamTaskID != nil && strings.TrimSpace(*claim.UpstreamTaskID) != "" {
			_ = s.deletePayload(claim.ID)
		} else if record, readErr := s.Repo.GetByID(context.WithoutCancel(ctx), claim.ID); readErr == nil && record != nil &&
			(record.InternalStatus == "manual_review" || (record.UpstreamTaskID != nil && strings.TrimSpace(*record.UpstreamTaskID) != "")) {
			_ = s.deletePayload(claim.ID)
		}
		return nil, consumeErr
	}
	_ = s.deletePayload(claim.ID)
	outcome := s.Settlement.Finish(context.WithoutCancel(ctx), claim, final, nil)
	if outcome == nil {
		return final, ErrGrsaiSettlementInvalidInput
	}
	return final, outcome.SettlementError
}

func (s *GrsaiTaskService) retryUnsubmitted(ctx context.Context, claim *GrsaiSettlement, cause error) error {
	next := s.now().Add(grsaiSettlementRetryDelay)
	err := s.Repo.MarkPendingUpstream(context.WithoutCancel(ctx), claim.ID, claim.ClaimVersion, next)
	return errors.Join(cause, err)
}

func (s *GrsaiTaskService) deletePayload(id int64) error {
	if s == nil || s.Payloads == nil || id <= 0 {
		return nil
	}
	return s.Payloads.DeleteBySettlementID(context.Background(), id)
}

func (s *GrsaiTaskService) markManualReview(ctx context.Context, claim *GrsaiSettlement, summary string) error {
	if claim == nil {
		return ErrGrsaiSettlementInvalidInput
	}
	if s.Settlement != nil {
		if err := s.Settlement.markManualReviewWithRelease(context.WithoutCancel(ctx), claim.ID, claim.ClaimVersion, summary); err == nil {
			_ = s.deletePayload(claim.ID)
			return ErrGrsaiSettlementInvalidState
		} else {
			// Never silently fall back to a status-only transition when a held
			// balance cannot be released. The caller must surface this failure.
			return errors.Join(ErrGrsaiSettlementInvalidState, err)
		}
	}
	return errors.Join(ErrGrsaiSettlementInvalidState, ErrGrsaiHoldReleaseUnavailable)
}

type GrsaiTaskView struct {
	ID             string    `json:"id"`
	UpstreamTaskID string    `json:"upstream_task_id,omitempty"`
	Status         string    `json:"status"`
	Progress       int       `json:"progress"`
	Model          string    `json:"model"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Results        []string  `json:"results,omitempty"`
	ErrorCode      string    `json:"error_code,omitempty"`
	ErrorSummary   string    `json:"error_summary,omitempty"`
}

func (s *GrsaiTaskService) GetPublicTaskView(ctx context.Context, userID, apiKeyID int64, publicID string) (*GrsaiTaskView, error) {
	if s == nil || s.Repo == nil || userID <= 0 || apiKeyID <= 0 || strings.TrimSpace(publicID) == "" {
		return nil, ErrGrsaiSettlementNotFound
	}
	record, err := s.Repo.GetOwnedByPublicOrUpstreamID(ctx, userID, apiKeyID, publicID)
	if err != nil {
		return nil, err
	}
	if (record.InternalStatus == "settled" || record.InternalStatus == "closed_no_charge") && record.ClosedAt != nil {
		retention := s.resultRetention()
		if record.ExpiresAt != nil && record.ExpiresAt.After(record.CreatedAt) {
			retention = record.ExpiresAt.Sub(record.CreatedAt)
		}
		if !s.now().Before(record.ClosedAt.Add(retention)) {
			return nil, ErrGrsaiSettlementNotFound
		}
	}
	return BuildGrsaiTaskView(record), nil
}

func BuildGrsaiTaskView(record *GrsaiSettlement) *GrsaiTaskView {
	if record == nil {
		return nil
	}
	view := &GrsaiTaskView{ID: record.PublicTaskID, Progress: record.Progress, Model: record.Model,
		CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt, Results: append([]string(nil), record.ResultURLs...)}
	if record.UpstreamTaskID != nil {
		view.UpstreamTaskID = strings.TrimSpace(*record.UpstreamTaskID)
	}
	switch {
	case record.InternalStatus == "settled":
		view.Status = "settled"
	case record.InternalStatus == "manual_review":
		view.Status, view.ErrorCode = "manual_review", "manual_review"
	case grsaiNeedsResultURLs(record):
		view.Status = "running"
	case record.InternalStatus == "pending_settlement":
		view.Status = "pending_settlement"
	case record.InternalStatus == "closed_no_charge":
		view.Status = "closed_no_charge"
		if strings.EqualFold(record.UpstreamStatus, GrsaiUpstreamStatusViolation) {
			view.ErrorCode = "policy_violation"
		} else {
			view.ErrorCode = "upstream_failed"
		}
	case record.UpstreamStatus == "submitting":
		view.Status = "submitting"
	case record.UpstreamStatus == "unknown":
		view.Status = "upstream_unknown"
	case view.UpstreamTaskID != "":
		view.Status = "running"
	case record.InternalStatus == "pending_upstream" || record.UpstreamStatus == "not_submitted":
		view.Status = "queued"
	default:
		view.Status = "running"
	}
	if record.LastErrorSummary != nil {
		view.ErrorSummary = sanitizeGrsaiPublicSummary(*record.LastErrorSummary)
	}
	if view.ErrorCode == "" && record.UpstreamStatus == GrsaiUpstreamStatusViolation {
		view.ErrorCode = "policy_violation"
	}
	return view
}

func sanitizeGrsaiPublicSummary(value string) string {
	var b strings.Builder
	for _, r := range value {
		if unicode.IsControl(r) {
			continue
		}
		_, _ = b.WriteRune(r)
		if b.Len() >= maxGrsaiPublicErrorSummary {
			break
		}
	}
	return strings.TrimSpace(b.String())
}
