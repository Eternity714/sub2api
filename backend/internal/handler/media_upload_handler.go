package handler

import (
	"errors"
	"mime"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const mediaUploadMultipartMemory int64 = 1 << 20

// MediaUploadHandler accepts reference media using each kind's independent S3 configuration.
type MediaUploadHandler struct {
	settings *service.MediaUploadSettingService
	uploads  *service.MediaUploadService
}

func NewMediaUploadHandler(settings *service.MediaUploadSettingService) *MediaUploadHandler {
	return &MediaUploadHandler{settings: settings, uploads: service.NewMediaUploadService(settings)}
}

func (h *MediaUploadHandler) Images(c *gin.Context) { h.upload(c, service.MediaUploadImages) }
func (h *MediaUploadHandler) Videos(c *gin.Context) { h.upload(c, service.MediaUploadVideos) }
func (h *MediaUploadHandler) Audios(c *gin.Context) { h.upload(c, service.MediaUploadAudios) }

func (h *MediaUploadHandler) upload(c *gin.Context, kind service.MediaUploadKind) {
	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.UserID <= 0 {
		mediaUploadError(c, http.StatusUnauthorized, "Invalid API key")
		return
	}
	if h == nil || h.settings == nil || h.uploads == nil {
		mediaUploadError(c, http.StatusServiceUnavailable, "Upload storage is unavailable")
		return
	}
	cfg, err := h.settings.Get(c.Request.Context(), kind)
	if err != nil || !cfg.Enabled {
		mediaUploadError(c, http.StatusServiceUnavailable, "Upload storage is unavailable")
		return
	}

	contentType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || contentType != "multipart/form-data" {
		mediaUploadError(c, http.StatusBadRequest, "Use multipart/form-data with a file field")
		return
	}
	// The upload limit is independent of generation's gateway.max_body_size.
	// Permit bounded multipart headers/fields in addition to the single file.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, cfg.MaxUploadBytes+mediaUploadMultipartMemory)
	defer func() {
		if c.Request.MultipartForm != nil {
			_ = c.Request.MultipartForm.RemoveAll()
		}
	}()
	if err := c.Request.ParseMultipartForm(mediaUploadMultipartMemory); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			mediaUploadError(c, http.StatusBadRequest, "File exceeds the configured upload limit")
		} else {
			mediaUploadError(c, http.StatusBadRequest, "Invalid multipart upload")
		}
		return
	}
	form := c.Request.MultipartForm
	if form == nil || len(form.File["file"]) != 1 {
		mediaUploadError(c, http.StatusBadRequest, "Exactly one file field is required")
		return
	}
	fileCount := 0
	for _, files := range form.File {
		fileCount += len(files)
	}
	if fileCount != 1 {
		mediaUploadError(c, http.StatusBadRequest, "Exactly one file field is required")
		return
	}
	fileHeader := form.File["file"][0]
	if fileHeader.Size <= 0 || fileHeader.Size > cfg.MaxUploadBytes {
		mediaUploadError(c, http.StatusBadRequest, "File is empty or exceeds the configured upload limit")
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		mediaUploadError(c, http.StatusBadRequest, "Unable to read uploaded file")
		return
	}
	defer func() { _ = file.Close() }()
	result, err := h.uploads.Upload(c.Request.Context(), kind, apiKey.UserID, fileHeader.Filename, file, fileHeader.Size)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMediaUploadTooLarge):
			mediaUploadError(c, http.StatusBadRequest, "File exceeds the configured upload limit")
		case errors.Is(err, service.ErrMediaUploadInvalidFile):
			mediaUploadError(c, http.StatusBadRequest, "Unsupported or invalid media file")
		case errors.Is(err, service.ErrMediaUploadUnavailable):
			mediaUploadError(c, http.StatusServiceUnavailable, "Upload storage is unavailable")
		default:
			logger.L().Warn("media_upload.failed", zap.String("kind", string(kind)), zap.Error(err))
			mediaUploadError(c, http.StatusInternalServerError, "Failed to upload file")
		}
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

func mediaUploadError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"success": false, "message": message})
}
