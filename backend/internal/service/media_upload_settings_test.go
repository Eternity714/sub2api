//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func newMediaUploadSettingsFixture(t *testing.T, encryptionKeyConfigured bool) (*MediaUploadSettingService, *stubSettingRepo, *[]config.ImageStorageConfig) {
	t.Helper()
	repo := newStubSettingRepo()
	encryptor := reversibleEncryptor{}
	backup := NewBackupService(repo, &config.Config{Totp: config.TotpConfig{EncryptionKeyConfigured: encryptionKeyConfigured}}, encryptor, nil, nil)
	var built []config.ImageStorageConfig
	factory := func(_ context.Context, cfg *config.ImageStorageConfig) (ImageStorage, error) {
		built = append(built, *cfg)
		return &mediaUploadRecordingStorage{}, nil
	}
	return NewMediaUploadSettingService(repo, encryptor, backup, factory), repo, &built
}

func TestMediaUploadSettingsRequireExplicitOptInAndKeepKindsIndependent(t *testing.T) {
	ctx := context.Background()
	svc, repo, built := newMediaUploadSettingsFixture(t, true)
	seedBackupS3(t, repo, BackupS3Config{Bucket: "backups", AccessKeyID: "backup-ak", SecretAccessKey: "backup-sk"})
	for _, tc := range []struct {
		kind  MediaUploadKind
		limit int64
	}{{MediaUploadImages, 10 << 20}, {MediaUploadVideos, 50 << 20}, {MediaUploadAudios, 50 << 20}} {
		got, err := svc.Get(ctx, tc.kind)
		require.NoError(t, err)
		require.False(t, got.Enabled)
		require.False(t, got.ReuseBackupS3)
		require.Empty(t, got.Bucket)
		require.Equal(t, tc.limit, got.MaxUploadBytes)
		require.Equal(t, "uploads/"+string(tc.kind)+"/", got.Prefix)
		_, _, err = svc.Resolve(ctx, tc.kind)
		require.ErrorIs(t, err, ErrMediaUploadUnavailable)
	}
	require.Empty(t, *built)
	_, err := svc.Update(ctx, MediaUploadImages, MediaUploadStorageSettings{Enabled: true})
	require.NoError(t, err)
	_, _, err = svc.Resolve(ctx, MediaUploadImages)
	require.ErrorIs(t, err, ErrMediaUploadUnavailable, "enabling alone must never borrow configured backup S3")
	_, err = svc.Update(ctx, MediaUploadVideos, MediaUploadStorageSettings{Enabled: true, Bucket: "videos", AccessKeyID: "video-ak", SecretAccessKey: "video-sk", MaxUploadBytes: 8 << 20})
	require.NoError(t, err)
	storage, settings, err := svc.Resolve(ctx, MediaUploadVideos)
	require.NoError(t, err)
	require.NotNil(t, storage)
	require.EqualValues(t, 8<<20, settings.MaxUploadBytes)
	require.Empty(t, settings.SecretAccessKey)
	require.Equal(t, "videos", (*built)[0].Bucket)
	require.Equal(t, "video-sk", (*built)[0].SecretAccessKey)
	images, err := svc.Get(ctx, MediaUploadImages)
	require.NoError(t, err)
	require.Empty(t, images.Bucket)
	audios, err := svc.Get(ctx, MediaUploadAudios)
	require.NoError(t, err)
	require.False(t, audios.Enabled)
}

func TestMediaUploadSettingsEncryptSecretAndPreserveBlankOnUpdate(t *testing.T) {
	ctx := context.Background()
	svc, repo, built := newMediaUploadSettingsFixture(t, true)
	in := MediaUploadStorageSettings{Enabled: true, Bucket: "images", AccessKeyID: "ak", SecretAccessKey: "my-secret"}
	out, err := svc.Update(ctx, MediaUploadImages, in)
	require.NoError(t, err)
	require.Empty(t, out.SecretAccessKey)
	raw, err := repo.GetValue(ctx, "media_upload_images_config")
	require.NoError(t, err)
	var stored MediaUploadStorageSettings
	require.NoError(t, json.Unmarshal([]byte(raw), &stored))
	require.Equal(t, "enc:my-secret", stored.SecretAccessKey)
	require.True(t, svc.SecretConfigured(ctx, MediaUploadImages))
	got, err := svc.Get(ctx, MediaUploadImages)
	require.NoError(t, err)
	require.Empty(t, got.SecretAccessKey)
	in.SecretAccessKey = ""
	in.Bucket = "new-images"
	_, err = svc.Update(ctx, MediaUploadImages, in)
	require.NoError(t, err)
	_, _, err = svc.Resolve(ctx, MediaUploadImages)
	require.NoError(t, err)
	require.Equal(t, "my-secret", (*built)[0].SecretAccessKey)
	require.Equal(t, "new-images", (*built)[0].Bucket)
	_, err = svc.Update(ctx, MediaUploadAudios, MediaUploadStorageSettings{Enabled: true, Bucket: "audio", AccessKeyID: "ak"})
	require.NoError(t, err)
	_, _, err = svc.Resolve(ctx, MediaUploadAudios)
	require.ErrorIs(t, err, ErrMediaUploadUnavailable, "blank secret only retains this kind's own secret")
}

func TestMediaUploadSettingsBackupReuseIsExplicitAndTracksBackupChanges(t *testing.T) {
	ctx := context.Background()
	svc, repo, built := newMediaUploadSettingsFixture(t, true)
	seedBackupS3(t, repo, BackupS3Config{Endpoint: "https://backup.example", Region: "us-east-1", Bucket: "backups", AccessKeyID: "backup-ak", SecretAccessKey: "backup-sk", ForcePathStyle: true})
	_, err := svc.Update(ctx, MediaUploadImages, MediaUploadStorageSettings{Enabled: true, ReuseBackupS3: true, Endpoint: "https://ignored.example", AccessKeyID: "ignored-ak", SecretAccessKey: "ignored-sk"})
	require.NoError(t, err)
	_, _, err = svc.Resolve(ctx, MediaUploadImages)
	require.NoError(t, err)
	require.Equal(t, "https://backup.example", (*built)[0].Endpoint)
	require.Equal(t, "backups", (*built)[0].Bucket)
	require.Equal(t, "backup-sk", (*built)[0].SecretAccessKey)
	require.True(t, (*built)[0].ForcePathStyle)
	raw, _ := repo.GetValue(ctx, "media_upload_images_config")
	require.NotContains(t, raw, "backup-sk")
	require.NotContains(t, raw, "ignored-sk")
	require.False(t, svc.SecretConfigured(ctx, MediaUploadImages), "backup credentials must not appear as a saved independent secret")
	_, err = svc.Update(ctx, MediaUploadVideos, MediaUploadStorageSettings{Enabled: true, ReuseBackupS3: true, Bucket: "video-only"})
	require.NoError(t, err)
	_, _, err = svc.Resolve(ctx, MediaUploadVideos)
	require.NoError(t, err)
	require.Equal(t, "video-only", (*built)[1].Bucket)
	seedBackupS3(t, repo, BackupS3Config{Bucket: "rotated", AccessKeyID: "new-ak", SecretAccessKey: "new-sk"})
	_, _, err = svc.Resolve(ctx, MediaUploadImages)
	require.NoError(t, err)
	require.Equal(t, "rotated", (*built)[2].Bucket)
	require.Equal(t, "new-sk", (*built)[2].SecretAccessKey)
}

func TestMediaUploadSettingsReuseSwitchPreservesIndependentCredentials(t *testing.T) {
	ctx := context.Background()
	svc, repo, built := newMediaUploadSettingsFixture(t, true)
	seedBackupS3(t, repo, BackupS3Config{Endpoint: "https://backup.example", Region: "auto", Bucket: "backup-bucket", AccessKeyID: "backup-ak", SecretAccessKey: "backup-sk"})
	_, err := svc.Update(ctx, MediaUploadImages, MediaUploadStorageSettings{Enabled: true, Endpoint: "https://independent.example", Region: "us-east-1", Bucket: "media", AccessKeyID: "own-ak", SecretAccessKey: "own-sk", ForcePathStyle: true})
	require.NoError(t, err)
	_, _, err = svc.Resolve(ctx, MediaUploadImages)
	require.NoError(t, err)
	require.Equal(t, "own-sk", (*built)[0].SecretAccessKey)
	_, err = svc.Update(ctx, MediaUploadImages, MediaUploadStorageSettings{Enabled: true, ReuseBackupS3: true, Bucket: "media"})
	require.NoError(t, err)
	got, err := svc.Get(ctx, MediaUploadImages)
	require.NoError(t, err)
	require.Equal(t, "https://independent.example", got.Endpoint)
	require.Equal(t, "us-east-1", got.Region)
	require.Equal(t, "own-ak", got.AccessKeyID)
	require.True(t, got.ForcePathStyle)
	require.Empty(t, got.SecretAccessKey)
	require.True(t, svc.SecretConfigured(ctx, MediaUploadImages))
	raw, err := repo.GetValue(ctx, "media_upload_images_config")
	require.NoError(t, err)
	require.Contains(t, raw, "enc:own-sk")
	require.NotContains(t, raw, "backup-sk")
	_, _, err = svc.Resolve(ctx, MediaUploadImages)
	require.NoError(t, err)
	require.Equal(t, "backup-ak", (*built)[1].AccessKeyID)
	require.Equal(t, "backup-sk", (*built)[1].SecretAccessKey)
	got.ReuseBackupS3 = false
	_, err = svc.Update(ctx, MediaUploadImages, *got)
	require.NoError(t, err)
	_, _, err = svc.Resolve(ctx, MediaUploadImages)
	require.NoError(t, err)
	require.Equal(t, "https://independent.example", (*built)[2].Endpoint)
	require.Equal(t, "us-east-1", (*built)[2].Region)
	require.Equal(t, "own-ak", (*built)[2].AccessKeyID)
	require.Equal(t, "own-sk", (*built)[2].SecretAccessKey)
	require.True(t, (*built)[2].ForcePathStyle)
}

func TestMediaUploadSettingsValidateLimitsAndURLs(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newMediaUploadSettingsFixture(t, true)
	for _, in := range []MediaUploadStorageSettings{
		{MaxUploadBytes: -1}, {MaxUploadBytes: (1 << 30) + 1},
		{PresignExpiry: 169},
		{Endpoint: "ftp://storage.example"}, {PublicBaseURL: "javascript:alert(1)"},
		{Prefix: "../escape"}, {Prefix: "uploads/../escape"},
	} {
		_, err := svc.Update(ctx, MediaUploadImages, in)
		require.ErrorIs(t, err, ErrMediaUploadInvalidConfig, "%+v", in)
	}
	got, err := svc.Update(ctx, MediaUploadImages, MediaUploadStorageSettings{MaxUploadBytes: 1 << 30, Prefix: " custom/ ", PublicBaseURL: "https://cdn.example/path/"})
	require.NoError(t, err)
	require.EqualValues(t, 1<<30, got.MaxUploadBytes)
	require.Equal(t, "custom/", got.Prefix)
	require.Equal(t, "https://cdn.example/path", got.PublicBaseURL)
	_, err = svc.Get(ctx, MediaUploadKind("unknown"))
	require.Error(t, err)
}

func TestMediaUploadSettingsRejectNonDurableKeyAndDecryptFailure(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newMediaUploadSettingsFixture(t, false)
	_, err := svc.Update(ctx, MediaUploadImages, MediaUploadStorageSettings{SecretAccessKey: "secret"})
	require.ErrorIs(t, err, ErrSecretEncryptionKeyNotConfigured)
	svc, repo, _ := newMediaUploadSettingsFixture(t, true)
	require.NoError(t, repo.Set(ctx, "media_upload_images_config", `{"enabled":true,"bucket":"b","access_key_id":"ak","secret_access_key":"undecryptable"}`))
	_, _, err = svc.Resolve(ctx, MediaUploadImages)
	require.ErrorIs(t, err, ErrMediaUploadUnavailable)
}

func TestMediaUploadSettingsTestConnectionUsesSavedSecretWithoutSaving(t *testing.T) {
	ctx := context.Background()
	svc, repo, built := newMediaUploadSettingsFixture(t, true)
	in := MediaUploadStorageSettings{Bucket: "bucket", AccessKeyID: "ak", SecretAccessKey: "secret"}
	_, err := svc.Update(ctx, MediaUploadImages, in)
	require.NoError(t, err)
	in.SecretAccessKey = ""
	in.Bucket = "test-bucket"
	require.NoError(t, svc.TestConnection(ctx, MediaUploadImages, in))
	require.Equal(t, "secret", (*built)[0].SecretAccessKey)
	got, err := svc.Get(ctx, MediaUploadImages)
	require.NoError(t, err)
	require.Equal(t, "bucket", got.Bucket)
	denied := errors.New("head bucket denied")
	svc.factory = func(context.Context, *config.ImageStorageConfig) (ImageStorage, error) {
		return &mediaUploadRecordingStorage{testError: denied}, nil
	}
	require.ErrorIs(t, svc.TestConnection(ctx, MediaUploadImages, in), denied)
	require.Len(t, repo.values, 1)
}
