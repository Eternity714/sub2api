package handler

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type grsaiGenerateSelector struct{ account *service.Account }

func (s grsaiGenerateSelector) SelectAccountWithLoadAwareness(context.Context, *int64, string, string, map[int64]struct{}, string, int64) (*service.AccountSelectionResult, error) {
	return &service.AccountSelectionResult{Account: s.account, Acquired: true, ReleaseFunc: func() {}}, nil
}
func (grsaiGenerateSelector) ResolveUserGroupRateMultiplier(_ context.Context, _, _ int64, rate float64) float64 {
	return rate
}

type grsaiGenerateEligibility struct{}

func (grsaiGenerateEligibility) CheckBillingEligibility(context.Context, *service.User, *service.APIKey, *service.Group, *service.UserSubscription, string) error {
	return nil
}

type grsaiGeneratePrice struct{}

func (grsaiGeneratePrice) GrsaiUnitPrice(context.Context, string, *service.Group) (float64, error) {
	return 0, nil
}

type grsaiGenerateBilling struct {
	err   error
	calls int
}

func (b *grsaiGenerateBilling) ApplyTx(context.Context, *sql.Tx, *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	b.calls++
	if b.err != nil {
		return nil, b.err
	}
	return &service.UsageBillingApplyResult{Applied: true}, nil
}

type grsaiGenerateRepo struct {
	service.GrsaiSettlementRepository
	record    *service.GrsaiSettlement
	events    int
	queueFull bool
}

func (r *grsaiGenerateRepo) Create(_ context.Context, p service.CreateGrsaiSettlementParams) (*service.GrsaiSettlement, error) {
	if r.queueFull && p.DeliveryMode == service.GrsaiDeliveryAsync {
		return nil, service.ErrGrsaiAsyncQueueFull
	}
	r.record = &service.GrsaiSettlement{
		ID: 19, AccountID: p.AccountID, GroupID: p.GroupID, UserID: p.UserID, APIKeyID: p.APIKeyID,
		Model: p.Model, BaseUnitPrice: p.BaseUnitPrice, GroupRateMultiplier: p.GroupRateMultiplier,
		AccountRateMultiplier: p.AccountRateMultiplier, BillableUnitPrice: p.BillableUnitPrice,
		RequestedImageCount: p.RequestedImageCount, ImageSize: p.ImageSize, Currency: p.Currency,
		BillingIdempotencyKey: "grsai_settlement:19", PublicTaskID: "public-19", DeliveryMode: p.DeliveryMode,
		InternalStatus: "pending_upstream", UpstreamStatus: "not_submitted", HoldState: "none",
		ClaimVersion: 0, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	return r.GetByID(context.Background(), 19)
}
func (r *grsaiGenerateRepo) GetByID(context.Context, int64) (*service.GrsaiSettlement, error) {
	copy := *r.record
	return &copy, nil
}
func (r *grsaiGenerateRepo) GetOwnedByPublicOrUpstreamID(_ context.Context, userID, apiKeyID int64, id string) (*service.GrsaiSettlement, error) {
	if r.record == nil || r.record.UserID != userID || r.record.APIKeyID != apiKeyID ||
		(id != r.record.PublicTaskID && (r.record.UpstreamTaskID == nil || id != *r.record.UpstreamTaskID)) {
		return nil, service.ErrGrsaiSettlementNotFound
	}
	return r.GetByID(context.Background(), r.record.ID)
}
func (r *grsaiGenerateRepo) ClaimByID(_ context.Context, _ int64, _ time.Time, lease time.Time) (*service.GrsaiSettlement, error) {
	r.record.InternalStatus = "processing"
	r.record.ClaimVersion++
	r.record.NextAttemptAt = lease
	return r.GetByID(context.Background(), r.record.ID)
}
func (r *grsaiGenerateRepo) MarkPendingUpstream(_ context.Context, _, _ int64, _ time.Time) error {
	r.record.InternalStatus = "pending_upstream"
	return nil
}
func (r *grsaiGenerateRepo) MarkManualReview(_ context.Context, _, _ int64, _ string) error {
	r.record.InternalStatus = "manual_review"
	return nil
}
func (r *grsaiGenerateRepo) MarkSubmitting(_ context.Context, _, version int64) (bool, error) {
	if r.record.ClaimVersion != version || r.record.UpstreamStatus != "not_submitted" {
		return false, nil
	}
	r.record.UpstreamStatus = "submitting"
	return true, nil
}
func (r *grsaiGenerateRepo) BindAndRecordStreamEvent(_ context.Context, _, _ int64, event service.GrsaiStreamEvent) (bool, error) {
	id := event.TaskID
	r.record.UpstreamTaskID = &id
	r.record.UpstreamStatus, r.record.Progress = event.Status, event.Progress
	r.record.ResultURLs = append([]string(nil), event.ResultURLs...)
	r.events++
	return true, nil
}
func (r *grsaiGenerateRepo) RecordStreamEvent(_ context.Context, _, _ int64, event service.GrsaiStreamEvent) (bool, error) {
	r.record.UpstreamStatus, r.record.Progress = event.Status, event.Progress
	r.record.ResultURLs = append([]string(nil), event.ResultURLs...)
	r.events++
	return true, nil
}
func (r *grsaiGenerateRepo) Settle(ctx context.Context, _, _ int64, amount float64, apply service.GrsaiSettlementTxFunc) (bool, error) {
	if err := apply(ctx, new(sql.Tx), r.record); err != nil {
		return false, err
	}
	r.record.InternalStatus = "settled"
	r.record.SettledAmount = &amount
	return true, nil
}
func (r *grsaiGenerateRepo) MarkPendingSettlement(_ context.Context, _, _ int64, next time.Time) error {
	r.record.InternalStatus, r.record.NextAttemptAt = "pending_settlement", next
	return nil
}

type grsaiGeneratePayload struct{ body []byte }

func (p *grsaiGeneratePayload) PutEncrypted(_ context.Context, _ int64, body []byte, _ time.Time) error {
	p.body = append([]byte(nil), body...)
	return nil
}
func (p *grsaiGeneratePayload) GetEncrypted(context.Context, int64) ([]byte, error) {
	return p.body, nil
}
func (p *grsaiGeneratePayload) DeleteBySettlementID(context.Context, int64) error {
	p.body = nil
	return nil
}
func (p *grsaiGeneratePayload) DeleteExpired(context.Context, time.Time) (int64, error) {
	return 0, nil
}

type grsaiGenerateAccountReader struct{ account *service.Account }

func (r grsaiGenerateAccountReader) GetByID(context.Context, int64) (*service.Account, error) {
	return r.account, nil
}

type grsaiGenerateUpstream struct {
	posts   int
	request []byte
	stream  string
}

func (u *grsaiGenerateUpstream) Generate(context.Context, *service.Account, []byte) (*service.GrsaiUpstreamResult, error) {
	return nil, errors.New("unexpected nonstream POST")
}
func (u *grsaiGenerateUpstream) Result(context.Context, *service.Account, string) (*service.GrsaiUpstreamResult, error) {
	return nil, errors.New("unexpected result query")
}
func (u *grsaiGenerateUpstream) OpenGenerateStream(_ context.Context, _ *service.Account, body []byte) (*service.GrsaiUpstreamStream, error) {
	u.posts++
	u.request = append([]byte(nil), body...)
	return &service.GrsaiUpstreamStream{StatusCode: 200, ContentType: "text/event-stream", Body: io.NopCloser(strings.NewReader(u.stream))}, nil
}

type grsaiGenerateFixture struct {
	handler  *GrsaiGatewayHandler
	repo     *grsaiGenerateRepo
	payload  *grsaiGeneratePayload
	upstream *grsaiGenerateUpstream
	billing  *grsaiGenerateBilling
}

func newGrsaiGenerateFixture() *grsaiGenerateFixture {
	account := &service.Account{ID: 3, Platform: service.PlatformGrsai, Type: service.AccountTypeAPIKey}
	repo := &grsaiGenerateRepo{}
	payload := &grsaiGeneratePayload{}
	upstream := &grsaiGenerateUpstream{stream: "data: {\"id\":\"up-19\",\"status\":\"running\",\"progress\":20}\n\ndata: {\"id\":\"up-19\",\"status\":\"succeeded\",\"progress\":100,\"results\":[{\"url\":\"https://img.invalid/19\"}]}\n\n"}
	billing := &grsaiGenerateBilling{}
	settlement := &service.GrsaiSettlementService{Repo: repo, Billing: billing, Pricing: grsaiGeneratePrice{}}
	tasks := &service.GrsaiTaskService{Settlement: settlement, Repo: repo, Payloads: payload, Accounts: grsaiGenerateAccountReader{account}, Upstream: upstream, Options: service.GrsaiTaskOptions{Enabled: true, StreamEnabled: true}}
	cache := &concurrencyCacheMock{acquireUserSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil }}
	return &grsaiGenerateFixture{handler: &GrsaiGatewayHandler{
		gatewayService: grsaiGenerateSelector{account}, billingCacheService: grsaiGenerateEligibility{},
		nativeClient: upstream, settlementService: settlement, taskService: tasks,
		concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, 0),
	}, repo: repo, payload: payload, upstream: upstream, billing: billing}
}

func (f *grsaiGenerateFixture) request(t *testing.T, mode string, writer http.ResponseWriter, ctx context.Context) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder, ok := writer.(*httptest.ResponseRecorder)
	if writer == nil {
		recorder = httptest.NewRecorder()
		writer = recorder
	} else if !ok {
		recorder = nil
	}
	c, _ := gin.CreateTestContext(writer)
	body := `{"model":"nano-banana-2-lite","prompt":"private","replyType":"` + mode + `"}`
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/api/generate", strings.NewReader(body)).WithContext(ctx)
	groupID := int64(2)
	group := &service.Group{ID: groupID, Platform: service.PlatformGrsai, RateMultiplier: 1, AllowImageGeneration: true}
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 4, UserID: 7, GroupID: &groupID, Group: group, User: &service.User{ID: 7}})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7, Concurrency: 1})
	return c, recorder
}

// @covers AC-003
func TestGrsaiGenerateAsyncReturns202WithoutPosting(t *testing.T) {
	f := newGrsaiGenerateFixture()
	c, recorder := f.request(t, "async", nil, context.Background())
	f.handler.Generate(c)
	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"id":"public-19"`)
	require.Contains(t, recorder.Body.String(), `"status":"queued"`)
	require.Zero(t, f.upstream.posts)
	require.NotEmpty(t, f.payload.body)
	query, result := newGrsaiGatewayResultContext(t, "public-19", 7, 4)
	f.handler.Result(query)
	require.Equal(t, http.StatusOK, result.Code)
	require.Contains(t, result.Body.String(), `"status":"queued"`)
}

func TestGrsaiGenerateAsyncDisabledDoesNotReserveBalance(t *testing.T) {
	f := newGrsaiGenerateFixture()
	f.handler.taskService.Options.Enabled = false
	c, recorder := f.request(t, "async", nil, context.Background())
	f.handler.Generate(c)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Nil(t, f.repo.record)
	require.Empty(t, f.payload.body)
	require.Zero(t, f.upstream.posts)
}

func TestGrsaiGenerateStreamDisabledDoesNotSubmit(t *testing.T) {
	f := newGrsaiGenerateFixture()
	f.handler.taskService.Options.StreamEnabled = false
	c, recorder := f.request(t, "stream", nil, context.Background())
	f.handler.Generate(c)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Nil(t, f.repo.record)
	require.Zero(t, f.upstream.posts)
}

// @covers AC-009
func TestGrsaiGenerateQueueFullReturns429WithoutPost(t *testing.T) {
	f := newGrsaiGenerateFixture()
	f.repo.queueFull = true
	c, recorder := f.request(t, "async", nil, context.Background())
	f.handler.Generate(c)
	require.Equal(t, http.StatusTooManyRequests, recorder.Code)
	require.Nil(t, f.repo.record, "full queue must not create a task or reach the hold step")
	require.Zero(t, f.upstream.posts)
	require.Empty(t, f.payload.body)
}

// @covers AC-001
func TestGrsaiGenerateMissingReplyTypeReturnsFinalJSON(t *testing.T) {
	f := newGrsaiGenerateFixture()
	c, recorder := f.request(t, "json", nil, context.Background())
	c.Request.Body = io.NopCloser(strings.NewReader(`{"model":"nano-banana-2-lite","prompt":"private"}`))
	f.handler.Generate(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Header().Get("Content-Type"), "application/json")
	require.Contains(t, recorder.Body.String(), `"status":"succeeded"`)
	require.Equal(t, 1, f.upstream.posts)
	require.JSONEq(t, `{"model":"nano-banana-2-lite","prompt":"private","replyType":"stream"}`, string(f.upstream.request))
}

func TestGrsaiGenerateJSONReturnsVerifiedTerminalAndDoesNotResubmitOnBillingFailure(t *testing.T) {
	f := newGrsaiGenerateFixture()
	f.billing.err = errors.New("temporary billing failure")
	c, recorder := f.request(t, "json", nil, context.Background())
	f.handler.Generate(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"status":"succeeded"`)
	require.Equal(t, "pending_settlement", f.repo.record.InternalStatus)
	require.Equal(t, 1, f.upstream.posts)
	require.Equal(t, 2, f.repo.events)
	require.JSONEq(t, `{"model":"nano-banana-2-lite","prompt":"private","replyType":"stream"}`, string(f.upstream.request))
}

func TestGrsaiGenerateResultlessSynchronousSuccessDoesNotBill(t *testing.T) {
	for _, mode := range []string{"json", "stream"} {
		t.Run(mode, func(t *testing.T) {
			f := newGrsaiGenerateFixture()
			f.upstream.stream = "data: {\"id\":\"up-19\",\"status\":\"succeeded\",\"progress\":100}\n\n"
			c, recorder := f.request(t, mode, nil, context.Background())
			f.handler.Generate(c)
			require.Equal(t, http.StatusBadGateway, recorder.Code)
			require.NotContains(t, recorder.Body.String(), `"status":"succeeded"`)
			require.Equal(t, "manual_review", f.repo.record.InternalStatus)
			require.Zero(t, f.billing.calls)
			require.Equal(t, 1, f.upstream.posts)
		})
	}
}

type grsaiDisconnectWriter struct {
	*httptest.ResponseRecorder
	cancel  func()
	repo    *grsaiGenerateRepo
	writes  int
	testing *testing.T
}

func (w *grsaiDisconnectWriter) Write(p []byte) (int, error) {
	if strings.HasPrefix(string(p), "data: ") {
		require.Greater(w.testing, w.repo.events, 0, "event must be durable before SSE output")
		w.writes++
		w.cancel()
		return 0, errors.New("downstream disconnected")
	}
	return w.ResponseRecorder.Write(p)
}

// @covers AC-007
func TestGrsaiGenerateStreamPersistsBeforeOutputAndContinuesAfterDisconnect(t *testing.T) {
	f := newGrsaiGenerateFixture()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := &grsaiDisconnectWriter{ResponseRecorder: httptest.NewRecorder(), cancel: cancel, repo: f.repo, testing: t}
	c, _ := f.request(t, "stream", w, ctx)
	f.handler.Generate(c)
	require.Equal(t, 1, w.writes)
	require.Equal(t, 2, f.repo.events, "terminal event must persist despite downstream disconnect")
	require.Equal(t, "settled", f.repo.record.InternalStatus)
	require.Equal(t, 1, f.upstream.posts)
	require.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	query, result := newGrsaiGatewayResultContext(t, "public-19", 7, 4)
	f.handler.Result(query)
	require.Equal(t, http.StatusOK, result.Code)
	require.Contains(t, result.Body.String(), `"status":"settled"`)
	require.Contains(t, result.Body.String(), `"https://img.invalid/19"`)
}

// @covers AC-002
func TestGrsaiGenerateStreamWritesPersistedFrames(t *testing.T) {
	f := newGrsaiGenerateFixture()
	c, recorder := f.request(t, "stream", nil, context.Background())
	f.handler.Generate(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
	require.Equal(t, 2, strings.Count(recorder.Body.String(), "data: "))
	require.Contains(t, recorder.Body.String(), "\n\ndata: ")
	require.Less(t, strings.Index(recorder.Body.String(), `"status":"running"`), strings.Index(recorder.Body.String(), `"status":"succeeded"`))
	require.Equal(t, 1, strings.Count(recorder.Body.String(), `"status":"succeeded"`))
	require.Equal(t, "settled", f.repo.record.InternalStatus)
	require.Equal(t, 1, f.upstream.posts)
}
