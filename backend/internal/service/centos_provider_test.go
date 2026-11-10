//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
	"github.com/stretchr/testify/require"
)

func centosTestRequest() *mediacapability.NormalizedRequest {
	return &mediacapability.NormalizedRequest{Values: map[string]any{"model": "gpt-image-2.6", "prompt": "cup", "size": "1:1", "metadata": map[string]any{"resolution": "1K", "quality": "auto"}}, ImageSpec: &mediacapability.ImageSpec{Width: 1024, Height: 1024, Resolution: "1K", AspectRatio: "1:1"}, RequestedCount: 1}
}

func TestCentOSCompletedStreamPersistsBeforeSettlementAndStorageRetryDoesNotRegenerate(t *testing.T) {
	var pixels bytes.Buffer
	require.NoError(t, png.Encode(&pixels, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	inline := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pixels.Bytes())
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts++
		require.Equal(t, "/v1/images/generations", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, true, body["stream"])
		require.Equal(t, "1024x1024", body["size"])
		require.NotContains(t, body, "imageSize")
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: {\"type\":\"image_generation.partial_image\",\"b64_json\":\"partial\"}\n\n")
		payload, _ := json.Marshal(map[string]any{"type": "image_generation.completed", "url": inline})
		_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
	}))
	defer server.Close()
	account := &Account{Platform: PlatformCentOS, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test-key", "base_url": server.URL}}
	upstream, err := NewUnifiedMediaUpstream(server.Client()).Submit(context.Background(), account, "image", "gpt-image-2.6-flare", centosTestRequest(), 1)
	require.NoError(t, err)
	require.Equal(t, "accepted", upstream.Disposition)
	require.Equal(t, "completed", upstream.Status)
	require.Equal(t, []string{inline}, upstream.URLs)
	storage := &grsaiMetadataStorage{failAt: 1}
	uploader := NewImageResultUploader(storage, "images/", 0, nil)
	persister := &UnifiedMediaS3Persister{resolve: func() (*ImageResultUploader, bool) { return uploader, true }}
	now := time.Now()
	svc := &UnifiedMediaTaskService{Repo: &unifiedMediaMemoryRepo{}, Persister: persister, Now: func() time.Time { return now }}
	task := &UnifiedMediaTask{ID: "media_centos_inline", MediaType: "image", RequestedCount: 1, UnitPrice: .08, Units: []UnifiedMediaUnit{{Count: 1}}}
	require.NoError(t, svc.acceptResult(context.Background(), task, 0, upstream))
	require.NoError(t, svc.persist(context.Background(), task, 0))
	require.Equal(t, "persisting", task.Units[0].State)
	require.Empty(t, task.PublicView().Result.Data)
	require.Zero(t, UnifiedMediaTaskCharge(task))
	storage.failAt = 0
	now = now.Add(time.Minute)
	snapshot, _ := json.Marshal(task)
	var restored UnifiedMediaTask
	require.NoError(t, json.Unmarshal(snapshot, &restored))
	require.NoError(t, svc.persist(context.Background(), &restored, 0))
	require.Equal(t, "completed", restored.Units[0].State)
	require.Equal(t, .08, UnifiedMediaTaskCharge(&restored))
	require.Equal(t, pixels.Bytes(), storage.saved[0].data)
	require.Empty(t, restored.Units[0].URLs)
	require.Equal(t, 1, posts)
}

func TestCentOSStreamAcceptsLargeFinalEventsAndRejectsIncompleteOrUnsafeResults(t *testing.T) {
	url := "data:image/png;base64," + strings.Repeat("A", 200000)
	raw, _ := json.Marshal(map[string]string{"type": "image_generation.completed", "url": url})
	urls, err := parseCentOSImageStream(strings.NewReader("event: image_generation.completed\ndata: " + string(raw) + "\n\n"))
	require.NoError(t, err)
	require.Equal(t, []string{url}, urls)
	for _, body := range []string{
		"data: [DONE]\n\n",
		"data: {\"type\":\"image_generation.partial_image\",\"url\":\"https://example.com/partial.png\"}\n\n",
		"data: {\"type\":\"image_generation.completed\",\"url\":\"file:///tmp/image.png\"}\n\n",
		"data: {\"type\":\"image_generation.completed\",\"url\":\"https://user:pass@example.com/image.png\"}\n\n",
		"data: malformed\n\n",
	} {
		urls, _ := parseCentOSImageStream(strings.NewReader(body))
		require.Empty(t, urls)
	}
}

func TestCentOSExplicitRejectionAndUnknownAcceptanceRemainDistinct(t *testing.T) {
	for _, tc := range []struct {
		code              int
		body, disposition string
	}{
		{400, `{"error":{"message":"invalid size"}}`, "rejected"},
		{429, `{"error":{"message":"rate limit"}}`, "rejected"},
		{400, `{"error":{"message":"service timeout, please try again later"}}`, "unknown"},
		{502, `bad gateway`, "unknown"},
		{200, `{"data":[]}`, "unknown"},
	} {
		t.Run(fmt.Sprintf("%d_%s", tc.code, tc.disposition), func(t *testing.T) {
			posts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				posts++
				w.WriteHeader(tc.code)
				_, _ = fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			account := &Account{Platform: PlatformCentOS, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test-key", "base_url": server.URL}}
			result, err := NewUnifiedMediaUpstream(server.Client()).Submit(context.Background(), account, "image", "gpt-image-2.6-flare", centosTestRequest(), 1)
			require.Equal(t, tc.disposition, result.Disposition)
			require.Empty(t, result.URLs)
			if tc.disposition == "unknown" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, 1, posts)
		})
	}
}

func TestCentOSReferenceURLsBecomeMultipartWithoutCredentialDisclosure(t *testing.T) {
	var imageBytes bytes.Buffer
	require.NoError(t, png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	gets, posts := 0, 0
	upstream := NewUnifiedMediaUpstream(nil).(*unifiedMediaHTTPUpstream)
	upstream.transport = &systemOneHTTPUpstream{do: func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet {
			gets++
			require.Equal(t, "reference.example", req.URL.Host)
			require.Empty(t, req.Header.Get("Authorization"))
			require.True(t, HTTPUpstreamPublicHostsOnly(req.Context()))
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(imageBytes.Bytes()))}, nil
		}
		posts++
		require.Equal(t, "/v1/images/edits", req.URL.Path)
		require.Equal(t, "Bearer test-key", req.Header.Get("Authorization"))
		require.True(t, HTTPUpstreamRedirectsDisabled(req.Context()))
		require.NoError(t, req.ParseMultipartForm(32<<20))
		defer func() { _ = req.MultipartForm.RemoveAll() }()
		require.Equal(t, "gpt-image-2.6-flare", req.FormValue("model"))
		require.Equal(t, "1024x1024", req.FormValue("size"))
		require.Equal(t, "true", req.FormValue("stream"))
		require.Equal(t, "auto", req.FormValue("quality"))
		require.Len(t, req.MultipartForm.File["image[]"], 2)
		for _, file := range req.MultipartForm.File["image[]"] {
			require.True(t, strings.HasSuffix(file.Filename, ".png"))
			reader, err := file.Open()
			require.NoError(t, err)
			data, err := io.ReadAll(reader)
			require.NoError(t, err)
			require.NoError(t, reader.Close())
			require.Equal(t, imageBytes.Bytes(), data)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader("data: {\"type\":\"image_edit.completed\",\"url\":\"https://results.example/edit.png\"}\n\n"))}, nil
	}}
	account := &Account{Platform: PlatformCentOS, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test-key", "base_url": CentOSDefaultBaseURL}}
	request := centosTestRequest()
	request.Values["image_urls"] = []string{"https://reference.example/a.png", "https://reference.example/b.png"}
	before, _ := json.Marshal(request)
	result, err := upstream.Submit(context.Background(), account, "image", "gpt-image-2.6-flare", request, 1)
	require.NoError(t, err)
	require.Equal(t, "accepted", result.Disposition)
	require.Equal(t, []string{"https://results.example/edit.png"}, result.URLs)
	require.Equal(t, 2, gets)
	require.Equal(t, 1, posts)
	after, _ := json.Marshal(request)
	require.JSONEq(t, string(before), string(after))
	request.Values["image_urls"] = []string{"http://127.0.0.1/private.png"}
	result, err = upstream.Submit(context.Background(), account, "image", "gpt-image-2.6-flare", request, 1)
	require.Error(t, err)
	require.Equal(t, "rejected", result.Disposition)
	require.Equal(t, 2, gets)
	require.Equal(t, 1, posts)
}
