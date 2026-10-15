package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func toAPIsAccountTestFixture(t *testing.T, status int, body string) (*AccountTestService, *systemOnePolicyAccountRepo, *[]*http.Request, *gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	account := &Account{ID: 97, Platform: "toapis", Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "toapis-private-key", "base_url": "https://toapis.com"}}
	repo := &systemOnePolicyAccountRepo{account: account}
	var requests []*http.Request
	upstream := &systemOneHTTPUpstream{do: func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req)
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream, cfg: &config.Config{}}
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/97/test", nil)
	return svc, repo, &requests, c, rec
}

func TestToAPIsAccountTestUsesReadOnlyModelsEndpoint(t *testing.T) {
	svc, _, requests, c, rec := toAPIsAccountTestFixture(t, http.StatusOK, `{"object":"list","data":[{"id":"image-model"}]}`)
	require.NoError(t, svc.TestAccountConnection(c, 97, "video-generation-model", "do not generate", AccountTestModeDefault))
	require.Len(t, *requests, 1)
	req := (*requests)[0]
	require.Equal(t, http.MethodGet, req.Method)
	require.Equal(t, "https://toapis.com/v1/models?type=all", req.URL.String())
	require.Equal(t, "Bearer toapis-private-key", req.Header.Get("Authorization"))
	require.Nil(t, req.Body)
	require.Contains(t, rec.Body.String(), `"type":"test_complete"`)
	require.NotContains(t, rec.Body.String(), "toapis-private-key")
}

func TestToAPIsAccountTestRejectionDoesNotLeakUpstreamBody(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			svc, repo, requests, c, rec := toAPIsAccountTestFixture(t, status, `{"error":"invalid key toapis-private-key"}`)
			require.Error(t, svc.TestAccountConnection(c, 97, "", "", AccountTestModeDefault))
			require.Len(t, *requests, 1)
			require.Equal(t, 1, repo.errorCalls)
			require.NotContains(t, rec.Body.String(), "toapis-private-key")
			require.NotContains(t, rec.Body.String(), `"type":"test_complete"`)
		})
	}
}

func TestToAPIsAccountTestInvalidResponseAndTransientErrorsDoNotMarkAccount(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"malformed", http.StatusOK, `<html>OK</html>`},
		{"missing-data", http.StatusOK, `{"message":"OK"}`},
		{"rate-limit", http.StatusTooManyRequests, `{"error":"busy"}`},
		{"unavailable", http.StatusServiceUnavailable, `{"error":"busy"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo, _, c, _ := toAPIsAccountTestFixture(t, tc.status, tc.body)
			require.Error(t, svc.TestAccountConnection(c, 97, "", "", AccountTestModeDefault))
			require.Zero(t, repo.errorCalls)
		})
	}
}

func TestToAPIsAccountTestRequiresConfiguredBaseAndKey(t *testing.T) {
	for _, missing := range []string{"base_url", "api_key"} {
		t.Run(missing, func(t *testing.T) {
			svc, repo, requests, c, _ := toAPIsAccountTestFixture(t, http.StatusOK, `{"data":[]}`)
			delete(repo.account.Credentials, missing)
			require.Error(t, svc.TestAccountConnection(c, 97, "", "", AccountTestModeDefault))
			require.Empty(t, *requests)
		})
	}
}
