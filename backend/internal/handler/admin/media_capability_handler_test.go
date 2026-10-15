//go:build unit

package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type mediaCapabilityHandlerRepo struct{ writes int }

func (r *mediaCapabilityHandlerRepo) List(context.Context, service.MediaCapabilityFilter) ([]service.MediaCapability, int64, error) {
	return []service.MediaCapability{}, 0, nil
}
func (r *mediaCapabilityHandlerRepo) Get(context.Context, int64) (*service.MediaCapability, error) {
	return nil, service.ErrMediaCapabilityNotFound
}
func (r *mediaCapabilityHandlerRepo) GetByModel(context.Context, string, string) (*service.MediaCapability, error) {
	return nil, service.ErrMediaCapabilityNotFound
}
func (r *mediaCapabilityHandlerRepo) Create(_ context.Context, row *service.MediaCapability) error {
	r.writes++
	row.ID = 1
	row.Revision = 1
	return nil
}
func (r *mediaCapabilityHandlerRepo) Update(context.Context, *service.MediaCapability, int64) error {
	r.writes++
	return service.ErrMediaCapabilityConflict
}
func (r *mediaCapabilityHandlerRepo) Delete(context.Context, int64, int64) error {
	r.writes++
	return nil
}
func mediaCapabilityTestRouter(repo *mediaCapabilityHandlerRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewMediaCapabilityHandler(service.NewMediaCapabilityService(repo))
	r.POST("/validate", h.Validate)
	r.POST("/", h.Create)
	r.GET("/templates", h.Templates)
	r.GET("/", h.List)
	r.PUT("/:id", h.Update)
	r.DELETE("/:id", h.Delete)
	return r
}
func mediaCapabilityHTTP(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}
func TestMediaCapabilityHandlerStrictBody(t *testing.T) {
	repo := &mediaCapabilityHandlerRepo{}
	r := mediaCapabilityTestRouter(repo)
	for _, body := range []string{
		`{"public_model":"example","public_model":"other","media_type":"image","schema":{"version":1,"fields":{}}}`,
		`{"Public_Model":"example","media_type":"image","schema":{"version":1,"fields":{}}}`,
		`{"public_model":"example","media_type":"image","schema":{"version":1,"fields":{}},"unknown":true}`,
		`{"public_model":"example","media_type":"image","schema":{"version":1,"fields":{}}} {}`,
	} {
		w := mediaCapabilityHTTP(r, http.MethodPost, "/", body)
		require.Equal(t, 400, w.Code, w.Body.String())
	}
	require.Zero(t, repo.writes)
}
func TestMediaCapabilityHandlerDryRunAndTemplateEnvelope(t *testing.T) {
	repo := &mediaCapabilityHandlerRepo{}
	r := mediaCapabilityTestRouter(repo)
	body := `{"public_model":"example","media_type":"image","schema":{"version":1,"fields":{"prompt":{"type":"string","required":true}}},"request_json":"{\"model\":\"example\",\"prompt\":\"test\"}"}`
	w := mediaCapabilityHTTP(r, http.MethodPost, "/validate", body)
	require.Equal(t, 200, w.Code, w.Body.String())
	var response struct {
		Data struct {
			Valid bool `json:"valid"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.True(t, response.Data.Valid)
	require.Zero(t, repo.writes)
	w = mediaCapabilityHTTP(r, http.MethodGet, "/templates", "")
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), `"items"`)
}
func TestMediaCapabilityHandlerDeleteRequiresRevision(t *testing.T) {
	repo := &mediaCapabilityHandlerRepo{}
	r := mediaCapabilityTestRouter(repo)
	w := mediaCapabilityHTTP(r, http.MethodDelete, "/1", "")
	require.Equal(t, 400, w.Code)
	require.Zero(t, repo.writes)
	w = mediaCapabilityHTTP(r, http.MethodDelete, "/1?revision=2", "")
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Equal(t, 1, repo.writes)
}
