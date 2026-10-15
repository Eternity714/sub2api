package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var (
	ErrUnifiedMediaTaskNotFound        = infraerrors.NotFound("MEDIA_TASK_NOT_FOUND", "media task not found")
	ErrUnifiedMediaClaimLost           = errors.New("media task claim lost")
	ErrUnifiedMediaAccountUnavailable  = errors.New("media account or shared quota unavailable")
	ErrUnifiedMediaInsufficientBalance = infraerrors.BadRequest("INSUFFICIENT_BALANCE", "insufficient available balance for media reservation")
	ErrUnifiedMediaKeyQuota            = infraerrors.New(429, "MEDIA_KEY_QUOTA_EXCEEDED", "API Key quota or rate limit cannot cover this media reservation")
)

type UnifiedMediaUpstreamResult struct {
	Disposition string
	TaskID      string
	Status      string
	URLs        []string
	Error       string
}
type UnifiedMediaUpstream interface {
	Submit(context.Context, *Account, string, string, *mediacapability.NormalizedRequest, int) (*UnifiedMediaUpstreamResult, error)
	Poll(context.Context, *Account, string, string, string) (*UnifiedMediaUpstreamResult, error)
}
type UnifiedMediaStoredResult struct {
	URL            string          `json:"url"`
	ExpiresAt      *time.Time      `json:"expires_at,omitempty"`
	ObjectMetadata json.RawMessage `json:"object_metadata,omitempty"`
}
type UnifiedMediaResultPersister interface {
	Persist(context.Context, string, string, int, int, string) (*UnifiedMediaStoredResult, error)
}
type UnifiedMediaCandidateSnapshot struct {
	Platform         string                  `json:"platform"`
	Model            string                  `json:"model"`
	RouteID          int64                   `json:"route_id"`
	Capability       MediaCapabilitySnapshot `json:"capability"`
	NativeMaxResults int                     `json:"native_max_results"`
	AllowSplit       bool                    `json:"allow_split"`
}
type UnifiedMediaTaskPayload struct {
	Normalized *mediacapability.NormalizedRequest `json:"normalized"`
	Capability MediaCapabilitySnapshot            `json:"capability"`
	Candidates []UnifiedMediaCandidateSnapshot    `json:"candidates"`
}
type UnifiedMediaOutput struct {
	State         string                    `json:"state"`
	Failures      int                       `json:"failures"`
	NextAttemptAt time.Time                 `json:"next_attempt_at,omitempty"`
	Stored        *UnifiedMediaStoredResult `json:"stored,omitempty"`
}

type UnifiedMediaUnit struct {
	State                 string                     `json:"state"`
	Count                 int                        `json:"count"`
	AccountID             int64                      `json:"account_id,omitempty"`
	AccountRateMultiplier *float64                   `json:"account_rate_multiplier,omitempty"`
	Platform              string                     `json:"platform,omitempty"`
	UpstreamModel         string                     `json:"upstream_model,omitempty"`
	UpstreamTaskID        string                     `json:"upstream_task_id,omitempty"`
	Attempt               int                        `json:"attempt"`
	TriedCandidates       []string                   `json:"tried_candidates,omitempty"`
	URLs                  []string                   `json:"urls,omitempty"`
	Stored                []UnifiedMediaStoredResult `json:"stored,omitempty"`
	Outputs               []UnifiedMediaOutput       `json:"outputs,omitempty"`
	Failures              int                        `json:"failures"`
	NextAttemptAt         time.Time                  `json:"next_attempt_at,omitempty"`
	Error                 string                     `json:"error,omitempty"`
}
type UnifiedMediaTask struct {
	ID               string
	UserID           int64
	APIKeyID         int64
	GroupID          int64
	Model            string
	MediaType        string
	Status           string
	RequestedCount   int
	UnitPrice        float64
	AccountUnitPrice float64
	HeldAmount       float64
	ChargedAmount    float64
	ClaimVersion     int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	CompletedAt      *time.Time
	ExpiresAt        *time.Time
	Deadline         time.Time
	Units            []UnifiedMediaUnit
	Payload          UnifiedMediaTaskPayload
	PayloadError     string
	WaitingLimit     int
}
type UnifiedMediaTaskView struct {
	ID          string                 `json:"id"`
	Model       string                 `json:"model"`
	Status      string                 `json:"status"`
	Progress    int                    `json:"progress"`
	CreatedAt   int64                  `json:"created_at"`
	CompletedAt *int64                 `json:"completed_at,omitempty"`
	ExpiresAt   *int64                 `json:"expires_at,omitempty"`
	Result      UnifiedMediaTaskResult `json:"result"`
	Error       string                 `json:"error"`
}
type UnifiedMediaTaskResult struct {
	Type string   `json:"type"`
	Data []string `json:"data"`
}

func (t *UnifiedMediaTask) PublicView() *UnifiedMediaTaskView {
	out := &UnifiedMediaTaskView{ID: t.ID, Model: t.Model, Status: t.Status, CreatedAt: t.CreatedAt.Unix(), Result: UnifiedMediaTaskResult{Type: t.MediaType, Data: []string{}}}
	switch t.Status {
	case "in_progress":
		out.Progress = 50
	case "completed":
		out.Progress = 100
		if t.CompletedAt != nil {
			v := t.CompletedAt.Unix()
			out.CompletedAt = &v
		}
		var summaries []string
		for _, unit := range t.Units {
			for _, stored := range unit.Stored {
				if stored.URL != "" {
					out.Result.Data = append(out.Result.Data, stored.URL)
				}
			}
			if unit.Error != "" {
				summaries = append(summaries, unit.Error)
			}
		}
		if len(out.Result.Data) > 0 && t.ExpiresAt != nil {
			v := t.ExpiresAt.Unix()
			out.ExpiresAt = &v
		}
		out.Error = strings.Join(summaries, "; ")
	}
	return out
}

type UnifiedMediaTaskRepository interface {
	Create(context.Context, *UnifiedMediaTask) error
	GetOwned(context.Context, int64, int64, string, string) (*UnifiedMediaTask, error)
	ClaimDue(context.Context, time.Time, int, time.Time) ([]*UnifiedMediaTask, error)
	Save(context.Context, *UnifiedMediaTask, time.Time) error
	ReleaseClaim(context.Context, *UnifiedMediaTask, time.Time) error
	ReserveAccount(context.Context, *UnifiedMediaTask, int, *Account, float64) error
	SettleAccount(context.Context, *UnifiedMediaTask, int, bool) error
	Finalize(context.Context, *UnifiedMediaTask) error
}
type UnifiedMediaTaskCreateInput struct {
	APIKey                   *APIKey
	MediaType                string
	Body                     []byte
	EffectiveGroupMultiplier *float64
}
type UnifiedMediaTaskService struct {
	Repo             UnifiedMediaTaskRepository
	Capabilities     *MediaCapabilityService
	Routes           *CompositeMediaRouteService
	Accounts         AccountRepository
	Pricing          GrsaiPricingResolver
	Upstream         UnifiedMediaUpstream
	Persister        UnifiedMediaResultPersister
	PollInterval     time.Duration
	MaxFailures      int
	MaxWaiting       int
	ResultTTL        time.Duration
	Now              func() time.Time
	AfterReservation func(context.Context, int64)
	AfterSettlement  func(context.Context, int64)
	mu               sync.Mutex
	cancel           context.CancelFunc
	done             chan struct{}
	wake             chan struct{}
}

func NewUnifiedMediaTaskService(repo UnifiedMediaTaskRepository, caps *MediaCapabilityService, routes *CompositeMediaRouteService, accounts AccountRepository, pricing GrsaiPricingResolver, upstream UnifiedMediaUpstream, persister UnifiedMediaResultPersister) *UnifiedMediaTaskService {
	return &UnifiedMediaTaskService{Repo: repo, Capabilities: caps, Routes: routes, Accounts: accounts, Pricing: pricing, Upstream: upstream, Persister: persister, PollInterval: 5 * time.Second, MaxFailures: 5, MaxWaiting: 20, ResultTTL: 24 * time.Hour, Now: time.Now, wake: make(chan struct{}, 1)}
}
func (s *UnifiedMediaTaskService) Create(ctx context.Context, input UnifiedMediaTaskCreateInput) (*UnifiedMediaTask, error) {
	key := input.APIKey
	if key == nil || key.Group == nil || key.GroupID == nil || key.ID <= 0 || key.UserID <= 0 || key.Group.Platform != PlatformComposite || key.Group.ID != *key.GroupID {
		return nil, infraerrors.BadRequest("INVALID_MEDIA_GROUP", "media generation requires a Composite group")
	}
	if key.Group.SubscriptionType == SubscriptionTypeSubscription {
		return nil, infraerrors.BadRequest("UNSUPPORTED_MEDIA_BILLING", "unified media generation requires balance billing")
	}
	if ready, ok := s.Persister.(interface{ Ready() bool }); ok && !ready.Ready() {
		return nil, infraerrors.New(503, "MEDIA_STORAGE_UNAVAILABLE", "media S3 storage is unavailable")
	}
	if s.Repo == nil || s.Capabilities == nil || s.Routes == nil || s.Pricing == nil || s.Persister == nil {
		return nil, infraerrors.New(503, "MEDIA_UNAVAILABLE", "media generation dependencies are unavailable")
	}
	var envelope struct {
		Model string `json:"model"`
	}
	if len(input.Body) > mediacapability.MaxRequestBytes || json.Unmarshal(input.Body, &envelope) != nil {
		return nil, infraerrors.BadRequest("INVALID_MEDIA_REQUEST", "request must be a valid media JSON object")
	}
	validated, err := s.Capabilities.ValidateActive(ctx, envelope.Model, input.MediaType, input.Body)
	if err != nil {
		return nil, err
	}
	candidates, err := s.Routes.MediaCandidates(ctx, key.Group.ID, envelope.Model, input.MediaType, validated.Normalized)
	if err != nil {
		return nil, err
	}
	normalized := validated.Normalized
	resolution := ""
	if metadata, ok := normalized.Values["metadata"].(map[string]any); ok {
		resolution, _ = metadata["resolution"].(string)
	}
	price, err := s.Pricing.ResolveGrsaiTaskPrice(ctx, envelope.Model, key.Group, resolution)
	if err != nil {
		return nil, infraerrors.BadRequest("MEDIA_PRICING_MISSING", "explicit model and output-specification pricing is required")
	}
	if (input.MediaType == "video" && price.Mode != BillingModeVideo && price.Mode != BillingModePerRequest) || (input.MediaType == "image" && price.Mode != BillingModeImage && price.Mode != BillingModePerRequest) {
		return nil, infraerrors.BadRequest("MEDIA_PRICING_INVALID", "billing mode does not match media type")
	}
	base := decimal.NewFromFloat(price.UnitPrice)
	if input.MediaType == "video" && price.Mode == BillingModeVideo {
		duration, ok := unifiedMediaNumber(normalized.Values["duration"])
		if !ok || duration <= 0 {
			return nil, infraerrors.BadRequest("MEDIA_DURATION_REQUIRED", "this video price requires an explicit positive duration")
		}
		base = base.Mul(decimal.NewFromFloat(duration))
	}
	rate := key.Group.RateMultiplier
	if input.EffectiveGroupMultiplier != nil {
		rate = *input.EffectiveGroupMultiplier
	}
	if input.MediaType == "video" {
		rate = resolveVideoRateMultiplier(key, rate)
	} else {
		rate = resolveImageRateMultiplier(key, rate)
	}
	if !grsaiFiniteNonNegative(price.UnitPrice) || !grsaiFiniteNonNegative(rate) {
		return nil, infraerrors.BadRequest("MEDIA_PRICING_INVALID", "media pricing is invalid")
	}
	unit, _ := base.Mul(decimal.NewFromFloat(rate)).Round(8).Float64()
	accountUnit, _ := base.Round(8).Float64()
	count := normalized.RequestedCount
	if count < 1 {
		count = 1
	}
	if count > 64 {
		return nil, infraerrors.BadRequest("MEDIA_COUNT_LIMIT", "media result count exceeds the service limit of 64")
	}
	held, _ := decimal.NewFromFloat(unit).Mul(decimal.NewFromInt(int64(count))).Round(8).Float64()
	now := s.Now().UTC()
	task := &UnifiedMediaTask{ID: "media_" + uuid.NewString(), UserID: key.UserID, APIKeyID: key.ID, GroupID: key.Group.ID, Model: envelope.Model, MediaType: input.MediaType, Status: "queued", RequestedCount: count, UnitPrice: unit, AccountUnitPrice: accountUnit, HeldAmount: held, CreatedAt: now, UpdatedAt: now, Deadline: now.Add(24 * time.Hour), Payload: UnifiedMediaTaskPayload{Normalized: normalized, Capability: validated.Capability}}
	for _, candidate := range candidates {
		task.Payload.Candidates = append(task.Payload.Candidates, UnifiedMediaCandidateSnapshot{Platform: candidate.Route.TargetPlatform, Model: candidate.Route.UpstreamModel, RouteID: candidate.Route.Route.ID, Capability: candidate.Capability, NativeMaxResults: candidate.Match.NativeBatchSize, AllowSplit: candidate.Match.NeedsSplit})
	}
	var publicSchema mediacapability.Schema
	_ = json.Unmarshal(validated.Capability.Schema, &publicSchema)
	if publicSchema.Batch.SemanticallyLinked || !publicSchema.Batch.AllowSplit && count > 1 {
		task.Units = []UnifiedMediaUnit{{State: "queued", Count: count}}
	} else {
		for i := 0; i < count; i++ {
			task.Units = append(task.Units, UnifiedMediaUnit{State: "queued", Count: 1})
		}
	}
	task.WaitingLimit = s.MaxWaiting
	if err = s.Repo.Create(ctx, task); err != nil {
		return nil, err
	}
	if s.AfterReservation != nil {
		s.AfterReservation(ctx, task.UserID)
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return task, nil
}
func unifiedMediaNumber(value any) (float64, bool) {
	switch value := value.(type) {
	case json.Number:
		n, err := value.Float64()
		return n, err == nil
	case float64:
		return value, true
	case int:
		return float64(value), true
	}
	return 0, false
}
func (s *UnifiedMediaTaskService) GetOwned(ctx context.Context, userID, keyID int64, mediaType, id string) (*UnifiedMediaTask, error) {
	task, err := s.Repo.GetOwned(ctx, userID, keyID, mediaType, id)
	if err != nil {
		return nil, err
	}
	if task.ExpiresAt != nil && !s.Now().Before(*task.ExpiresAt) {
		return nil, infraerrors.New(410, "MEDIA_RESULT_EXPIRED", "media result has expired")
	}
	return task, nil
}
func (s *UnifiedMediaTaskService) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.done = make(chan struct{})
	done := s.done
	go func() {
		defer close(done)
		ticker := time.NewTicker(s.interval())
		defer ticker.Stop()
		for {
			_ = s.RunOnce(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			case <-s.wake:
			}
		}
	}()
}
func (s *UnifiedMediaTaskService) Stop() {
	s.mu.Lock()
	cancel, done := s.cancel, s.done
	s.cancel = nil
	s.done = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}
func (s *UnifiedMediaTaskService) interval() time.Duration {
	if s.PollInterval <= 0 {
		return 5 * time.Second
	}
	return s.PollInterval
}
func (s *UnifiedMediaTaskService) RunOnce(ctx context.Context) error {
	if s.Repo == nil || s.Upstream == nil || s.Persister == nil || s.Accounts == nil {
		return errors.New("unified media worker is not configured")
	}
	now := s.Now()
	tasks, err := s.Repo.ClaimDue(ctx, now, 4, now.Add(30*time.Minute))
	if err != nil {
		return err
	}
	var joined error
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, task := range tasks {
		wg.Add(1)
		go func(task *UnifiedMediaTask) {
			defer wg.Done()
			callCtx, cancel := context.WithTimeout(ctx, 25*time.Minute)
			defer cancel()
			if err := s.process(callCtx, task); err != nil {
				mu.Lock()
				joined = errors.Join(joined, err)
				mu.Unlock()
			}
		}(task)
	}
	wg.Wait()
	return joined
}
func (s *UnifiedMediaTaskService) process(ctx context.Context, task *UnifiedMediaTask) error {
	for index := range task.Units {
		unit := &task.Units[index]
		if unit.State == "completed" || unit.State == "failed" {
			continue
		}
		if !task.Deadline.IsZero() && !s.Now().Before(task.Deadline) {
			unit.State = "failed"
			unit.Error = "media_task_expired"
			if unit.AccountID > 0 {
				if err := s.Repo.SettleAccount(ctx, task, index, true); err != nil {
					return err
				}
			}
			continue
		}
		if unit.State == "submitting" {
			unit.State = "failed"
			unit.Error = "upstream_submission_outcome_unknown"
			if err := s.Repo.SettleAccount(ctx, task, index, true); err != nil {
				return err
			}
			continue
		}
		if !unit.NextAttemptAt.IsZero() && s.Now().Before(unit.NextAttemptAt) {
			continue
		}
		if unit.State == "queued" && task.PayloadError != "" {
			unit.State = "failed"
			unit.Error = "media_payload_unavailable"
			continue
		}
		if unit.State == "queued" {
			if err := s.submit(ctx, task, index); err != nil {
				return err
			}
		}
		if unit.State == "polling" {
			account, err := s.Accounts.GetByID(ctx, unit.AccountID)
			if err != nil {
				unit.Error = "upstream_account_unavailable"
				s.retry(unit)
				continue
			}
			result, err := s.Upstream.Poll(ctx, account, task.MediaType, unit.UpstreamModel, unit.UpstreamTaskID)
			if err != nil || result == nil {
				s.retry(unit)
				continue
			}
			if result.Status == "completed" {
				if err := s.acceptResult(ctx, task, index, result); err != nil {
					return err
				}
			} else {
				unit.Failures = 0
				unit.NextAttemptAt = s.Now().Add(s.interval())
			}
		}
		if unit.State == "persisting" {
			if err := s.persist(ctx, task, index); err != nil {
				return err
			}
		}
	}
	complete := true
	next := s.Now().Add(s.interval())
	for index := range task.Units {
		unit := &task.Units[index]
		if unit.State != "completed" && unit.State != "failed" {
			complete = false
			if !unit.NextAttemptAt.IsZero() && unit.NextAttemptAt.Before(next) {
				next = unit.NextAttemptAt
			}
		}
	}
	if complete {
		if err := s.Repo.Finalize(ctx, task); err != nil {
			return err
		}
		if s.AfterSettlement != nil {
			s.AfterSettlement(ctx, task.UserID)
		}
		return nil
	}
	task.Status = "in_progress"
	if err := s.Repo.Save(ctx, task, next); err != nil {
		return err
	}
	return s.Repo.ReleaseClaim(ctx, task, next)
}
func (s *UnifiedMediaTaskService) retry(unit *UnifiedMediaUnit) {
	unit.Failures++
	limit := s.MaxFailures
	if limit <= 0 {
		limit = 5
	}
	if unit.Failures >= limit {
		unit.State = "failed"
		if unit.Error == "" {
			unit.Error = "media_result_retries_exhausted"
		}
		return
	}
	delay := 15 * time.Second * time.Duration(1<<min(unit.Failures-1, 5))
	unit.NextAttemptAt = s.Now().Add(delay)
}
func (s *UnifiedMediaTaskService) submit(ctx context.Context, task *UnifiedMediaTask, index int) error {
	unit := &task.Units[index]
	accounts, err := s.Accounts.ListSchedulableByGroupID(ctx, task.GroupID)
	if err != nil {
		return err
	}
	sort.SliceStable(accounts, func(i, j int) bool {
		if accounts[i].Priority != accounts[j].Priority {
			return accounts[i].Priority < accounts[j].Priority
		}
		return accounts[i].ID < accounts[j].ID
	})
	for _, candidate := range task.Payload.Candidates {
		if candidate.NativeMaxResults > 0 && unit.Count > candidate.NativeMaxResults {
			continue
		}
		for _, account := range accounts {
			if account.Platform != candidate.Platform || !account.IsSchedulable() || !account.IsModelSupported(candidate.Model) {
				continue
			}
			candidateKey := fmt.Sprintf("%d:%s:%s:%d", candidate.RouteID, candidate.Platform, candidate.Model, account.ID)
			tried := false
			for _, key := range unit.TriedCandidates {
				if key == candidateKey {
					tried = true
					break
				}
			}
			if tried {
				continue
			}
			candidateRequest, requestErr := unifiedMediaCandidateRequest(task.Payload.Normalized, candidate)
			if requestErr != nil {
				continue
			}
			accountRate := account.BillingRateMultiplier()
			if !grsaiFiniteNonNegative(accountRate) {
				continue
			}
			unit.AccountID = account.ID
			unit.AccountRateMultiplier = &accountRate
			unit.Platform = account.Platform
			unit.UpstreamModel = candidate.Model
			unit.Attempt++
			unit.State = "submitting"
			unit.TriedCandidates = append(unit.TriedCandidates, candidateKey)
			amount, _ := decimal.NewFromFloat(task.AccountUnitPrice).Mul(decimal.NewFromFloat(accountRate)).Mul(decimal.NewFromInt(int64(unit.Count))).Round(8).Float64()
			if err := s.Repo.ReserveAccount(ctx, task, index, &account, amount); err != nil {
				unit.State = "queued"
				if errors.Is(err, ErrUnifiedMediaAccountUnavailable) {
					continue
				}
				return err
			}
			result, callErr := s.Upstream.Submit(ctx, &account, task.MediaType, candidate.Model, candidateRequest, unit.Count)
			if result != nil && result.Disposition == "rejected" {
				if err := s.Repo.SettleAccount(ctx, task, index, false); err != nil {
					return err
				}
				unit.State = "queued"
				unit.Error = "upstream_submission_rejected"
				if err := s.Repo.Save(ctx, task, s.Now()); err != nil {
					return err
				}
				continue
			}
			if callErr != nil || result == nil || result.Disposition != "accepted" {
				unit.State = "failed"
				unit.Error = "upstream_submission_outcome_unknown"
				return s.Repo.SettleAccount(ctx, task, index, true)
			}
			unit.UpstreamTaskID = result.TaskID
			unit.Error = ""
			if result.Status == "completed" {
				return s.acceptResult(ctx, task, index, result)
			}
			if result.TaskID == "" {
				unit.State = "failed"
				unit.Error = "upstream_submission_outcome_unknown"
				return s.Repo.SettleAccount(ctx, task, index, true)
			}
			unit.State = "polling"
			unit.NextAttemptAt = s.Now().Add(s.interval())
			return s.Repo.Save(ctx, task, s.Now().Add(s.interval()))
		}
	}
	unit.State = "failed"
	unit.Error = "no_available_media_account"
	return nil
}
func (s *UnifiedMediaTaskService) acceptResult(ctx context.Context, task *UnifiedMediaTask, index int, result *UnifiedMediaUpstreamResult) error {
	unit := &task.Units[index]
	if err := s.Repo.SettleAccount(ctx, task, index, true); err != nil {
		return err
	}
	unit.Failures = 0
	unit.NextAttemptAt = time.Time{}
	unit.URLs = nil
	for _, url := range result.URLs {
		if strings.TrimSpace(url) != "" {
			unit.URLs = append(unit.URLs, url)
		}
	}
	if len(unit.URLs) > unit.Count {
		unit.State = "failed"
		unit.Error = "unexpected_upstream_result_count"
		return s.Repo.Save(ctx, task, s.Now())
	}
	if len(unit.URLs) == 0 {
		unit.State = "failed"
		unit.Error = "upstream_generation_failed"
	} else {
		unit.State = "persisting"
		if len(unit.URLs) < unit.Count || result.Error != "" {
			unit.Error = "some_media_results_failed"
		}
	}
	return s.Repo.Save(ctx, task, s.Now())
}
func (s *UnifiedMediaTaskService) persist(ctx context.Context, task *UnifiedMediaTask, index int) error {
	unit := &task.Units[index]
	if len(unit.Outputs) == 0 {
		unit.Outputs = make([]UnifiedMediaOutput, len(unit.URLs))
		// Older snapshots stored only the successfully persisted prefix.
		for resultIndex := range unit.Stored {
			if resultIndex >= len(unit.Outputs) {
				break
			}
			stored := unit.Stored[resultIndex]
			unit.Outputs[resultIndex] = UnifiedMediaOutput{State: "completed", Stored: &stored}
		}
	}
	for resultIndex := range unit.Outputs {
		output := &unit.Outputs[resultIndex]
		if output.State == "completed" || output.State == "failed" || (!output.NextAttemptAt.IsZero() && s.Now().Before(output.NextAttemptAt)) {
			continue
		}
		stored, err := s.Persister.Persist(ctx, task.ID, task.MediaType, index, resultIndex, unit.URLs[resultIndex])
		if err != nil || stored == nil || stored.URL == "" {
			output.Failures++
			limit := s.MaxFailures
			if limit <= 0 {
				limit = 5
			}
			if output.Failures >= limit {
				output.State = "failed"
				output.NextAttemptAt = time.Time{}
			} else {
				output.NextAttemptAt = s.Now().Add(15 * time.Second * time.Duration(1<<min(output.Failures-1, 5)))
			}
			unit.Error = "media_result_storage_failed"
		} else {
			if stored.ExpiresAt == nil {
				ttl := s.ResultTTL
				if ttl <= 0 {
					ttl = 24 * time.Hour
				}
				expiry := s.Now().Add(ttl)
				stored.ExpiresAt = &expiry
			}
			output.State = "completed"
			output.Stored = stored
			output.NextAttemptAt = time.Time{}
		}
		unit.Stored = nil
		for _, result := range unit.Outputs {
			if result.State == "completed" && result.Stored != nil {
				unit.Stored = append(unit.Stored, *result.Stored)
			}
		}
		if err := s.Repo.Save(ctx, task, s.Now()); err != nil {
			return err
		}
	}
	pending, failed := false, false
	unit.NextAttemptAt = time.Time{}
	for _, output := range unit.Outputs {
		switch output.State {
		case "completed":
		case "failed":
			failed = true
		default:
			pending = true
			if unit.NextAttemptAt.IsZero() || output.NextAttemptAt.Before(unit.NextAttemptAt) {
				unit.NextAttemptAt = output.NextAttemptAt
			}
		}
	}
	if pending {
		unit.State = "persisting"
	} else if failed {
		unit.State = "failed"
		unit.Error = "media_result_storage_failed"
	} else {
		unit.State = "completed"
		if len(unit.Stored) == unit.Count {
			unit.Error = ""
		}
	}
	return nil
}

func UnifiedMediaTaskSuccessCount(task *UnifiedMediaTask) int {
	count := 0
	for _, unit := range task.Units {
		count += len(unit.Stored)
	}
	return min(count, task.RequestedCount)
}
func UnifiedMediaTaskCharge(task *UnifiedMediaTask) float64 {
	amount, _ := decimal.NewFromFloat(task.UnitPrice).Mul(decimal.NewFromInt(int64(UnifiedMediaTaskSuccessCount(task)))).Round(8).Float64()
	return amount
}
func UnifiedMediaTaskResultExpiry(task *UnifiedMediaTask) *time.Time {
	var earliest *time.Time
	for _, unit := range task.Units {
		for _, stored := range unit.Stored {
			if stored.ExpiresAt != nil && (earliest == nil || stored.ExpiresAt.Before(*earliest)) {
				cp := *stored.ExpiresAt
				earliest = &cp
			}
		}
	}
	return earliest
}
func unifiedMediaSafeError(err error) error {
	return fmt.Errorf("unified media task persistence: %w", err)
}

func unifiedMediaCandidateRequest(original *mediacapability.NormalizedRequest, candidate UnifiedMediaCandidateSnapshot) (*mediacapability.NormalizedRequest, error) {
	if original == nil {
		return nil, errors.New("media normalized payload is unavailable")
	}
	compiled, err := mediacapability.Compile(candidate.Capability.MediaType, candidate.Capability.Schema)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(original.Values)
	if err != nil {
		return nil, err
	}
	validated, fields := compiled.Validate(raw)
	if len(fields) > 0 {
		return nil, errors.New("captured candidate parameters are invalid")
	}
	copy := *original
	copy.ImageSpec = validated.ImageSpec
	return &copy, nil
}
