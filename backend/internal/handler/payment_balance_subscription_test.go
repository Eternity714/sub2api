//go:build unit

package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// AC-031.7: anonymous purchase attempts never reach the balance service.
func TestBalanceSubscriptionHandlerRejectsAnonymous(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/payment/orders/balance-subscription", strings.NewReader(`{"plan_id":1,"user_id":42}`))
	(&PaymentHandler{}).PurchaseSubscriptionWithBalance(c)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

// AC-031.6/.7: malformed requests cannot cause a debit or choose another user's identity.
func TestBalanceSubscriptionHandlerValidatesBodyAndAuthenticatedIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, body string
		userID     int64
		wantReason string
	}{
		{"malformed JSON", "{", 42, ""},
		{"missing plan", `{"idempotency_key":"98bbff24-5369-4d6a-b69b-dc278b835fc0"}`, 42, ""},
		{"invalid key", `{"plan_id":1,"idempotency_key":"not-a-uuid"}`, 42, "IDEMPOTENCY_KEY_INVALID"},
		{"body user cannot replace identity", `{"plan_id":1,"user_id":42,"idempotency_key":"98bbff24-5369-4d6a-b69b-dc278b835fc0"}`, 0, "INVALID_INPUT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/payment/orders/balance-subscription", strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: tc.userID})
			(&PaymentHandler{paymentService: &service.PaymentService{}}).PurchaseSubscriptionWithBalance(c)
			require.Equal(t, http.StatusBadRequest, w.Code)
			if tc.wantReason != "" {
				require.Contains(t, w.Body.String(), tc.wantReason)
			}
		})
	}
}
