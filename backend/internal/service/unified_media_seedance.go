package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

func validUnifiedVideoTaskID(id string) bool {
	return strings.TrimSpace(id) == id && id != "" && id != "." && id != ".." && len(id) <= 512 && !strings.ContainsAny(id, "/ ?#\\%\r\n\t")
}

func buildUnifiedSeedancePayload(mediaType, model string, values, metadata map[string]any, count int) ([]byte, string, error) {
	if mediaType != "video" || model != "seedance-2.5" || count != 1 {
		return nil, "", errors.New("unsupported native video model or result count")
	}
	for field := range values {
		switch field {
		case "model", "prompt", "duration", "aspect_ratio", "metadata":
		default:
			return nil, "", errors.New("unsupported Seedance input field")
		}
	}
	for field := range metadata {
		if field != "resolution" {
			return nil, "", errors.New("unsupported Seedance metadata field")
		}
	}
	out := map[string]any{"model": model}
	for _, field := range []string{"prompt", "aspect_ratio"} {
		if value, ok := values[field]; ok {
			out[field] = value
		}
	}
	if value, ok := values["duration"]; ok {
		out["seconds"] = value
	}
	if value, ok := metadata["resolution"]; ok {
		out["resolution"] = value
	}
	body, err := json.Marshal(out)
	return body, "/v1/videos", err
}

// Credentials are supplied only to the configured account's content endpoint.
// Redirects are never followed, including redirects to a different public host.
type openAIVideoContentDownloader struct {
	downloader *grsaiVideoDownloader
	key        string
}

func newOpenAIVideoContentDownloader(client *http.Client, key string) *openAIVideoContentDownloader {
	if client == nil {
		client = &http.Client{Timeout: defaultGrsaiVideoTimeout, Transport: safeVideoTransport()}
	}
	clone := *client
	clone.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &openAIVideoContentDownloader{downloader: NewGrsaiVideoDownloader(&clone, 0, "").(*grsaiVideoDownloader), key: key}
}
func (d *openAIVideoContentDownloader) Download(ctx context.Context, raw string) (body io.ReadCloser, size int64, err error) {
	return d.downloader.download(ctx, raw, "Bearer "+d.key)
}
