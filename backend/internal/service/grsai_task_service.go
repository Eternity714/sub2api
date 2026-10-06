package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

var ErrGrsaiDeliveryDisabled = errors.New("durable media delivery is disabled")

type GrsaiTaskCreateInput struct {
	Account                  *Account
	APIKey                   *APIKey
	Body                     []byte
	EffectiveGroupMultiplier *float64
}

type GrsaiTaskService struct {
	Repo         GrsaiV2TaskRepository
	Balance      GrsaiBalanceHoldRepository
	Pricing      GrsaiPricingResolver
	Enabled      bool
	PayloadTTL   time.Duration
	MaxWaiting   int
	StorageReady func() bool
}

func (s *GrsaiTaskService) Create(ctx context.Context, input GrsaiTaskCreateInput) (*GrsaiSettlement, error) {
	if s == nil || !s.Enabled {
		return nil, ErrGrsaiDeliveryDisabled
	}
	if s.StorageReady != nil && !s.StorageReady() {
		return nil, ErrImageTaskUnavailable
	}
	if s.Repo == nil || s.Balance == nil || s.Pricing == nil || input.Account == nil ||
		input.APIKey == nil || input.APIKey.Group == nil {
		return nil, ErrGrsaiSettlementInvalidInput
	}
	request, err := ParseGrsaiDeliveryRequest(input.Body)
	if err != nil {
		return nil, err
	}
	group := input.APIKey.Group
	if !IsNativeMediaAccount(input.Account) || group.Platform != PlatformGrsai ||
		group.SubscriptionType == SubscriptionTypeSubscription ||
		input.Account.ID <= 0 || input.APIKey.ID <= 0 || input.APIKey.UserID <= 0 ||
		input.APIKey.GroupID == nil || *input.APIKey.GroupID != group.ID || request.Model == "" {
		return nil, ErrGrsaiSettlementInvalidInput
	}
	price, err := s.Pricing.ResolveGrsaiTaskPrice(ctx, request.Model, group, request.Resolution)
	if err != nil {
		return nil, err
	}
	if price.Mode != BillingModeVideo && price.Mode != BillingModeImage && price.Mode != BillingModePerRequest {
		return nil, ErrGrsaiSettlementPricingMissing
	}
	if price.Mode == BillingModeVideo {
		if request.VideoFieldsError != nil {
			return nil, request.VideoFieldsError
		}
		if err := ValidateGrsaiVideoRequest(request.Model, request.Resolution, request.DurationSeconds); err != nil {
			return nil, err
		}
	}
	base := price.UnitPrice
	groupRate := group.RateMultiplier
	if input.EffectiveGroupMultiplier != nil {
		groupRate = *input.EffectiveGroupMultiplier
	}
	if price.Mode == BillingModeVideo {
		groupRate = resolveVideoRateMultiplier(input.APIKey, groupRate)
	} else {
		groupRate = resolveImageRateMultiplier(input.APIKey, groupRate)
	}
	accountRate := input.Account.BillingRateMultiplier()
	if !grsaiFiniteNonNegative(base) || !grsaiFiniteNonNegative(groupRate) || !grsaiFiniteNonNegative(accountRate) {
		return nil, ErrGrsaiSettlementPricingMissing
	}
	base, _ = decimal.NewFromFloat(base).Round(10).Float64()
	groupRate, _ = decimal.NewFromFloat(groupRate).Round(4).Float64()
	accountRate, _ = decimal.NewFromFloat(accountRate).Round(4).Float64()
	billableDecimal := decimal.NewFromFloat(base).Mul(decimal.NewFromFloat(groupRate)).Mul(decimal.NewFromFloat(accountRate))
	if price.Mode == BillingModeVideo {
		billableDecimal = billableDecimal.Mul(decimal.NewFromInt(int64(request.DurationSeconds)))
	}
	billable, _ := billableDecimal.Round(10).Float64()
	if !grsaiFiniteNonNegative(billable) {
		return nil, ErrGrsaiSettlementPricingMissing
	}
	ttl := s.PayloadTTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	mediaKind, taskVersion, requestedImageCount := "image", 2, request.ImageCount
	videoDuration, videoResolution := 0, ""
	if price.Mode == BillingModeVideo {
		mediaKind, taskVersion, requestedImageCount = "video", 3, 1
		videoDuration, videoResolution = request.DurationSeconds, price.Resolution
	}
	now := time.Now().UTC()
	return s.Repo.CreateV2GrsaiTask(ctx, CreateV2GrsaiTaskParams{
		CreateGrsaiSettlementParams: CreateGrsaiSettlementParams{
			AccountID: input.Account.ID, GroupID: group.ID, UserID: input.APIKey.UserID, APIKeyID: input.APIKey.ID,
			Model: strings.TrimSpace(request.Model), BaseUnitPrice: base, GroupRateMultiplier: groupRate,
			AccountRateMultiplier: accountRate, BillableUnitPrice: billable,
			RequestedImageCount: requestedImageCount, ImageSize: request.ImageSize, Currency: "USD",
			MediaKind: mediaKind, TaskVersion: taskVersion, VideoDurationSeconds: videoDuration, VideoResolution: videoResolution,
			LocalTaskID: NewGrsaiLocalTaskID(), DeliveryMode: string(request.Mode), NextAttemptAt: now,
		},
		PayloadExpiresAt: now.Add(ttl), UpstreamPayload: request.UpstreamBody, MaxWaiting: s.MaxWaiting,
	}, s.Balance.ReserveGrsaiBalance)
}

func (s *GrsaiTaskService) GetOwned(ctx context.Context, userID, apiKeyID int64, localID string) (*GrsaiSettlement, error) {
	if s == nil || s.Repo == nil {
		return nil, ErrGrsaiSettlementInvalidInput
	}
	return s.Repo.GetOwnedV2(ctx, userID, apiKeyID, localID)
}

func (s *GrsaiTaskService) ListOwned(ctx context.Context, userID, apiKeyID int64, limit, offset int) ([]*GrsaiSettlement, error) {
	if s == nil || s.Repo == nil {
		return nil, ErrGrsaiSettlementInvalidInput
	}
	return s.Repo.ListOwnedV2(ctx, userID, apiKeyID, limit, offset)
}
