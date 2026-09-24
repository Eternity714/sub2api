package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// grsaiDeliveryResultRepo implements only the owner-scoped query used by the
// public result endpoint. Embedding keeps this focused test independent from
// storage implementation details unrelated to result visibility.
type grsaiDeliveryResultRepo struct {
	service.GrsaiSettlementRepository
	record *service.GrsaiSettlement
}

func (r *grsaiDeliveryResultRepo) GetOwnedByPublicOrUpstreamID(_ context.Context, userID, apiKeyID int64, id string) (*service.GrsaiSettlement, error) {
	if r.record == nil || r.record.UserID != userID || r.record.APIKeyID != apiKeyID {
		return nil, service.ErrGrsaiSettlementNotFound
	}
	if id != r.record.PublicTaskID && (r.record.UpstreamTaskID == nil || id != *r.record.UpstreamTaskID) {
		return nil, service.ErrGrsaiSettlementNotFound
	}
	copy := *r.record
	return &copy, nil
}

// @covers AC-004
func TestGrsaiGatewayResultUsesOwnerAndAPIKeyScopedLocalOrUpstreamID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstreamID := "upstream-42"
	repo := &grsaiDeliveryResultRepo{record: &service.GrsaiSettlement{
		ID: 42, PublicTaskID: "public-42", UpstreamTaskID: &upstreamID,
		UserID: 7, APIKeyID: 11, AccountID: 99, Model: "nano-banana-2-lite",
		InternalStatus: "settled", UpstreamStatus: service.GrsaiUpstreamStatusSucceeded,
		ResultURLs: []string{"https://images.example.test/42.png"}, BillableUnitPrice: 123,
		CreatedAt: time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC),
	}}
	h := &GrsaiGatewayHandler{taskService: &service.GrsaiTaskService{Repo: repo}}

	for _, id := range []string{"public-42", "upstream-42"} {
		t.Run(id, func(t *testing.T) {
			c, recorder := newGrsaiGatewayResultContext(t, id, 7, 11)
			h.Result(c)

			require.Equal(t, http.StatusOK, recorder.Code)
			require.Contains(t, recorder.Body.String(), `"id":"public-42"`)
			require.Contains(t, recorder.Body.String(), `"upstream_task_id":"upstream-42"`)
			require.NotContains(t, recorder.Body.String(), "account_id")
			require.NotContains(t, recorder.Body.String(), "billable_unit_price")
			require.NotContains(t, recorder.Body.String(), "upstream_body")
		})
	}

	for _, owner := range []struct {
		name           string
		userID, apiKey int64
	}{
		{name: "foreign user", userID: 8, apiKey: 11},
		{name: "foreign api key", userID: 7, apiKey: 12},
	} {
		t.Run(owner.name, func(t *testing.T) {
			c, recorder := newGrsaiGatewayResultContext(t, "public-42", owner.userID, owner.apiKey)
			h.Result(c)
			require.Equal(t, http.StatusNotFound, recorder.Code)
			require.JSONEq(t, `{"error":{"type":"not_found","message":"task not found"}}`, recorder.Body.String())
		})
	}
}

func TestGrsaiGatewayResultRejectsEmptyIDForAuthenticatedCaller(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &GrsaiGatewayHandler{taskService: &service.GrsaiTaskService{Repo: &grsaiDeliveryResultRepo{}}}
	c, recorder := newGrsaiGatewayResultContext(t, "", 7, 11)

	h.Result(c)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.JSONEq(t, `{"error":{"type":"not_found","message":"task not found"}}`, recorder.Body.String())
}

func newGrsaiGatewayResultContext(t *testing.T, id string, userID, apiKeyID int64) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, "/v1/api/result", nil)
	query := req.URL.Query()
	query.Set("id", id)
	req.URL.RawQuery = query.Encode()
	c.Request = req
	groupID := int64(2)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: apiKeyID, UserID: userID, GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformGrsai}})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
	return c, recorder
}
