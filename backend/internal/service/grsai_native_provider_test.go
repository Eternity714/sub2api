package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func grsaiNativeMediaTestAccount(baseURL string) *Account {
	return &Account{ID: 3, Platform: PlatformGrsai, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": baseURL, "api_key": "custom-media-key"}}
}

func TestGrsaiNativeClientCustomBaseURLAvoidsDuplicateV1(t *testing.T) {
	for _, suffix := range []string{"/tenant", "/tenant/v1/"} {
		t.Run(suffix, func(t *testing.T) {
			var paths []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				require.Equal(t, "Bearer custom-media-key", r.Header.Get("Authorization"))
				_, _ = io.WriteString(w, `{"id":"task-custom","status":"running"}`)
			}))
			defer server.Close()
			account := grsaiNativeMediaTestAccount(server.URL + suffix)
			client := NewGrsaiAsyncNativeClient(server.Client())
			_, err := client.Generate(context.Background(), account, []byte(`{"model":"media-model"}`))
			require.NoError(t, err)
			_, err = client.GenerateAsync(context.Background(), account, []byte(`{"model":"media-model","replyType":"async"}`))
			require.NoError(t, err)
			_, err = client.Result(context.Background(), account, "task-custom")
			require.NoError(t, err)
			require.Equal(t, []string{"/tenant/v1/api/generate", "/tenant/v1/api/generate", "/tenant/v1/api/result"}, paths)
		})
	}
}

func TestGrsaiNativeClientRejectsAllOpenAIAccountsBeforeHTTP(t *testing.T) {
	for _, tc := range []struct{ name, accountType, baseURL, key string }{
		{"custom API key account", AccountTypeAPIKey, "https://media.example.test/v1", "key"},
		{"OAuth", AccountTypeOAuth, "https://media.example.test", "key"},
		{"default official account", AccountTypeAPIKey, "", "key"},
		{"explicit official account", AccountTypeAPIKey, "https://api.openai.com/v1", "key"},
		{"missing key", AccountTypeAPIKey, "https://media.example.test", ""},
		{"invalid URL", AccountTypeAPIKey, "file:///tmp/media", "key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			client := NewGrsaiNativeClient(&http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				requests++
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"status":"running"}`))}, nil
			})})
			account := &Account{Platform: PlatformOpenAI, Type: tc.accountType, Credentials: map[string]any{"base_url": tc.baseURL, "api_key": tc.key}}
			_, err := client.Generate(context.Background(), account, []byte(`{"model":"media-model"}`))
			require.ErrorIs(t, err, ErrGrsaiInvalidAccount)
			require.Zero(t, requests)
		})
	}
}

func TestGrsaiTaskServiceNativeMediaPricing(t *testing.T) {
	for _, tc := range []struct {
		name, body, mediaKind string
		price                 GrsaiTaskPrice
		wantAmount            float64
	}{
		{"image", `{"model":"media-model","replyType":"async"}`, "image", GrsaiTaskPrice{Mode: BillingModeImage, UnitPrice: .25}, .25},
		{"video", `{"model":"minimax-h3","duration":5,"resolution":"768p","replyType":"async"}`, "video", GrsaiTaskPrice{Mode: BillingModeVideo, UnitPrice: .14, Resolution: "768p"}, .7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &grsaiTaskCreateRepoStub{}
			group := &Group{ID: 2, Platform: PlatformGrsai, RateMultiplier: 1}
			svc := &GrsaiTaskService{Repo: repo, Balance: grsaiRuntimeBalanceStub{}, Pricing: grsaiVideoPriceStub{price: tc.price}, Enabled: true}
			record, err := svc.Create(context.Background(), GrsaiTaskCreateInput{Account: grsaiNativeMediaTestAccount("https://arbitrary-upstream.example.test/v1"),
				APIKey: &APIKey{ID: 4, UserID: 1, GroupID: &group.ID, Group: group}, Body: []byte(tc.body)})
			require.NoError(t, err)
			require.NotNil(t, record.LocalTaskID)
			require.Equal(t, tc.mediaKind, repo.params.MediaKind)
			require.InDelta(t, tc.wantAmount, repo.params.BillableUnitPrice, 1e-10)
		})
	}
}

func TestGrsaiSettlementNativeAccountBillsOnce(t *testing.T) {
	repo, billing := &grsaiSettlementMemoryRepo{}, &grsaiBillingSpy{}
	svc := &GrsaiSettlementService{Repo: repo, Billing: billing, Pricing: &grsaiPriceStub{price: .25}}
	group := &Group{ID: 2, Platform: PlatformGrsai, RateMultiplier: 1}
	record, err := svc.Prepare(context.Background(), GrsaiPrepareInput{Account: grsaiNativeMediaTestAccount("https://media.example.test"),
		APIKey: &APIKey{ID: 4, UserID: 1, GroupID: &group.ID, Group: group}, Model: "media-model", ImageCount: 1})
	require.NoError(t, err)
	upstream := &GrsaiUpstreamResult{TaskID: "custom-task", Status: GrsaiUpstreamStatusSucceeded}
	require.NoError(t, svc.Finish(context.Background(), record, upstream, nil).SettlementError)
	require.NoError(t, svc.Finish(context.Background(), record, upstream, nil).SettlementError)
	require.Len(t, billing.commands, 1)
	require.InDelta(t, .25, *repo.record.SettledAmount, 1e-10)
}

func TestGrsaiTaskRuntimeNativeAccountSubmitsAndPolls(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.Method == http.MethodPost {
			_, _ = io.WriteString(w, `{"id":"custom-task","status":"running"}`)
		} else {
			_, _ = io.WriteString(w, `{"id":"custom-task","status":"succeeded","results":[{"url":"https://output.example.test/a"}]}`)
		}
	}))
	defer server.Close()
	repo := &grsaiRuntimeRepoStub{}
	runtime := NewGrsaiTaskRuntime(repo, &grsaiRuntimePayloadStub{}, grsaiRecoveryAccounts{account: grsaiNativeMediaTestAccount(server.URL)},
		NewGrsaiAsyncNativeClient(server.Client()), &grsaiRuntimeImagesStub{}, grsaiRuntimeBalanceStub{}, GrsaiTaskRuntimeOptions{})
	claim := grsaiRuntimeClaim()
	require.NoError(t, runtime.processClaim(context.Background(), claim))
	taskID := "custom-task"
	claim.UpstreamTaskID = &taskID
	require.NoError(t, runtime.processClaim(context.Background(), claim))
	require.Equal(t, []string{"/v1/api/generate", "/v1/api/result"}, paths)
	require.Equal(t, 1, repo.completed)
	require.Zero(t, repo.failed)
	require.InDelta(t, .25, repo.amount, 1e-10)
}

func TestGrsaiSettlementRecoveryNativeAccountPollsWithoutResubmitting(t *testing.T) {
	now := time.Now().UTC()
	taskID := "custom-task"
	client := &grsaiRecoveryClient{result: &GrsaiUpstreamResult{TaskID: taskID, Status: GrsaiUpstreamStatusSucceeded}}
	billing := &grsaiRecoveryBilling{}
	runtime, repo := newGrsaiRecoveryRuntime(now, grsaiRecoveryRecord(now, &taskID, GrsaiUpstreamStatusRunning, "pending_upstream"), client, billing)
	runtime.accounts = grsaiRecoveryAccounts{account: grsaiNativeMediaTestAccount("https://media.example.test")}
	runtime.RunOnce(context.Background())
	require.Equal(t, "settled", repo.record.InternalStatus)
	require.Len(t, billing.commands, 1)
	require.Zero(t, client.generateCalls)
}

func TestGrsaiTaskServiceRejectsOpenAICustomAccountBeforeTaskCreation(t *testing.T) {
	account := grsaiNativeMediaTestAccount("https://media.example.test")
	account.Platform = PlatformOpenAI
	group := &Group{ID: 2, Platform: PlatformOpenAI, RateMultiplier: 1}
	repo := &grsaiTaskCreateRepoStub{}
	svc := &GrsaiTaskService{Repo: repo, Balance: grsaiRuntimeBalanceStub{}, Pricing: &grsaiPriceStub{price: .25}, Enabled: true}
	record, err := svc.Create(context.Background(), GrsaiTaskCreateInput{Account: account,
		APIKey: &APIKey{ID: 4, UserID: 1, GroupID: &group.ID, Group: group}, Body: []byte(`{"model":"media-model","replyType":"async"}`)})
	require.ErrorIs(t, err, ErrGrsaiSettlementInvalidInput)
	require.Nil(t, record)
	require.Zero(t, repo.called)
}

func TestGrsaiSettlementRejectsOpenAICustomAccountBeforePreparing(t *testing.T) {
	account := grsaiNativeMediaTestAccount("https://media.example.test")
	account.Platform = PlatformOpenAI
	group := &Group{ID: 2, Platform: PlatformOpenAI, RateMultiplier: 1}
	repo := &grsaiSettlementMemoryRepo{}
	svc := &GrsaiSettlementService{Repo: repo, Billing: &grsaiBillingSpy{}, Pricing: &grsaiPriceStub{price: .25}}
	record, err := svc.Prepare(context.Background(), GrsaiPrepareInput{Account: account,
		APIKey: &APIKey{ID: 4, UserID: 1, GroupID: &group.ID, Group: group}, Model: "media-model", ImageCount: 1})
	require.ErrorIs(t, err, ErrGrsaiSettlementInvalidInput)
	require.Nil(t, record)
	require.Nil(t, repo.record)
}

func TestGrsaiTaskRuntimeRejectsOpenAIAccountBeforeSubmittingOrPolling(t *testing.T) {
	for _, bound := range []bool{false, true} {
		t.Run(map[bool]string{false: "submit", true: "poll"}[bound], func(t *testing.T) {
			account := grsaiNativeMediaTestAccount("https://media.example.test")
			account.Platform = PlatformOpenAI
			repo, upstream := &grsaiRuntimeRepoStub{}, &grsaiRuntimeUpstreamStub{result: &GrsaiUpstreamResult{Status: GrsaiUpstreamStatusRunning}}
			runtime := NewGrsaiTaskRuntime(repo, &grsaiRuntimePayloadStub{}, grsaiRecoveryAccounts{account: account},
				upstream, &grsaiRuntimeImagesStub{}, grsaiRuntimeBalanceStub{}, GrsaiTaskRuntimeOptions{})
			claim := grsaiRuntimeClaim()
			if bound {
				id := "custom-task"
				claim.UpstreamTaskID = &id
			}
			require.NoError(t, runtime.processClaim(context.Background(), claim))
			require.Zero(t, upstream.posts)
			require.Zero(t, upstream.polls)
			require.Zero(t, repo.bound)
			if bound {
				require.Equal(t, 1, repo.deferred)
			} else {
				require.Equal(t, 1, repo.failed)
			}
		})
	}
}

func TestGrsaiSettlementRecoveryRejectsOpenAIAccountBeforePolling(t *testing.T) {
	now := time.Now().UTC()
	id := "custom-task"
	client := &grsaiRecoveryClient{result: &GrsaiUpstreamResult{TaskID: id, Status: GrsaiUpstreamStatusSucceeded}}
	billing := &grsaiRecoveryBilling{}
	runtime, repo := newGrsaiRecoveryRuntime(now, grsaiRecoveryRecord(now, &id, GrsaiUpstreamStatusRunning, "pending_upstream"), client, billing)
	account := grsaiNativeMediaTestAccount("https://media.example.test")
	account.Platform = PlatformOpenAI
	runtime.accounts = grsaiRecoveryAccounts{account: account}
	runtime.RunOnce(context.Background())
	require.Equal(t, "pending_upstream", repo.record.InternalStatus)
	require.Empty(t, billing.commands)
	require.Zero(t, client.resultCalls)
	require.Zero(t, client.generateCalls)
}
