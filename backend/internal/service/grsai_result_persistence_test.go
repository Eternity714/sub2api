package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type grsaiMetadataStorage struct {
	saved    []savedImage
	expiry   *time.Time
	expiries []*time.Time
	failAt   int
}

func (s *grsaiMetadataStorage) Save(ctx context.Context, key, contentType string, data []byte) (string, error) {
	metadata, err := s.SaveWithMetadata(ctx, key, contentType, data)
	return metadata.URL, err
}

func (s *grsaiMetadataStorage) SaveWithMetadata(_ context.Context, key, contentType string, data []byte) (ImageStorageSaveMetadata, error) {
	if s.failAt > 0 && len(s.saved)+1 == s.failAt {
		return ImageStorageSaveMetadata{}, errors.New("storage unavailable")
	}
	expiry := s.expiry
	if len(s.expiries) > len(s.saved) {
		expiry = s.expiries[len(s.saved)]
	}
	s.saved = append(s.saved, savedImage{key: key, contentType: contentType, data: append([]byte(nil), data...)})
	return ImageStorageSaveMetadata{URL: "https://cdn.example/" + key, ObjectKey: key, ContentType: contentType, LinkExpiresAt: expiry}, nil
}

func TestGrsaiResultPersistenceStoresAllImagesBeforeReturning(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer upstream.Close()
	storage := &grsaiMetadataStorage{}
	uploader := NewImageResultUploader(storage, "images/", 0, nil)
	result, err := uploader.PersistGrsaiImages(context.Background(), "grsai_local", &GrsaiUpstreamResult{
		TaskID: "upstream-secret", Status: GrsaiUpstreamStatusSucceeded,
		RawBody:   []byte(`{"id":"upstream-secret","account":"private"}`),
		ImageURLs: []string{upstream.URL + "/a", upstream.URL + "/b"},
	})
	require.NoError(t, err)
	require.Len(t, storage.saved, 2)
	require.Equal(t, "images/grsai_local-0.png", storage.saved[0].key)
	require.Equal(t, "images/grsai_local-1.png", storage.saved[1].key)
	require.Nil(t, result.LinkExpiresAt)
	require.JSONEq(t, `{"results":[{"url":"https://cdn.example/images/grsai_local-0.png"},{"url":"https://cdn.example/images/grsai_local-1.png"}]}`, string(result.ResultJSON))
	require.NotContains(t, string(result.ResultJSON), "upstream-secret")
	require.NotContains(t, string(result.ResultJSON), upstream.URL)
	require.JSONEq(t, `[{"object_key":"images/grsai_local-0.png","content_type":"image/png"},{"object_key":"images/grsai_local-1.png","content_type":"image/png"}]`, string(result.ObjectMetadata))
}

func TestGrsaiResultPersistenceDoesNotReturnPartialSuccess(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer upstream.Close()
	storage := &grsaiMetadataStorage{failAt: 2}
	uploader := NewImageResultUploader(storage, "images/", 0, nil)
	input := &GrsaiUpstreamResult{Status: GrsaiUpstreamStatusSucceeded, ImageURLs: []string{upstream.URL + "/a", upstream.URL + "/b"}}
	result, err := uploader.PersistGrsaiImages(context.Background(), "grsai_local", input)
	require.Error(t, err)
	require.Nil(t, result)
	require.NotContains(t, err.Error(), upstream.URL)
	storage.failAt = 0
	result, err = uploader.PersistGrsaiImages(context.Background(), "grsai_local", input)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "images/grsai_local-0.png", storage.saved[1].key, "retry must use the same object key")
}

func TestGrsaiResultPersistenceKeepsEarliestLinkExpiry(t *testing.T) {
	first := time.Now().UTC().Add(2 * time.Hour)
	second := first.Add(-time.Hour)
	storage := &grsaiMetadataStorage{expiries: []*time.Time{&first, &second}}
	uploader := NewImageResultUploader(storage, "images/", 0, nil)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer upstream.Close()
	result, err := uploader.PersistGrsaiImages(context.Background(), "grsai_local", &GrsaiUpstreamResult{
		Status: GrsaiUpstreamStatusSucceeded, ImageURLs: []string{upstream.URL + "/a", upstream.URL + "/b"},
	})
	require.NoError(t, err)
	require.NotNil(t, result.LinkExpiresAt)
	require.Equal(t, second, *result.LinkExpiresAt)
	require.True(t, strings.Contains(string(result.ResultJSON), "https://cdn.example/"))
}

func TestGrsaiResultPersistenceRejectsUnfinishedResult(t *testing.T) {
	storage := &grsaiMetadataStorage{}
	uploader := NewImageResultUploader(storage, "images/", 0, nil)
	result, err := uploader.PersistGrsaiImages(context.Background(), "grsai_local", &GrsaiUpstreamResult{
		Status: GrsaiUpstreamStatusRunning, ImageURLs: []string{"https://example.test/a.png"},
	})
	require.Error(t, err)
	require.Nil(t, result)
	require.Empty(t, storage.saved)
}
