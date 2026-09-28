package repository

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVideoStorageReaderStreamsMP4(t *testing.T) {
	var l string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { l = r.Header.Get("Content-Length"); w.WriteHeader(200) }))
	defer srv.Close()
	s, e := NewS3ImageStorage(context.Background(), &config.ImageStorageConfig{Endpoint: srv.URL, Region: "us-east-1", Bucket: "media", AccessKeyID: "x", SecretAccessKey: "y", ForcePathStyle: true, PublicBaseURL: "https://cdn.example"})
	require.NoError(t, e)
	m, e := s.SaveVideoReader(context.Background(), "grsai/id/video-0.mp4", strings.NewReader("video"), 5)
	require.NoError(t, e)
	require.Equal(t, "5", l)
	require.Equal(t, "video/mp4", m.ContentType)
}
