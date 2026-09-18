//go:build integration

package repository

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type grsaiFailAfterBilling struct {
	inner service.UsageBillingTransactionalRepository
	fail bool
}

func (b *grsaiFailAfterBilling) ApplyTx(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	result, err := b.inner.ApplyTx(ctx, tx, cmd)
	if err == nil && b.fail { return nil, errors.New("injected failure after real billing effects") }
	return result, err
}

func grsaiRealBillingFixture(t *testing.T) (*grsaiSettlementRepository, *service.GrsaiSettlementService, *service.GrsaiSettlement) {
	t.Helper()
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{Email: "grsai-bill-"+uuid.NewString()+"@example.com", PasswordHash: "hash", Balance: 100})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-grsai-bill-"+uuid.NewString(), Name: "grsai-bill", Quota: 100})
	account := mustCreateAccount(t, client, &service.Account{Name: "grsai-bill-"+uuid.NewString(), Type: service.AccountTypeAPIKey})
	params := grsaiSettlementTestParams(t, "real-billing")
	params.UserID, params.APIKeyID, params.AccountID = user.ID, key.ID, account.ID
	params.BaseUnitPrice, params.GroupRateMultiplier, params.AccountRateMultiplier = 0.1, 2, 3
	params.BillableUnitPrice, params.RequestedImageCount = 0.6, 2
	cleanupGrsaiSettlements(t, params.BillingIdempotencyKey)
	repo := NewGrsaiSettlementRepository(integrationDB)
	record, err := repo.Create(ctx, params)
	require.NoError(t, err)
	cleanupGrsaiBillingMarker(t, service.GrsaiSettlementRequestID(record.ID), key.ID)
	record, err = repo.ClaimByID(ctx, record.ID, time.Now(), time.Now().Add(time.Minute))
	require.NoError(t, err)
	billing := NewUsageBillingRepository(client, integrationDB).(service.UsageBillingTransactionalRepository)
	return repo, &service.GrsaiSettlementService{Repo: repo, Billing: billing}, record
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
	assertGrsaiBalanceAndQuota(t, record, 100, 0)
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
	for settleErr := range errs { require.NoError(t, settleErr) }
	winners := 0
	for applied := range results { if applied { winners++ } }
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
