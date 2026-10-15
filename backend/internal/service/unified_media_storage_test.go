//go:build unit

package service

import (
	"bytes"
	"context"
	"github.com/stretchr/testify/require"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestUnifiedMediaStorageRejectsPrivateUpstreamImage(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer srv.Close()
	storage := &grsaiMetadataStorage{}
	uploader := NewImageResultUploader(storage, "images/", 0, nil)
	persister := &UnifiedMediaS3Persister{resolve: func() (*ImageResultUploader, bool) { return uploader, true }}
	_, err := persister.Persist(context.Background(), "media_test", "image", 0, 0, srv.URL+"/private.png")
	require.Error(t, err)
	require.Zero(t, hits.Load())
	require.Empty(t, storage.saved)
}
func TestUnifiedMediaStorageRequiresUsableS3(t *testing.T) {
	p := &UnifiedMediaS3Persister{resolve: func() (*ImageResultUploader, bool) { return nil, false }}
	require.False(t, p.Ready())
	_, err := p.Persist(context.Background(), "media_test", "image", 0, 0, "https://example.com/image.png")
	require.ErrorIs(t, err, ErrImageTaskUnavailable)
}

func TestUnifiedMediaImageStorageValidatesBytesBeforeS3(t *testing.T) {
	for _, body := range [][]byte{nil, []byte("<html>upstream temporarily unavailable</html>"), []byte("not an image")} {
		storage := &grsaiMetadataStorage{}
		safe := unifiedMediaValidatedImageStorage{storage}
		_, err := safe.SaveWithMetadata(context.Background(), "image.png", "image/png", body)
		require.Error(t, err)
		require.Empty(t, storage.saved)
	}
	storage := &grsaiMetadataStorage{}
	safe := unifiedMediaValidatedImageStorage{storage}
	var validPNG bytes.Buffer
	require.NoError(t, png.Encode(&validPNG, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	_, err := safe.SaveWithMetadata(context.Background(), "image.png", "image/png", validPNG.Bytes())
	require.NoError(t, err)
	require.Len(t, storage.saved, 1)
}
