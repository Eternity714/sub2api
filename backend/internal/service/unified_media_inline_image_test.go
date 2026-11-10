//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
	"github.com/stretchr/testify/require"
)

func TestKuaipaoInlineFallbackPersistsBeforeDeliveryAndSurvivesStorageRetry(t *testing.T) {
	var imageBytes bytes.Buffer
	require.NoError(t, png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	encoded := base64.StdEncoding.EncodeToString(imageBytes.Bytes())
	for _, field := range []string{"url", "b64_json"} {
		t.Run(field, func(t *testing.T) {
			value := encoded
			if field == "url" {
				value = "data:image/png;base64," + encoded
			}
			posts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				posts++
				_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{field: value}}})
			}))
			defer server.Close()
			account := &Account{Platform: PlatformKuaipao, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "key", "base_url": server.URL + "/v2"}}
			upstream, err := NewUnifiedMediaUpstream(server.Client()).Submit(context.Background(), account, "image", "gpt-image-2.6-2k", &mediacapability.NormalizedRequest{Values: map[string]any{"prompt": "cup", "size": "1:1"}}, 1)
			require.NoError(t, err)
			require.Equal(t, "accepted", upstream.Disposition)
			require.Equal(t, "completed", upstream.Status)
			require.Len(t, upstream.URLs, 1)
			storage := &grsaiMetadataStorage{failAt: 1}
			uploader := NewImageResultUploader(storage, "images/", 0, nil)
			persister := &UnifiedMediaS3Persister{resolve: func() (*ImageResultUploader, bool) { return uploader, true }}
			now := time.Now()
			svc := &UnifiedMediaTaskService{Repo: &unifiedMediaMemoryRepo{}, Persister: persister, Now: func() time.Time { return now }}
			task := &UnifiedMediaTask{ID: "media_inline", MediaType: "image", RequestedCount: 1, UnitPrice: .2, Units: []UnifiedMediaUnit{{Count: 1}}}
			require.NoError(t, svc.acceptResult(context.Background(), task, 0, upstream))
			require.NoError(t, svc.persist(context.Background(), task, 0))
			require.Equal(t, "persisting", task.Units[0].State)
			require.NotEmpty(t, task.Units[0].URLs, "inline data must remain recoverable until storage succeeds")
			require.Empty(t, task.PublicView().Result.Data)
			require.Zero(t, UnifiedMediaTaskCharge(task))
			storage.failAt = 0
			now = now.Add(time.Minute)
			snapshot, _ := json.Marshal(task)
			var restored UnifiedMediaTask
			require.NoError(t, json.Unmarshal(snapshot, &restored))
			require.NoError(t, svc.persist(context.Background(), &restored, 0))
			require.Equal(t, "completed", restored.Units[0].State)
			require.Empty(t, restored.Units[0].URLs, "completed tasks must discard inline image bytes")
			require.Len(t, storage.saved, 1)
			require.Equal(t, imageBytes.Bytes(), storage.saved[0].data)
			require.Equal(t, "image/png", storage.saved[0].contentType)
			require.Equal(t, .2, UnifiedMediaTaskCharge(&restored))
			require.Equal(t, 1, posts, "storage recovery must not regenerate the image")
		})
	}
}

func TestUnifiedInlineResultsDoNotWeakenURLOrImageValidation(t *testing.T) {
	require.Empty(t, parseUnifiedOpenAIImageURLs([]byte(`{"data":[{"b64_json":"aW1hZ2U="}]}`)))
	for _, value := range []string{"data:text/html;base64,PGh0bWw+", "file:///tmp/image.png", "data:image/png;base64,bm90LWFuLWltYWdl"} {
		storage := &grsaiMetadataStorage{}
		uploader := NewImageResultUploader(storage, "images/", 0, nil)
		persister := &UnifiedMediaS3Persister{resolve: func() (*ImageResultUploader, bool) { return uploader, true }}
		_, err := persister.Persist(context.Background(), "media_bad_inline", "image", 0, 0, value)
		require.Error(t, err)
		require.Empty(t, storage.saved)
	}
}
