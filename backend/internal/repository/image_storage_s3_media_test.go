package repository

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestS3MediaReaderPreservesContentTypeAndBody(t *testing.T) {
	payload := bytes.Repeat([]byte("audio-data"), 1024)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/media/uploads/audio.mp3", r.URL.Path)
		require.Equal(t, "audio/mpeg", r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, payload, body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	storage, err := NewS3ImageStorage(context.Background(), &config.ImageStorageConfig{
		Endpoint: server.URL, Region: "us-east-1", Bucket: "media", AccessKeyID: "test", SecretAccessKey: "secret", ForcePathStyle: true,
		PublicBaseURL: "https://cdn.example/media", PresignExpiry: 2,
	})
	require.NoError(t, err)
	meta, err := storage.SaveReader(context.Background(), "uploads/audio.mp3", "audio/mpeg", bytes.NewReader(payload), int64(len(payload)))
	require.NoError(t, err)
	require.Equal(t, "https://cdn.example/media/uploads/audio.mp3", meta.URL)
	require.Equal(t, "audio/mpeg", meta.ContentType)
	require.Nil(t, meta.LinkExpiresAt)
	_, err = storage.SaveReader(context.Background(), "x", "audio/mpeg", nil, 5)
	require.Error(t, err)
	_, err = storage.SaveReader(context.Background(), "x", "audio/mpeg", bytes.NewReader(nil), 0)
	require.Error(t, err)
}

func TestS3MediaConnectionChecksBucketAccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodHead, r.Method)
		require.Equal(t, "/media", r.URL.Path)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	storage, err := NewS3ImageStorage(context.Background(), &config.ImageStorageConfig{Endpoint: server.URL, Region: "us-east-1", Bucket: "media", AccessKeyID: "test", SecretAccessKey: "secret", ForcePathStyle: true})
	require.NoError(t, err)
	require.Error(t, storage.TestConnection(context.Background()))
}
