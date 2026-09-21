package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

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
	_, err := ParseGrsaiSSE(strings.NewReader("data: {\"id\":\"a\",\"status\":\"succeeded\",\"results\":[{\"url\":\"https://example.invalid/a\"}]}\n\n"), func(GrsaiStreamEvent) error {
		return nil
	})
	require.NoError(t, err)
}

func TestParseGrsaiSSEInterruptionIsReturnedForPreAndPostBindHandling(t *testing.T) {
	for _, tc := range []struct {
		name  string
		body  string
		bound bool
	}{
		{name: "pre-bind", body: "data: {\"status\":\"running\"}\n", bound: false},
		{name: "post-bind", body: "data: {\"id\":\"a\",\"status\":\"running\"}\n", bound: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseGrsaiSSE(errReader{err: errors.New("connection reset")}, nil)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrGrsaiSSEProtocol)
		})
	}
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
