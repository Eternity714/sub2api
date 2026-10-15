package service

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/config"
)

func ProvideUnifiedMediaTaskService(repo UnifiedMediaTaskRepository, caps *MediaCapabilityService,
	routes *CompositeMediaRouteService, accounts AccountRepository, pricing *ModelPricingResolver,
	storage *ImageStorageSettingService, billing *BillingCacheService, auth APIKeyAuthCacheInvalidator, transport HTTPUpstream, cfg *config.Config) *UnifiedMediaTaskService {
	upstream := NewUnifiedMediaUpstream(nil).(*unifiedMediaHTTPUpstream)
	upstream.transport = transport
	svc := NewUnifiedMediaTaskService(repo, caps, routes, accounts, &UnifiedMediaPricingResolver{Resolver: pricing}, upstream, NewUnifiedMediaS3Persister(storage))
	svc.AfterSettlement = func(ctx context.Context, userID int64) {
		if billing != nil {
			_ = billing.InvalidateUserBalance(ctx, userID)
		}
		if auth != nil {
			auth.InvalidateAuthCacheByUserID(ctx, userID)
		}
	}
	svc.AfterReservation = svc.AfterSettlement
	if cfg != nil {
		svc.MaxWaiting = cfg.GrsaiDelivery.MaxWaiting
	}
	svc.Start()
	return svc
}
