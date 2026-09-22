package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseGrsaiSSERequiresStableIDAndMonotonicProgress(t *testing.T) {
	body := strings.NewReader("data: {\"id\":\"task-1\",\"status\":\"running\",\"progress\":10}\n\n" +
		"data: {\"id\":\"task-1\",\"status\":\"succeeded\",\"progress\":100,\"results\":[{\"url\":\"https://example.invalid/a.png\"}]}\n\n")
	var persisted int
	final, err := ParseGrsaiSSE(body, func(event GrsaiStreamEvent) error {
		persisted++
		require.NotEmpty(t, event.RawData)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 2, persisted)
	require.Equal(t, GrsaiUpstreamStatusSucceeded, final.Status)
	require.Equal(t, "task-1", final.TaskID)
	require.Equal(t, 100, final.Progress)
	require.Len(t, final.ResultURLs, 1)
}

func TestParseGrsaiSSERejectsChangedIDRegressiveProgressAndResultlessSuccess(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"changed ID", "data: {\"id\":\"a\",\"progress\":20}\n\ndata: {\"id\":\"b\",\"progress\":30}\n\n"},
		{"regressive progress", "data: {\"id\":\"a\",\"progress\":20}\n\ndata: {\"id\":\"a\",\"progress\":10}\n\n"},
		{"resultless success", "data: {\"id\":\"a\",\"status\":\"succeeded\",\"progress\":100}\n\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseGrsaiSSE(strings.NewReader(tt.body), nil)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrGrsaiSSEProtocol)
		})
	}
}

func TestParseGrsaiSSERequiresTaskIDAndTerminal(t *testing.T) {
	_, err := ParseGrsaiSSE(strings.NewReader("data: {\"status\":\"running\"}\n\n"), nil)
	require.ErrorIs(t, err, ErrGrsaiSSEProtocol)
	_, err = ParseGrsaiSSE(strings.NewReader("data: {\"id\":\"a\",\"status\":\"running\"}\n\n"), nil)
	require.ErrorIs(t, err, ErrGrsaiSSEProtocol)
}

func TestOpenGenerateStreamRejectsNonSSEAndClosesBody(t *testing.T) {
	var closed atomic.Bool
	client := NewGrsaiNativeClient(&http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: closeTrackingReader{Reader: strings.NewReader(`{"id":"task"}`), closed: &closed}}, nil
	})})
	_, err := client.(GrsaiStreamClient).OpenGenerateStream(context.Background(), grsaiTestAccount("https://api.grsai.example", "secret"), []byte(`{"model":"nano-banana-2-lite"}`))
	require.ErrorIs(t, err, ErrGrsaiInvalidStreamResponse)
	require.True(t, closed.Load())
}

func TestOpenGenerateStreamRejectsNon2xxRedactsAndClosesBody(t *testing.T) {
	const secret = "stream-secret-token"
	var closed atomic.Bool
	client := NewGrsaiNativeClient(&http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusBadGateway, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: closeTrackingReader{Reader: strings.NewReader("upstream " + secret), closed: &closed}}, nil
	})})
	_, err := client.(GrsaiStreamClient).OpenGenerateStream(context.Background(), grsaiTestAccount("https://api.grsai.example", secret), []byte(`{"model":"nano-banana-2-lite"}`))
	require.ErrorIs(t, err, ErrGrsaiInvalidStreamResponse)
	require.NotContains(t, err.Error(), secret)
	require.True(t, closed.Load())
}

func TestOpenGenerateStreamExposesValidatedBodyAndHeaders(t *testing.T) {
	var gotAccept string
	client := NewGrsaiNativeClient(&http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		gotAccept = req.Header.Get("Accept")
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream; charset=utf-8"}}, Body: io.NopCloser(strings.NewReader("data: {}\n\n"))}, nil
	})})
	stream, err := client.(GrsaiStreamClient).OpenGenerateStream(context.Background(), grsaiTestAccount("https://api.grsai.example", "secret"), []byte(`{"model":"nano-banana-2-lite","replyType":"json"}`))
	require.NoError(t, err)
	require.Equal(t, "text/event-stream", gotAccept)
	require.Equal(t, http.StatusOK, stream.StatusCode)
	require.Contains(t, stream.ContentType, "text/event-stream")
	require.NoError(t, stream.Body.Close())
}

func TestParseGrsaiSSEPersistsBeforeCallback(t *testing.T) {
	s, repo, claim := grsaiStreamServiceFixture(t)
	_, err := s.ConsumeGrsaiSSE(context.Background(), claim, strings.NewReader("data: {\"id\":\"a\",\"status\":\"succeeded\",\"results\":[{\"url\":\"https://example.invalid/a\"}]}\n\n"), func(GrsaiStreamEvent) error {
		repo.order = append(repo.order, "callback")
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, []string{"persist", "callback"}, repo.order)
	require.Equal(t, "a", *claim.UpstreamTaskID)
}

func TestConsumeGrsaiSSECallbackFailureAfterBindKeepsHoldAndSchedulesPolling(t *testing.T) {
	s, repo, claim := grsaiStreamServiceFixture(t)
	callbackErr := errors.New("downstream callback failed")
	_, err := s.ConsumeGrsaiSSE(context.Background(), claim, strings.NewReader("data: {\"id\":\"a\",\"status\":\"succeeded\",\"results\":[{\"url\":\"https://example.invalid/a\"}]}\n\n"), func(GrsaiStreamEvent) error {
		return callbackErr
	})
	require.ErrorIs(t, err, callbackErr)
	require.Equal(t, "a", *claim.UpstreamTaskID)
	require.Zero(t, repo.manualReviewCalls)
	require.Zero(t, repo.releaseCalls)
	require.Equal(t, 1, repo.updateResultCalls)
	require.Equal(t, 1, repo.pendingUpstreamCalls)
}

func TestConsumeGrsaiSSEAtomicFirstEventFailureDoesNotBindOrCallback(t *testing.T) {
	s, repo, claim := grsaiStreamServiceFixture(t)
	repo.bindEventErr = errors.New("event write failed")
	callbackCalled := false
	_, err := s.ConsumeGrsaiSSE(context.Background(), claim, strings.NewReader("data: {\"id\":\"a\",\"status\":\"running\"}\n\n"), func(GrsaiStreamEvent) error {
		callbackCalled = true
		return nil
	})
	require.ErrorIs(t, err, repo.bindEventErr)
	require.False(t, callbackCalled)
	require.Nil(t, claim.UpstreamTaskID)
	require.Zero(t, repo.recordEventCalls)
}

func TestConsumeGrsaiSSEPreBindInterruptionFailsClosedWithoutHoldExtension(t *testing.T) {
	s, repo, claim := grsaiStreamServiceFixture(t)
	s.Repo = &noHoldStreamRepo{GrsaiSettlementRepository: repo, stream: repo}
	_, err := s.ConsumeGrsaiSSE(context.Background(), claim, errReader{err: errors.New("connection reset")}, nil)
	require.ErrorIs(t, err, ErrGrsaiHoldReleaseUnavailable)
	require.Zero(t, repo.manualReviewCalls)
	require.Nil(t, claim.UpstreamTaskID)
}

func TestParseGrsaiSSEInterruptionIsReturnedForPreAndPostBindHandling(t *testing.T) {
	t.Run("pre-bind releases and manual reviews", func(t *testing.T) {
		s, repo, claim := grsaiStreamServiceFixture(t)
		_, err := s.ConsumeGrsaiSSE(context.Background(), claim, errReader{err: errors.New("connection reset")}, nil)
		require.ErrorIs(t, err, ErrGrsaiSSERead)
		require.Equal(t, 1, repo.manualReviewCalls)
		require.Equal(t, 1, repo.releaseCalls)
		require.Zero(t, repo.recordEventCalls)
	})
	t.Run("post-bind retains hold and schedules polling", func(t *testing.T) {
		s, repo, claim := grsaiStreamServiceFixture(t)
		body := "data: {\"id\":\"a\",\"status\":\"running\",\"progress\":5}\n\n"
		_, err := s.ConsumeGrsaiSSE(context.Background(), claim, &sequenceReader{parts: [][]byte{[]byte(body), nil}, err: errors.New("connection reset")}, nil)
		require.ErrorIs(t, err, ErrGrsaiSSERead)
		require.Equal(t, 0, repo.manualReviewCalls)
		require.Zero(t, repo.releaseCalls)
		require.Equal(t, 1, repo.recordEventCalls)
		require.Equal(t, 1, repo.updateResultCalls)
		require.Equal(t, 1, repo.pendingUpstreamCalls)
	})
}

func grsaiStreamServiceFixture(t *testing.T) (*GrsaiSettlementService, *grsaiStreamRepo, *GrsaiSettlement) {
	t.Helper()
	repo := &grsaiStreamRepo{record: &GrsaiSettlement{ID: 77, ClaimVersion: 1, InternalStatus: "processing", HoldAmount: 1, UpstreamStatus: "not_submitted"}}
	billing := &grsaiBillingSpy{}
	s := &GrsaiSettlementService{Repo: repo, Billing: billing, HoldBilling: billing}
	return s, repo, repo.record
}

type grsaiStreamRepo struct {
	*grsaiSettlementMemoryRepo
	record                                                                                     *GrsaiSettlement
	order                                                                                      []string
	recordEventCalls, manualReviewCalls, releaseCalls, updateResultCalls, pendingUpstreamCalls int
	bindEventErr                                                                               error
}

type noHoldStreamRepo struct {
	GrsaiSettlementRepository
	stream *grsaiStreamRepo
}

func (r *noHoldStreamRepo) BindAndRecordStreamEvent(ctx context.Context, id, version int64, event GrsaiStreamEvent) (bool, error) {
	return r.stream.BindAndRecordStreamEvent(ctx, id, version, event)
}
func (r *noHoldStreamRepo) RecordStreamEvent(ctx context.Context, id, version int64, event GrsaiStreamEvent) (bool, error) {
	return r.stream.RecordStreamEvent(ctx, id, version, event)
}

func (r *grsaiStreamRepo) GetByID(context.Context, int64) (*GrsaiSettlement, error) {
	c := *r.record
	return &c, nil
}
func (r *grsaiStreamRepo) BindUpstreamTask(_ context.Context, _ int64, _ int64, id, status string) (bool, error) {
	r.record.UpstreamTaskID = &id
	r.record.UpstreamStatus = status
	return true, nil
}
func (r *grsaiStreamRepo) BindAndRecordStreamEvent(_ context.Context, _ int64, _ int64, event GrsaiStreamEvent) (bool, error) {
	if r.bindEventErr != nil {
		return false, r.bindEventErr
	}
	id := event.TaskID
	r.record.UpstreamTaskID = &id
	r.record.UpstreamStatus = event.Status
	r.order = append(r.order, "persist")
	r.recordEventCalls++
	return true, nil
}
func (r *grsaiStreamRepo) RecordStreamEvent(_ context.Context, _ int64, _ int64, event GrsaiStreamEvent) (bool, error) {
	r.order = append(r.order, "persist")
	r.recordEventCalls++
	r.record.Progress = event.Progress
	r.record.UpstreamStatus = event.Status
	r.record.ResultURLs = event.ResultURLs
	return true, nil
}
func (r *grsaiStreamRepo) UpdateResult(_ context.Context, _ int64, _ int64, status, _ string, _ time.Time) (bool, error) {
	r.updateResultCalls++
	r.record.UpstreamStatus = status
	return true, nil
}
func (r *grsaiStreamRepo) MarkPendingUpstream(context.Context, int64, int64, time.Time) error {
	r.pendingUpstreamCalls++
	return nil
}
func (r *grsaiStreamRepo) MarkManualReviewWithRelease(_ context.Context, _ int64, _ int64, _ string, release GrsaiSettlementTxFunc) error {
	r.manualReviewCalls++
	r.releaseCalls++
	if err := release(context.Background(), nil, r.record); err != nil {
		return err
	}
	return nil
}
func (r *grsaiStreamRepo) CloseNoChargeWithRelease(context.Context, int64, int64, string, GrsaiSettlementTxFunc) error {
	return nil
}
func (r *grsaiStreamRepo) MarkHoldHeld(context.Context, int64, int64) error { return nil }

type sequenceReader struct {
	parts [][]byte
	err   error
}

func (r *sequenceReader) Read(p []byte) (int, error) {
	if len(r.parts) == 0 {
		return 0, r.err
	}
	part := r.parts[0]
	r.parts = r.parts[1:]
	if len(part) == 0 {
		return 0, r.err
	}
	n := copy(p, part)
	return n, nil
}

type closeTrackingReader struct {
	io.Reader
	closed *atomic.Bool
}

func (r closeTrackingReader) Close() error {
	r.closed.Store(true)
	return nil
}

type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }
