package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
)

// UnifiedMediaAdapterProfile describes protocol differences using trusted, embedded
// provider documentation. User metadata never selects an endpoint or transport.
type UnifiedMediaAdapterProfile struct {
	TopLevelMetadata    []string          `json:"top_level_metadata,omitempty"`
	FieldMap            map[string]string `json:"field_map,omitempty"`
	ImageURLObjects     bool              `json:"image_url_objects,omitempty"`
	PixelSize           bool              `json:"pixel_size,omitempty"`
	StringDuration      bool              `json:"string_duration,omitempty"`
	ResolutionLowercase bool              `json:"resolution_lowercase,omitempty"`
	SubmitPath          string            `json:"submit_path,omitempty"`
	QueryPath           string            `json:"query_path,omitempty"`
	FixedFields         map[string]any    `json:"fixed_fields,omitempty"`
}

type unifiedMediaHTTPUpstream struct {
	client    *http.Client
	transport HTTPUpstream
}

func NewUnifiedMediaUpstream(client *http.Client) UnifiedMediaUpstream {
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	}
	return &unifiedMediaHTTPUpstream{client: client}
}

// BuildUnifiedMediaPayload operates on a copy; captured input and prices remain
// immutable across retries and provider failover.
func BuildUnifiedMediaPayload(platform, mediaType, model string, request *mediacapability.NormalizedRequest, count int) ([]byte, string, error) {
	if request == nil || request.Values == nil || model == "" || (mediaType != "image" && mediaType != "video") || count < 1 {
		return nil, "", errors.New("invalid normalized media request")
	}
	raw, err := json.Marshal(request.Values)
	if err != nil {
		return nil, "", err
	}
	values := map[string]any{}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err = d.Decode(&values); err != nil {
		return nil, "", err
	}
	values["model"] = model
	metadata, _ := values["metadata"].(map[string]any)
	if metadata == nil {
		metadata = map[string]any{}
	}
	if platform == PlatformGrsai {
		out := map[string]any{"model": model, "replyType": "async"}
		for _, field := range []string{"prompt", "duration"} {
			if value, ok := values[field]; ok {
				out[field] = value
			}
		}
		if images, ok := values["image_urls"]; ok {
			out["images"] = images
		}
		if mediaType == "image" {
			if size, ok := values["size"]; ok {
				out["aspectRatio"] = size
			}
			if strings.HasPrefix(model, "nano-") {
				if value, ok := metadata["resolution"]; ok {
					out["imageSize"] = value
				}
			}
			if grsaiRequiresPixelSize(model) {
				if request.ImageSpec == nil {
					return nil, "", errors.New("model requires a configured aspect ratio and resolution pixel mapping")
				}
				out["aspectRatio"] = fmt.Sprintf("%dx%d", request.ImageSpec.Width, request.ImageSpec.Height)
			}
		} else {
			if ratio, ok := values["aspect_ratio"]; ok {
				if model == "minimax-h3" {
					switch ratio {
					case "16:9":
						ratio = "landscape"
					case "9:16":
						ratio = "portrait"
					default:
						return nil, "", errors.New("minimax-h3 supports only 16:9 and 9:16")
					}
				}
				out["aspectRatio"] = ratio
			}
			if value, ok := metadata["resolution"]; ok {
				out["resolution"] = value
			}
		}
		for key, value := range metadata {
			if key == "resolution" {
				continue
			}
			if key == "mask_url" {
				key = "mask"
			}
			if key == "audio_urls" {
				key = "audios"
			}
			switch key {
			case "model", "prompt", "images", "aspectRatio", "imageSize", "duration", "replyType", "n", "stream", "async":
				return nil, "", errors.New("metadata conflicts with media protocol fields")
			}
			out[key] = value
		}
		encoded, err := json.Marshal(out)
		return encoded, grsaiGeneratePath, err
	}
	if platform != "toapis" {
		return nil, "", errors.New("unsupported unified media provider")
	}
	profile := mediaAdapterProfile(platform, mediaType, model)
	out := map[string]any{"model": model}
	fields := []string{"prompt", "image_urls"}
	if mediaType == "image" {
		fields = append(fields, "size")
		out["n"] = count
	} else {
		fields = append(fields, "duration", "aspect_ratio")
	}
	for _, field := range fields {
		if value, ok := values[field]; ok {
			out[field] = value
		}
	}
	if len(metadata) > 0 {
		out["metadata"] = metadata
	}
	for _, field := range profile.TopLevelMetadata {
		if value, ok := metadata[field]; ok {
			if _, exists := out[field]; exists {
				return nil, "", errors.New("metadata conflicts with media protocol fields")
			}
			out[field] = value
			delete(metadata, field)
		}
	}
	for from, to := range profile.FieldMap {
		if value, exists := unifiedMediaTakePath(out, from); exists {
			if !unifiedMediaSetPath(out, to, value) {
				return nil, "", errors.New("media field mapping conflict")
			}
		}
	}
	if profile.ImageURLObjects {
		if images, ok := out["image_urls"].([]any); ok {
			mapped := make([]any, 0, len(images))
			for _, image := range images {
				mapped = append(mapped, map[string]any{"url": image})
			}
			out["image_urls"] = mapped
		}
	}
	if profile.PixelSize {
		if request.ImageSpec == nil {
			return nil, "", errors.New("model requires a configured aspect ratio and resolution pixel mapping")
		}
		out["size"] = fmt.Sprintf("%dx%d", request.ImageSpec.Width, request.ImageSpec.Height)
	}
	if profile.StringDuration {
		for _, key := range []string{"duration", "seconds"} {
			if value, ok := out[key]; ok {
				out[key] = fmt.Sprint(value)
			}
		}
	}
	if profile.ResolutionLowercase {
		if value, ok := out["resolution"].(string); ok {
			out["resolution"] = strings.ToLower(value)
		}
		if value, ok := metadata["resolution"].(string); ok {
			metadata["resolution"] = strings.ToLower(value)
		}
	}
	for key, value := range profile.FixedFields {
		if existing, ok := out[key]; ok {
			a, _ := json.Marshal(existing)
			b, _ := json.Marshal(value)
			if !bytes.Equal(a, b) {
				return nil, "", errors.New("request conflicts with fixed upstream field")
			}
		}
		out[key] = value
	}
	if len(metadata) == 0 {
		delete(out, "metadata")
	}
	endpoint := "/v1/" + mediaType + "s/generations"
	if profile.SubmitPath != "" {
		endpoint = profile.SubmitPath
	}
	if strings.EqualFold(model, "midjourney") {
		endpoint = "/v1/midjourney/generations/imagine"
		delete(out, "n")
		if mediaType == "video" {
			endpoint = "/v1/midjourney/generations/video"
		}
		if meta, ok := out["metadata"].(map[string]any); ok {
			for key, value := range meta {
				if _, exists := out[key]; exists {
					return nil, "", errors.New("midjourney metadata conflicts with protocol fields")
				}
				out[key] = value
			}
			delete(out, "metadata")
		}
	}
	encoded, err := json.Marshal(out)
	return encoded, endpoint, err
}

func grsaiRequiresPixelSize(model string) bool {
	return model == "gpt-image-2-vip" || strings.HasPrefix(model, "gpt-image-2.5-flare") || strings.HasPrefix(model, "gpt-image-2.5-sunburst")
}
func unifiedMediaTakePath(root map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	current := root
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			return nil, false
		}
		current = next
	}
	v, ok := current[parts[len(parts)-1]]
	delete(current, parts[len(parts)-1])
	return v, ok
}
func unifiedMediaSetPath(root map[string]any, path string, value any) bool {
	parts := strings.Split(path, ".")
	current := root
	for _, part := range parts[:len(parts)-1] {
		if current[part] == nil {
			current[part] = map[string]any{}
		}
		next, ok := current[part].(map[string]any)
		if !ok {
			return false
		}
		current = next
	}
	key := parts[len(parts)-1]
	if _, exists := current[key]; exists {
		return false
	}
	current[key] = value
	return true
}

func (u *unifiedMediaHTTPUpstream) Submit(ctx context.Context, account *Account, mediaType, model string, normalized *mediacapability.NormalizedRequest, count int) (*UnifiedMediaUpstreamResult, error) {
	if account == nil {
		return &UnifiedMediaUpstreamResult{Disposition: "rejected"}, errors.New("media account unavailable")
	}
	payload, endpoint, err := BuildUnifiedMediaPayload(account.Platform, mediaType, model, normalized, count)
	if err != nil {
		return &UnifiedMediaUpstreamResult{Disposition: "rejected", Error: err.Error()}, err
	}
	return u.do(ctx, account, http.MethodPost, endpoint, payload, mediaType, model)
}
func (u *unifiedMediaHTTPUpstream) Poll(ctx context.Context, account *Account, mediaType, model, taskID string) (*UnifiedMediaUpstreamResult, error) {
	if account == nil || strings.TrimSpace(taskID) == "" || len(taskID) > 512 || strings.ContainsAny(taskID, "/?#\\%") {
		return nil, errors.New("media task is unavailable")
	}
	endpoint := "/v1/" + mediaType + "s/generations/" + url.PathEscape(taskID)
	if account.Platform == PlatformGrsai {
		endpoint = grsaiResultPath + "?id=" + url.QueryEscape(taskID)
	} else {
		profile := mediaAdapterProfile(account.Platform, mediaType, model)
		if profile.QueryPath != "" {
			endpoint = strings.ReplaceAll(profile.QueryPath, "{id}", url.PathEscape(taskID))
		}
		if model == "midjourney" {
			endpoint = "/v1/midjourney/" + url.PathEscape(taskID)
		}
	}
	return u.do(ctx, account, http.MethodGet, endpoint, nil, mediaType, model)
}
func (u *unifiedMediaHTTPUpstream) do(ctx context.Context, account *Account, method, endpoint string, payload []byte, mediaType, model string) (*UnifiedMediaUpstreamResult, error) {
	result := &UnifiedMediaUpstreamResult{Disposition: "unknown", Status: "in_progress"}
	if account.Type != AccountTypeAPIKey || (account.Platform != PlatformGrsai && account.Platform != "toapis") {
		result.Disposition = "rejected"
		return result, errors.New("invalid media account")
	}
	base := account.GetCredential("base_url")
	key := account.GetCredential("api_key")
	if key == "" {
		result.Disposition = "rejected"
		return result, errors.New("media API key is not configured")
	}
	parsedEndpoint, err := url.Parse(endpoint)
	if err != nil {
		return result, errors.New("invalid media endpoint")
	}
	target, err := buildGrsaiEndpointURL(base, parsedEndpoint.Path)
	if err != nil {
		result.Disposition = "rejected"
		return result, err
	}
	if parsedEndpoint.RawQuery != "" {
		target += "?" + parsedEndpoint.RawQuery
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(payload))
	if err != nil {
		result.Disposition = "rejected"
		return result, errors.New("media request setup failed")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// Never follow redirects with provider credentials or automatically repeat a POST.
	client := *u.client
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	var resp *http.Response
	if u.transport != nil {
		req = req.WithContext(WithHTTPUpstreamRedirectsDisabled(req.Context()))
		proxyURL := ""
		if account.ProxyID != nil && account.Proxy != nil {
			proxyURL = account.Proxy.URL()
		}
		resp, err = u.transport.Do(req, proxyURL, account.ID, account.Concurrency)
	} else {
		resp, err = client.Do(req)
	}
	if err != nil {
		return result, errors.New("upstream media connection failed; submission acceptance is unknown")
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, grsaiMaxResponseBodyLen+1))
	if err != nil || len(raw) > grsaiMaxResponseBodyLen {
		return result, errors.New("invalid upstream media response")
	}
	responseValid := false
	if account.Platform == PlatformGrsai {
		grs, parseErr := parseGrsaiUpstreamResult(resp.StatusCode, raw, key)
		if grs != nil {
			responseValid = grs.Status != ""
			result.TaskID = grs.TaskID
			result.URLs = grs.ImageURLs
			if grs.Status == GrsaiUpstreamStatusSucceeded {
				result.Status = "completed"
			}
			if grs.Status == GrsaiUpstreamStatusFailed || grs.Status == GrsaiUpstreamStatusViolation {
				result.Status = "completed"
				result.Error = "Upstream generation failed"
			}
		}
		if parseErr != nil && resp.StatusCode < 400 {
			return result, errors.New("invalid upstream media response")
		}
	} else {
		var root map[string]json.RawMessage
		if json.Unmarshal(raw, &root) == nil && root != nil {
			data := rawJSONObject(root["data"])
			result.TaskID = firstRawScalar(root, data, "id", "task_id", "taskId")
			state := strings.ToLower(firstRawScalar(root, data, "status", "state"))
			switch state {
			case "pending", "queued", "in_progress", "running", "processing", "completed", "succeeded", "success", "failed", "failure", "cancelled", "canceled":
				responseValid = true
			}
			if state == "completed" || state == "succeeded" || state == "success" {
				result.Status = "completed"
			}
			if state == "failed" || state == "failure" || state == "cancelled" || state == "canceled" {
				result.Status = "completed"
				result.Error = "Upstream generation failed"
			}
			objects := []map[string]json.RawMessage{rawJSONObject(root["result"]), rawJSONObject(data["result"]), data, root}
			for _, object := range objects {
				if object == nil {
					continue
				}
				if model == "midjourney" {
					if mediaType == "image" {
						if grid := firstRawScalar(object, nil, "grid_image_url"); grid != "" {
							result.URLs = []string{grid}
							break
						}
					} else {
						var videos []string
						if json.Unmarshal(object["video_urls"], &videos) == nil && len(videos) > 0 {
							result.URLs = videos
							break
						}
					}
				} else {
					var rows []struct {
						URL string `json:"url"`
					}
					if json.Unmarshal(object["data"], &rows) == nil {
						for _, row := range rows {
							if row.URL != "" {
								result.URLs = append(result.URLs, row.URL)
							}
						}
						if len(result.URLs) > 0 {
							break
						}
					}
				}
			}
		}
	}
	if method == http.MethodPost && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		// GRS documents a terminal failed task delivered with HTTP 400. A real
		// failed task must not be resubmitted, but no HTTP error can deliver URLs.
		if account.Platform == PlatformGrsai && (resp.StatusCode == 400 || resp.StatusCode == 422) && responseValid && result.TaskID != "" && result.Status == "completed" && result.Error != "" {
			result.Disposition = "accepted"
			result.URLs = nil
			return result, nil
		}
		result.TaskID = ""
		result.URLs = nil
		result.Status = "in_progress"
		if resp.StatusCode == 400 || resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 404 || resp.StatusCode == 422 || resp.StatusCode == 429 {
			result.Disposition = "rejected"
			result.Error = fmt.Sprintf("Upstream rejected generation (HTTP %d)", resp.StatusCode)
			return result, nil
		}
		return result, errors.New("upstream media response did not confirm acceptance")
	}
	if method == http.MethodGet && (resp.StatusCode < 200 || resp.StatusCode >= 300 || !responseValid) {
		return result, errors.New("upstream media status lookup failed")
	}
	if result.TaskID != "" {
		result.Disposition = "accepted"
		return result, nil
	}
	if result.Status == "completed" && len(result.URLs) > 0 {
		result.Disposition = "accepted"
		return result, nil
	}
	if resp.StatusCode == 400 || resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 404 || resp.StatusCode == 422 || resp.StatusCode == 429 {
		result.Disposition = "rejected"
		result.Error = fmt.Sprintf("Upstream rejected generation (HTTP %d)", resp.StatusCode)
		return result, nil
	}
	if method == http.MethodGet && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.Disposition = "accepted"
		return result, nil
	}
	return result, errors.New("upstream media response did not confirm acceptance")
}
