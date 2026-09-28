package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"strings"
	"testing"
)

type videoRoundTripFunc func(*http.Request) (*http.Response, error)

func (f videoRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestGrsaiVideoDownloadAcceptsMP4OctetStream(t *testing.T) {
	c := &http.Client{Transport: videoRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/octet-stream"}}, Body: io.NopCloser(strings.NewReader("....ftypisompayload")), ContentLength: 19, Request: r}, nil
	})}
	d := NewGrsaiVideoDownloader(c, 512<<20, "")
	body, size, e := d.Download(context.Background(), "https://example.com/video")
	require.NoError(t, e)
	defer body.Close()
	require.Equal(t, int64(19), size)
}
func TestGrsaiVideoDownloadRejectsUnsafeURL(t *testing.T) {
	d := NewGrsaiVideoDownloader(nil, 0, "")
	for _, u := range []string{"http://example.com/a", "https://127.0.0.1/a", "https://10.0.0.1/a"} {
		_, _, e := d.Download(context.Background(), u)
		require.Error(t, e, u)
	}
}
