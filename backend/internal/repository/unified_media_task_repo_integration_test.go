//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func unifiedMediaIntegrationFixture(t *testing.T) (service.UnifiedMediaTaskRepository, *service.UnifiedMediaTask, int64) {
	t.Helper()
	ctx := context.Background()
	suffix := uuid.NewString()
	var user, key, group, account int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,balance) VALUES($1,'test',10) RETURNING id`, suffix+"@media.test").Scan(&user))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO groups(name,platform) VALUES($1,'composite') RETURNING id`, "media-"+suffix).Scan(&group))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO api_keys(user_id,key,name,group_id,quota) VALUES($1,$2,'media',$3,2) RETURNING id`, user, "media-"+suffix, group).Scan(&key))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO accounts(name,platform,type,extra) VALUES($1,'grsai','apikey','{"quota_limit":2,"quota_daily_limit":2,"quota_weekly_limit":2}'::jsonb) RETURNING id`, "media-"+suffix).Scan(&account))
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO account_groups(account_id,group_id) VALUES($1,$2)`, account, group)
	require.NoError(t, err)
	encryptor, err := NewAESEncryptor(&config.Config{Totp: config.TotpConfig{EncryptionKey: "0000000000000000000000000000000000000000000000000000000000000000"}})
	require.NoError(t, err)
	repo := NewUnifiedMediaTaskRepository(integrationDB, encryptor)
	now := time.Now().UTC()
	task := &service.UnifiedMediaTask{ID: "media_" + uuid.NewString(), UserID: user, APIKeyID: key, GroupID: group, Model: "example", MediaType: "image", Status: "queued", RequestedCount: 2, UnitPrice: 0.5, AccountUnitPrice: 0.25, HeldAmount: 1, CreatedAt: now, Deadline: now.Add(time.Hour), Units: []service.UnifiedMediaUnit{{State: "queued", Count: 1}, {State: "queued", Count: 1}}, Payload: service.UnifiedMediaTaskPayload{Normalized: &mediacapability.NormalizedRequest{Values: map[string]any{"model": "example", "prompt": "private-prompt"}, RequestedCount: 2}}}
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM usage_billing_dedup WHERE api_key_id=$1`, key)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM usage_logs WHERE user_id=$1`, user)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM unified_media_account_holds WHERE account_id=$1`, account)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM unified_media_tasks WHERE user_id=$1`, user)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM account_groups WHERE account_id=$1`, account)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id=$1`, account)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM api_keys WHERE id=$1`, key)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id=$1`, group)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, user)
	})
	return repo, task, account
}

// AC-006, AC-008 (transaction scope): partial success captures only its charge; repeated finalization cannot capture twice.
// AC-011 (repository scope): another key or media type cannot retrieve this persisted task.
func TestUnifiedMediaRepositoryCreateClaimPartialFinalize(t *testing.T) {
	ctx := context.Background()
	repo, task, accountID := unifiedMediaIntegrationFixture(t)
	require.NoError(t, repo.Create(ctx, task))
	var balance, frozen float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance,frozen_balance FROM users WHERE id=$1`, task.UserID).Scan(&balance, &frozen))
	require.Equal(t, 9.0, balance)
	require.Equal(t, 1.0, frozen)
	var ciphertext string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT payload_ciphertext FROM unified_media_tasks WHERE id=$1`, task.ID).Scan(&ciphertext))
	require.NotContains(t, ciphertext, "private-prompt")
	_, err := repo.GetOwned(ctx, task.UserID, task.APIKeyID+1, "image", task.ID)
	require.ErrorIs(t, err, service.ErrUnifiedMediaTaskNotFound)
	_, err = repo.GetOwned(ctx, task.UserID, task.APIKeyID, "video", task.ID)
	require.ErrorIs(t, err, service.ErrUnifiedMediaTaskNotFound)
	claims, err := repo.ClaimDue(ctx, time.Now(), 10, time.Now().Add(time.Hour))
	require.NoError(t, err)
	var claim *service.UnifiedMediaTask
	for _, candidate := range claims {
		if candidate.ID == task.ID {
			claim = candidate
		}
	}
	require.NotNil(t, claim)
	require.Equal(t, "private-prompt", claim.Payload.Normalized.Values["prompt"])
	claim.Units[0].State = "submitting"
	claim.Units[0].AccountID = accountID
	claim.Units[0].Attempt = 1
	require.NoError(t, repo.ReserveAccount(ctx, claim, 0, &service.Account{ID: accountID, Platform: "grsai"}, 0.25))
	require.NoError(t, repo.SettleAccount(ctx, claim, 0, true))
	require.NoError(t, repo.SettleAccount(ctx, claim, 0, true))
	expiry := time.Now().Add(time.Hour)
	claim.Units[0].State = "completed"
	claim.Units[0].Stored = []service.UnifiedMediaStoredResult{{URL: "https://s3/result", ExpiresAt: &expiry}}
	claim.Units[1].State = "failed"
	claim.Units[1].Error = "upstream failed"
	require.NoError(t, repo.Finalize(ctx, claim))
	require.NoError(t, repo.Finalize(ctx, claim))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance,frozen_balance FROM users WHERE id=$1`, task.UserID).Scan(&balance, &frozen))
	require.Equal(t, 9.5, balance)
	require.Zero(t, frozen)
	var used, accountUsed float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT quota_used FROM api_keys WHERE id=$1`, task.APIKeyID).Scan(&used))
	require.Equal(t, 0.5, used)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT (extra->>'quota_used')::numeric FROM accounts WHERE id=$1`, accountID).Scan(&accountUsed))
	require.Equal(t, 0.25, accountUsed)
	got, err := repo.GetOwned(ctx, task.UserID, task.APIKeyID, "image", task.ID)
	require.NoError(t, err)
	require.Equal(t, "completed", got.Status)
	require.Len(t, got.PublicView().Result.Data, 1)
	require.ErrorIs(t, repo.Save(ctx, claim, time.Now()), service.ErrUnifiedMediaClaimLost)
}
func TestUnifiedMediaRepositoryConcurrentKeyReservation(t *testing.T) {
	ctx := context.Background()
	repo, task, _ := unifiedMediaIntegrationFixture(t)
	var wg sync.WaitGroup
	errs := make(chan error, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			copy := *task
			copy.ID = "media_" + uuid.NewString()
			errs <- repo.Create(ctx, &copy)
		}()
	}
	wg.Wait()
	close(errs)
	success, rejected := 0, 0
	for err := range errs {
		switch {
		case err == nil:
			success++
		case errors.Is(err, service.ErrUnifiedMediaKeyQuota):
			rejected++
		default:
			require.NoError(t, err)
		}
	}
	require.Equal(t, 2, success)
	require.Equal(t, 1, rejected)
	var balance, frozen float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance,frozen_balance FROM users WHERE id=$1`, task.UserID).Scan(&balance, &frozen))
	require.Equal(t, 8.0, balance)
	require.Equal(t, 2.0, frozen)
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM unified_media_tasks WHERE api_key_id=$1`, task.APIKeyID).Scan(&count))
	require.Equal(t, 2, count)
}

var _ = json.RawMessage{}

type unifiedMediaIntegrationAccounts struct {
	service.AccountRepository
	account service.Account
}

func (a *unifiedMediaIntegrationAccounts) ListSchedulableByGroupID(context.Context, int64) ([]service.Account, error) {
	return []service.Account{a.account}, nil
}
func (a *unifiedMediaIntegrationAccounts) GetByID(context.Context, int64) (*service.Account, error) {
	copy := a.account
	return &copy, nil
}

type unifiedMediaIntegrationPrice struct{}

func (unifiedMediaIntegrationPrice) GrsaiUnitPrice(context.Context, string, *service.Group) (float64, error) {
	return 0.5, nil
}
func (unifiedMediaIntegrationPrice) ResolveGrsaiTaskPrice(context.Context, string, *service.Group, string) (service.GrsaiTaskPrice, error) {
	return service.GrsaiTaskPrice{Mode: service.BillingModeImage, UnitPrice: 0.5}, nil
}

type unifiedMediaIntegrationStorage struct{ saved int }

func (s *unifiedMediaIntegrationStorage) Save(ctx context.Context, key, contentType string, data []byte) (string, error) {
	metadata, err := s.SaveWithMetadata(ctx, key, contentType, data)
	return metadata.URL, err
}
func (s *unifiedMediaIntegrationStorage) SaveWithMetadata(_ context.Context, key, contentType string, data []byte) (service.ImageStorageSaveMetadata, error) {
	s.saved++
	expiry := time.Now().Add(time.Hour)
	return service.ImageStorageSaveMetadata{URL: "https://s3.example/" + key, ObjectKey: key, ContentType: contentType, LinkExpiresAt: &expiry}, nil
}

type unifiedMediaIntegrationPersister struct{ uploader *service.ImageResultUploader }

func (p unifiedMediaIntegrationPersister) Persist(ctx context.Context, id, media string, unit, index int, url string) (*service.UnifiedMediaStoredResult, error) {
	stored, err := p.uploader.PersistGrsaiImages(ctx, fmt.Sprintf("%s-%d-%d", id, unit, index), &service.GrsaiUpstreamResult{Status: service.GrsaiUpstreamStatusSucceeded, ImageURLs: []string{url}})
	if err != nil {
		return nil, err
	}
	var result struct {
		Results []struct {
			URL string `json:"url"`
		} `json:"results"`
	}
	if err = json.Unmarshal(stored.ResultJSON, &result); err != nil {
		return nil, err
	}
	return &service.UnifiedMediaStoredResult{URL: result.Results[0].URL, ExpiresAt: stored.LinkExpiresAt, ObjectMetadata: stored.ObjectMetadata}, nil
}

// AC-005, AC-023 (synthetic provider/storage scope): single and split requests return stored S3 results and settle exactly once.
func TestUnifiedMediaRepositoryHTTPWorkerEndToEnd(t *testing.T) {
	for _, count := range []int{1, 3} {
		t.Run(fmt.Sprintf("n_%d", count), func(t *testing.T) {
			ctx := context.Background()
			repo, fixture, accountID := unifiedMediaIntegrationFixture(t)
			model := "media-e2e-" + uuid.NewString()
			fixture.Model = model
			capRepo := NewMediaCapabilityRepository(integrationDB)
			caps := service.NewMediaCapabilityService(capRepo)
			cap, err := caps.Create(ctx, service.MediaCapabilityInput{PublicModel: model, MediaType: "image", Enabled: true, Schema: json.RawMessage(`{"version":1,"fields":{"prompt":{"type":"string","required":true},"n":{"type":"integer","minimum":1,"maximum":8,"default":1}},"batch":{"native_max_results":1,"allow_split":true}}`)})
			require.NoError(t, err)
			t.Cleanup(func() {
				_, _ = integrationDB.ExecContext(ctx, `DELETE FROM media_model_capabilities WHERE id=$1`, cap.ID)
			})
			routeRepo := NewCompositeModelRouteRepository(integrationEntClient)
			route := &service.CompositeModelRoute{GroupID: fixture.GroupID, PublicModel: model, MatchType: "exact", TargetPlatform: "grsai", UpstreamModel: model, Endpoint: "images", Priority: 100, Enabled: true}
			require.NoError(t, routeRepo.Create(ctx, route))
			t.Cleanup(func() {
				_, _ = integrationDB.ExecContext(ctx, `DELETE FROM composite_model_routes WHERE id=$1`, route.ID)
			})
			posts, polls, downloads := 0, 0, 0
			submitted := map[string]bool{}
			polled := map[string]int{}
			var upstream *httptest.Server
			upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v1/api/generate":
					posts++
					require.Equal(t, "Bearer synthetic-key", r.Header.Get("Authorization"))
					var body map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					require.Equal(t, model, body["model"])
					require.Equal(t, "async", body["replyType"])
					taskID := fmt.Sprintf("synthetic-task-%d", posts)
					submitted[taskID] = true
					_, _ = fmt.Fprintf(w, `{"id":%q,"status":"running"}`, taskID)
				case "/v1/api/result":
					polls++
					taskID := r.URL.Query().Get("id")
					require.True(t, submitted[taskID], "poll must use its submitted upstream task ID")
					polled[taskID]++
					_, _ = fmt.Fprintf(w, `{"id":%q,"status":"succeeded","results":[{"url":%q}]}`, taskID, upstream.URL+"/image.png?task="+taskID)
				case "/image.png":
					downloads++
					require.True(t, submitted[r.URL.Query().Get("task")], "download must belong to a submitted task")
					w.Header().Set("Content-Type", "image/png")
					_, _ = w.Write([]byte("\x89PNG\r\n\x1a\nsynthetic-test-image"))
				default:
					w.WriteHeader(404)
				}
			}))
			defer upstream.Close()
			account := service.Account{ID: accountID, Platform: "grsai", Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"base_url": upstream.URL, "api_key": "synthetic-key"}}
			storage := &unifiedMediaIntegrationStorage{}
			persister := unifiedMediaIntegrationPersister{uploader: service.NewImageResultUploader(storage, "media/", 1024, upstream.Client())}
			svc := service.NewUnifiedMediaTaskService(repo, caps, service.NewCompositeMediaRouteService(service.NewCompositeRouteResolver(routeRepo), capRepo), &unifiedMediaIntegrationAccounts{account: account}, unifiedMediaIntegrationPrice{}, service.NewUnifiedMediaUpstream(upstream.Client()), persister)
			svc.PollInterval = time.Millisecond
			now := time.Now().UTC()
			svc.Now = func() time.Time { return now }
			group := &service.Group{ID: fixture.GroupID, Platform: service.PlatformComposite, RateMultiplier: 1, SubscriptionType: service.SubscriptionTypeStandard}
			key := &service.APIKey{ID: fixture.APIKeyID, UserID: fixture.UserID, GroupID: &fixture.GroupID, Group: group}
			task, err := svc.Create(ctx, service.UnifiedMediaTaskCreateInput{APIKey: key, MediaType: "image", Body: []byte(fmt.Sprintf(`{"model":%q,"prompt":"synthetic cat","n":%d}`, model, count))})
			require.NoError(t, err)
			require.Equal(t, "queued", task.PublicView().Status)
			require.Zero(t, posts)
			require.Equal(t, count, task.RequestedCount)
			require.Len(t, task.Units, count)
			for _, unit := range task.Units {
				require.Equal(t, 1, unit.Count, "native_max_results=1 must split the requested count")
			}
			require.Equal(t, float64(count)*0.5, task.HeldAmount)
			var result *service.UnifiedMediaTask
			for i := 0; i < 8; i++ {
				require.NoError(t, svc.RunOnce(ctx))
				result, err = svc.GetOwned(ctx, fixture.UserID, fixture.APIKeyID, "image", task.ID)
				require.NoError(t, err)
				if result.Status == "completed" {
					break
				}
				now = now.Add(time.Second)
			}
			require.Equal(t, "completed", result.Status)
			require.Equal(t, 100, result.PublicView().Progress)
			require.Empty(t, result.PublicView().Error)
			require.Len(t, result.PublicView().Result.Data, count)
			for _, url := range result.PublicView().Result.Data {
				require.Contains(t, url, "https://s3.example/")
			}
			require.Equal(t, count, posts)
			require.Equal(t, count, polls)
			require.Equal(t, count, downloads)
			require.Equal(t, count, storage.saved)
			require.Len(t, submitted, count)
			require.Len(t, polled, count)
			for _, times := range polled {
				require.Equal(t, 1, times)
			}
			require.NoError(t, svc.RunOnce(ctx))
			require.Equal(t, count, posts)
			require.Equal(t, count, polls)
			require.Equal(t, count, downloads)
			require.Equal(t, count, storage.saved)
			var used, charge float64
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT quota_used FROM api_keys WHERE id=$1`, fixture.APIKeyID).Scan(&used))
			require.Equal(t, float64(count)*0.5, used)
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT SUM(actual_cost) FROM usage_logs WHERE request_id LIKE $1`, "media_task:"+task.ID+":%").Scan(&charge))
			require.Equal(t, used, charge)
			var balance, frozen float64
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance,frozen_balance FROM users WHERE id=$1`, fixture.UserID).Scan(&balance, &frozen))
			require.Equal(t, 10-float64(count)*0.5, balance)
			require.Zero(t, frozen)
			var usageRows int
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_logs WHERE request_id LIKE $1`, "media_task:"+task.ID+":%").Scan(&usageRows))
			require.Equal(t, count, usageRows)
		})
	}
}

func claimUnifiedMediaIntegrationTask(t *testing.T, repo service.UnifiedMediaTaskRepository, id string) *service.UnifiedMediaTask {
	t.Helper()
	claims, err := repo.ClaimDue(context.Background(), time.Now(), 20, time.Now().Add(time.Hour))
	require.NoError(t, err)
	for _, task := range claims {
		if task.ID == id {
			return task
		}
	}
	t.Fatalf("task %s was not claimed", id)
	return nil
}

// AC-007, AC-008 (transaction scope): all failures release the full hold, with completed/error and no expiry or usage charge.
func TestUnifiedMediaRepositoryFailedTaskReleasesEntireReservation(t *testing.T) {
	ctx := context.Background()
	repo, task, _ := unifiedMediaIntegrationFixture(t)
	require.NoError(t, repo.Create(ctx, task))
	claim := claimUnifiedMediaIntegrationTask(t, repo, task.ID)
	for i := range claim.Units {
		claim.Units[i].State = "failed"
		claim.Units[i].Error = "upstream_generation_failed"
	}
	require.NoError(t, repo.Finalize(ctx, claim))
	require.NoError(t, repo.Finalize(ctx, claim))
	var balance, frozen, used float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT u.balance,u.frozen_balance,k.quota_used FROM users u JOIN api_keys k ON k.user_id=u.id WHERE k.id=$1`, task.APIKeyID).Scan(&balance, &frozen, &used))
	require.Equal(t, 10.0, balance)
	require.Zero(t, frozen)
	require.Zero(t, used)
	got, err := repo.GetOwned(ctx, task.UserID, task.APIKeyID, "image", task.ID)
	require.NoError(t, err)
	require.Equal(t, "completed", got.Status)
	require.NotNil(t, got.CompletedAt)
	require.Nil(t, got.ExpiresAt)
	require.Empty(t, got.PublicView().Result.Data)
	require.NotEmpty(t, got.PublicView().Error)
	var logs int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM usage_logs WHERE api_key_id=$1`, task.APIKeyID).Scan(&logs))
	require.Zero(t, logs)
}

func TestUnifiedMediaRepositoryAccountLimitsAreShared(t *testing.T) {
	for _, mode := range []string{"quota", "concurrency"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			repo, first, accountID := unifiedMediaIntegrationFixture(t)
			extra, concurrency := `{"quota_limit":0.5}`, 3
			if mode == "concurrency" {
				extra = `{"quota_limit":100}`
				concurrency = 1
			}
			_, err := integrationDB.ExecContext(ctx, `UPDATE accounts SET extra=$1::jsonb,concurrency=$2 WHERE id=$3`, extra, concurrency, accountID)
			require.NoError(t, err)
			second := *first
			second.ID = "media_" + uuid.NewString()
			require.NoError(t, repo.Create(ctx, first))
			require.NoError(t, repo.Create(ctx, &second))
			claims, err := repo.ClaimDue(ctx, time.Now(), 20, time.Now().Add(time.Hour))
			require.NoError(t, err)
			var a, b *service.UnifiedMediaTask
			for _, claim := range claims {
				if claim.ID == first.ID {
					a = claim
				}
				if claim.ID == second.ID {
					b = claim
				}
			}
			require.NotNil(t, a)
			require.NotNil(t, b)
			account := &service.Account{ID: accountID, Platform: "grsai"}
			for _, claim := range []*service.UnifiedMediaTask{a, b} {
				claim.Units[0].State = "submitting"
				claim.Units[0].AccountID = accountID
				claim.Units[0].Attempt = 1
			}
			require.NoError(t, repo.ReserveAccount(ctx, a, 0, account, 0.4))
			reserveErr := repo.ReserveAccount(ctx, b, 0, account, 0.4)
			if mode == "concurrency" {
				require.ErrorIs(t, reserveErr, service.ErrUnifiedMediaAccountBusy)
			} else {
				require.ErrorIs(t, reserveErr, service.ErrUnifiedMediaAccountUnavailable)
			}
			denied, err := repo.GetOwned(ctx, b.UserID, b.APIKeyID, b.MediaType, b.ID)
			require.NoError(t, err)
			require.Equal(t, "queued", denied.Units[0].State, "denied reservation must not persist a submitting intent")
			require.Zero(t, denied.Units[0].Attempt)
			require.Zero(t, denied.Units[0].AccountID)
			var deniedHolds int
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM unified_media_account_holds WHERE task_id=$1`, b.ID).Scan(&deniedHolds))
			require.Zero(t, deniedHolds)
			require.NoError(t, repo.SettleAccount(ctx, a, 0, false))
			require.NoError(t, repo.ReserveAccount(ctx, b, 0, account, 0.4))
			require.NoError(t, repo.SettleAccount(ctx, b, 0, true))
			if mode == "quota" {
				a.Units[0].Attempt = 2
				require.ErrorIs(t, repo.ReserveAccount(ctx, a, 0, account, 0.4), service.ErrUnifiedMediaAccountUnavailable)
				require.NoError(t, repo.ReserveAccount(ctx, a, 0, account, 0.1))
			}
		})
	}
}

func TestUnifiedMediaRepositoryQuotaOrDisabledAccountIsUnavailableEvenWhenBusy(t *testing.T) {
	ctx := context.Background()
	repo, first, accountID := unifiedMediaIntegrationFixture(t)
	second := *first
	second.ID = "media_" + uuid.NewString()
	require.NoError(t, repo.Create(ctx, first))
	a := claimUnifiedMediaIntegrationTask(t, repo, first.ID)
	require.NoError(t, repo.Create(ctx, &second))
	b := claimUnifiedMediaIntegrationTask(t, repo, second.ID)
	for _, task := range []*service.UnifiedMediaTask{a, b} {
		task.Units[0].State = "submitting"
		task.Units[0].AccountID = accountID
		task.Units[0].Attempt = 1
	}
	_, err := integrationDB.ExecContext(ctx, `UPDATE accounts SET extra='{"quota_limit":0.5}'::jsonb,concurrency=1 WHERE id=$1`, accountID)
	require.NoError(t, err)
	account := &service.Account{ID: accountID, Platform: "grsai"}
	require.NoError(t, repo.ReserveAccount(ctx, a, 0, account, 0.4))
	err = repo.ReserveAccount(ctx, b, 0, account, 0.4)
	require.ErrorIs(t, err, service.ErrUnifiedMediaAccountUnavailable, "monetary quota refusal retains terminal semantics even when all slots are occupied")
	require.NotErrorIs(t, err, service.ErrUnifiedMediaAccountBusy)
	require.NoError(t, repo.SettleAccount(ctx, a, 0, false))
	_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET extra='{"quota_limit":100}'::jsonb,concurrency=0 WHERE id=$1`, accountID)
	require.NoError(t, err)
	err = repo.ReserveAccount(ctx, b, 0, account, 0.4)
	require.ErrorIs(t, err, service.ErrUnifiedMediaAccountUnavailable)
	require.NotErrorIs(t, err, service.ErrUnifiedMediaAccountBusy)
}

type unifiedMediaIntegrationNoSubmit struct{ submits int }

func (u *unifiedMediaIntegrationNoSubmit) Submit(context.Context, *service.Account, string, string, *mediacapability.NormalizedRequest, int) (*service.UnifiedMediaUpstreamResult, error) {
	u.submits++
	return nil, errors.New("unexpected submit")
}
func (u *unifiedMediaIntegrationNoSubmit) Poll(context.Context, *service.Account, string, string, string) (*service.UnifiedMediaUpstreamResult, error) {
	return nil, errors.New("unexpected poll")
}
func TestUnifiedMediaRepositoryLostLeaseRecoversSubmissionWithoutResubmit(t *testing.T) {
	ctx := context.Background()
	repo, task, accountID := unifiedMediaIntegrationFixture(t)
	require.NoError(t, repo.Create(ctx, task))
	stale := claimUnifiedMediaIntegrationTask(t, repo, task.ID)
	stale.Units[0].State = "submitting"
	stale.Units[0].Attempt = 1
	stale.Units[0].AccountID = accountID
	require.NoError(t, repo.ReserveAccount(ctx, stale, 0, &service.Account{ID: accountID, Platform: "grsai"}, 0.25))
	_, err := integrationDB.ExecContext(ctx, `UPDATE unified_media_tasks SET lease_until=NOW()-INTERVAL '1 second',next_attempt_at=NOW()-INTERVAL '1 second' WHERE id=$1`, task.ID)
	require.NoError(t, err)
	upstream := &unifiedMediaIntegrationNoSubmit{}
	svc := service.NewUnifiedMediaTaskService(repo, nil, nil, &unifiedMediaIntegrationAccounts{}, nil, upstream, unifiedMediaIntegrationPersister{})
	require.NoError(t, svc.RunOnce(ctx))
	require.Zero(t, upstream.submits)
	require.ErrorIs(t, repo.Save(ctx, stale, time.Now()), service.ErrUnifiedMediaClaimLost)
	require.ErrorIs(t, repo.SettleAccount(ctx, stale, 0, false), service.ErrUnifiedMediaClaimLost)
	got, err := repo.GetOwned(ctx, task.UserID, task.APIKeyID, "image", task.ID)
	require.NoError(t, err)
	require.Equal(t, "completed", got.Status)
	require.Contains(t, got.PublicView().Error, "upstream_submission_outcome_unknown")
	require.Empty(t, got.PublicView().Result.Data)
	var balance, frozen, used, accountUsed float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT u.balance,u.frozen_balance,k.quota_used FROM users u JOIN api_keys k ON k.user_id=u.id WHERE k.id=$1`, task.APIKeyID).Scan(&balance, &frozen, &used))
	require.Equal(t, 10.0, balance)
	require.Zero(t, frozen)
	require.Zero(t, used)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT (extra->>'quota_used')::numeric FROM accounts WHERE id=$1`, accountID).Scan(&accountUsed))
	require.Equal(t, 0.25, accountUsed)
}

func TestUnifiedMediaRepositorySoftDeletedKeyAndAccountCanFinishReservedWork(t *testing.T) {
	ctx := context.Background()
	repo, task, accountID := unifiedMediaIntegrationFixture(t)
	require.NoError(t, repo.Create(ctx, task))
	claim := claimUnifiedMediaIntegrationTask(t, repo, task.ID)
	claim.Units[0].State = "submitting"
	claim.Units[0].Attempt = 1
	claim.Units[0].AccountID = accountID
	require.NoError(t, repo.ReserveAccount(ctx, claim, 0, &service.Account{ID: accountID, Platform: "grsai"}, 0.25))
	_, err := integrationDB.ExecContext(ctx, `UPDATE api_keys SET deleted_at=NOW() WHERE id=$1`, task.APIKeyID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET deleted_at=NOW() WHERE id=$1`, accountID)
	require.NoError(t, err)
	claim.Units[0].State = "completed"
	claim.Units[0].Stored = []service.UnifiedMediaStoredResult{{URL: "https://s3/result"}}
	claim.Units[1].State = "failed"
	claim.Units[1].Error = "upstream_generation_failed"
	require.NoError(t, repo.Finalize(ctx, claim))
	require.NoError(t, repo.Finalize(ctx, claim))
	var balance, frozen, used, accountUsed float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT u.balance,u.frozen_balance,k.quota_used FROM users u JOIN api_keys k ON k.user_id=u.id WHERE k.id=$1`, task.APIKeyID).Scan(&balance, &frozen, &used))
	require.Equal(t, 9.5, balance)
	require.Zero(t, frozen)
	require.Equal(t, 0.5, used)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT (extra->>'quota_used')::numeric FROM accounts WHERE id=$1`, accountID).Scan(&accountUsed))
	require.Equal(t, 0.25, accountUsed)
}
