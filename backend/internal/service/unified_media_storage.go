package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"net/url"
	"time"
)

// UnifiedMediaS3Persister reuses the existing protected image/video downloaders
// and S3 metadata. Each output uses a stable key so retries do not create a new
// public result or charge.
type UnifiedMediaS3Persister struct{ resolve ImageStorageResolver }

func NewUnifiedMediaS3Persister(storage *ImageStorageSettingService) *UnifiedMediaS3Persister {
	return &UnifiedMediaS3Persister{resolve: storage.Resolver()}
}
func (p *UnifiedMediaS3Persister) Ready() bool {
	if p == nil || p.resolve == nil {
		return false
	}
	u, enabled := p.resolve()
	return enabled && u != nil
}
func (p *UnifiedMediaS3Persister) Persist(ctx context.Context, taskID, mediaType string, unitIndex, resultIndex int, upstreamURL string) (*UnifiedMediaStoredResult, error) {
	if !p.Ready() {
		return nil, ErrImageTaskUnavailable
	}
	uploader, _ := p.resolve()
	parsedURL, err := url.Parse(upstreamURL)
	if err != nil || parsedURL == nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.User != nil || parsedURL.Hostname() == "" {
		return nil, errors.New("invalid upstream result URL")
	}
	// The shared uploader also serves existing protocols. Give unified media its
	// own public-address-only downloader without changing their transport.
	isolated := *uploader
	isolated.httpClient = &http.Client{Timeout: 60 * time.Second, Transport: safeVideoTransport(), CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("too many media redirects")
		}
		if req.URL.Scheme != "http" && req.URL.Scheme != "https" || req.URL.User != nil {
			return errors.New("invalid media redirect")
		}
		return nil
	}}
	uploader = &isolated
	result := &GrsaiUpstreamResult{Status: GrsaiUpstreamStatusSucceeded, ImageURLs: []string{upstreamURL}}
	id := fmt.Sprintf("%s-unit-%d-output-%d", taskID, unitIndex, resultIndex)
	var stored *GrsaiStoredResult

	if mediaType == "video" {
		storage, ok := uploader.storage.(VideoStorageWithMetadata)
		if !ok {
			return nil, ErrImageTaskUnavailable
		}
		stored, err = NewGrsaiVideoPersister(NewGrsaiVideoDownloader(nil, 0, ""), storage).PersistGrsaiVideo(ctx, id, result)
	} else {
		storage, ok := uploader.storage.(ImageStorageWithMetadata)
		if !ok {
			return nil, ErrImageTaskUnavailable
		}
		uploader.storage = unifiedMediaValidatedImageStorage{storage}
		stored, err = uploader.PersistGrsaiImages(ctx, id, result)
	}
	if err != nil {
		return nil, err
	}
	if stored == nil {
		return nil, ErrImageTaskUnavailable
	}
	var content struct {
		Results []struct {
			URL string `json:"url"`
		} `json:"results"`
	}
	if json.Unmarshal(stored.ResultJSON, &content) != nil || len(content.Results) != 1 || content.Results[0].URL == "" {
		return nil, ErrImageTaskUnavailable
	}
	return &UnifiedMediaStoredResult{URL: content.Results[0].URL, ExpiresAt: stored.LinkExpiresAt, ObjectMetadata: json.RawMessage(stored.ObjectMetadata)}, nil
}

// Validation is local to unified media so legacy gateway output behavior is
// preserved. A CDN error page or empty response is never a billable result.
type unifiedMediaValidatedImageStorage struct{ ImageStorageWithMetadata }

func (s unifiedMediaValidatedImageStorage) Save(ctx context.Context, key, contentType string, data []byte) (string, error) {
	m, err := s.SaveWithMetadata(ctx, key, contentType, data)
	return m.URL, err
}
func (s unifiedMediaValidatedImageStorage) SaveWithMetadata(ctx context.Context, key, _ string, data []byte) (ImageStorageSaveMetadata, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > 100000000 {
		return ImageStorageSaveMetadata{}, errors.New("upstream result is not a valid supported raster image")
	}
	var contentType string
	switch format {
	case "png", "jpeg", "gif", "webp":
		contentType = "image/" + format
	default:
		return ImageStorageSaveMetadata{}, errors.New("unsupported result image format")
	}
	return s.ImageStorageWithMetadata.SaveWithMetadata(ctx, key, contentType, data)
}
