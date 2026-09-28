package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type GrsaiDeliveryMode string

const (
	GrsaiDeliveryJSON   GrsaiDeliveryMode = "json"
	GrsaiDeliveryStream GrsaiDeliveryMode = "stream"
	GrsaiDeliveryAsync  GrsaiDeliveryMode = "async"
)

type GrsaiDeliveryRequest struct {
	Mode             GrsaiDeliveryMode
	OriginalBody     []byte
	UpstreamBody     []byte
	Model            string
	ImageCount       int
	ImageSize        string
	DurationSeconds  int
	Resolution       string
	VideoFieldsError error
}

func ParseGrsaiDeliveryRequest(body []byte) (*GrsaiDeliveryRequest, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, fmt.Errorf("%w: body must be a JSON object", ErrGrsaiInvalidRequest)
	}
	fields := make(map[string]json.RawMessage)
	var videoFieldsError error
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("%w: body must be valid JSON", ErrGrsaiInvalidRequest)
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, fmt.Errorf("%w: body must be a JSON object", ErrGrsaiInvalidRequest)
		}
		if key == "replyType" || key == "stream" || key == "async" {
			if _, duplicate := fields[key]; duplicate {
				return nil, fmt.Errorf("%w: duplicate control field", ErrGrsaiInvalidRequest)
			}
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("%w: body must be valid JSON", ErrGrsaiInvalidRequest)
		}
		if key == "duration" || key == "resolution" {
			if _, duplicate := fields[key]; duplicate {
				videoFieldsError = fmt.Errorf("%w: duplicate video billing field", ErrGrsaiInvalidRequest)
			}
		}
		fields[key] = value
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return nil, fmt.Errorf("%w: body must be a JSON object", ErrGrsaiInvalidRequest)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("%w: body must contain one JSON object", ErrGrsaiInvalidRequest)
	}
	if _, exists := fields["stream"]; exists {
		return nil, fmt.Errorf("%w: stream control is not supported", ErrGrsaiInvalidRequest)
	}
	if _, exists := fields["async"]; exists {
		return nil, fmt.Errorf("%w: async control is not supported", ErrGrsaiInvalidRequest)
	}
	mode := GrsaiDeliveryJSON
	if raw, exists := fields["replyType"]; exists {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("%w: replyType must be json, stream, or async", ErrGrsaiInvalidRequest)
		}
		mode = GrsaiDeliveryMode(value)
		if mode != GrsaiDeliveryJSON && mode != GrsaiDeliveryStream && mode != GrsaiDeliveryAsync {
			return nil, fmt.Errorf("%w: replyType must be json, stream, or async", ErrGrsaiInvalidRequest)
		}
	}
	var model string
	_ = json.Unmarshal(fields["model"], &model)
	// GRS.AI /v1/api/generate does not expose an image quantity parameter.
	// Each request is one upstream generation task.
	imageCount := 1
	var imageSize string
	_ = json.Unmarshal(fields["imageSize"], &imageSize)
	duration := 0
	if raw, exists := fields["duration"]; exists {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			videoFieldsError = fmt.Errorf("%w: duration must be an integer", ErrGrsaiInvalidRequest)
		} else if err := json.Unmarshal(raw, &duration); err != nil || duration <= 0 {
			videoFieldsError = fmt.Errorf("%w: duration must be a positive integer", ErrGrsaiInvalidRequest)
		}
	}
	resolution := ""
	if raw, exists := fields["resolution"]; exists {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			videoFieldsError = fmt.Errorf("%w: resolution must be a string", ErrGrsaiInvalidRequest)
		} else if err := json.Unmarshal(raw, &resolution); err != nil || strings.TrimSpace(resolution) == "" {
			videoFieldsError = fmt.Errorf("%w: resolution must be a non-empty string", ErrGrsaiInvalidRequest)
		}
	}
	for _, key := range []string{"n", "numImages", "num_images", "imageCount", "image_count", "requested_image_count"} {
		delete(fields, key)
	}
	fields["replyType"] = json.RawMessage(`"async"`)
	upstreamBody, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("%w: body must be valid JSON", ErrGrsaiInvalidRequest)
	}
	return &GrsaiDeliveryRequest{
		Mode:             mode,
		OriginalBody:     bytes.Clone(body),
		UpstreamBody:     upstreamBody,
		Model:            strings.TrimSpace(model),
		ImageCount:       imageCount,
		ImageSize:        NormalizeImageBillingTierOrDefault(imageSize),
		DurationSeconds:  duration,
		Resolution:       resolution,
		VideoFieldsError: videoFieldsError,
	}, nil
}
