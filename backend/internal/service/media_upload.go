package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"
)

var (
	ErrMediaUploadInvalidFile = errors.New("invalid or unsupported media file")
	ErrMediaUploadTooLarge    = errors.New("media file exceeds configured upload limit")
)

// MediaStorageWithReader 将文件直接送入 S3 SDK，避免复制完整视频或音频到内存。
type MediaStorageWithReader interface {
	SaveReader(ctx context.Context, key, contentType string, body io.Reader, size int64) (ImageStorageSaveMetadata, error)
}

type MediaUploadResult struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
}

type MediaUploadService struct{ settings *MediaUploadSettingService }

func NewMediaUploadService(settings *MediaUploadSettingService) *MediaUploadService {
	return &MediaUploadService{settings: settings}
}

func (s *MediaUploadService) Upload(ctx context.Context, kind MediaUploadKind, userID int64, fileName string, body io.ReadSeeker, size int64) (*MediaUploadResult, error) {
	if s == nil || s.settings == nil {
		return nil, ErrMediaUploadUnavailable
	}
	storage, settings, err := s.settings.Resolve(ctx, kind)
	if err != nil {
		return nil, err
	}
	stream, ok := storage.(MediaStorageWithReader)
	if !ok {
		return nil, ErrMediaUploadUnavailable
	}
	if body == nil || size <= 0 || userID <= 0 {
		return nil, ErrMediaUploadInvalidFile
	}
	if size > settings.MaxUploadBytes {
		return nil, ErrMediaUploadTooLarge
	}
	actualSize, err := body.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, fmt.Errorf("%w: determine file size: %w", ErrMediaUploadInvalidFile, err)
	}
	if actualSize > settings.MaxUploadBytes {
		return nil, ErrMediaUploadTooLarge
	}
	if actualSize != size {
		return nil, fmt.Errorf("%w: file size mismatch", ErrMediaUploadInvalidFile)
	}
	if _, err := body.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("%w: rewind file: %w", ErrMediaUploadInvalidFile, err)
	}
	detected, err := mimetype.DetectReader(body)
	if err != nil {
		return nil, fmt.Errorf("%w: detect content: %w", ErrMediaUploadInvalidFile, err)
	}
	contentType, extension := mediaUploadContentType(kind, detected.String())
	if contentType == "" {
		return nil, ErrMediaUploadInvalidFile
	}
	if _, err := body.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("%w: rewind file: %w", ErrMediaUploadInvalidFile, err)
	}
	id := uuid.NewString()
	// The untrusted file name is intentionally excluded from the storage key.
	key := settings.Prefix + strconv.FormatInt(userID, 10) + "/" + id + extension
	metadata, err := stream.SaveReader(ctx, key, contentType, body, size)
	if err != nil {
		return nil, fmt.Errorf("upload media to object storage: %w", err)
	}
	if strings.TrimSpace(metadata.URL) == "" {
		return nil, errors.New("object storage returned an empty media URL")
	}
	return &MediaUploadResult{ID: id, URL: metadata.URL, MimeType: contentType, Size: size}, nil
}

func mediaUploadContentType(kind MediaUploadKind, detected string) (contentType, extension string) {
	switch kind {
	case MediaUploadImages:
		switch detected {
		case "image/jpeg":
			return detected, ".jpg"
		case "image/png":
			return detected, ".png"
		case "image/webp":
			return detected, ".webp"
		case "image/gif":
			return detected, ".gif"
		}
	case MediaUploadVideos:
		switch detected {
		case "video/mp4":
			return detected, ".mp4"
		case "video/webm":
			return detected, ".webm"
		case "video/quicktime":
			return detected, ".mov"
		}
	case MediaUploadAudios:
		switch detected {
		case "audio/mpeg":
			return detected, ".mp3"
		case "audio/wav", "audio/x-wav", "audio/vnd.wave":
			return "audio/wav", ".wav"
		case "audio/mp4", "audio/x-m4a":
			return "audio/mp4", ".m4a"
		}
	}
	return "", ""
}
