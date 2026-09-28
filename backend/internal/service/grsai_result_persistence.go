package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type GrsaiStoredResult struct {
	ResultJSON     []byte
	ResultCount    int
	ObjectMetadata []byte
	LinkExpiresAt  *time.Time
}

type grsaiStoredImage struct {
	URL string `json:"url"`
}

type grsaiStoredObject struct {
	ObjectKey   string `json:"object_key"`
	ContentType string `json:"content_type"`
}

func (u *ImageResultUploader) PersistGrsaiImages(ctx context.Context, localTaskID string, upstream *GrsaiUpstreamResult) (*GrsaiStoredResult, error) {
	if u == nil || u.storage == nil || strings.TrimSpace(localTaskID) == "" || upstream == nil ||
		upstream.Status != GrsaiUpstreamStatusSucceeded || len(upstream.ImageURLs) == 0 {
		return nil, errors.New("grsai result is not ready for image persistence")
	}
	storage, ok := u.storage.(ImageStorageWithMetadata)
	if !ok {
		return nil, errors.New("image storage does not provide link metadata")
	}
	images := make([]grsaiStoredImage, 0, len(upstream.ImageURLs))
	objects := make([]grsaiStoredObject, 0, len(upstream.ImageURLs))
	var earliest *time.Time
	for index, imageURL := range upstream.ImageURLs {
		if strings.TrimSpace(imageURL) == "" {
			return nil, fmt.Errorf("grsai image %d is missing", index)
		}
		urlJSON, err := json.Marshal(imageURL)
		if err != nil {
			return nil, fmt.Errorf("grsai image %d URL is invalid", index)
		}
		data, contentType, err := u.fetchImageBytes(ctx, map[string]json.RawMessage{"url": urlJSON})
		if err != nil {
			return nil, fmt.Errorf("grsai image %d download failed: %w", index, err)
		}
		key := u.buildKey(localTaskID, index, contentType)
		metadata, err := storage.SaveWithMetadata(ctx, key, contentType, data)
		if err != nil {
			return nil, fmt.Errorf("grsai image %d upload failed: %w", index, err)
		}
		if metadata.URL == "" || metadata.ObjectKey != key || metadata.ContentType == "" {
			return nil, fmt.Errorf("grsai image %d storage metadata is incomplete", index)
		}
		if metadata.LinkExpiresAt != nil && (earliest == nil || metadata.LinkExpiresAt.Before(*earliest)) {
			value := *metadata.LinkExpiresAt
			earliest = &value
		}
		images = append(images, grsaiStoredImage{URL: metadata.URL})
		objects = append(objects, grsaiStoredObject{ObjectKey: key, ContentType: metadata.ContentType})
	}
	resultJSON, err := json.Marshal(struct {
		Results []grsaiStoredImage `json:"results"`
	}{Results: images})
	if err != nil {
		return nil, err
	}
	objectJSON, err := json.Marshal(objects)
	if err != nil {
		return nil, err
	}
	return &GrsaiStoredResult{ResultJSON: resultJSON, ResultCount: len(images), ObjectMetadata: objectJSON, LinkExpiresAt: earliest}, nil
}
