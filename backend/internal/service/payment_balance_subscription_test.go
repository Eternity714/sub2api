//go:build unit

package service

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

// AC-031.3/.10: stale or invalid confirmation amounts never silently authorize a larger debit.
func TestBalanceSubscriptionRejectsChangedOrInvalidExpectedAmount(t *testing.T) {
	for _, tc := range []struct {
		name   string
		amount float64
		reason string
	}{
		{"price increased", 10, "PLAN_PRICE_CHANGED"},
		{"invalid zero", 0, "INVALID_AMOUNT"},
		{"invalid negative", -1, "INVALID_AMOUNT"},
		{"invalid NaN", math.NaN(), "INVALID_AMOUNT"},
		{"invalid infinity", math.Inf(1), "INVALID_AMOUNT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBalanceSubscriptionExpectedAmount(&tc.amount, 20)
			require.Error(t, err)
			require.Equal(t, tc.reason, infraerrors.Reason(err))
		})
	}
	require.NoError(t, validateBalanceSubscriptionExpectedAmount(nil, 20))
	confirmed := 20.0
	require.NoError(t, validateBalanceSubscriptionExpectedAmount(&confirmed, 20))
}

// AC-031.6/.7: purchase IDs and idempotency keys are validated before accessing storage.
func TestBalanceSubscriptionRejectsInvalidPurchaseInput(t *testing.T) {
	for _, tc := range []struct {
		name   string
		req    BalanceSubscriptionPurchaseRequest
		reason string
	}{
		{"missing user", BalanceSubscriptionPurchaseRequest{PlanID: 1, IdempotencyKey: "98bbff24-5369-4d6a-b69b-dc278b835fc0"}, "INVALID_INPUT"},
		{"negative user", BalanceSubscriptionPurchaseRequest{UserID: -1, PlanID: 1, IdempotencyKey: "98bbff24-5369-4d6a-b69b-dc278b835fc0"}, "INVALID_INPUT"},
		{"missing plan", BalanceSubscriptionPurchaseRequest{UserID: 1, IdempotencyKey: "98bbff24-5369-4d6a-b69b-dc278b835fc0"}, "INVALID_INPUT"},
		{"missing key", BalanceSubscriptionPurchaseRequest{UserID: 1, PlanID: 1}, "IDEMPOTENCY_KEY_INVALID"},
		{"malformed key", BalanceSubscriptionPurchaseRequest{UserID: 1, PlanID: 1, IdempotencyKey: "same-key"}, "IDEMPOTENCY_KEY_INVALID"},
		{"zero UUID", BalanceSubscriptionPurchaseRequest{UserID: 1, PlanID: 1, IdempotencyKey: "00000000-0000-0000-0000-000000000000"}, "IDEMPOTENCY_KEY_INVALID"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := (&PaymentService{}).PurchaseSubscriptionWithBalance(context.Background(), tc.req)
			require.Nil(t, result)
			require.Error(t, err)
			require.Equal(t, tc.reason, infraerrors.Reason(err))
		})
	}
}

// AC-031.9: the reserved internal method can only be used by the authenticated balance purchase API.
func TestBalanceSubscriptionRejectsGenericGatewayOrder(t *testing.T) {
	config := NewPaymentConfigService(nil, &paymentConfigSettingRepoStub{values: map[string]string{}}, nil)
	for _, paymentType := range []string{"balance", "BALANCE", " balance "} {
		t.Run(paymentType, func(t *testing.T) {
			result, err := (&PaymentService{configService: config}).CreateOrder(context.Background(), CreateOrderRequest{
				UserID: 1, PlanID: 1, OrderType: payment.OrderTypeSubscription, PaymentType: paymentType,
			})
			require.Nil(t, result)
			require.Error(t, err)
			require.Equal(t, "BALANCE_ORDER_EXTERNAL_PAYMENT_UNSUPPORTED", infraerrors.Reason(err))
		})
	}
}

// AC-031.2: spending remains available without external providers or recharge permission.
func TestBalanceSubscriptionAvailabilityUsesPaymentAndSubscriptionSwitch(t *testing.T) {
	for _, tc := range []struct {
		name, paymentEnabled, subscriptionEnabled string
		want                                      bool
	}{
		{"default subscription", "true", "", true},
		{"enabled", "true", "true", true},
		{"subscription disabled", "true", "false", false},
		{"payment disabled", "false", "true", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := (&PaymentConfigService{}).parsePaymentConfig(map[string]string{
				SettingPaymentEnabled: tc.paymentEnabled, SettingKeySubscriptionEnabled: tc.subscriptionEnabled,
				SettingBalancePayDisabled: "true",
			})
			body, err := json.Marshal(cfg)
			require.NoError(t, err)
			var fields map[string]any
			require.NoError(t, json.Unmarshal(body, &fields))
			require.Equal(t, tc.want, fields["subscription_balance_enabled"])
		})
	}
}

// AC-031.9: balance consumption must not reward an inviter again after the original top-up.
func TestBalanceSubscriptionDoesNotAccrueAffiliateRebate(t *testing.T) {
	order := &dbent.PaymentOrder{OrderType: payment.OrderTypeSubscription, PaymentType: "balance", Amount: 15}
	require.Zero(t, affiliateRebateBaseAmount(order))
}

// AC-031.9: internal balance orders never enter a gateway callback, retry, or refund flow.
func TestBalanceSubscriptionRejectsExternalFulfillmentAndRefund(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, OrderStatusCompleted, time.Now())
	_, err := client.PaymentOrder.UpdateOneID(order.ID).SetPaymentType("balance").Save(ctx)
	require.NoError(t, err)
	svc := &PaymentService{entClient: client}
	for name, fulfill := range map[string]func(context.Context, int64) error{
		"subscription": svc.ExecuteSubscriptionFulfillment,
		"recharge":     svc.ExecuteBalanceFulfillment,
		"retry":        svc.RetryFulfillment,
	} {
		t.Run(name, func(t *testing.T) {
			err := fulfill(ctx, order.ID)
			require.Error(t, err)
			require.Equal(t, "BALANCE_ORDER_EXTERNAL_PAYMENT_UNSUPPORTED", infraerrors.Reason(err))
		})
	}
	t.Run("callback", func(t *testing.T) {
		err := svc.confirmPayment(ctx, order.ID, "external-trade", order.PayAmount, "balance", nil)
		require.Error(t, err)
		require.Equal(t, "BALANCE_ORDER_EXTERNAL_PAYMENT_UNSUPPORTED", infraerrors.Reason(err))
	})
	t.Run("refund", func(t *testing.T) {
		_, _, err := svc.PrepareRefund(ctx, order.ID, order.Amount, "refund", false, true)
		require.Error(t, err)
		require.Equal(t, "BALANCE_ORDER_REFUND_UNSUPPORTED", infraerrors.Reason(err))
	})
	t.Run("refund status query", func(t *testing.T) {
		_, err := svc.QueryAndFinalizeRefund(ctx, order.ID)
		require.Error(t, err)
		require.Equal(t, "BALANCE_ORDER_REFUND_UNSUPPORTED", infraerrors.Reason(err))
	})
	t.Run("direct refund execution", func(t *testing.T) {
		current, err := client.PaymentOrder.Get(ctx, order.ID)
		require.NoError(t, err)
		current.PaymentType = "alipay" // A stale refund plan cannot bypass the persisted internal-order boundary.
		_, err = svc.ExecuteRefund(ctx, &RefundPlan{OrderID: order.ID, Order: current, RefundAmount: order.Amount})
		require.Error(t, err)
		require.Equal(t, "BALANCE_ORDER_REFUND_UNSUPPORTED", infraerrors.Reason(err))
		current, err = client.PaymentOrder.Get(ctx, order.ID)
		require.NoError(t, err)
		require.Equal(t, OrderStatusCompleted, current.Status)
	})
	t.Run("anonymous order lookup", func(t *testing.T) {
		_, err := svc.VerifyOrderPublic(ctx, order.OutTradeNo)
		require.Error(t, err)
		require.Equal(t, "NOT_FOUND", infraerrors.Reason(err))
	})
}

// AC-031.9: consuming a previously recharged balance does not count as new cash received.
func TestBalanceSubscriptionExcludedFromExternalPaymentStats(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, OrderStatusCompleted, time.Now())
	_, err := client.PaymentOrder.UpdateOneID(order.ID).
		SetPaymentType("balance").SetPaidAt(time.Now()).SetProviderSnapshot(map[string]any{"currency": "USD"}).Save(ctx)
	require.NoError(t, err)
	svc := &PaymentService{entClient: client}
	stats, err := svc.GetDashboardStats(ctx, 1)
	require.NoError(t, err)
	require.Zero(t, stats.TotalCount)
	require.Empty(t, stats.TotalAmount)
	require.Empty(t, stats.PaymentMethods)
	require.Empty(t, stats.TopUsers)
	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	require.NoError(t, svc.checkDailyLimit(ctx, tx, order.UserID, 1, 1), "spending balance must not consume the cash recharge limit")
}
