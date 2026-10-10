//go:build integration

package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/usersubscription"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// These tests use the migrated PostgreSQL database from integration_harness_test.go.
// Every fixture has its own user, group, plan and Ent hooks; no preview or production
// database is used. Settings are isolated per service rather than changed globally.
type balanceSubscriptionSettings struct {
	service.SettingRepository
	values map[string]string
}

func (r *balanceSubscriptionSettings) GetValue(_ context.Context, key string) (string, error) {
	return r.values[key], nil
}

func (r *balanceSubscriptionSettings) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			values[key] = value
		}
	}
	return values, nil
}

type balanceSubscriptionFixture struct {
	client       *dbent.Client
	user         *service.User
	group        *service.Group
	plan         *dbent.SubscriptionPlan
	users        []int64
	settings     *balanceSubscriptionSettings
	userRepo     service.UserRepository
	groupRepo    service.GroupRepository
	subRepo      service.UserSubscriptionRepository
	affiliate    *service.AffiliateService
	billingCache *service.BillingCacheService
	service      *service.PaymentService
}

func newBalanceSubscriptionFixture(t *testing.T, balance, price float64) *balanceSubscriptionFixture {
	t.Helper()
	// The harness owns integrationDB. This lightweight client gives each fixture
	// isolated hooks and must not Close the shared database connection pool.
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, integrationDB)))
	f := &balanceSubscriptionFixture{client: client}
	f.user = mustCreateUser(t, client, &service.User{Email: "balance-purchase-" + uuid.NewString() + "@example.invalid", Balance: balance})
	f.users = append(f.users, f.user.ID)
	f.group = mustCreateGroup(t, client, &service.Group{Name: "balance-purchase-" + uuid.NewString(), SubscriptionType: service.SubscriptionTypeSubscription})
	var err error
	f.plan, err = client.SubscriptionPlan.Create().SetGroupID(f.group.ID).SetName("balance purchase").SetPrice(price).SetCurrency("USD").SetValidityDays(7).SetValidityUnit("day").SetForSale(true).Save(context.Background())
	require.NoError(t, err)
	f.settings = &balanceSubscriptionSettings{SettingRepository: NewSettingRepository(client), values: map[string]string{
		service.SettingPaymentEnabled:           "true",
		service.SettingKeySubscriptionEnabled:   "true",
		service.SettingRechargeFeeRate:          "12.5",
		service.SettingBalanceRechargeMult:      "2",
		service.SettingSubscriptionUSDToCNYRate: "20",
		service.SettingKeyAffiliateEnabled:      "true",
		service.SettingKeyAffiliateRebateRate:   "10",
	}}
	f.userRepo = NewUserRepository(client, integrationDB)
	f.groupRepo = NewGroupRepository(client, integrationDB)
	f.subRepo = NewUserSubscriptionRepository(client)
	f.affiliate = service.NewAffiliateService(NewAffiliateRepository(client, integrationDB), service.NewSettingService(f.settings, nil), nil, nil)
	f.rebuildService()
	t.Cleanup(func() {
		ctx := context.Background()
		for _, userID := range f.users {
			_, err := integrationDB.ExecContext(ctx, `DELETE FROM payment_audit_logs WHERE order_id IN (SELECT id::text FROM payment_orders WHERE user_id = $1)`, userID)
			require.NoError(t, err)
			_, err = integrationDB.ExecContext(ctx, `DELETE FROM user_affiliate_ledger WHERE user_id = $1 OR source_user_id = $1`, userID)
			require.NoError(t, err)
			_, err = integrationDB.ExecContext(ctx, `DELETE FROM payment_orders WHERE user_id = $1`, userID)
			require.NoError(t, err)
			_, err = integrationDB.ExecContext(ctx, `DELETE FROM user_subscriptions WHERE user_id = $1`, userID)
			require.NoError(t, err)
			_, err = integrationDB.ExecContext(ctx, `DELETE FROM user_affiliates WHERE user_id = $1`, userID)
			require.NoError(t, err)
		}
		_, err := integrationDB.ExecContext(ctx, `DELETE FROM subscription_plans WHERE group_id = $1`, f.group.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id = $1`, f.group.ID)
		require.NoError(t, err)
		for _, userID := range f.users {
			_, err = integrationDB.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
			require.NoError(t, err)
		}
	})
	return f
}

func (f *balanceSubscriptionFixture) rebuildService() {
	subscription := service.NewSubscriptionService(f.groupRepo, f.subRepo, f.billingCache, f.client, nil)
	config := service.NewPaymentConfigService(f.client, f.settings, nil)
	f.service = service.NewPaymentService(f.client, nil, nil, nil, subscription, config, f.userRepo, f.groupRepo, f.affiliate)
}

func (f *balanceSubscriptionFixture) request(key string) service.BalanceSubscriptionPurchaseRequest {
	operationID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(strconv.FormatInt(f.user.ID, 10)+":"+key)).String()
	return service.BalanceSubscriptionPurchaseRequest{UserID: f.user.ID, PlanID: f.plan.ID, IdempotencyKey: operationID, ClientIP: "127.0.0.1", Locale: "zh-CN"}
}

func (f *balanceSubscriptionFixture) balance(t *testing.T) float64 {
	t.Helper()
	u, err := f.client.User.Get(context.Background(), f.user.ID)
	require.NoError(t, err)
	return u.Balance
}

func (f *balanceSubscriptionFixture) subscription(t *testing.T) *dbent.UserSubscription {
	t.Helper()
	sub, err := f.client.UserSubscription.Query().Where(usersubscription.UserIDEQ(f.user.ID), usersubscription.GroupIDEQ(f.group.ID)).Only(context.Background())
	require.NoError(t, err)
	return sub
}

func (f *balanceSubscriptionFixture) orderCount(t *testing.T) int {
	t.Helper()
	n, err := f.client.PaymentOrder.Query().Where(paymentorder.UserIDEQ(f.user.ID)).Count(context.Background())
	require.NoError(t, err)
	return n
}

// AC-031.3/.4/.8/.9: USD order settlement and subscription renewal share one transaction.
func TestBalanceSubscriptionPurchaseCreatesAndRenewsAtomically(t *testing.T) {
	f := newBalanceSubscriptionFixture(t, 100, 12.34)
	before := time.Now()
	first, err := f.service.PurchaseSubscriptionWithBalance(context.Background(), f.request("first"))
	require.NoError(t, err)
	require.Equal(t, payment.OrderStatusCompleted, first.Status)
	require.Equal(t, "balance", first.PaymentType)
	require.Equal(t, "USD", first.Currency)
	require.InDelta(t, 12.34, first.Amount, 1e-8)
	require.InDelta(t, first.Amount, first.PayAmount, 1e-8, "balance spending must not apply recharge fees or FX")
	require.InDelta(t, 87.66, first.Balance, 1e-8)
	sub := f.subscription(t)
	require.Equal(t, first.SubscriptionID, sub.ID)
	require.WithinDuration(t, before.AddDate(0, 0, 7), sub.ExpiresAt, 10*time.Second)
	second, err := f.service.PurchaseSubscriptionWithBalance(context.Background(), f.request("renewal"))
	require.NoError(t, err)
	require.NotEqual(t, first.OrderID, second.OrderID)
	require.Equal(t, sub.ID, second.SubscriptionID)
	require.WithinDuration(t, sub.ExpiresAt.AddDate(0, 0, 7), f.subscription(t).ExpiresAt, time.Millisecond)
	require.InDelta(t, 75.32, f.balance(t), 1e-8)
	require.Equal(t, 2, f.orderCount(t))
	replay, err := f.service.PurchaseSubscriptionWithBalance(context.Background(), f.request("first"))
	require.NoError(t, err)
	require.Equal(t, first.OrderID, replay.OrderID)
	require.InDelta(t, first.Balance, replay.Balance, 1e-8, "replay returns the original purchase receipt")
	require.InDelta(t, 75.32, f.balance(t), 1e-8, "historical replay cannot charge again")
	require.Equal(t, 2, f.orderCount(t))
	for _, orderID := range []int64{first.OrderID, second.OrderID} {
		order, err := f.client.PaymentOrder.Get(context.Background(), orderID)
		require.NoError(t, err)
		require.Equal(t, payment.OrderTypeSubscription, order.OrderType)
		require.Equal(t, payment.OrderStatusCompleted, order.Status)
		require.Zero(t, order.FeeRate)
		require.Zero(t, order.BonusAmount)
		require.Nil(t, order.ProviderInstanceID)
		logs, err := f.client.PaymentAuditLog.Query().Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(orderID, 10))).All(context.Background())
		require.NoError(t, err)
		require.NotEmpty(t, logs, "successful purchase must leave durable audit evidence")
	}
}

// AC-031.5: strict funds checks never overdraft or leave a partial purchase.
func TestBalanceSubscriptionPurchaseInsufficientAndExactFunds(t *testing.T) {
	for _, tc := range []struct {
		name    string
		balance float64
		success bool
	}{{"insufficient", 9.99, false}, {"exact", 10, true}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newBalanceSubscriptionFixture(t, tc.balance, 10)
			result, err := f.service.PurchaseSubscriptionWithBalance(context.Background(), f.request("purchase"))
			if tc.success {
				require.NoError(t, err)
				require.Equal(t, payment.OrderStatusCompleted, result.Status)
				require.Zero(t, f.balance(t))
				return
			}
			require.Error(t, err)
			require.Equal(t, "BALANCE_INSUFFICIENT", infraerrors.Reason(err))
			require.Nil(t, result)
			require.InDelta(t, tc.balance, f.balance(t), 1e-8)
			require.Zero(t, f.orderCount(t))
			n, err := f.client.UserSubscription.Query().Where(usersubscription.UserIDEQ(f.user.ID)).Count(context.Background())
			require.NoError(t, err)
			require.Zero(t, n)
		})
	}
}

// AC-031.5/.8: competing purchases serialize balances and accumulate paid terms.
func TestBalanceSubscriptionPurchaseConcurrentDifferentKeys(t *testing.T) {
	for _, tc := range []struct {
		name    string
		balance float64
		success int
	}{{"funds_for_one", 10, 1}, {"funds_for_both", 20, 2}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newBalanceSubscriptionFixture(t, tc.balance, 10)
			before := time.Now()
			results := concurrentBalanceSubscriptionPurchases(f.service, []service.BalanceSubscriptionPurchaseRequest{f.request("left"), f.request("right")})
			successes := 0
			for _, result := range results {
				if result.err == nil {
					successes++
					require.NotNil(t, result.response)
					require.Equal(t, payment.OrderStatusCompleted, result.response.Status)
				} else {
					require.Equal(t, "BALANCE_INSUFFICIENT", infraerrors.Reason(result.err), "concurrent failure must reflect funds, not lock or uniqueness failures")
				}
			}
			require.Equal(t, tc.success, successes)
			require.Zero(t, f.balance(t))
			require.Equal(t, tc.success, f.orderCount(t))
			require.WithinDuration(t, before.AddDate(0, 0, 7*tc.success), f.subscription(t).ExpiresAt, 10*time.Second, "concurrent renewals must accumulate all paid days")
		})
	}
}

type balanceSubscriptionOutcome struct {
	response *service.BalanceSubscriptionPurchaseResponse
	err      error
}

func concurrentBalanceSubscriptionPurchases(svc *service.PaymentService, requests []service.BalanceSubscriptionPurchaseRequest) []balanceSubscriptionOutcome {
	start := make(chan struct{})
	results := make([]balanceSubscriptionOutcome, len(requests))
	var wg sync.WaitGroup
	for i, request := range requests {
		wg.Add(1)
		go func(i int, request service.BalanceSubscriptionPurchaseRequest) {
			defer wg.Done()
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			results[i].response, results[i].err = svc.PurchaseSubscriptionWithBalance(ctx, request)
		}(i, request)
	}
	close(start)
	wg.Wait()
	return results
}

// AC-031.6: financial replay is durable and independent of the current catalog.
func TestBalanceSubscriptionPurchaseConcurrentReplayAndCatalogChanges(t *testing.T) {
	f := newBalanceSubscriptionFixture(t, 20, 10)
	request := f.request("same-operation")
	results := concurrentBalanceSubscriptionPurchases(f.service, []service.BalanceSubscriptionPurchaseRequest{request, request, request, request})
	for _, result := range results {
		require.NoError(t, result.err)
		require.NotNil(t, result.response)
		require.Equal(t, results[0].response.OrderID, result.response.OrderID)
	}
	require.Equal(t, 1, f.orderCount(t))
	require.InDelta(t, 10, f.balance(t), 1e-8)
	expiry := f.subscription(t).ExpiresAt
	_, err := f.client.SubscriptionPlan.UpdateOneID(f.plan.ID).SetPrice(99).SetForSale(false).Save(context.Background())
	require.NoError(t, err)
	// A new service instance proves replay survives process-local state loss and
	// does not revalidate a catalog price or sale flag changed after settlement.
	f.rebuildService()
	replay, err := f.service.PurchaseSubscriptionWithBalance(context.Background(), request)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.Equal(t, results[0].response.OrderID, replay.OrderID)
	require.InDelta(t, 10, replay.Amount, 1e-8)
	require.InDelta(t, 10, f.balance(t), 1e-8)
	require.Equal(t, expiry, f.subscription(t).ExpiresAt)
	require.Equal(t, 1, f.orderCount(t))

	otherPlan, err := f.client.SubscriptionPlan.Create().SetGroupID(f.group.ID).SetName("different plan").SetPrice(3).SetCurrency("USD").SetValidityDays(1).SetForSale(true).Save(context.Background())
	require.NoError(t, err)
	request.PlanID = otherPlan.ID
	_, err = f.service.PurchaseSubscriptionWithBalance(context.Background(), request)
	require.Error(t, err, "same operation cannot be repurposed for another plan")
	require.Equal(t, "IDEMPOTENCY_KEY_CONFLICT", infraerrors.Reason(err))
	require.InDelta(t, 10, f.balance(t), 1e-8)
	require.Equal(t, 1, f.orderCount(t))
}

type failingBalanceSubscriptionRepository struct {
	service.UserSubscriptionRepository
	userID      int64
	wantBalance float64
	observed    bool
}

func (r *failingBalanceSubscriptionRepository) Create(ctx context.Context, sub *service.UserSubscription) error {
	tx := dbent.TxFromContext(ctx)
	if tx == nil {
		return errors.New("subscription creation escaped purchase transaction")
	}
	u, err := tx.Client().User.Get(ctx, r.userID)
	if err != nil {
		return err
	}
	if u.Balance != r.wantBalance {
		return fmt.Errorf("expected balance deduction before assignment, got %v", u.Balance)
	}
	if err := r.UserSubscriptionRepository.Create(ctx, sub); err != nil {
		return err
	}
	r.observed = true
	return errors.New("injected failure after subscription creation")
}

// AC-031.4: errors after real writes roll back balance, order, term and audit.
func TestBalanceSubscriptionPurchaseRollsBackAfterAssignmentOrAuditFailure(t *testing.T) {
	for _, point := range []string{"subscription", "audit"} {
		t.Run(point, func(t *testing.T) {
			f := newBalanceSubscriptionFixture(t, 20, 10)
			observed := false
			attemptedOrderID := ""
			var failing *failingBalanceSubscriptionRepository
			if point == "subscription" {
				failing = &failingBalanceSubscriptionRepository{UserSubscriptionRepository: f.subRepo, userID: f.user.ID, wantBalance: 10}
				f.subRepo = failing
				f.rebuildService()
			} else {
				f.client.PaymentAuditLog.Use(func(next dbent.Mutator) dbent.Mutator {
					return dbent.MutateFunc(func(ctx context.Context, mutation dbent.Mutation) (dbent.Value, error) {
						tx := dbent.TxFromContext(ctx)
						if tx == nil {
							return nil, errors.New("audit escaped purchase transaction")
						}
						u, err := tx.Client().User.Get(ctx, f.user.ID)
						if err != nil {
							return nil, err
						}
						count, err := tx.Client().UserSubscription.Query().Where(usersubscription.UserIDEQ(f.user.ID)).Count(ctx)
						if err != nil {
							return nil, err
						}
						if u.Balance != 10 || count != 1 {
							return next.Mutate(ctx, mutation)
						}
						observed = true
						if audit, ok := mutation.(*dbent.PaymentAuditLogMutation); ok {
							attemptedOrderID, _ = audit.OrderID()
						}
						if _, err := next.Mutate(ctx, mutation); err != nil {
							return nil, err
						}
						return nil, errors.New("injected failure after purchase audit")
					})
				})
			}
			response, err := f.service.PurchaseSubscriptionWithBalance(context.Background(), f.request("rollback"))
			require.ErrorContains(t, err, "injected failure")
			require.Nil(t, response)
			if failing != nil {
				observed = failing.observed
			}
			require.True(t, observed, "must exercise failure after real transactional writes")
			require.InDelta(t, 20, f.balance(t), 1e-8)
			require.Zero(t, f.orderCount(t))
			n, err := f.client.UserSubscription.Query().Where(usersubscription.UserIDEQ(f.user.ID)).Count(context.Background())
			require.NoError(t, err)
			require.Zero(t, n)
			if attemptedOrderID != "" {
				n, err := f.client.PaymentAuditLog.Query().Where(paymentauditlog.OrderIDEQ(attemptedOrderID)).Count(context.Background())
				require.NoError(t, err)
				require.Zero(t, n, "a rolled-back audit cannot survive its purchase")
			}
		})
	}
}

// AC-031.7: an operation key never resolves to a different user's purchase.
func TestBalanceSubscriptionPurchaseScopesKeysToUserAndRejectsInactiveUser(t *testing.T) {
	f := newBalanceSubscriptionFixture(t, 20, 10)
	first, err := f.service.PurchaseSubscriptionWithBalance(context.Background(), f.request("shared-key"))
	require.NoError(t, err)
	other := mustCreateUser(t, f.client, &service.User{Email: "balance-other-" + uuid.NewString() + "@example.invalid", Balance: 10})
	f.users = append(f.users, other.ID)
	request := f.request("shared-key")
	request.UserID = other.ID
	second, err := f.service.PurchaseSubscriptionWithBalance(context.Background(), request)
	require.NoError(t, err)
	require.NotEqual(t, first.OrderID, second.OrderID)
	otherAfter, err := f.client.User.Get(context.Background(), other.ID)
	require.NoError(t, err)
	require.Zero(t, otherAfter.Balance)
	require.InDelta(t, 10, f.balance(t), 1e-8)
	_, err = f.client.User.UpdateOneID(f.user.ID).SetStatus("disabled").Save(context.Background())
	require.NoError(t, err)
	_, err = f.service.PurchaseSubscriptionWithBalance(context.Background(), f.request("inactive"))
	require.Error(t, err)
	require.Equal(t, 1, f.orderCount(t))
	require.InDelta(t, 10, f.balance(t), 1e-8)
}

// AC-031.9: already-funded spending does not issue a second affiliate rebate.
func TestBalanceSubscriptionPurchaseDoesNotRebateSpentBalance(t *testing.T) {
	f := newBalanceSubscriptionFixture(t, 20, 10)
	inviter := mustCreateUser(t, f.client, &service.User{Email: "balance-inviter-" + uuid.NewString() + "@example.invalid"})
	f.users = append(f.users, inviter.ID)
	ctx := context.Background()
	repo := NewAffiliateRepository(f.client, integrationDB)
	_, err := repo.EnsureUserAffiliate(ctx, inviter.ID)
	require.NoError(t, err)
	_, err = repo.EnsureUserAffiliate(ctx, f.user.ID)
	require.NoError(t, err)
	bound, err := repo.BindInviter(ctx, f.user.ID, inviter.ID)
	require.NoError(t, err)
	require.True(t, bound)
	// Prove affiliate issuance is enabled and functional for this exact fixture.
	initialRebate, err := f.affiliate.AccrueInviteRebateForOrder(ctx, f.user.ID, 1, nil)
	require.NoError(t, err)
	require.InDelta(t, 0.1, initialRebate, 1e-8)
	_, err = f.service.PurchaseSubscriptionWithBalance(ctx, f.request("spend"))
	require.NoError(t, err)
	summary, err := repo.EnsureUserAffiliate(ctx, inviter.ID)
	require.NoError(t, err)
	require.InDelta(t, initialRebate, summary.AffHistoryQuota, 1e-8, "spending already-funded balance must not issue another affiliate rebate")
}

// AC-031.3/.5: cent-priced plans preserve the user's eight-decimal USD balance.
func TestBalanceSubscriptionPurchasePreservesDecimalBalance(t *testing.T) {
	for _, tc := range []struct {
		name               string
		balance, remainder float64
	}{{"exact_decimal", 0.3, 0}, {"last_balance_decimal", 0.30000001, 0.00000001}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newBalanceSubscriptionFixture(t, tc.balance, 0.1)
			_, err := f.service.PurchaseSubscriptionWithBalance(context.Background(), f.request("ten-cents"))
			require.NoError(t, err)
			_, err = f.client.SubscriptionPlan.UpdateOneID(f.plan.ID).SetPrice(0.2).Save(context.Background())
			require.NoError(t, err)
			second, err := f.service.PurchaseSubscriptionWithBalance(context.Background(), f.request("twenty-cents"))
			require.NoError(t, err)
			require.InDelta(t, 0.2, second.PayAmount, 1e-12)
			require.InDelta(t, tc.remainder, second.Balance, 1e-12)
			require.InDelta(t, tc.remainder, f.balance(t), 1e-12)
			require.Equal(t, 2, f.orderCount(t))
		})
	}
}

// AC-031.2: authoritative feature flags are checked before any money is spent.
func TestBalanceSubscriptionPurchaseRejectsDisabledFeatures(t *testing.T) {
	for _, key := range []string{service.SettingPaymentEnabled, service.SettingKeySubscriptionEnabled} {
		t.Run(key, func(t *testing.T) {
			f := newBalanceSubscriptionFixture(t, 20, 10)
			f.settings.values[key] = "false"
			_, err := f.service.PurchaseSubscriptionWithBalance(context.Background(), f.request("disabled-feature"))
			require.Error(t, err)
			require.InDelta(t, 20, f.balance(t), 1e-8)
			require.Zero(t, f.orderCount(t))
		})
	}
}

// AC-031.5: a purchase must not overwrite an unrelated atomic debit.
func TestBalanceSubscriptionPurchasePreservesConcurrentOtherDebit(t *testing.T) {
	f := newBalanceSubscriptionFixture(t, 13, 10)
	start := make(chan struct{})
	purchaseDone := make(chan error, 1)
	debitDone := make(chan error, 1)
	go func() {
		<-start
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := f.service.PurchaseSubscriptionWithBalance(ctx, f.request("purchase"))
		purchaseDone <- err
	}()
	go func() {
		<-start
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := f.userRepo.AdjustBalance(ctx, f.user.ID, -3)
		debitDone <- err
	}()
	close(start)
	purchaseErr, debitErr := <-purchaseDone, <-debitDone
	require.NoError(t, purchaseErr)
	require.NoError(t, debitErr)
	require.Zero(t, f.balance(t))
	require.Equal(t, 1, f.orderCount(t))
}

type balanceSubscriptionObservedCache struct {
	service.BillingCache
	fixture                         *balanceSubscriptionFixture
	mu                              sync.Mutex
	fail                            bool
	balanceCalls, subscriptionCalls int
	commitErrors                    []error
}

func (c *balanceSubscriptionObservedCache) observe(ctx context.Context, balance bool) error {
	// Use the root client without the purchase transaction: cache invalidation
	// must see committed data, not only writes visible to the transaction owner.
	u, err := c.fixture.client.User.Get(context.Background(), c.fixture.user.ID)
	if err == nil && u.Balance != 10 {
		err = fmt.Errorf("cache invalidation saw uncommitted balance %v", u.Balance)
	}
	if err == nil {
		count, countErr := c.fixture.client.PaymentOrder.Query().Where(paymentorder.UserIDEQ(u.ID), paymentorder.StatusEQ(payment.OrderStatusCompleted)).Count(context.Background())
		if countErr != nil {
			err = countErr
		} else if count != 1 {
			err = fmt.Errorf("cache invalidation saw %d completed purchases", count)
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if balance {
		c.balanceCalls++
	} else {
		c.subscriptionCalls++
	}
	if err != nil {
		c.commitErrors = append(c.commitErrors, err)
		return err
	}
	if c.fail {
		return errors.New("injected cache invalidation failure")
	}
	return nil
}

func (c *balanceSubscriptionObservedCache) InvalidateUserBalance(ctx context.Context, userID int64) error {
	if err := c.observe(ctx, true); err != nil {
		return err
	}
	return c.BillingCache.InvalidateUserBalance(ctx, userID)
}

func (c *balanceSubscriptionObservedCache) InvalidateSubscriptionCache(ctx context.Context, userID, groupID int64) error {
	if err := c.observe(ctx, false); err != nil {
		return err
	}
	return c.BillingCache.InvalidateSubscriptionCache(ctx, userID, groupID)
}

// AC-031.12: committed purchases survive cache errors and replay repairs both caches.
func TestBalanceSubscriptionPurchaseReplayRepairsPostCommitCaches(t *testing.T) {
	f := newBalanceSubscriptionFixture(t, 20, 10)
	cache := &balanceSubscriptionObservedCache{BillingCache: NewBillingCache(testRedis(t)), fixture: f, fail: true}
	f.billingCache = service.NewBillingCacheService(cache, f.userRepo, f.subRepo, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(f.billingCache.Stop)
	f.rebuildService()
	ctx := context.Background()
	require.NoError(t, cache.SetUserBalance(ctx, f.user.ID, 20))
	require.NoError(t, cache.SetSubscriptionCache(ctx, f.user.ID, f.group.ID, &service.SubscriptionCacheData{Status: service.SubscriptionStatusExpired, ExpiresAt: time.Now().Add(-time.Hour)}))
	request := f.request("cache-recovery")
	result, err := f.service.PurchaseSubscriptionWithBalance(ctx, request)
	require.NoError(t, err, "cache failure cannot turn a committed purchase into payment failure")
	require.Equal(t, payment.OrderStatusCompleted, result.Status)
	require.InDelta(t, 10, f.balance(t), 1e-8)
	expiry := f.subscription(t).ExpiresAt
	cache.mu.Lock()
	firstBalanceCalls, firstSubscriptionCalls := cache.balanceCalls, cache.subscriptionCalls
	commitErrors := append([]error(nil), cache.commitErrors...)
	cache.fail = false
	cache.mu.Unlock()
	require.Empty(t, commitErrors)
	require.Positive(t, firstBalanceCalls)
	require.Positive(t, firstSubscriptionCalls)
	f.rebuildService()
	replay, err := f.service.PurchaseSubscriptionWithBalance(ctx, request)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.Equal(t, result.OrderID, replay.OrderID)
	require.Equal(t, 1, f.orderCount(t))
	require.InDelta(t, 10, f.balance(t), 1e-8)
	require.Equal(t, expiry, f.subscription(t).ExpiresAt)
	cache.mu.Lock()
	finalBalanceCalls, finalSubscriptionCalls := cache.balanceCalls, cache.subscriptionCalls
	commitErrors = append([]error(nil), cache.commitErrors...)
	cache.mu.Unlock()
	require.Empty(t, commitErrors)
	require.Greater(t, finalBalanceCalls, firstBalanceCalls)
	require.Greater(t, finalSubscriptionCalls, firstSubscriptionCalls)
	_, err = cache.GetUserBalance(ctx, f.user.ID)
	require.ErrorIs(t, err, redis.Nil)
	_, err = cache.GetSubscriptionCache(ctx, f.user.ID, f.group.ID)
	require.ErrorIs(t, err, redis.Nil)
}

// AC-031.3/.7: the real HTTP handler ignores caller-supplied identity and price.
func TestBalanceSubscriptionPurchaseHandlerUsesSubjectAndServerPrice(t *testing.T) {
	f := newBalanceSubscriptionFixture(t, 20, 10)
	other := mustCreateUser(t, f.client, &service.User{Email: "balance-forged-" + uuid.NewString() + "@example.invalid", Balance: 50})
	f.users = append(f.users, other.ID)
	api := handler.NewPaymentHandler(f.service, service.NewPaymentConfigService(f.client, f.settings, nil))
	request := f.request("handler-operation")
	body, err := json.Marshal(map[string]any{"plan_id": f.plan.ID, "idempotency_key": request.IdempotencyKey, "user_id": other.ID, "amount": 0.01, "pay_amount": 0.01})
	require.NoError(t, err)
	serve := func(authenticated bool) *httptest.ResponseRecorder {
		router := gin.New()
		router.POST("/api/v1/payment/orders/balance-subscription", func(c *gin.Context) {
			if authenticated {
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: f.user.ID})
			}
			api.PurchaseSubscriptionWithBalance(c)
		})
		r := httptest.NewRequest(http.MethodPost, "/api/v1/payment/orders/balance-subscription", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", request.IdempotencyKey)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	unauthorized := serve(false)
	require.Equal(t, http.StatusUnauthorized, unauthorized.Code)
	require.Zero(t, f.orderCount(t))
	response := serve(true)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var payload struct {
		Data service.BalanceSubscriptionPurchaseResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
	require.Positive(t, payload.Data.OrderID)
	require.InDelta(t, 10, payload.Data.Amount, 1e-8)
	require.InDelta(t, 10, f.balance(t), 1e-8)
	otherAfter, err := f.client.User.Get(context.Background(), other.ID)
	require.NoError(t, err)
	require.InDelta(t, 50, otherAfter.Balance, 1e-8)
	order, err := f.client.PaymentOrder.Get(context.Background(), payload.Data.OrderID)
	require.NoError(t, err)
	require.Equal(t, f.user.ID, order.UserID)
	require.Equal(t, 1, f.orderCount(t))
}

// AC-031.8: unavailable product or group state cannot create a fresh purchase.
func TestBalanceSubscriptionPurchaseRejectsUnavailableCatalog(t *testing.T) {
	for _, state := range []string{"not_for_sale", "zero_price", "inactive_group", "non_subscription_group", "missing_plan"} {
		t.Run(state, func(t *testing.T) {
			f := newBalanceSubscriptionFixture(t, 20, 10)
			ctx := context.Background()
			request := f.request("unavailable")
			var err error
			switch state {
			case "not_for_sale":
				_, err = f.client.SubscriptionPlan.UpdateOneID(f.plan.ID).SetForSale(false).Save(ctx)
			case "zero_price":
				_, err = f.client.SubscriptionPlan.UpdateOneID(f.plan.ID).SetPrice(0).Save(ctx)
			case "inactive_group":
				_, err = f.client.Group.UpdateOneID(f.group.ID).SetStatus("disabled").Save(ctx)
			case "non_subscription_group":
				_, err = f.client.Group.UpdateOneID(f.group.ID).SetSubscriptionType(service.SubscriptionTypeStandard).Save(ctx)
			case "missing_plan":
				request.PlanID = 1 << 62
			}
			require.NoError(t, err)
			_, err = f.service.PurchaseSubscriptionWithBalance(ctx, request)
			require.Error(t, err)
			require.InDelta(t, 20, f.balance(t), 1e-8)
			require.Zero(t, f.orderCount(t))
			count, err := f.client.UserSubscription.Query().Where(usersubscription.UserIDEQ(f.user.ID)).Count(ctx)
			require.NoError(t, err)
			require.Zero(t, count)
		})
	}
}

// AC-031.5: frozen holds already reduced balance and must not be deducted again.
func TestBalanceSubscriptionPurchaseUsesAvailableBalanceWithoutTouchingFrozenFunds(t *testing.T) {
	f := newBalanceSubscriptionFixture(t, 10, 10)
	_, err := integrationDB.ExecContext(context.Background(), `UPDATE users SET frozen_balance = 90 WHERE id = $1`, f.user.ID)
	require.NoError(t, err)
	result, err := f.service.PurchaseSubscriptionWithBalance(context.Background(), f.request("available-balance"))
	require.NoError(t, err)
	require.Equal(t, payment.OrderStatusCompleted, result.Status)
	require.Zero(t, f.balance(t))
	var frozen float64
	require.NoError(t, integrationDB.QueryRowContext(context.Background(), `SELECT frozen_balance FROM users WHERE id = $1`, f.user.ID).Scan(&frozen))
	require.InDelta(t, 90, frozen, 1e-8)
}

// AC-031.3: an old confirmation amount rejects the purchase before any writes;
// correcting the quote can reuse the same unconsumed idempotency key.
func TestBalanceSubscriptionPurchaseRejectsChangedConfirmationAmount(t *testing.T) {
	f := newBalanceSubscriptionFixture(t, 50, 20)
	ctx := context.Background()
	request := f.request("confirmed-quote")
	expected := 10.0
	request.ExpectedAmount = &expected
	_, err := f.service.PurchaseSubscriptionWithBalance(ctx, request)
	require.Error(t, err)
	require.Equal(t, "PLAN_PRICE_CHANGED", infraerrors.Reason(err))
	require.InDelta(t, 50, f.balance(t), 1e-8)
	require.Zero(t, f.orderCount(t))
	count, err := f.client.UserSubscription.Query().Where(usersubscription.UserIDEQ(f.user.ID)).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
	expected = 20
	result, err := f.service.PurchaseSubscriptionWithBalance(ctx, request)
	require.NoError(t, err)
	require.False(t, result.Replayed)
	require.InDelta(t, 20, result.Amount, 1e-8)
	require.InDelta(t, 30, f.balance(t), 1e-8)
	require.Equal(t, 1, f.orderCount(t))
	require.Equal(t, result.SubscriptionID, f.subscription(t).ID)
	_, err = f.client.SubscriptionPlan.UpdateOneID(f.plan.ID).SetPrice(30).Save(ctx)
	require.NoError(t, err)
	replay, err := f.service.PurchaseSubscriptionWithBalance(ctx, request)
	require.NoError(t, err, "a committed receipt must replay before checking a new quote")
	require.True(t, replay.Replayed)
	require.Equal(t, result.OrderID, replay.OrderID)
	require.InDelta(t, result.Amount, replay.Amount, 1e-8)
	require.InDelta(t, 30, f.balance(t), 1e-8)
	require.Equal(t, 1, f.orderCount(t))
}
