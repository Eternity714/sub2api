package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIResponses_CodexPageContextReachesConcurrencyAdmission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(
		`{"model":"gpt-6.1-sol","input":[{"type":"function_call_output","namespace":"agent","name":"codex_apps_open_page","output":"{\"page_id\":null}"},{"type":"function_call_output","namespace":"agent","name":"codex_apps_selected_page_part_1_of_1","output":"Companion Page snapshot part 1 of 1:\n{\"page_id\":null,\"document_view\":null}"},{"role":"user","content":"hello"}]}`,
	))
	c.Request.Header.Set("Content-Type", "application/json")
	groupID := int64(2)
	c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{
		ID: 101, GroupID: &groupID, User: &service.User{ID: 1},
	})
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1, Concurrency: 1})
	admissionCalls := 0
	h := newOpenAIHandlerForPreviousResponseIDValidation(t, &concurrencyCacheMock{
		acquireUserSlotFn: func(context.Context, int64, int, string) (bool, error) {
			admissionCalls++
			return false, errors.New("test concurrency backend unavailable")
		},
	})
	h.Responses(c)

	// The injected snapshots must pass request validation and reach admission.
	// The unavailable concurrency backend stops before billing or an upstream call.
	require.Equal(t, 1, admissionCalls)
	require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	require.NotContains(t, w.Body.String(), "function_call_output requires call_id")
	require.Contains(t, w.Body.String(), `"type":"api_error"`)
}
