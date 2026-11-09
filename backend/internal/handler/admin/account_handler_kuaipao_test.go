package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestKuaipaoAccountModelsNeverFallBackToChat(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mapping map[string]any
		want    string
	}{
		{"default", nil, "gpt-image-2.6-flare"},
		{"public alias", map[string]any{"gpt-image-2.6": "gpt-image-2.6-1k"}, "gpt-image-2.6"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &availableModelsAdminService{stubAdminService: newStubAdminService(), account: service.Account{ID: 44, Platform: service.PlatformKuaipao, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": tc.mapping}}}
			router := setupAvailableModelsRouter(svc)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/44/models", nil))
			require.Equal(t, http.StatusOK, rec.Code)
			require.Contains(t, rec.Body.String(), `"id":"`+tc.want+`"`)
			require.NotContains(t, rec.Body.String(), "claude")
		})
	}
}
