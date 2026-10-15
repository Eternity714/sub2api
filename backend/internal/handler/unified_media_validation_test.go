//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type mediaValidationCapabilities struct {
	service.MediaCapabilityRepository
}

func (mediaValidationCapabilities) GetByModel(_ context.Context, model, kind string) (*service.MediaCapability, error) {
	if model != kind+"-only" {
		return nil, service.ErrMediaCapabilityNotFound
	}
	schema := `{"version":1,"fields":{"prompt":{"type":"string","required":true},"metadata":{"type":"object","properties":{"resolution":{"type":"string","enum":["2K"]}}}}}`
	return &service.MediaCapability{ID: 1, PublicModel: model, MediaType: kind, Enabled: true, Revision: 1, SchemaVersion: 1, Schema: json.RawMessage(schema)}, nil
}

type mediaValidationTasks struct {
	service.UnifiedMediaTaskRepository
	creates int
}

func (r *mediaValidationTasks) Create(context.Context, *service.UnifiedMediaTask) error {
	r.creates++
	return nil
}

type mediaValidationPricing struct{ service.GrsaiPricingResolver }
type mediaValidationStorage struct{ calls int }

func (p *mediaValidationStorage) Persist(context.Context, string, string, int, int, string) (*service.UnifiedMediaStoredResult, error) {
	p.calls++
	return nil, nil
}

// AC-015, AC-017, AC-019, AC-020: public HTTP rejects invalid input before
// reaching the repository transaction that creates tasks and reserves money.
func TestUnifiedMediaInvalidRequestsCannotCreateOrFreeze(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct{ name, kind, body string }{
		{"video model on image path", "image", `{"model":"video-only","prompt":"p"}`},
		{"image model on video path", "video", `{"model":"image-only","prompt":"p"}`},
		{"missing schema", "image", `{"model":"not-configured","prompt":"p"}`},
		{"video fields on image path", "image", `{"model":"image-only","prompt":"p","duration":5}`},
		{"image fields on video path", "video", `{"model":"video-only","prompt":"p","n":2}`},
		{"unknown metadata", "image", `{"model":"image-only","prompt":"p","metadata":{"secret_option":true}}`},
		{"unsupported resolution", "image", `{"model":"image-only","prompt":"p","metadata":{"resolution":"4K"}}`},
		{"wrong metadata type", "image", `{"model":"image-only","prompt":"p","metadata":{"resolution":2}}`},
		{"required prompt", "image", `{"model":"image-only"}`},
		{"duplicate JSON key", "image", `{"model":"image-only","prompt":"p","prompt":"q"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &mediaValidationTasks{}
			storage := &mediaValidationStorage{}
			svc := service.NewUnifiedMediaTaskService(repo, service.NewMediaCapabilityService(mediaValidationCapabilities{}), &service.CompositeMediaRouteService{}, nil, mediaValidationPricing{}, nil, storage)
			h := &UnifiedMediaHandler{tasks: svc}
			r := gin.New()
			r.Use(func(c *gin.Context) {
				id := int64(1)
				c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 7, UserID: 8, GroupID: &id, Group: &service.Group{ID: 1, Platform: service.PlatformComposite, AllowImageGeneration: true}})
			})
			r.POST("/v1/images/generations", h.CreateImage)
			r.POST("/v1/videos/generations", h.CreateVideo)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/"+tc.kind+"s/generations", strings.NewReader(tc.body)))
			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			require.Contains(t, w.Body.String(), `"error"`)
			require.Zero(t, repo.creates, "invalid requests must never create a task or reserve funds")
			require.Zero(t, storage.calls)
		})
	}
}
