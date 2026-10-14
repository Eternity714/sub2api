package routes

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMediaUploadRoutesRequireAPIKey(t *testing.T) {
	for _, kind := range []string{"images", "videos", "audios"} {
		t.Run(kind, func(t *testing.T) {
			router, _, _ := newKeyBillingRouteTestRouter(config.RunModeStandard)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/uploads/"+kind, nil))
			require.Equal(t, http.StatusUnauthorized, w.Code)
			require.Contains(t, w.Header().Get("Content-Type"), "application/json")
		})
	}
}

type mediaUploadRouteSettings struct {
	service.SettingRepository
	values map[string]string
}

func (r *mediaUploadRouteSettings) GetValue(_ context.Context, key string) (string, error) {
	value, ok := r.values[key]
	if !ok {
		return "", service.ErrSettingNotFound
	}
	return value, nil
}

func (r *mediaUploadRouteSettings) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

type mediaUploadRouteAuthRepo struct{ keyBillingRouteAPIKeyRepo }

func (*mediaUploadRouteAuthRepo) UpdateLastUsed(context.Context, int64, time.Time) error {
	return nil
}

func newMediaUploadRouteFixture(t *testing.T, endpoint string) (*gin.Engine, *service.MediaUploadSettingService, *service.APIKey) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		RunMode: config.RunModeStandard,
		Totp:    config.TotpConfig{EncryptionKeyConfigured: true, EncryptionKey: strings.Repeat("ab", 32)},
	}
	// Uploads must use their configured cap even when generation's body cap is smaller.
	cfg.Gateway.MaxBodySize = 32
	encryptor, err := repository.NewAESEncryptor(cfg)
	require.NoError(t, err)
	repo := &mediaUploadRouteSettings{values: make(map[string]string)}
	backup := service.NewBackupService(repo, cfg, encryptor, nil, nil)
	settings := service.NewMediaUploadSettingService(repo, encryptor, backup, func(ctx context.Context, storageCfg *config.ImageStorageConfig) (service.ImageStorage, error) {
		return repository.NewS3ImageStorage(ctx, storageCfg)
	})
	for _, kind := range []service.MediaUploadKind{service.MediaUploadImages, service.MediaUploadVideos, service.MediaUploadAudios} {
		_, err = settings.Update(context.Background(), kind, service.MediaUploadStorageSettings{
			Enabled: true, Endpoint: endpoint, Region: "us-east-1", Bucket: string(kind),
			AccessKeyID: "upload-test-key", SecretAccessKey: "upload-test-secret", ForcePathStyle: true,
			PublicBaseURL: "https://cdn.example/" + string(kind),
		})
		require.NoError(t, err)
	}
	// An ungrouped key proves reference uploads do not enter model/group routing.
	key := &service.APIKey{ID: 100, UserID: 7, Key: "upload-route-test-key", Status: service.StatusActive,
		User: &service.User{ID: 7, Role: service.RoleUser, Status: service.StatusActive, Balance: 10}}
	authRepo := &mediaUploadRouteAuthRepo{keyBillingRouteAPIKeyRepo{apiKey: key}}
	apiKeys := service.NewAPIKeyService(authRepo, nil, nil, nil, &keyBillingRouteRateRepo{}, nil, cfg)
	router := gin.New()
	RegisterGatewayRoutes(router, &handler.Handlers{
		Gateway: &handler.GatewayHandler{}, OpenAIGateway: &handler.OpenAIGatewayHandler{},
		MediaUpload: handler.NewMediaUploadHandler(settings),
	}, servermiddleware.NewAPIKeyAuthMiddleware(apiKeys, nil, cfg), apiKeys, nil, nil, nil, nil, cfg)
	return router, settings, key
}

func mediaUploadMultipartRequest(t *testing.T, kind string, files ...[]byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("purpose", "generation"))
	for _, file := range files {
		part, err := writer.CreateFormFile("file", "client-name.exe")
		require.NoError(t, err)
		_, err = part.Write(file)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, "/v1/uploads/"+kind, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer upload-route-test-key")
	return req
}

func mediaUploadRoutePNG(t *testing.T) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+j6WQAAAAASUVORK5CYII=")
	require.NoError(t, err)
	return data
}

func TestMediaUploadRoutesStoreOriginalMediaAndReturnToAPIsResponse(t *testing.T) {
	for _, tc := range []struct {
		kind, mime, extension string
		data                  []byte
	}{
		{"images", "image/png", ".png", append(mediaUploadRoutePNG(t), bytes.Repeat([]byte{0}, 2<<20)...)},
		{"videos", "video/mp4", ".mp4", []byte("\x00\x00\x00\x18ftypisom\x00\x00\x00\x00isommp41")},
		{"audios", "audio/mpeg", ".mp3", []byte("ID3\x03\x00\x00\x00\x00\x00\x00\xff\xfb\x90\x64")},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			type storedObject struct {
				path, mime string
				data       []byte
				method     string
			}
			objects := make(chan storedObject, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				data, _ := io.ReadAll(req.Body)
				objects <- storedObject{path: req.URL.Path, mime: req.Header.Get("Content-Type"), data: data, method: req.Method}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			router, _, _ := newMediaUploadRouteFixture(t, server.URL)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, mediaUploadMultipartRequest(t, tc.kind, tc.data))
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
			var result struct {
				Success bool                      `json:"success"`
				Message string                    `json:"message"`
				Data    service.MediaUploadResult `json:"data"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
			require.True(t, result.Success)
			require.Empty(t, result.Message)
			require.NoError(t, uuid.Validate(result.Data.ID))
			require.Equal(t, tc.mime, result.Data.MimeType)
			require.EqualValues(t, len(tc.data), result.Data.Size)
			select {
			case object := <-objects:
				wantKey := "uploads/" + tc.kind + "/7/" + result.Data.ID + tc.extension
				require.Equal(t, "/"+tc.kind+"/"+wantKey, object.path)
				require.Equal(t, http.MethodPut, object.method)
				require.Equal(t, tc.mime, object.mime)
				require.Equal(t, tc.data, object.data)
				require.Equal(t, "https://cdn.example/"+tc.kind+"/"+wantKey, result.Data.URL)
			default:
				t.Fatal("successful upload did not store an object")
			}
		})
	}
}

func TestMediaUploadRoutesRejectInvalidMultipartAndApplyChangedLimits(t *testing.T) {
	var mu sync.Mutex
	puts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		puts++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	router, settings, _ := newMediaUploadRouteFixture(t, server.URL)
	data := mediaUploadRoutePNG(t)
	for _, tc := range []struct {
		name  string
		files [][]byte
	}{
		{"missing", nil},
		{"empty", [][]byte{nil}},
		{"multiple", [][]byte{data, data}},
		{"spoofed", [][]byte{[]byte("<html>not an image</html>")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, mediaUploadMultipartRequest(t, "images", tc.files...))
			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		})
	}
	updateLimit := func(limit int64) {
		cfg, err := settings.Get(context.Background(), service.MediaUploadImages)
		require.NoError(t, err)
		cfg.MaxUploadBytes = limit
		_, err = settings.Update(context.Background(), service.MediaUploadImages, *cfg)
		require.NoError(t, err)
	}
	updateLimit(int64(len(data) - 1))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, mediaUploadMultipartRequest(t, "images", data))
	require.Equal(t, http.StatusBadRequest, w.Code)
	// Restoring the cap must take effect on the next request, without a restart.
	updateLimit(int64(len(data)))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, mediaUploadMultipartRequest(t, "images", data))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, puts, "rejected uploads must not write objects")
}

func TestMediaUploadRoutesKeepAPIKeyAdmissionRules(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*service.APIKey)
		code int
	}{
		{"disabled key", func(key *service.APIKey) { key.Status = service.StatusDisabled }, http.StatusUnauthorized},
		{"expired key", func(key *service.APIKey) { expiry := time.Now().Add(-time.Hour); key.ExpiresAt = &expiry }, http.StatusForbidden},
		{"disabled user", func(key *service.APIKey) { key.User.Status = service.StatusDisabled }, http.StatusUnauthorized},
		{"no balance", func(key *service.APIKey) { key.User.Balance = 0 }, http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router, _, key := newMediaUploadRouteFixture(t, "https://storage.example")
			tc.edit(key)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, mediaUploadMultipartRequest(t, "images", mediaUploadRoutePNG(t)))
			require.Equal(t, tc.code, w.Code, w.Body.String())
		})
	}
}

func TestMediaUploadRoutesReportUnavailableAndStorageFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, "<Error><Code>AccessDenied</Code><Message>upload-test-secret</Message></Error>")
	}))
	defer server.Close()
	router, settings, _ := newMediaUploadRouteFixture(t, server.URL)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, mediaUploadMultipartRequest(t, "images", mediaUploadRoutePNG(t)))
	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	require.JSONEq(t, `{"success":false,"message":"Failed to upload file"}`, w.Body.String())
	cfg, err := settings.Get(context.Background(), service.MediaUploadImages)
	require.NoError(t, err)
	cfg.Enabled = false
	_, err = settings.Update(context.Background(), service.MediaUploadImages, *cfg)
	require.NoError(t, err)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, mediaUploadMultipartRequest(t, "images", mediaUploadRoutePNG(t)))
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestMediaUploadStorageUpdatesRequireStepUp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerBackupRoutes(router.Group("/api/v1/admin"), &handler.Handlers{
		Admin: &handler.AdminHandlers{Backup: &admin.BackupHandler{}},
	}, servermiddleware.StepUpAuthMiddleware(func(c *gin.Context) {
		c.AbortWithStatus(http.StatusForbidden)
	}))
	for _, kind := range []string{"images", "videos", "audios"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/api/v1/admin/backups/media-upload-storage/"+kind, nil))
		require.Equal(t, http.StatusForbidden, w.Code, "kind=%s", kind)
	}
}
