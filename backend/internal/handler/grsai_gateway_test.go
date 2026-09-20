package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type grsaiSlotCache struct {
	concurrencyCacheMock
	allowWait      bool
	waitIncrements atomic.Int32
	waitDecrements atomic.Int32
}

func (c *grsaiSlotCache) IncrementAccountWaitCount(context.Context, int64, int) (bool, error) {
	c.waitIncrements.Add(1)
	return c.allowWait, nil
}

func (c *grsaiSlotCache) DecrementAccountWaitCount(context.Context, int64) error {
	c.waitDecrements.Add(1)
	return nil
}

func TestGrsaiGatewayHandlerGenerateRejectsUnauthenticatedAndWrongPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &GrsaiGatewayHandler{}

	t.Run("unauthenticated", func(t *testing.T) {
		c, recorder := newGrsaiGatewayTestContext(t)
		h.Generate(c)
		require.Equal(t, http.StatusUnauthorized, recorder.Code)
	})

	t.Run("wrong platform", func(t *testing.T) {
		c, recorder := newGrsaiGatewayTestContext(t)
		groupID := int64(17)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI}})
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7, Concurrency: 1})
		h.Generate(c)
		require.Equal(t, http.StatusNotFound, recorder.Code)
	})
}

func TestGrsaiGatewayHandlerAcquireAccountSlotWaitPlan(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cache := &grsaiSlotCache{allowWait: true}
	cache.acquireAccountSlotFn = func(context.Context, int64, int, string) (bool, error) { return true, nil }
	h := &GrsaiGatewayHandler{concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second)}
	c, recorder := newGrsaiGatewayTestContext(t)

	release, ok := h.acquireAccountSlot(c, &service.AccountSelectionResult{
		Account:  &service.Account{ID: 31, Platform: service.PlatformGrsai, Type: service.AccountTypeAPIKey},
		WaitPlan: &service.AccountWaitPlan{AccountID: 31, MaxConcurrency: 1, MaxWaiting: 3, Timeout: time.Second},
	}, new(bool), zap.NewNop())

	require.True(t, ok)
	require.NotNil(t, release)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, int32(1), cache.waitIncrements.Load())
	require.Equal(t, int32(1), cache.waitDecrements.Load(), "waiting counter must be released once the slot is acquired")
	release()
	require.Equal(t, int32(1), atomic.LoadInt32(&cache.releaseAccountCalled))
}

func TestGrsaiGatewayHandlerAcquireAccountSlotQueueFullDoesNotAcquire(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cache := &grsaiSlotCache{allowWait: false}
	h := &GrsaiGatewayHandler{concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second)}
	c, recorder := newGrsaiGatewayTestContext(t)

	release, ok := h.acquireAccountSlot(c, &service.AccountSelectionResult{
		Account:  &service.Account{ID: 32, Platform: service.PlatformGrsai, Type: service.AccountTypeAPIKey},
		WaitPlan: &service.AccountWaitPlan{AccountID: 32, MaxConcurrency: 1, MaxWaiting: 0, Timeout: time.Second},
	}, new(bool), zap.NewNop())

	require.False(t, ok)
	require.Nil(t, release)
	require.Equal(t, http.StatusTooManyRequests, recorder.Code)
	require.Zero(t, cache.waitDecrements.Load())
	require.Zero(t, atomic.LoadInt32(&cache.releaseAccountCalled))
}

func TestRedactGrsaiUpstreamBodyPreservesNormalPayloadAndRedactsCredential(t *testing.T) {
	account := &service.Account{Credentials: map[string]any{"api_key": "sk-grsai-secret"}}

	for _, raw := range [][]byte{
		[]byte(`{"status":"failed","error":"Bearer sk-grsai-secret"}`),
		[]byte("upstream rejected api key sk-grsai-secret"),
	} {
		out := redactGrsaiUpstreamBody(raw, account)
		require.NotContains(t, string(out), "sk-grsai-secret")
		require.Contains(t, string(out), "REDACTED")
	}

	success := []byte(`{"status":"succeeded","data":{"id":"task-1"}}`)
	require.Equal(t, success, redactGrsaiUpstreamBody(success, account))
}

func TestParseGrsaiGenerateRequestSnapshotsImageSize(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		want string
	}{
		{name: "explicit 1K", body: `{"model":"nano-banana-2","imageSize":"1K"}`, want: service.ImageBillingSize1K},
		{name: "dimensions", body: `{"model":"nano-banana-2","imageSize":"4096x4096"}`, want: service.ImageBillingSize4K},
		{name: "missing defaults", body: `{"model":"nano-banana-2"}`, want: service.ImageBillingSize2K},
		{name: "unknown defaults", body: `{"model":"nano-banana-2","imageSize":"native"}`, want: service.ImageBillingSize2K},
	} {
		t.Run(tt.name, func(t *testing.T) {
			model, count, imageSize := parseGrsaiGenerateRequest([]byte(tt.body))
			require.Equal(t, "nano-banana-2", model)
			require.Equal(t, 1, count)
			require.Equal(t, tt.want, imageSize)
		})
	}
}

func newGrsaiGatewayTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/api/generate", nil)
	return c, recorder
}
