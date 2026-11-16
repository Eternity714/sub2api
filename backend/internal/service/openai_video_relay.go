package service

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

// Only explicitly opted-in API-key accounts use the OpenAI video task protocol.
func usesOpenAIVideoProtocol(account *Account) bool {
	return account != nil && account.Platform == PlatformGrok && account.Type == AccountTypeAPIKey && account.GetExtraString("grok_video_protocol") == "openai"
}

func grokMediaUsageForAccount(account *Account, endpoint GrokMediaEndpoint, requestInfo GrokMediaRequestInfo, body []byte) grokMediaUsageMetadata {
	if !usesOpenAIVideoProtocol(account) || endpoint != GrokMediaEndpointVideoStatus {
		return grokMediaUsageFromResponse(endpoint, requestInfo, body)
	}
	meta := grokMediaUsageMetadata{ResponseID: extractGrokMediaVideoRequestID(body)}
	if !gjson.ValidBytes(body) || gjson.GetBytes(body, "status").String() != "completed" {
		return meta
	}
	meta.VideoCount = 1
	meta.Model = strings.TrimSpace(gjson.GetBytes(body, "model").String())
	meta.BillingModel = meta.Model
	// Missing fields stay unset so the handler restores the accepted request snapshot.
	if resolution := gjson.GetBytes(body, "resolution").String(); resolution != "" {
		meta.VideoResolution = NormalizeVideoBillingResolutionOrDefault(resolution)
	}
	if seconds := gjson.GetBytes(body, "seconds"); seconds.Exists() {
		meta.VideoDurationSeconds = int(seconds.Int())
	}
	return meta
}

// Reference-video input has an additional measured-duration charge upstream.
// Until that billing unit is supported, reject it before any paid submission.
func validateOpenAIVideoRelayRequest(contentType string, body []byte) error {
	if gjson.ValidBytes(body) {
		seconds, err := strconv.Atoi(gjson.GetBytes(body, "seconds").String())
		resolution := gjson.GetBytes(body, "resolution").String()
		if err != nil || seconds < 4 || seconds > 30 || (resolution != "480p" && resolution != "720p" && resolution != "1080p") {
			return fmt.Errorf("specify integer seconds (4-30) and resolution (480p, 720p or 1080p)")
		}
		for _, field := range []string{"reference_videos", "video", "video_url", "reference_video_seconds"} {
			if v := gjson.GetBytes(body, field); v.Exists() && v.Type != gjson.Null && v.Raw != "[]" && v.String() != "" {
				return fmt.Errorf("reference-video billing is not yet supported on this channel")
			}
		}
		for _, item := range gjson.GetBytes(body, "content").Array() {
			if strings.Contains(strings.ToLower(item.Get("type").String()), "video") {
				return fmt.Errorf("reference-video billing is not yet supported on this channel")
			}
		}
		if d := gjson.GetBytes(body, "duration"); d.Exists() {
			return fmt.Errorf("use seconds for this video channel")
		}
		return nil
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "multipart/form-data" {
		return fmt.Errorf("expected JSON or multipart video request")
	}
	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	secondsValue, resolutionValue := "", ""
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			seconds, parseErr := strconv.Atoi(secondsValue)
			if parseErr != nil || seconds < 4 || seconds > 30 || (resolutionValue != "480p" && resolutionValue != "720p" && resolutionValue != "1080p") {
				return fmt.Errorf("specify integer seconds (4-30) and resolution (480p, 720p or 1080p)")
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("invalid multipart video request")
		}
		name := part.FormName()
		if name == "seconds" || name == "resolution" {
			value, readErr := io.ReadAll(part)
			if readErr != nil {
				part.Close()
				return fmt.Errorf("invalid multipart video request")
			}
			if name == "seconds" {
				secondsValue = strings.TrimSpace(string(value))
			} else {
				resolutionValue = strings.TrimSpace(string(value))
			}
		}
		if name == "reference_videos" || name == "reference_video_seconds" || name == "duration" {
			part.Close()
			return fmt.Errorf("use seconds; reference-video billing is not yet supported on this channel")
		}
		if name == "content" {
			data, _ := io.ReadAll(part)
			for _, item := range gjson.ParseBytes(data).Array() {
				if strings.Contains(strings.ToLower(item.Get("type").String()), "video") {
					part.Close()
					return fmt.Errorf("reference-video billing is not yet supported on this channel")
				}
			}
		}
		part.Close()
	}
}
