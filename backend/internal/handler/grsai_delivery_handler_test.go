package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type grsaiTaskQueryRepoStub struct {
	service.GrsaiV2TaskRepository
	record *service.GrsaiSettlement
	items  []*service.GrsaiSettlement
}

func (r *grsaiTaskQueryRepoStub) GetOwnedV2(_ context.Context, userID, keyID int64, id string) (*service.GrsaiSettlement, error) {
	if r.record == nil || r.record.LocalTaskID == nil || userID != r.record.UserID || keyID != r.record.APIKeyID || id != *r.record.LocalTaskID {
		return nil, service.ErrGrsaiSettlementNotFound
	}
	return r.record, nil
}

func (r *grsaiTaskQueryRepoStub) ListOwnedV2(_ context.Context, userID, keyID int64, _, _ int) ([]*service.GrsaiSettlement, error) {
	var owned []*service.GrsaiSettlement
	for _, item := range r.items {
		if item.UserID == userID && item.APIKeyID == keyID {
			owned = append(owned, item)
		}
	}
	return owned, nil
}

func grsaiQueryTestContext(t *testing.T, path string, userID, keyID int64) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, path, nil)
	if keyID != 0 {
		groupID := int64(1)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: keyID, UserID: userID,
			GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformGrsai}})
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
	}
	return c, w
}

func TestGrsaiDeliveryResultIsScopedToCreatingAPIKey(t *testing.T) {
	id, upstream := "grsai-local", "private-upstream-id"
	repo := &grsaiTaskQueryRepoStub{record: &service.GrsaiSettlement{LocalTaskID: &id,
		UserID: 7, APIKeyID: 8, UpstreamTaskID: &upstream, PublicStatus: "succeeded",
		ResultJSON: []byte(`{"results":[{"url":"https://local.example/a"}]}`), CreatedAt: time.Now()}}
	h := &GrsaiGatewayHandler{taskService: &service.GrsaiTaskService{Repo: repo}}
	for _, tc := range []struct {
		keyID int64
		want  int
	}{{0, 401}, {9, 404}, {8, 200}} {
		c, w := grsaiQueryTestContext(t, "/v1/api/result?id="+id, 7, tc.keyID)
		h.Result(c)
		require.Equal(t, tc.want, w.Code)
		require.NotContains(t, w.Body.String(), upstream)
		if tc.want == 200 {
			require.Contains(t, w.Body.String(), "https://local.example/a")
		}
	}
}

func TestGrsaiDeliveryTasksListOnlyIncludesCreatingKey(t *testing.T) {
	firstID, otherID := "grsai-own", "grsai-other"
	repo := &grsaiTaskQueryRepoStub{items: []*service.GrsaiSettlement{
		{LocalTaskID: &firstID, UserID: 7, APIKeyID: 8, PublicStatus: "queued"},
		{LocalTaskID: &otherID, UserID: 7, APIKeyID: 9, PublicStatus: "queued"},
	}}
	h := &GrsaiGatewayHandler{taskService: &service.GrsaiTaskService{Repo: repo}}
	c, w := grsaiQueryTestContext(t, "/v1/api/tasks?page=1&page_size=20", 7, 8)
	h.Tasks(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), firstID)
	require.NotContains(t, w.Body.String(), otherID)
}

func TestGrsaiDeliveryModesExposeOnlyLocalTask(t *testing.T) {
	id, upstream := "grsai-local", "private-upstream-id"
	for _, mode := range []string{"async", "json", "stream"} {
		t.Run(mode, func(t *testing.T) {
			record := &service.GrsaiSettlement{LocalTaskID: &id, UserID: 7, APIKeyID: 8,
				UpstreamTaskID: &upstream, DeliveryMode: mode, PublicStatus: "succeeded", Progress: 100,
				ResultJSON: []byte(`{"results":[{"url":"https://local.example/a"}]}`)}
			h := &GrsaiGatewayHandler{taskService: &service.GrsaiTaskService{Repo: &grsaiTaskQueryRepoStub{record: record}}}
			c, w := grsaiQueryTestContext(t, "/v1/api/generate", 7, 8)
			h.deliverTask(c, record)
			require.NotContains(t, w.Body.String(), upstream)
			switch mode {
			case "async":
				require.Equal(t, http.StatusAccepted, w.Code)
				require.NotContains(t, w.Body.String(), "https://local.example/a")
			case "json":
				require.Equal(t, http.StatusOK, w.Code)
				require.Contains(t, w.Body.String(), "https://local.example/a")
			case "stream":
				require.Equal(t, http.StatusOK, w.Code)
				require.Contains(t, w.Body.String(), "event:task")
				require.Less(t, strings.Index(w.Body.String(), `"progress":5`), strings.Index(w.Body.String(), `"progress":100`))
				require.Contains(t, w.Body.String(), `"progress":100`)
			}
		})
	}
}

func TestGrsaiDeliveryFailedStreamNeverReportsSuccess(t *testing.T) {
	id := "grsai-failed"
	record := &service.GrsaiSettlement{LocalTaskID: &id, UserID: 7, APIKeyID: 8,
		DeliveryMode: "stream", PublicStatus: "failed"}
	h := &GrsaiGatewayHandler{taskService: &service.GrsaiTaskService{Repo: &grsaiTaskQueryRepoStub{record: record}}}
	c, w := grsaiQueryTestContext(t, "/v1/api/generate", 7, 8)
	h.deliverTask(c, record)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"status":"failed"`)
	require.NotContains(t, w.Body.String(), `"progress":100`)
}
