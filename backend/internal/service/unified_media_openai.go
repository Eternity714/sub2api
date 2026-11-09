package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
)

// Compatible image providers can join unified tasks without changing their
// account platform. This adapter deliberately requires synchronous URL results.
func buildUnifiedOpenAIImagePayload(model string, request *mediacapability.NormalizedRequest, values, metadata map[string]any, count int) ([]byte, string, error) {
	if _, exists := values["image_urls"]; exists {
		return nil, "", errors.New("OpenAI unified image editing is not supported")
	}
	out := map[string]any{"model": model, "n": count, "response_format": "url"}
	if prompt, exists := values["prompt"]; exists {
		out["prompt"] = prompt
	}
	if request.ImageSpec != nil {
		out["size"] = fmt.Sprintf("%dx%d", request.ImageSpec.Width, request.ImageSpec.Height)
	} else if size, exists := values["size"]; exists {
		if size != "auto" {
			return nil, "", errors.New("OpenAI unified images require a configured pixel size mapping")
		}
		out["size"] = size
	}
	for field, value := range metadata {
		switch field {
		case "resolution":
			// The validated image spec already captures ratio and resolution.
		case "quality", "background", "output_format", "output_compression":
			out[field] = value
		default:
			return nil, "", errors.New("unsupported OpenAI unified image metadata field")
		}
	}
	body, err := json.Marshal(out)
	return body, "/v1/images/generations", err
}

func parseUnifiedOpenAIImageURLs(raw []byte) []string {
	return parseUnifiedImageResults(raw, false)
}

func isUnifiedInlineImageURL(value string) bool {
	const prefix = "data:image/"
	return len(value) >= len(prefix) && strings.EqualFold(value[:len(prefix)], prefix)
}

// Kuaipao documents inline image fallback when its URL storage is unavailable.
// This opt-in applies only to upstream results, never client reference inputs.
func parseUnifiedImageResults(raw []byte, allowInline bool) []string {
	var response struct {
		Data []struct {
			URL    string `json:"url"`
			Base64 string `json:"b64_json"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &response) != nil || len(response.Data) == 0 {
		return nil
	}
	urls := make([]string, 0, len(response.Data))
	for _, row := range response.Data {
		value := strings.TrimSpace(row.URL)
		if allowInline {
			if value == "" && strings.TrimSpace(row.Base64) != "" {
				value = "data:image/png;base64," + strings.TrimSpace(row.Base64)
			}
			if isUnifiedInlineImageURL(value) {
				urls = append(urls, value)
				continue
			}
		}
		u, err := url.Parse(value)
		if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
			return nil
		}
		urls = append(urls, u.String())
	}
	return urls
}
