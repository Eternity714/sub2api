//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type unifiedMediaHandlerStub struct {
	creates   int
	gets      int
	user, key int64
	kind, id  string
	err       error
}

func (s *unifiedMediaHandlerStub) Create(_ context.Context, input service.UnifiedMediaTaskCreateInput) (*service.UnifiedMediaTask, error) {
	s.creates++
	s.kind = input.MediaType
	if s.err != nil {
		return nil, s.err
	}
	return &service.UnifiedMediaTask{ID: "media_local", Model: "public-model", Status: "in_progress", CreatedAt: time.Unix(100, 0)}, nil
}
func (s *unifiedMediaHandlerStub) GetOwned(_ context.Context, user, key int64, kind, id string) (*service.UnifiedMediaTask, error) {
	s.gets++
	s.user = user
	s.key = key
	s.kind = kind
	s.id = id
	return &service.UnifiedMediaTask{ID: id, Model: "public-model", MediaType: kind, Status: "completed", CreatedAt: time.Unix(100, 0), Units: []service.UnifiedMediaUnit{{State: "failed", Error: "upstream_failed"}}}, nil
}
func unifiedMediaHandlerRouter(stub *unifiedMediaHandlerStub) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 7, UserID: 8, Group: &service.Group{Platform: service.PlatformComposite, AllowImageGeneration: true}})
	})
	h := &UnifiedMediaHandler{tasks: stub}
	r.POST("/v1/images/generations", h.CreateImage)
	r.POST("/v1/videos/generations", h.CreateVideo)
	r.GET("/v1/videos/generations/:task_id", h.GetVideo)
	return r
}

// AC-001 (handler scope): image creation returns exactly the five public 202 fields with queued/0.
// AC-001, AC-002: both creation paths return only the asynchronous public envelope.
func TestUnifiedMediaHandlerCreationHasExactlyPublicEnvelope(t *testing.T) {
	for _, kind := range []string{"image", "video"} {
		t.Run(kind, func(t *testing.T) {
			s := &unifiedMediaHandlerStub{}
			r := unifiedMediaHandlerRouter(s)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/"+kind+"s/generations", strings.NewReader(`{"model":"public-model"}`)))
			require.Equal(t, 202, w.Code)
			var body map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			require.Len(t, body, 5)
			require.Equal(t, "queued", body["status"])
			require.EqualValues(t, 0, body["progress"])
			require.EqualValues(t, 100, body["created_at"])
			require.Equal(t, "public-model", body["model"])
			require.NotEmpty(t, body["id"])
			require.NotContains(t, body, "result")
			require.Equal(t, kind, s.kind)
			require.Equal(t, 1, s.creates)
		})
	}
}
func TestUnifiedMediaHandlerQueryUsesAuthenticatedOwnerAndPathMedia(t *testing.T) {
	s := &unifiedMediaHandlerStub{}
	r := unifiedMediaHandlerRouter(s)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/videos/generations/media_local", nil))
	require.Equal(t, 200, w.Code)
	require.EqualValues(t, 8, s.user)
	require.EqualValues(t, 7, s.key)
	require.Equal(t, "video", s.kind)
	require.Zero(t, s.creates)
	require.Contains(t, w.Body.String(), `"error":"upstream_failed"`)
}
