//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type mediaUploadRecordingStorage struct {
	key         string
	contentType string
	data        []byte
	size        int64
	reader      io.Reader
	saveError   error
	testError   error
	emptyURL    bool
}

func (s *mediaUploadRecordingStorage) Save(context.Context, string, string, []byte) (string, error) {
	return "", errors.New("upload must use the streaming method")
}

func (s *mediaUploadRecordingStorage) SaveReader(_ context.Context, key, contentType string, body io.Reader, size int64) (ImageStorageSaveMetadata, error) {
	s.reader, s.key, s.contentType, s.size = body, key, contentType, size
	data, err := io.ReadAll(body)
	if err != nil {
		return ImageStorageSaveMetadata{}, err
	}
	s.data = data
	url := "https://cdn.example/" + key
	if s.emptyURL {
		url = ""
	}
	return ImageStorageSaveMetadata{URL: url, ObjectKey: key, ContentType: contentType}, s.saveError
}

func (s *mediaUploadRecordingStorage) TestConnection(context.Context) error { return s.testError }

func mediaUploadPNG(t *testing.T) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+j6WQAAAAASUVORK5CYII=")
	require.NoError(t, err)
	return data
}

func newMediaUploadFixture(t *testing.T, kind MediaUploadKind, limit int64) (*MediaUploadService, *mediaUploadRecordingStorage) {
	t.Helper()
	settings, _, _ := newMediaUploadSettingsFixture(t, true)
	storage := &mediaUploadRecordingStorage{}
	settings.factory = func(context.Context, *config.ImageStorageConfig) (ImageStorage, error) { return storage, nil }
	_, err := settings.Update(context.Background(), kind, MediaUploadStorageSettings{Enabled: true, Bucket: "b", AccessKeyID: "ak", SecretAccessKey: "sk", Prefix: "uploads", MaxUploadBytes: limit})
	require.NoError(t, err)
	return NewMediaUploadService(settings), storage
}

func TestMediaUploadSniffsContentAndUsesUniqueKeysAndStreaming(t *testing.T) {
	svc, storage := newMediaUploadFixture(t, MediaUploadImages, 0)
	data := mediaUploadPNG(t)
	body := bytes.NewReader(data)
	result, err := svc.Upload(context.Background(), MediaUploadImages, 42, "../../unsafe.jpg", body, int64(len(data)))
	require.NoError(t, err)
	require.Equal(t, "image/png", result.MimeType)
	require.EqualValues(t, len(data), result.Size)
	require.Equal(t, "image/png", storage.contentType)
	require.Equal(t, data, storage.data, "sniffing must rewind the reader")
	require.Same(t, body, storage.reader, "the storage receives the original reader without buffering the file")
	require.True(t, strings.HasPrefix(storage.key, "uploads/42/"))
	require.NotContains(t, storage.key, "/images/")
	require.True(t, strings.HasSuffix(storage.key, ".png"))
	require.NotContains(t, storage.key, "unsafe")
	require.NotContains(t, storage.key, "..")
	require.NoError(t, uuid.Validate(result.ID))
	require.Equal(t, "https://cdn.example/"+storage.key, result.URL)
	second, err := svc.Upload(context.Background(), MediaUploadImages, 42, "x.png", bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	require.NotEqual(t, result.ID, second.ID)
}

func TestMediaUploadUsesSeparateDefaultPrefixes(t *testing.T) {
	for _, tc := range []struct {
		kind   MediaUploadKind
		data   []byte
		prefix string
	}{
		{MediaUploadImages, mediaUploadPNG(t), "uploads/images/42/"},
		{MediaUploadVideos, []byte("\x00\x00\x00\x18ftypisom\x00\x00\x00\x00isommp41"), "uploads/videos/42/"},
		{MediaUploadAudios, []byte("ID3\x03\x00\x00\x00\x00\x00\x00\xff\xfb\x90\x64"), "uploads/audios/42/"},
	} {
		t.Run(string(tc.kind), func(t *testing.T) {
			svc, storage := newMediaUploadFixture(t, tc.kind, 0)
			_, err := svc.settings.Update(context.Background(), tc.kind, MediaUploadStorageSettings{Enabled: true, Bucket: "b", AccessKeyID: "ak"})
			require.NoError(t, err)
			got, err := svc.Upload(context.Background(), tc.kind, 42, "name", bytes.NewReader(tc.data), int64(len(tc.data)))
			require.NoError(t, err)
			require.True(t, strings.HasPrefix(storage.key, tc.prefix), storage.key)
			require.Equal(t, 1, strings.Count(storage.key, "/"+string(tc.kind)+"/"))
			require.Contains(t, storage.key, got.ID)
		})
	}
}

func TestMediaUploadRejectsEmptyStorageURL(t *testing.T) {
	svc, storage := newMediaUploadFixture(t, MediaUploadImages, 0)
	storage.emptyURL = true
	data := mediaUploadPNG(t)
	got, err := svc.Upload(context.Background(), MediaUploadImages, 42, "photo.png", bytes.NewReader(data), int64(len(data)))
	require.Error(t, err)
	require.Nil(t, got)
}

func TestMediaUploadRejectsWrongKindsSpoofsEmptyAndMismatchedSize(t *testing.T) {
	ctx := context.Background()
	svc, storage := newMediaUploadFixture(t, MediaUploadImages, 100)
	for _, tc := range []struct {
		name string
		body []byte
		size int64
	}{
		{"spoof.png", []byte("<html><script>alert(1)</script></html>"), 38},
		{"empty.png", nil, 0},
		{"short.png", mediaUploadPNG(t), 1},
		{"long.png", mediaUploadPNG(t), 90},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Upload(ctx, MediaUploadImages, 1, tc.name, bytes.NewReader(tc.body), tc.size)
			require.ErrorIs(t, err, ErrMediaUploadInvalidFile)
		})
	}
	require.Nil(t, storage.reader, "invalid uploads never reach S3")
	for _, kind := range []MediaUploadKind{MediaUploadVideos, MediaUploadAudios} {
		svc, storage = newMediaUploadFixture(t, kind, 100)
		data := mediaUploadPNG(t)
		_, err := svc.Upload(ctx, kind, 1, "fake.mp4", bytes.NewReader(data), int64(len(data)))
		require.ErrorIs(t, err, ErrMediaUploadInvalidFile)
		require.Nil(t, storage.reader)
	}
}

func TestMediaUploadEnforcesConfiguredSizeAndUnavailableStorage(t *testing.T) {
	ctx := context.Background()
	data := mediaUploadPNG(t)
	svc, storage := newMediaUploadFixture(t, MediaUploadImages, int64(len(data)-1))
	_, err := svc.Upload(ctx, MediaUploadImages, 1, "photo.png", bytes.NewReader(data), int64(len(data)))
	require.ErrorIs(t, err, ErrMediaUploadTooLarge)
	require.Nil(t, storage.reader)
	// A caller cannot bypass the configured cap by understating its size argument.
	_, err = svc.Upload(ctx, MediaUploadImages, 1, "photo.png", bytes.NewReader(data), 1)
	require.ErrorIs(t, err, ErrMediaUploadTooLarge)
	svc, storage = newMediaUploadFixture(t, MediaUploadImages, int64(len(data)))
	_, err = svc.Upload(ctx, MediaUploadImages, 1, "photo.png", bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	storage.saveError = errors.New("S3 outage")
	_, err = svc.Upload(ctx, MediaUploadImages, 1, "photo.png", bytes.NewReader(data), int64(len(data)))
	require.ErrorIs(t, err, storage.saveError)
	svc = NewMediaUploadService(nil)
	_, err = svc.Upload(ctx, MediaUploadImages, 1, "photo.png", bytes.NewReader(data), int64(len(data)))
	require.ErrorIs(t, err, ErrMediaUploadUnavailable)
}

func TestMediaUploadSupportedFormats(t *testing.T) {
	// Byte headers use each format's actual magic; file names intentionally have no extension.
	for _, tc := range []struct {
		kind MediaUploadKind
		data []byte
		mime string
	}{
		{MediaUploadImages, []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00"), "image/gif"},
		{MediaUploadImages, []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00"), "image/jpeg"},
		{MediaUploadImages, []byte("RIFF\x18\x00\x00\x00WEBPVP8 \x0c\x00\x00\x00\x00\x00\x00\x9d\x01\x2a\x01\x00\x01\x00"), "image/webp"},
		{MediaUploadVideos, []byte("\x00\x00\x00\x18ftypisom\x00\x00\x00\x00isommp41"), "video/mp4"},
		{MediaUploadVideos, []byte("\x00\x00\x00\x18ftypqt  \x00\x00\x00\x00qt  \x00\x00\x00\x00"), "video/quicktime"},
		{MediaUploadVideos, []byte("\x1a\x45\xdf\xa3\x9f\x42\x86\x81\x01\x42\xf7\x81\x01\x42\xf2\x81\x04\x42\xf3\x81\x08\x42\x82\x84webm\x42\x87\x81\x04\x42\x85\x81\x02"), "video/webm"},
		{MediaUploadAudios, []byte("ID3\x03\x00\x00\x00\x00\x00\x00\xff\xfb\x90\x64"), "audio/mpeg"},
		{MediaUploadAudios, []byte("RIFF\x24\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00\x44\xac\x00\x00\x88\x58\x01\x00\x02\x00\x10\x00data\x00\x00\x00\x00"), "audio/wav"},
		{MediaUploadAudios, []byte("\x00\x00\x00\x18ftypM4A \x00\x00\x00\x00M4A isom"), "audio/mp4"},
	} {
		t.Run(tc.mime, func(t *testing.T) {
			svc, _ := newMediaUploadFixture(t, tc.kind, 0)
			got, err := svc.Upload(context.Background(), tc.kind, 1, "no-extension", bytes.NewReader(tc.data), int64(len(tc.data)))
			require.NoError(t, err)
			require.Equal(t, tc.mime, got.MimeType)
		})
	}
}
