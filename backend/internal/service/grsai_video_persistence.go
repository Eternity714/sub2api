package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

type VideoStorageWithMetadata interface {
	SaveVideoReader(context.Context, string, io.Reader, int64) (ImageStorageSaveMetadata, error)
}
type GrsaiVideoPersister struct {
	downloader GrsaiVideoDownloader
	storage    VideoStorageWithMetadata
}

func NewGrsaiVideoPersister(d GrsaiVideoDownloader, s VideoStorageWithMetadata) *GrsaiVideoPersister {
	return &GrsaiVideoPersister{d, s}
}
func (p *GrsaiVideoPersister) PersistGrsaiVideo(ctx context.Context, id string, r *GrsaiUpstreamResult) (*GrsaiStoredResult, error) {
	if p == nil || p.downloader == nil || p.storage == nil || strings.TrimSpace(id) == "" || r == nil || r.Status != GrsaiUpstreamStatusSucceeded || len(r.ImageURLs) == 0 {
		return nil, errors.New("grsai result is not ready for video persistence")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	results := make([]grsaiStoredImage, 0, len(r.ImageURLs))
	objects := make([]grsaiStoredObject, 0, len(r.ImageURLs))
	var earliest *time.Time
	for i, raw := range r.ImageURLs {
		if strings.TrimSpace(raw) == "" {
			return nil, fmt.Errorf("grsai video %d is missing", i)
		}
		body, size, err := p.downloader.Download(ctx, raw)
		if err != nil {
			return nil, fmt.Errorf("grsai video %d download failed: %w", i, err)
		}
		key := fmt.Sprintf("grsai/%s/video-%d.mp4", id, i)
		meta, err := p.storage.SaveVideoReader(ctx, key, body, size)
		_ = body.Close()
		if err != nil {
			return nil, fmt.Errorf("grsai video %d upload failed: %w", i, err)
		}
		if meta.URL == "" || meta.ObjectKey != key || meta.ContentType != "video/mp4" {
			return nil, fmt.Errorf("grsai video %d storage metadata is incomplete", i)
		}
		if meta.LinkExpiresAt != nil && (earliest == nil || meta.LinkExpiresAt.Before(*earliest)) {
			v := *meta.LinkExpiresAt
			earliest = &v
		}
		results = append(results, grsaiStoredImage{URL: meta.URL})
		objects = append(objects, grsaiStoredObject{ObjectKey: key, ContentType: "video/mp4"})
	}
	out, _ := json.Marshal(struct {
		Results []grsaiStoredImage `json:"results"`
	}{results})
	obj, _ := json.Marshal(objects)
	return &GrsaiStoredResult{ResultJSON: out, ResultCount: len(results), ObjectMetadata: obj, LinkExpiresAt: earliest}, nil
}
