//go:build unit

package middleware

import (
	"context"
	"errors"

	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type mediaReservationMiddlewareRepo struct{ *stubApiKeyRepo }

func (r *mediaReservationMiddlewareRepo) GetReservedMediaAmount(context.Context, int64) (float64, error) {
	return 0, errors.New("synthetic reservation database failure")
}

func TestMediaReservationReadFailureDoesNotBlockOwnedTaskPolling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key := &service.APIKey{ID: 1, UserID: 2, Key: "synthetic-reservation-key", Status: service.StatusAPIKeyActive, User: &service.User{ID: 2, Status: service.StatusActive, Balance: 100}}
	repo := &mediaReservationMiddlewareRepo{stubApiKeyRepo: &stubApiKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) { copy := *key; return &copy, nil }}}
	cfg := &config.Config{RunMode: config.RunModeStandard}
	svc := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)
	router := gin.New()
	router.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(svc, nil, cfg)))
	for _, path := range []string{"/v1/images/generations/media_test", "/v1/videos/generations/media_test"} {
		router.GET(path, func(c *gin.Context) { c.Status(200) })
	}
	router.POST("/v1/chat/completions", func(c *gin.Context) { c.Status(200) })
	for _, tc := range []struct {
		method, path string
		status       int
	}{{"GET", "/v1/images/generations/media_test", 200}, {"GET", "/v1/videos/generations/media_test", 200}, {"POST", "/v1/chat/completions", 503}} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.Header.Set("Authorization", "Bearer "+key.Key)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		require.Equal(t, tc.status, res.Code, tc.path)
	}
}
