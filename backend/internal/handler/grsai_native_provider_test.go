//go:build unit

package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type nativeMediaHandlerTaskRepo struct {
	service.GrsaiV2TaskRepository
	params service.CreateV2GrsaiTaskParams
}

func (r *nativeMediaHandlerTaskRepo) CreateV2GrsaiTask(_ context.Context, p service.CreateV2GrsaiTaskParams, _ service.GrsaiSettlementTxFunc) (*service.GrsaiSettlement, error) {
	r.params = p
	id := p.LocalTaskID
	return &service.GrsaiSettlement{LocalTaskID: &id, DeliveryMode: "async"}, nil
}

type nativeMediaHandlerBalance struct {
	service.GrsaiBalanceHoldRepository
}

func TestGrsaiGatewayNativeGroupReleasesSkippedLeaseBeforeSubmitting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	price := .25
	group := &service.Group{ID: 2, Hydrated: true, Platform: service.PlatformGrsai, Status: service.StatusActive,
		AllowImageGeneration: true, RateMultiplier: 1,
		ModelPricing: []service.ChannelModelPricing{{Platform: service.PlatformGrsai, Models: []string{"media-model"}, BillingMode: service.BillingModeImage, PerRequestPrice: &price}}}
	accounts := []*service.Account{}
	for i, credential := range []map[string]any{
		{"access_token": "OAuth-token", "base_url": "https://media.example.test", "model_mapping": map[string]any{"media-model": "media-model"}},
		{"api_key": "custom-media-key", "base_url": "https://arbitrary-provider.example.test/v1"},
	} {
		accountType := service.AccountTypeAPIKey
		if i == 0 {
			accountType = service.AccountTypeOAuth
		}
		accounts = append(accounts, &service.Account{ID: int64(i + 1), Platform: service.PlatformGrsai, Type: accountType,
			Credentials: credential, Priority: i, Concurrency: 1, Status: service.StatusActive, Schedulable: true, GroupIDs: []int64{group.ID}})
	}
	base, cleanup := newTestGatewayHandler(t, group, accounts)
	defer cleanup()
	cache := &grsaiSlotCache{}
	cache.acquireAccountSlotFn = func(context.Context, int64, int, string) (bool, error) { return true, nil }
	snapshot := service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: accounts}, nil, nil, nil, nil)
	base.gatewayService = service.NewGatewayService(
		nil, &fakeGroupRepo{group: group}, nil, nil, nil, nil, nil, nil,
		&config.Config{}, snapshot, service.NewConcurrencyService(cache), nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	repo := &nativeMediaHandlerTaskRepo{}
	h := &GrsaiGatewayHandler{gatewayService: base.gatewayService, billingCacheService: base.billingCacheService,
		concurrencyHelper: base.concurrencyHelper, nativeClient: service.NewGrsaiNativeClient(nil), settlementService: &service.GrsaiSettlementService{},
		taskService: &service.GrsaiTaskService{Repo: repo, Balance: nativeMediaHandlerBalance{}, Enabled: true,
			Pricing: &service.GrsaiModelPricingResolver{Resolver: service.NewModelPricingResolver(nil, nil)}}}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/api/generate", bytes.NewBufferString(`{"model":"media-model","replyType":"async"}`))
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 4, UserID: 7, GroupID: &group.ID, Group: group, User: &service.User{ID: 7, Balance: 100}})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
	h.Generate(c)
	require.Equal(t, http.StatusAccepted, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(2), repo.params.AccountID)
	require.Equal(t, int32(2), atomic.LoadInt32(&cache.releaseAccountCalled), "the skipped lease and submitted account lease must be released")
	require.NotEmpty(t, repo.params.LocalTaskID)
	require.NotContains(t, recorder.Body.String(), "GRS.AI")
}
