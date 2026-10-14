package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type MediaUploadKind string

const (
	MediaUploadImages   MediaUploadKind = "images"
	MediaUploadVideos   MediaUploadKind = "videos"
	MediaUploadAudios   MediaUploadKind = "audios"
	MaxMediaUploadBytes int64           = 1 << 30
)

var (
	ErrMediaUploadUnavailable   = errors.New("media upload is disabled or object storage is unavailable")
	ErrMediaUploadInvalidConfig = errors.New("invalid media upload configuration")
)

// MediaUploadStorageSettings 对每一类上传独立保存；只有显式打开复用开关才读取备份 S3。
type MediaUploadStorageSettings struct {
	Enabled         bool   `json:"enabled"`
	ReuseBackupS3   bool   `json:"reuse_backup_s3"`
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key,omitempty"` //nolint:revive // AWS naming convention
	ForcePathStyle  bool   `json:"force_path_style"`
	Prefix          string `json:"prefix"`
	PublicBaseURL   string `json:"public_base_url"`
	PresignExpiry   int    `json:"presign_expiry_hours"`
	MaxUploadBytes  int64  `json:"max_upload_bytes"`
}

type mediaUploadStorageCache struct {
	fingerprint [sha256.Size]byte
	storage     ImageStorage
}

type MediaUploadSettingService struct {
	settingRepo SettingRepository
	encryptor   SecretEncryptor
	backup      *BackupService
	factory     ImageStorageFactory
	writeMu     sync.Mutex
	mu          sync.Mutex
	clients     map[MediaUploadKind]mediaUploadStorageCache
}

func NewMediaUploadSettingService(repo SettingRepository, encryptor SecretEncryptor, backup *BackupService, factory ImageStorageFactory) *MediaUploadSettingService {
	return &MediaUploadSettingService{settingRepo: repo, encryptor: encryptor, backup: backup, factory: factory, clients: make(map[MediaUploadKind]mediaUploadStorageCache)}
}

func (s *MediaUploadSettingService) Get(ctx context.Context, kind MediaUploadKind) (*MediaUploadStorageSettings, error) {
	settings, err := s.load(ctx, kind)
	if err != nil {
		return nil, err
	}
	settings.SecretAccessKey = ""
	return settings, nil
}

// SecretConfigured 仅反映独立密钥是否已保存；备份复用不改变独立凭证的状态。
func (s *MediaUploadSettingService) SecretConfigured(ctx context.Context, kind MediaUploadKind) bool {
	settings, err := s.load(ctx, kind)
	if err != nil {
		return false
	}
	return settings.SecretAccessKey != ""
}

// Update 留空密钥只保留同类已存密文；复用开关保留独立凭证，不复制备份密钥。
func (s *MediaUploadSettingService) Update(ctx context.Context, kind MediaUploadKind, in MediaUploadStorageSettings) (*MediaUploadStorageSettings, error) {
	if err := normalizeMediaUploadSettings(kind, &in); err != nil {
		return nil, err
	}
	if s == nil || s.settingRepo == nil {
		return nil, ErrMediaUploadUnavailable
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if in.ReuseBackupS3 {
		old, err := s.load(ctx, kind)
		if err != nil {
			return nil, err
		}
		in.Endpoint, in.Region = old.Endpoint, old.Region
		in.AccessKeyID, in.SecretAccessKey = old.AccessKeyID, old.SecretAccessKey
		in.ForcePathStyle = old.ForcePathStyle
	} else if in.SecretAccessKey == "" {
		old, err := s.load(ctx, kind)
		if err != nil {
			return nil, err
		}
		in.SecretAccessKey = old.SecretAccessKey
	} else {
		if s.encryptor == nil || s.backup == nil || !s.backup.EncryptionKeyConfigured() {
			return nil, ErrSecretEncryptionKeyNotConfigured
		}
		secret, err := s.encryptor.Encrypt(in.SecretAccessKey)
		if err != nil {
			return nil, fmt.Errorf("encrypt media upload secret: %w", err)
		}
		in.SecretAccessKey = secret
	}
	data, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("marshal media upload settings: %w", err)
	}
	if err := s.settingRepo.Set(ctx, mediaUploadSettingKey(kind), string(data)); err != nil {
		return nil, fmt.Errorf("save media upload settings: %w", err)
	}
	s.Invalidate()
	in.SecretAccessKey = ""
	return &in, nil
}

// Resolve 重新读取独立配置和备份引用，按有效配置复用 S3 客户端，备份轮换立即生效。
func (s *MediaUploadSettingService) Resolve(ctx context.Context, kind MediaUploadKind) (ImageStorage, *MediaUploadStorageSettings, error) {
	if s == nil || s.factory == nil {
		return nil, nil, ErrMediaUploadUnavailable
	}
	settings, err := s.load(ctx, kind)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: load configuration: %w", ErrMediaUploadUnavailable, err)
	}
	if !settings.Enabled {
		return nil, nil, ErrMediaUploadUnavailable
	}
	cfg, err := s.toStorageConfig(ctx, settings, true)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", ErrMediaUploadUnavailable, err)
	}
	if !cfg.IsConfigured() {
		return nil, nil, ErrMediaUploadUnavailable
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: encode configuration: %w", ErrMediaUploadUnavailable, err)
	}
	fingerprint := sha256.Sum256(data)
	s.mu.Lock()
	defer s.mu.Unlock()
	cached, ok := s.clients[kind]
	if !ok || cached.fingerprint != fingerprint {
		storage, err := s.factory(ctx, cfg)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: build object storage: %w", ErrMediaUploadUnavailable, err)
		}
		if storage == nil {
			return nil, nil, ErrMediaUploadUnavailable
		}
		cached = mediaUploadStorageCache{fingerprint: fingerprint, storage: storage}
		if s.clients == nil {
			s.clients = make(map[MediaUploadKind]mediaUploadStorageCache)
		}
		s.clients[kind] = cached
	}
	settings.SecretAccessKey = ""
	return cached.storage, settings, nil
}

func (s *MediaUploadSettingService) Invalidate() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.clients = make(map[MediaUploadKind]mediaUploadStorageCache)
	s.mu.Unlock()
}

type mediaUploadConnectionTester interface {
	TestConnection(ctx context.Context) error
}

// TestConnection 执行存储端连接检查，不保存表单。留空密钥可使用同类已保存的密钥。
func (s *MediaUploadSettingService) TestConnection(ctx context.Context, kind MediaUploadKind, in MediaUploadStorageSettings) error {
	if err := normalizeMediaUploadSettings(kind, &in); err != nil {
		return err
	}
	if s == nil || s.factory == nil {
		return ErrMediaUploadUnavailable
	}
	storedSecret := false
	if !in.ReuseBackupS3 && in.SecretAccessKey == "" {
		old, err := s.load(ctx, kind)
		if err != nil {
			return err
		}
		in.SecretAccessKey, storedSecret = old.SecretAccessKey, true
	}
	cfg, err := s.toStorageConfig(ctx, &in, storedSecret)
	if err != nil {
		return err
	}
	if !cfg.IsConfigured() {
		return ErrMediaUploadUnavailable
	}
	storage, err := s.factory(ctx, cfg)
	if err != nil {
		return err
	}
	tester, ok := storage.(mediaUploadConnectionTester)
	if !ok {
		return errors.New("object storage does not support connection testing")
	}
	return tester.TestConnection(ctx)
}

func (s *MediaUploadSettingService) toStorageConfig(ctx context.Context, settings *MediaUploadStorageSettings, encryptedSecret bool) (*config.ImageStorageConfig, error) {
	cfg := &config.ImageStorageConfig{Enabled: settings.Enabled, Endpoint: settings.Endpoint, Region: settings.Region,
		Bucket: settings.Bucket, AccessKeyID: settings.AccessKeyID, SecretAccessKey: settings.SecretAccessKey,
		ForcePathStyle: settings.ForcePathStyle, Prefix: settings.Prefix, PublicBaseURL: settings.PublicBaseURL,
		PresignExpiry: settings.PresignExpiry, MaxDownloadByte: settings.MaxUploadBytes}
	if settings.ReuseBackupS3 {
		backupCfg, err := s.backupCredentials(ctx)
		if err != nil {
			return nil, err
		}
		if backupCfg == nil {
			return nil, errors.New("backup S3 is not configured")
		}
		cfg.Endpoint, cfg.Region = backupCfg.Endpoint, backupCfg.Region
		cfg.AccessKeyID, cfg.SecretAccessKey = backupCfg.AccessKeyID, backupCfg.SecretAccessKey
		cfg.ForcePathStyle = backupCfg.ForcePathStyle
		if cfg.Bucket == "" {
			cfg.Bucket = backupCfg.Bucket
		}
	} else if encryptedSecret && cfg.SecretAccessKey != "" {
		if s.encryptor == nil {
			return nil, errors.New("secret encryptor is unavailable")
		}
		secret, err := s.encryptor.Decrypt(cfg.SecretAccessKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt media upload secret: %w", err)
		}
		cfg.SecretAccessKey = secret
	}
	if cfg.Region == "" {
		cfg.Region = "auto"
	}
	return cfg, nil
}

func (s *MediaUploadSettingService) backupCredentials(ctx context.Context) (*BackupS3Config, error) {
	if s.backup == nil {
		return nil, errors.New("backup service is unavailable")
	}
	return s.backup.loadS3Config(ctx)
}

func (s *MediaUploadSettingService) load(ctx context.Context, kind MediaUploadKind) (*MediaUploadStorageSettings, error) {
	settings := &MediaUploadStorageSettings{}
	if !validMediaUploadKind(kind) {
		return nil, fmt.Errorf("%w: unsupported media upload kind %q", ErrMediaUploadInvalidConfig, kind)
	}
	if s != nil && s.settingRepo != nil {
		raw, err := s.settingRepo.GetValue(ctx, mediaUploadSettingKey(kind))
		if err != nil && !errors.Is(err, ErrSettingNotFound) {
			return nil, fmt.Errorf("load media upload settings: %w", err)
		}
		if strings.TrimSpace(raw) != "" {
			if err := json.Unmarshal([]byte(raw), settings); err != nil {
				return nil, fmt.Errorf("parse media upload settings: %w", err)
			}
		}
	}
	if err := normalizeMediaUploadSettings(kind, settings); err != nil {
		return nil, err
	}
	return settings, nil
}

func mediaUploadSettingKey(kind MediaUploadKind) string {
	return "media_upload_" + string(kind) + "_config"
}

func validMediaUploadKind(kind MediaUploadKind) bool {
	return kind == MediaUploadImages || kind == MediaUploadVideos || kind == MediaUploadAudios
}

func normalizeMediaUploadSettings(kind MediaUploadKind, in *MediaUploadStorageSettings) error {
	if !validMediaUploadKind(kind) {
		return fmt.Errorf("%w: unsupported media upload kind %q", ErrMediaUploadInvalidConfig, kind)
	}
	if in.MaxUploadBytes < 0 || in.MaxUploadBytes > MaxMediaUploadBytes {
		return fmt.Errorf("%w: max_upload_bytes must be between 0 and %d", ErrMediaUploadInvalidConfig, MaxMediaUploadBytes)
	}
	if in.MaxUploadBytes == 0 {
		in.MaxUploadBytes = 50 << 20
		if kind == MediaUploadImages {
			in.MaxUploadBytes = 10 << 20
		}
	}
	if in.PresignExpiry <= 0 {
		in.PresignExpiry = 24
	}
	if in.PresignExpiry > 168 {
		return fmt.Errorf("%w: presign_expiry_hours cannot exceed 168", ErrMediaUploadInvalidConfig)
	}
	in.Endpoint, in.Region, in.Bucket = strings.TrimSpace(in.Endpoint), strings.TrimSpace(in.Region), strings.TrimSpace(in.Bucket)
	in.AccessKeyID, in.SecretAccessKey = strings.TrimSpace(in.AccessKeyID), strings.TrimSpace(in.SecretAccessKey)
	in.PublicBaseURL = strings.TrimRight(strings.TrimSpace(in.PublicBaseURL), "/")
	for _, raw := range []string{in.Endpoint, in.PublicBaseURL} {
		if raw == "" {
			continue
		}
		parsed, err := url.Parse(raw)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("%w: object storage URLs must be absolute HTTP or HTTPS URLs without credentials, query or fragment", ErrMediaUploadInvalidConfig)
		}
	}
	in.Prefix = strings.Trim(strings.TrimSpace(in.Prefix), "/")
	if in.Prefix == "" {
		in.Prefix = "uploads/" + string(kind)
	}
	for _, segment := range strings.Split(in.Prefix, "/") {
		if segment == "." || segment == ".." || strings.ContainsAny(segment, "\\?#\r\n\x00") {
			return fmt.Errorf("%w: invalid object storage prefix", ErrMediaUploadInvalidConfig)
		}
	}
	in.Prefix += "/"
	if in.Region == "" {
		in.Region = "auto"
	}
	return nil
}
