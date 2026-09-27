package repository

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestImageStorageS3ReturnsPublicURLWithoutExpiry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	storage, err := NewS3ImageStorage(context.Background(), &config.ImageStorageConfig{
		Endpoint: server.URL, Region: "us-east-1", Bucket: "images", AccessKeyID: "test", SecretAccessKey: "secret", ForcePathStyle: true,
		PublicBaseURL: "https://cdn.example/images/", PresignExpiry: 2,
	})
	require.NoError(t, err)
	meta, err := storage.SaveWithMetadata(context.Background(), "folder/photo.png", "image/png", []byte("data"))
	require.NoError(t, err)
	require.Equal(t, "https://cdn.example/images/folder/photo.png", meta.URL)
	require.Nil(t, meta.LinkExpiresAt)
	require.Equal(t, "folder/photo.png", meta.ObjectKey)
}

func TestImageStorageS3PresignedExpiryMatchesURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	storage, err := NewS3ImageStorage(context.Background(), &config.ImageStorageConfig{
		Endpoint: server.URL, Region: "us-east-1", Bucket: "images", AccessKeyID: "test", SecretAccessKey: "secret", ForcePathStyle: true,
		PresignExpiry: 2,
	})
	require.NoError(t, err)
	meta, err := storage.SaveWithMetadata(context.Background(), "photo.png", "image/png", []byte("data"))
	require.NoError(t, err)
	parsed, err := url.Parse(meta.URL)
	require.NoError(t, err)
	require.Equal(t, "7200", parsed.Query().Get("X-Amz-Expires"))
	signedAt, err := time.Parse("20060102T150405Z", parsed.Query().Get("X-Amz-Date"))
	require.NoError(t, err)
	require.NotNil(t, meta.LinkExpiresAt)
	require.Equal(t, signedAt.Add(2*time.Hour), *meta.LinkExpiresAt)
	legacyURL, err := storage.Save(context.Background(), "photo.png", "image/png", []byte("data"))
	require.NoError(t, err)
	require.NotEmpty(t, legacyURL)
}
