//go:build integration

package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type grsaiFailAfterBilling struct {
	inner service.UsageBillingTransactionalRepository
	fail  bool
}

func (b *grsaiFailAfterBilling) ApplyTx(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	result, err := b.inner.ApplyTx(ctx, tx, cmd)
	if err == nil && b.fail {
		return nil, errors.New("injected failure after real billing effects")
	}
	return result, err
}

func grsaiRealBillingFixture(t *testing.T) (*grsaiSettlementRepository, *service.GrsaiSettlementService, *service.GrsaiSettlement) {
	t.Helper()
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{Email: "grsai-bill-" + uuid.NewString() + "@example.com", PasswordHash: "hash", Balance: 100})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-grsai-bill-" + uuid.NewString(), Name: "grsai-bill", Quota: 100})
	account := mustCreateAccount(t, client, &service.Account{Name: "grsai-bill-" + uuid.NewString(), Type: service.AccountTypeAPIKey})
	params := grsaiSettlementTestParams(t, "real-billing")
	params.UserID, params.APIKeyID, params.AccountID = user.ID, key.ID, account.ID
	params.BaseUnitPrice, params.GroupRateMultiplier, params.AccountRateMultiplier = 0.1, 2, 3
	params.BillableUnitPrice, params.RequestedImageCount = 0.6, 2
	params.HoldAmount, params.HoldState = 1.2, "none"
	repo := NewGrsaiSettlementRepository(integrationDB)
	record, err := repo.Create(ctx, params)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, record)
	cleanupGrsaiBillingMarker(t, service.GrsaiSettlementRequestID(record.ID), key.ID)
	cleanupGrsaiBillingMarker(t, service.GrsaiHoldReserveRequestID(record.ID), key.ID)
	cleanupGrsaiBillingMarker(t, service.GrsaiHoldCaptureRequestID(record.ID), key.ID)
	cleanupGrsaiBillingMarker(t, service.GrsaiHoldReleaseRequestID(record.ID), key.ID)
	billing := NewUsageBillingRepository(client, integrationDB).(service.UsageBillingTransactionalRepository)
	holdBilling := billing.(service.GrsaiHoldBillingRepository)
	require.NoError(t, repo.ReserveHold(ctx, record.ID, record.ClaimVersion, func(ctx context.Context, tx *sql.Tx, locked *service.GrsaiSettlement) error {
		_, reserveErr := holdBilling.ReserveGrsaiBalanceTx(ctx, tx, &service.GrsaiBalanceHoldCommand{
			SettlementID: locked.ID, UserID: locked.UserID, APIKeyID: locked.APIKeyID,
			Amount: locked.HoldAmount, IdempotencyKey: service.GrsaiHoldReserveRequestID(locked.ID),
		})
		return reserveErr
	}))
	record, err = repo.ClaimByID(ctx, record.ID, time.Now(), time.Now().Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, "held", record.HoldState)
	return repo, &service.GrsaiSettlementService{Repo: repo, Billing: billing, HoldBilling: holdBilling}, record
}

func TestGrsaiLegacyUnquantizedHoldCanBeCaptured(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{Email: "grsai-precision-" + uuid.NewString() + "@example.com", PasswordHash: "hash", Balance: 100})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-grsai-precision-" + uuid.NewString(), Name: "grsai-precision", Quota: 100})
	account := mustCreateAccount(t, client, &service.Account{Name: "grsai-precision-" + uuid.NewString(), Type: service.AccountTypeAPIKey})
	params := grsaiSettlementTestParams(t, "precision")
	params.UserID, params.APIKeyID, params.AccountID = user.ID, key.ID, account.ID
	params.BaseUnitPrice, params.GroupRateMultiplier, params.AccountRateMultiplier = 0.01234567, 1.0005, 1
	params.BillableUnitPrice, params.RequestedImageCount = 0.0123518428, 1
	params.HoldAmount, params.HoldState = 0.0123518428, "none"
	repo := NewGrsaiSettlementRepository(integrationDB)
	record, err := repo.Create(ctx, params)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, record)
	for _, requestID := range []string{service.GrsaiSettlementRequestID(record.ID), service.GrsaiHoldReserveRequestID(record.ID), service.GrsaiHoldCaptureRequestID(record.ID)} {
		cleanupGrsaiBillingMarker(t, requestID, key.ID)
	}
	billing := NewUsageBillingRepository(client, integrationDB).(service.UsageBillingTransactionalRepository)
	holdBilling := billing.(service.GrsaiHoldBillingRepository)
	require.NoError(t, repo.ReserveHold(ctx, record.ID, record.ClaimVersion, func(ctx context.Context, tx *sql.Tx, locked *service.GrsaiSettlement) error {
		_, reserveErr := holdBilling.ReserveGrsaiBalanceTx(ctx, tx, &service.GrsaiBalanceHoldCommand{
			SettlementID: locked.ID, UserID: locked.UserID, APIKeyID: locked.APIKeyID,
			Amount: locked.HoldAmount, IdempotencyKey: service.GrsaiHoldReserveRequestID(locked.ID),
		})
		return reserveErr
	}))
	var frozen float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT frozen_balance FROM users WHERE id = $1`, user.ID).Scan(&frozen))
	require.InDelta(t, 0.01235184, frozen, 1e-10)
	claim, err := repo.ClaimByID(ctx, record.ID, time.Now(), time.Now().Add(time.Minute))
	require.NoError(t, err)
	_, err = repo.UpdateResult(ctx, record.ID, claim.ClaimVersion, "succeeded", "", time.Now().Add(time.Minute))
	require.NoError(t, err)
	svc := &service.GrsaiSettlementService{Repo: repo, Billing: billing, HoldBilling: holdBilling}
	applied, err := svc.Settle(ctx, record.ID, claim.ClaimVersion)
	require.NoError(t, err)
	require.True(t, applied)
	assertGrsaiBalanceAndQuota(t, record, 99.98764816, 0.01235184)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT frozen_balance FROM users WHERE id = $1`, user.ID).Scan(&frozen))
	require.Zero(t, frozen)
}

func TestGrsaiResultlessAsyncSuccessRetainsFirstTimestampAndSettlesAfterPoll(t *testing.T) {
	ctx := context.Background()
	repo, svc, claim := grsaiRealBillingFixture(t)
	require.NoError(t, func() error {
		_, err := integrationDB.ExecContext(ctx, `UPDATE grsai_settlements SET delivery_mode = 'async' WHERE id = $1`, claim.ID)
		return err
	}())
	claim.DeliveryMode = service.GrsaiDeliveryAsync
	_, err := svc.ConsumeGrsaiSSE(ctx, claim, strings.NewReader("data: {\"id\":\"resultless-real\",\"status\":\"succeeded\",\"progress\":100}\n\n"), nil)
	require.ErrorIs(t, err, service.ErrGrsaiSSEProtocol)
	stored, err := repo.GetByID(ctx, claim.ID)
	require.NoError(t, err)
	require.Equal(t, "pending_upstream", stored.InternalStatus)
	require.Equal(t, "succeeded", stored.UpstreamStatus)
	require.Empty(t, stored.ResultURLs)
	require.NotNil(t, stored.ResultUpdatedAt)
	firstMissing := *stored.ResultUpdatedAt
	assertGrsaiBalanceAndQuota(t, claim, 98.8, 0)

	claim, err = repo.ClaimByID(ctx, claim.ID, time.Now().Add(2*time.Minute), time.Now().Add(3*time.Minute))
	require.NoError(t, err)
	out := svc.FinishAt(ctx, claim, nil, errors.New("temporary poll failure"), time.Now().Add(2*time.Minute))
	require.NoError(t, out.SettlementError)
	require.Equal(t, service.GrsaiStateAwaitingResult, out.State)
	stored, err = repo.GetByID(ctx, claim.ID)
	require.NoError(t, err)
	require.Equal(t, firstMissing, *stored.ResultUpdatedAt)

	claim, err = repo.ClaimByID(ctx, claim.ID, time.Now().Add(4*time.Minute), time.Now().Add(5*time.Minute))
	require.NoError(t, err)
	out = svc.FinishAt(ctx, claim, &service.GrsaiUpstreamResult{HTTPStatus: 200, TaskID: "resultless-real",
		Status: service.GrsaiUpstreamStatusSucceeded, ResultURLs: []string{"https://img.invalid/final.png"}}, nil, time.Now().Add(4*time.Minute))
	require.NoError(t, out.SettlementError)
	require.Equal(t, service.GrsaiStateSettled, out.State)
	assertGrsaiBalanceAndQuota(t, claim, 98.8, 1.2)
}

func TestGrsaiResultlessAsyncSuccessDeadlineReleasesHoldAndStopsClaims(t *testing.T) {
	ctx := context.Background()
	repo, svc, claim := grsaiRealBillingFixture(t)
	_, err := integrationDB.ExecContext(ctx, `UPDATE grsai_settlements SET delivery_mode = 'async' WHERE id = $1`, claim.ID)
	require.NoError(t, err)
	claim.DeliveryMode = service.GrsaiDeliveryAsync
	_, err = svc.ConsumeGrsaiSSE(ctx, claim, strings.NewReader("data: {\"id\":\"resultless-expired\",\"status\":\"succeeded\"}\n\n"), nil)
	require.ErrorIs(t, err, service.ErrGrsaiSSEProtocol)
	_, err = integrationDB.ExecContext(ctx, `UPDATE grsai_settlements SET result_updated_at = NOW() - INTERVAL '25 hours' WHERE id = $1`, claim.ID)
	require.NoError(t, err)
	claim, err = repo.ClaimByID(ctx, claim.ID, time.Now().Add(2*time.Minute), time.Now().Add(3*time.Minute))
	require.NoError(t, err)
	out := svc.FinishAt(ctx, claim, &service.GrsaiUpstreamResult{HTTPStatus: 200, TaskID: "resultless-expired",
		Status: service.GrsaiUpstreamStatusSucceeded}, nil, time.Now().Add(2*time.Minute))
	require.NoError(t, out.SettlementError)
	require.Equal(t, service.GrsaiStateManualReview, out.State)
	stored, err := repo.GetByID(ctx, claim.ID)
	require.NoError(t, err)
	require.Equal(t, "released", stored.HoldState)
	require.Equal(t, "manual_review", service.BuildGrsaiTaskView(stored).Status)
	assertGrsaiBalanceAndQuota(t, claim, 100, 0)
	var frozen float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT frozen_balance FROM users WHERE id = $1`, claim.UserID).Scan(&frozen))
	require.Zero(t, frozen)
	_, err = repo.ClaimByID(ctx, claim.ID, time.Now().Add(4*time.Minute), time.Now().Add(5*time.Minute))
	require.ErrorIs(t, err, service.ErrGrsaiSettlementClaimLost)
}

func TestGrsaiResultlessSynchronousSuccessDoesNotChargeAfter502(t *testing.T) {
	for _, mode := range []service.GrsaiDeliveryMode{service.GrsaiDeliveryJSON, service.GrsaiDeliveryStream} {
		t.Run(string(mode), func(t *testing.T) {
			ctx := context.Background()
			repo, svc, claim := grsaiRealBillingFixture(t)
			_, err := integrationDB.ExecContext(ctx, `UPDATE grsai_settlements SET delivery_mode = $2 WHERE id = $1`, claim.ID, mode)
			require.NoError(t, err)
			claim.DeliveryMode = mode
			_, err = svc.ConsumeGrsaiSSE(ctx, claim, strings.NewReader("data: {\"id\":\"resultless-sync\",\"status\":\"succeeded\"}\n\n"), nil)
			require.ErrorIs(t, err, service.ErrGrsaiSSEProtocol)
			stored, err := repo.GetByID(ctx, claim.ID)
			require.NoError(t, err)
			require.Equal(t, "manual_review", stored.InternalStatus)
			require.Equal(t, "released", stored.HoldState)
			assertGrsaiBalanceAndQuota(t, claim, 100, 0)
			_, err = repo.ClaimByID(ctx, claim.ID, time.Now().Add(time.Hour), time.Now().Add(2*time.Hour))
			require.ErrorIs(t, err, service.ErrGrsaiSettlementClaimLost)
		})
	}
}

func TestGrsaiLegacyUnreservedSettlementDebitsBalanceOnce(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{Email: "grsai-legacy-" + uuid.NewString() + "@example.com", PasswordHash: "hash", Balance: 100})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-grsai-legacy-" + uuid.NewString(), Name: "grsai-legacy", Quota: 100})
	account := mustCreateAccount(t, client, &service.Account{Name: "grsai-legacy-" + uuid.NewString(), Type: service.AccountTypeAPIKey})
	params := grsaiSettlementTestParams(t, "legacy-unreserved")
	params.UserID, params.APIKeyID, params.AccountID = user.ID, key.ID, account.ID
	params.BaseUnitPrice, params.GroupRateMultiplier, params.AccountRateMultiplier = 0.1, 2, 3
	params.BillableUnitPrice, params.RequestedImageCount = 0.6, 2
	params.HoldAmount, params.HoldState = 0, "none"
	repo := NewGrsaiSettlementRepository(integrationDB)
	record, err := repo.Create(ctx, params)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, record)
	cleanupGrsaiBillingMarker(t, service.GrsaiSettlementRequestID(record.ID), key.ID)
	claim, err := repo.ClaimByID(ctx, record.ID, time.Now(), time.Now().Add(time.Minute))
	require.NoError(t, err)
	_, err = repo.UpdateResult(ctx, record.ID, claim.ClaimVersion, "succeeded", "", time.Now().Add(time.Minute))
	require.NoError(t, err)
	billing := NewUsageBillingRepository(client, integrationDB).(service.UsageBillingTransactionalRepository)
	svc := &service.GrsaiSettlementService{Repo: repo, Billing: billing}
	applied, err := svc.Settle(ctx, record.ID, claim.ClaimVersion)
	require.NoError(t, err)
	require.True(t, applied)
	assertGrsaiBalanceAndQuota(t, record, 98.8, 1.2)
	applied, err = svc.Settle(ctx, record.ID, claim.ClaimVersion)
	require.NoError(t, err)
	require.False(t, applied)
	assertGrsaiBalanceAndQuota(t, record, 98.8, 1.2)
}

func TestGrsaiSettlementRepository_ReservationAndHoldStateAreAtomic(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{Email: "grsai-reserve-" + uuid.NewString() + "@example.com", PasswordHash: "hash", Balance: 100})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-grsai-reserve-" + uuid.NewString(), Name: "grsai-reserve", Quota: 100})
	account := mustCreateAccount(t, client, &service.Account{Name: "grsai-reserve-" + uuid.NewString(), Type: service.AccountTypeAPIKey})
	params := grsaiSettlementTestParams(t, "atomic-reserve")
	params.UserID, params.APIKeyID, params.AccountID = user.ID, key.ID, account.ID
	params.HoldAmount, params.HoldState = 1.2, "none"
	repo := NewGrsaiSettlementRepository(integrationDB)
	record, err := repo.Create(ctx, params)
	require.NoError(t, err)
	cleanupCreatedGrsaiSettlements(t, record)
	cleanupGrsaiBillingMarker(t, service.GrsaiHoldReserveRequestID(record.ID), key.ID)
	cleanupGrsaiBillingMarker(t, service.GrsaiHoldReleaseRequestID(record.ID), key.ID)
	billing := NewUsageBillingRepository(client, integrationDB).(service.GrsaiHoldBillingRepository)
	reserve := func(ctx context.Context, tx *sql.Tx, locked *service.GrsaiSettlement) error {
		_, err := billing.ReserveGrsaiBalanceTx(ctx, tx, &service.GrsaiBalanceHoldCommand{SettlementID: locked.ID, UserID: locked.UserID, APIKeyID: locked.APIKeyID, Amount: locked.HoldAmount, IdempotencyKey: service.GrsaiHoldReserveRequestID(locked.ID)})
		return err
	}
	injected := errors.New("failure after frozen balance update")
	err = repo.ReserveHold(ctx, record.ID, 0, func(ctx context.Context, tx *sql.Tx, locked *service.GrsaiSettlement) error {
		if err := reserve(ctx, tx, locked); err != nil {
			return err
		}
		return injected
	})
	require.ErrorIs(t, err, injected)
	assertGrsaiBalanceAndQuota(t, record, 100, 0)
	var frozen float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT frozen_balance FROM users WHERE id = $1`, user.ID).Scan(&frozen))
	require.Zero(t, frozen)
	require.Zero(t, countGrsaiBillingMarkers(t, service.GrsaiHoldReserveRequestID(record.ID), key.ID))
	stored, err := repo.GetByID(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, "none", stored.HoldState)
	require.NoError(t, repo.ReserveHold(ctx, record.ID, 0, reserve))
	stored, err = repo.GetByID(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, "held", stored.HoldState)
	require.Equal(t, 1, countGrsaiBillingMarkers(t, service.GrsaiHoldReserveRequestID(record.ID), key.ID))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT frozen_balance FROM users WHERE id = $1`, user.ID).Scan(&frozen))
	require.InDelta(t, 1.2, frozen, 1e-8)
	require.ErrorIs(t, repo.ReserveHold(ctx, record.ID, 0, reserve), service.ErrGrsaiSettlementClaimLost)
	claim, err := repo.ClaimByID(ctx, record.ID, time.Now(), time.Now().Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, int64(1), claim.ClaimVersion)
	require.NoError(t, repo.AbortUnsubmitted(ctx, record.ID, "initial claim response lost", func(ctx context.Context, tx *sql.Tx, locked *service.GrsaiSettlement) error {
		_, err := billing.ReleaseGrsaiBalanceTx(ctx, tx, &service.GrsaiBalanceHoldCommand{SettlementID: locked.ID, UserID: locked.UserID, APIKeyID: locked.APIKeyID, Amount: locked.HoldAmount, IdempotencyKey: service.GrsaiHoldReleaseRequestID(locked.ID)})
		return err
	}))
	stored, err = repo.GetByID(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, "closed_no_charge", stored.InternalStatus)
	require.Equal(t, "released", stored.HoldState)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT frozen_balance FROM users WHERE id = $1`, user.ID).Scan(&frozen))
	require.Zero(t, frozen)
	assertGrsaiBalanceAndQuota(t, record, 100, 0)
}

// If ApplyTx opened/committed its own transaction, these balances and dedup
// assertions would expose charges that escaped the settlement rollback.
func TestGrsaiSettlementService_RealBillingRollsBackThenRetries(t *testing.T) {
	ctx := context.Background()
	repo, svc, record := grsaiRealBillingFixture(t)
	billing := &grsaiFailAfterBilling{inner: svc.Billing, fail: true}
	svc.Billing = billing
	upstream := &service.GrsaiUpstreamResult{HTTPStatus: 200, Status: "succeeded"}
	out := svc.Finish(ctx, record, upstream, nil)
	require.Same(t, upstream, out.Upstream)
	require.NoError(t, out.UpstreamError)
	require.Error(t, out.SettlementError)
	require.Equal(t, service.GrsaiStateSettlementPending, out.State)
	assertGrsaiBalanceAndQuota(t, record, 98.8, 0)
	var frozen float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT frozen_balance FROM users WHERE id = $1`, record.UserID).Scan(&frozen))
	require.InDelta(t, 1.2, frozen, 1e-8)
	require.Equal(t, 0, countGrsaiBillingMarkers(t, service.GrsaiSettlementRequestID(record.ID), record.APIKeyID))
	stored, err := repo.GetByID(ctx, record.ID)
	require.NoError(t, err)
	require.Nil(t, stored.SettledAmount)
	require.Equal(t, "succeeded", stored.UpstreamStatus)
	billing.fail = false
	retry, err := repo.ClaimByID(ctx, record.ID, time.Now(), time.Now().Add(time.Minute))
	require.NoError(t, err)
	require.Greater(t, retry.ClaimVersion, record.ClaimVersion)
	_, err = svc.Settle(ctx, record.ID, record.ClaimVersion)
	require.ErrorIs(t, err, service.ErrGrsaiSettlementClaimLost)
	applied, err := svc.Settle(ctx, retry.ID, retry.ClaimVersion)
	require.NoError(t, err)
	require.True(t, applied)
	assertGrsaiBalanceAndQuota(t, record, 98.8, 1.2)
	require.Equal(t, 1, countGrsaiBillingMarkers(t, service.GrsaiSettlementRequestID(record.ID), record.APIKeyID))
	stored, err = repo.GetByID(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, "settled", stored.InternalStatus)
	require.InDelta(t, 1.2, *stored.SettledAmount, 1e-8)
	applied, err = svc.Settle(ctx, retry.ID, retry.ClaimVersion)
	require.NoError(t, err)
	require.False(t, applied)
	assertGrsaiBalanceAndQuota(t, record, 98.8, 1.2)
}

// @covers AC-005
func TestGrsaiSettlementService_ConcurrentSuccessChargesOnce(t *testing.T) {
	ctx := context.Background()
	repo, svc, record := grsaiRealBillingFixture(t)
	_, err := repo.UpdateResult(ctx, record.ID, record.ClaimVersion, "succeeded", "", time.Now().Add(time.Minute))
	require.NoError(t, err)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	results := make(chan bool, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			applied, settleErr := svc.Settle(ctx, record.ID, record.ClaimVersion)
			results <- applied
			errs <- settleErr
		}()
	}
	wg.Wait()
	close(errs)
	close(results)
	for settleErr := range errs {
		require.NoError(t, settleErr)
	}
	winners := 0
	for applied := range results {
		if applied {
			winners++
		}
	}
	require.Equal(t, 1, winners)
	assertGrsaiBalanceAndQuota(t, record, 98.8, 1.2)
	_, err = repo.ClaimByID(ctx, record.ID, time.Now().Add(time.Hour), time.Now().Add(2*time.Hour))
	require.ErrorIs(t, err, service.ErrGrsaiSettlementClaimLost)
}

func TestGrsaiSettlementRepository_ClaimByIDRespectsLeaseAndRelease(t *testing.T) {
	ctx := context.Background()
	repo, _, record := grsaiRealBillingFixture(t)
	_, err := repo.ClaimByID(ctx, record.ID, time.Now(), time.Now().Add(time.Minute))
	require.ErrorIs(t, err, service.ErrGrsaiSettlementClaimLost)
	require.NoError(t, repo.MarkPendingUpstream(ctx, record.ID, record.ClaimVersion, time.Now().Add(time.Minute)))
	claimed, err := repo.ClaimByID(ctx, record.ID, time.Now(), time.Now().Add(time.Minute))
	require.NoError(t, err)
	require.Greater(t, claimed.ClaimVersion, record.ClaimVersion)
	require.ErrorIs(t, repo.MarkPendingUpstream(ctx, record.ID, record.ClaimVersion, time.Now()), service.ErrGrsaiSettlementClaimLost)
}

func TestGrsaiSettlementRepository_TerminalUpstreamResultCannotRegress(t *testing.T) {
	ctx := context.Background()
	repo, _, record := grsaiRealBillingFixture(t)
	_, err := repo.UpdateResult(ctx, record.ID, record.ClaimVersion, "succeeded", "", time.Now().Add(time.Minute))
	require.NoError(t, err)
	_, err = repo.UpdateResult(ctx, record.ID, record.ClaimVersion, "running", "", time.Now().Add(time.Minute))
	require.ErrorIs(t, err, service.ErrGrsaiSettlementClaimLost)
	stored, err := repo.GetByID(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, "succeeded", stored.UpstreamStatus)
}

// @covers AC-006
func TestGrsaiSettlementService_RealFailuresNeverBill(t *testing.T) {
	for _, status := range []string{"failed", "violation"} {
		t.Run(status, func(t *testing.T) {
			_, svc, record := grsaiRealBillingFixture(t)
			out := svc.Finish(context.Background(), record, &service.GrsaiUpstreamResult{Status: status}, nil)
			require.NoError(t, out.SettlementError)
			require.Equal(t, service.GrsaiStateClosedNoCharge, out.State)
			assertGrsaiBalanceAndQuota(t, record, 100, 0)
			require.Equal(t, 0, countGrsaiBillingMarkers(t, service.GrsaiSettlementRequestID(record.ID), record.APIKeyID))
		})
	}
}

func assertGrsaiBalanceAndQuota(t *testing.T, record *service.GrsaiSettlement, wantBalance, wantQuota float64) {
	t.Helper()
	var balance, quota float64
	require.NoError(t, integrationDB.QueryRow("SELECT balance FROM users WHERE id = $1", record.UserID).Scan(&balance))
	require.NoError(t, integrationDB.QueryRow("SELECT quota_used FROM api_keys WHERE id = $1", record.APIKeyID).Scan(&quota))
	require.InDelta(t, wantBalance, balance, 1e-8)
	require.InDelta(t, wantQuota, quota, 1e-8)
}
