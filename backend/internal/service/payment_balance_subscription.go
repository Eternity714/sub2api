package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionplan"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const balanceSubscriptionPaymentType = "balance"

// BalanceSubscriptionPurchaseRequest is authenticated by the handler; callers cannot select another user.
type BalanceSubscriptionPurchaseRequest struct {
	UserID         int64
	PlanID         int64
	ExpectedAmount *float64
	IdempotencyKey string
	ClientIP       string
	SrcHost        string
	Locale         string
}

type BalanceSubscriptionPurchaseResponse struct {
	OrderID        int64   `json:"order_id"`
	Status         string  `json:"status"`
	PaymentType    string  `json:"payment_type"`
	Currency       string  `json:"currency"`
	Amount         float64 `json:"amount"`
	PayAmount      float64 `json:"pay_amount"`
	Balance        float64 `json:"balance"`
	SubscriptionID int64   `json:"subscription_id"`
	Replayed       bool    `json:"replayed"`
}

func (s *PaymentService) PurchaseSubscriptionWithBalance(ctx context.Context, req BalanceSubscriptionPurchaseRequest) (*BalanceSubscriptionPurchaseResponse, error) {
	if req.UserID <= 0 || req.PlanID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_INPUT", "subscription purchase requires a user and plan")
	}
	key, err := uuid.Parse(strings.TrimSpace(req.IdempotencyKey))
	if err != nil || key == uuid.Nil {
		return nil, infraerrors.BadRequest("IDEMPOTENCY_KEY_INVALID", "idempotency key must be a nonzero UUID")
	}
	if s.entClient == nil || s.configService == nil || s.userRepo == nil || s.groupRepo == nil || s.subscriptionSvc == nil {
		return nil, infraerrors.ServiceUnavailable("BALANCE_PURCHASE_UNAVAILABLE", "balance subscription purchase is unavailable")
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin balance subscription purchase: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	// Serialize purchases for this user, including first-time subscription creation.
	// AdjustBalance still performs the strict, atomic funds check used by other debits.
	account, err := tx.User.Query().Where(user.IDEQ(req.UserID), user.DeletedAtIsNil()).ForUpdate().Only(txCtx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, infraerrors.NotFound("USER_NOT_FOUND", "user not found")
		}
		return nil, fmt.Errorf("lock balance subscription account: %w", err)
	}
	outTradeNo := balanceSubscriptionOutTradeNo(req.UserID, key)
	existing, err := tx.PaymentOrder.Query().Where(paymentorder.OutTradeNoEQ(outTradeNo), paymentorder.UserIDEQ(req.UserID)).Only(txCtx)
	if err == nil {
		if !isInternalBalanceOrder(existing) || existing.OrderType != payment.OrderTypeSubscription || existing.PlanID == nil || *existing.PlanID != req.PlanID || existing.Status != OrderStatusCompleted {
			return nil, infraerrors.Conflict("IDEMPOTENCY_KEY_CONFLICT", "idempotency key belongs to a different purchase")
		}
		result, err := balanceSubscriptionResult(existing, true)
		if err != nil {
			return nil, err
		}
		// Replay precedes current feature, sale and price checks: a committed purchase
		// retains its original result even when the catalog or settings change.
		_ = tx.Rollback()
		s.invalidateBalanceSubscriptionCaches(existing)
		return result, nil
	}
	if !dbent.IsNotFound(err) {
		return nil, fmt.Errorf("find balance subscription purchase: %w", err)
	}
	if account.Status != payment.EntityStatusActive {
		return nil, infraerrors.Forbidden("USER_INACTIVE", "user account is disabled")
	}
	cfg, err := s.configService.GetPaymentConfig(txCtx)
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return nil, infraerrors.Forbidden("PAYMENT_DISABLED", "payment system is disabled")
	}
	if !cfg.SubscriptionBalanceEnabled {
		return nil, infraerrors.Forbidden("SUBSCRIPTION_DISABLED", "subscription purchases are disabled")
	}
	plan, err := tx.SubscriptionPlan.Query().Where(subscriptionplan.IDEQ(req.PlanID)).ForShare().Only(txCtx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, infraerrors.NotFound("PLAN_NOT_AVAILABLE", "plan not found or not for sale")
		}
		return nil, fmt.Errorf("read balance subscription plan: %w", err)
	}
	if !plan.ForSale {
		return nil, infraerrors.NotFound("PLAN_NOT_AVAILABLE", "plan not found or not for sale")
	}
	group, err := s.groupRepo.GetByID(txCtx, plan.GroupID)
	if err != nil {
		return nil, fmt.Errorf("read balance subscription group: %w", err)
	}
	if group.Status != payment.EntityStatusActive {
		return nil, infraerrors.NotFound("GROUP_NOT_FOUND", "subscription group is no longer available")
	}
	if !group.IsSubscriptionType() {
		return nil, infraerrors.BadRequest("GROUP_TYPE_MISMATCH", "group is not a subscription type")
	}
	cost, err := balanceSubscriptionPrice(plan.Price)
	if err != nil {
		return nil, err
	}
	if err := validateBalanceSubscriptionExpectedAmount(req.ExpectedAmount, cost); err != nil {
		return nil, err
	}
	validityDays := plan.ValidityDays
	if validityDays <= 0 {
		return nil, infraerrors.BadRequest("INVALID_PLAN_VALIDITY", "subscription plan validity must be positive")
	}
	if validityDays > MaxValidityDays {
		validityDays = MaxValidityDays
	}
	days := psComputeValidityDays(validityDays, plan.ValidityUnit)
	if days > MaxValidityDays {
		days = MaxValidityDays
	}
	change, err := s.userRepo.AdjustBalance(txCtx, req.UserID, -cost)
	if err != nil {
		if errors.Is(err, ErrBalanceNegative) {
			return nil, infraerrors.BadRequest("BALANCE_INSUFFICIENT", "insufficient account balance").WithMetadata(map[string]string{
				"required_amount": fmt.Sprintf("%.2f", cost), "balance": strconv.FormatFloat(change.Old, 'f', -1, 64),
			})
		}
		return nil, fmt.Errorf("debit balance subscription purchase: %w", err)
	}
	now := time.Now()
	snapshot := map[string]any{"schema_version": 2, "currency": "USD", "balance_before": change.Old, "balance_after": change.New}
	order, err := tx.PaymentOrder.Create().
		SetUserID(req.UserID).SetUserEmail(account.Email).SetUserName(account.Username).
		SetNillableUserNotes(psNilIfEmpty(account.Notes)).
		SetAmount(cost).SetPayAmount(cost).SetFeeRate(0).SetBonusAmount(0).
		SetRechargeCode("").SetOutTradeNo(outTradeNo).SetPaymentType(balanceSubscriptionPaymentType).SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeSubscription).SetPlanID(plan.ID).
		SetSubscriptionGroupID(plan.GroupID).SetSubscriptionDays(days).
		SetStatus(OrderStatusCompleted).SetExpiresAt(now).SetPaidAt(now).SetCompletedAt(now).
		SetClientIP(req.ClientIP).SetSrcHost(req.SrcHost).SetProviderSnapshot(snapshot).Save(txCtx)
	if err != nil {
		return nil, fmt.Errorf("record balance subscription order: %w", err)
	}
	sub, renewed, err := s.subscriptionSvc.assignOrExtendSubscription(txCtx, &AssignSubscriptionInput{
		UserID: req.UserID, GroupID: plan.GroupID, ValidityDays: days, Notes: paymentSubscriptionOrderNote(order.ID),
	}, true)
	if err != nil {
		return nil, fmt.Errorf("assign balance subscription: %w", err)
	}
	snapshot["subscription_id"] = sub.ID
	order, err = tx.PaymentOrder.UpdateOneID(order.ID).SetProviderSnapshot(snapshot).Save(txCtx)
	if err != nil {
		return nil, fmt.Errorf("record balance subscription result: %w", err)
	}
	detail, err := json.Marshal(map[string]any{
		"currency": "USD", "amount": cost, "balanceBefore": change.Old, "balanceAfter": change.New,
		"groupID": plan.GroupID, "validityDays": days, "subscriptionID": sub.ID, "renewed": renewed,
	})
	if err != nil {
		return nil, fmt.Errorf("encode balance subscription audit: %w", err)
	}
	for _, action := range []string{"SUBSCRIPTION_ASSIGNED", "SUBSCRIPTION_SUCCESS"} {
		if _, err := tx.PaymentAuditLog.Create().SetOrderID(strconv.FormatInt(order.ID, 10)).
			SetAction(action).SetDetail(string(detail)).SetOperator("user:" + strconv.FormatInt(req.UserID, 10)).Save(txCtx); err != nil {
			return nil, fmt.Errorf("record balance subscription audit: %w", err)
		}
	}
	result, err := balanceSubscriptionResult(order, false)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit balance subscription purchase: %w", err)
	}
	// A failure after commit must never turn a completed purchase into a payment
	// error. Retrying the durable key will retry cache invalidation without charging.
	s.invalidateBalanceSubscriptionCaches(order)
	if s.notificationEmailService != nil {
		s.notificationEmailService.RememberRecipientLocale(context.Background(), req.UserID, account.Email, req.Locale)
	}
	s.dispatchPaymentFulfillmentNotification(order, "SUBSCRIPTION_SUCCESS")
	return result, nil
}

func balanceSubscriptionOutTradeNo(userID int64, key uuid.UUID) string {
	digest := sha256.Sum256([]byte(strconv.FormatInt(userID, 10) + ":" + key.String()))
	return "bal_" + hex.EncodeToString(digest[:])[:60]
}

func balanceSubscriptionPrice(price float64) (float64, error) {
	if math.IsNaN(price) || math.IsInf(price, 0) || price <= 0 || price >= 1e18 {
		return 0, infraerrors.BadRequest("INVALID_PLAN_PRICE", "subscription plan price must be a valid positive USD amount")
	}
	amount := decimal.NewFromFloat(price).Round(2)
	if !amount.IsPositive() {
		return 0, infraerrors.BadRequest("INVALID_PLAN_PRICE", "subscription plan price must be at least one USD cent")
	}
	return amount.InexactFloat64(), nil
}

func validateBalanceSubscriptionExpectedAmount(expected *float64, cost float64) error {
	if expected == nil {
		return nil
	}
	if math.IsNaN(*expected) || math.IsInf(*expected, 0) || *expected <= 0 || *expected >= 1e18 {
		return infraerrors.BadRequest("INVALID_AMOUNT", "confirmed subscription amount must be a positive USD amount")
	}
	if !decimal.NewFromFloat(*expected).Round(2).Equal(decimal.NewFromFloat(cost)) {
		return infraerrors.Conflict("PLAN_PRICE_CHANGED", "subscription price changed; confirm the updated price before purchasing").WithMetadata(map[string]string{
			"amount": fmt.Sprintf("%.2f", cost), "currency": "USD",
		})
	}
	return nil
}

func balanceSubscriptionResult(order *dbent.PaymentOrder, replayed bool) (*BalanceSubscriptionPurchaseResponse, error) {
	var snapshot struct {
		Currency       string  `json:"currency"`
		Balance        float64 `json:"balance_after"`
		SubscriptionID int64   `json:"subscription_id"`
	}
	body, err := json.Marshal(order.ProviderSnapshot)
	if err == nil {
		err = json.Unmarshal(body, &snapshot)
	}
	if err != nil || snapshot.Currency != "USD" || snapshot.SubscriptionID <= 0 || math.IsNaN(snapshot.Balance) || math.IsInf(snapshot.Balance, 0) || snapshot.Balance < 0 {
		return nil, infraerrors.InternalServer("BALANCE_ORDER_RESULT_INVALID", "completed balance purchase result is unavailable")
	}
	return &BalanceSubscriptionPurchaseResponse{
		OrderID: order.ID, Status: order.Status, PaymentType: order.PaymentType, Currency: snapshot.Currency,
		Amount: order.Amount, PayAmount: order.PayAmount, Balance: snapshot.Balance,
		SubscriptionID: snapshot.SubscriptionID, Replayed: replayed,
	}, nil
}

func isInternalBalanceOrder(order *dbent.PaymentOrder) bool {
	return order != nil && strings.EqualFold(strings.TrimSpace(order.PaymentType), balanceSubscriptionPaymentType)
}

func (s *PaymentService) invalidateBalanceSubscriptionCaches(order *dbent.PaymentOrder) {
	cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var billingCache *BillingCacheService
	if s.subscriptionSvc != nil {
		billingCache = s.subscriptionSvc.billingCacheService
		if order.SubscriptionGroupID != nil {
			if err := s.subscriptionSvc.invalidateSubscriptionCaches(order.UserID, *order.SubscriptionGroupID); err != nil {
				slog.Warn("balance purchase subscription cache invalidation failed", "order_id", order.ID, "error", err)
			}
		}
	}
	if s.redeemService != nil {
		if billingCache == nil {
			billingCache = s.redeemService.billingCacheService
		}
		if s.redeemService.authCacheInvalidator != nil {
			s.redeemService.authCacheInvalidator.InvalidateAuthCacheByUserID(cacheCtx, order.UserID)
		}
	}
	if billingCache != nil {
		if err := billingCache.InvalidateUserBalance(cacheCtx, order.UserID); err != nil {
			slog.Warn("balance purchase balance cache invalidation failed", "order_id", order.ID, "error", err)
		}
	}
}
