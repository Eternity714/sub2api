package repository

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/servertiming"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// S3ImageStorage 用 S3 兼容对象存储实现 service.ImageStorage。
type S3ImageStorage struct {
	client        *s3.Client
	bucket        string
	publicBaseURL string
	presignExpiry time.Duration
}

var _ service.ImageStorage = (*S3ImageStorage)(nil)
var _ service.ImageStorageWithMetadata = (*S3ImageStorage)(nil)

// NewS3ImageStorage 依据配置构造 S3 图片存储（调用方应先确认 cfg.Active()）。
func NewS3ImageStorage(ctx context.Context, cfg *config.ImageStorageConfig) (*S3ImageStorage, error) {
	client, err := newS3Client(ctx, s3ClientParams{
		Endpoint:        cfg.Endpoint,
		Region:          cfg.Region,
		AccessKeyID:     cfg.AccessKeyID,
		SecretAccessKey: cfg.SecretAccessKey,
		ForcePathStyle:  cfg.ForcePathStyle,
	})
	if err != nil {
		return nil, err
	}

	expiry := time.Duration(cfg.PresignExpiry) * time.Hour
	if expiry <= 0 {
		expiry = 24 * time.Hour
	}

	return &S3ImageStorage{
		client:        client,
		bucket:        cfg.Bucket,
		publicBaseURL: strings.TrimRight(cfg.PublicBaseURL, "/"),
		presignExpiry: expiry,
	}, nil
}

// Save 上传图片字节，返回可访问 URL：配了 public_base_url 则返回公开直链，否则返回 presigned 临时链接。
func (s *S3ImageStorage) Save(ctx context.Context, key, contentType string, data []byte) (string, error) {
	metadata, err := s.SaveWithMetadata(ctx, key, contentType, data)
	return metadata.URL, err
}

func (s *S3ImageStorage) SaveWithMetadata(ctx context.Context, key, contentType string, data []byte) (service.ImageStorageSaveMetadata, error) {
	finish := servertiming.ObserveDependency(ctx, "s3")
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &s.bucket,
		Key:         &key,
		Body:        bytes.NewReader(data),
		ContentType: &contentType,
	})
	finish()
	if err != nil {
		return service.ImageStorageSaveMetadata{}, fmt.Errorf("S3 PutObject: %w", err)
	}
	metadata := service.ImageStorageSaveMetadata{ObjectKey: key, ContentType: contentType}

	if s.publicBaseURL != "" {
		metadata.URL = s.publicBaseURL + "/" + strings.TrimLeft(key, "/")
		return metadata, nil
	}

	presignClient := s3.NewPresignClient(s.client)
	result, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	}, s3.WithPresignExpires(s.presignExpiry))
	if err != nil {
		return service.ImageStorageSaveMetadata{}, fmt.Errorf("presign url: %w", err)
	}
	parsed, err := url.Parse(result.URL)
	if err != nil {
		return service.ImageStorageSaveMetadata{}, fmt.Errorf("parse presigned url: %w", err)
	}
	signedAt, err := time.Parse("20060102T150405Z", parsed.Query().Get("X-Amz-Date"))
	if err != nil {
		return service.ImageStorageSaveMetadata{}, fmt.Errorf("parse presigned date: %w", err)
	}
	expiresSeconds, err := strconv.ParseInt(parsed.Query().Get("X-Amz-Expires"), 10, 64)
	if err != nil || expiresSeconds <= 0 {
		return service.ImageStorageSaveMetadata{}, fmt.Errorf("parse presigned expiry: invalid seconds")
	}
	expiresAt := signedAt.Add(time.Duration(expiresSeconds) * time.Second)
	metadata.URL = result.URL
	metadata.LinkExpiresAt = &expiresAt
	return metadata, nil
}
