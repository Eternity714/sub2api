package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// GrsaiDeliveryMode is the requested downstream delivery contract. GRS.AI
// upstream generation is always streamed regardless of this value.
type GrsaiDeliveryMode string

const (
	GrsaiDeliveryJSON   GrsaiDeliveryMode = "json"
	GrsaiDeliveryStream GrsaiDeliveryMode = "stream"
	GrsaiDeliveryAsync  GrsaiDeliveryMode = "async"
)

var (
	ErrGrsaiInvalidReplyType   = fmt.Errorf("%w: invalid reply type", ErrGrsaiInvalidRequest)
	ErrGrsaiLegacyDeliveryFlag = fmt.Errorf("%w: legacy delivery flag is not supported", ErrGrsaiInvalidRequest)
)

// GrsaiDeliveryRequest keeps the downstream request snapshot and the separate,
// provider-safe upstream representation. Model and image fields are snapshots
// for later accounting; they do not affect upstream serialization.
type GrsaiDeliveryRequest struct {
	Mode         GrsaiDeliveryMode
	OriginalBody []byte
	UpstreamBody []byte
	Model        string
	ImageCount   int
	ImageSize    string
}

// ParseGrsaiDeliveryRequest accepts the fixed downstream delivery protocol and
// rebuilds a fresh upstream payload with replyType forced to stream.
func ParseGrsaiDeliveryRequest(raw []byte) (*GrsaiDeliveryRequest, error) {
	original := append([]byte(nil), raw...)
	fields, err := decodeGrsaiDeliveryObject(original)
	if err != nil {
		return nil, err
	}
	if _, exists := fields["stream"]; exists {
		return nil, fmt.Errorf("%w: stream", ErrGrsaiLegacyDeliveryFlag)
	}
	if _, exists := fields["async"]; exists {
		return nil, fmt.Errorf("%w: async", ErrGrsaiLegacyDeliveryFlag)
	}

	mode, err := parseGrsaiDeliveryMode(fields["replyType"])
	if err != nil {
		return nil, err
	}

	upstreamFields := make(map[string]json.RawMessage, len(fields)+1)
	for key, value := range fields {
		if key == "replyType" {
			continue
		}
		upstreamFields[key] = append(json.RawMessage(nil), value...)
	}
	upstreamFields["replyType"] = json.RawMessage(`"stream"`)
	upstream, err := json.Marshal(upstreamFields)
	if err != nil {
		return nil, fmt.Errorf("%w: could not rebuild upstream request", ErrGrsaiInvalidRequest)
	}

	model, imageCount, imageSize := grsaiDeliveryBillingFields(fields)
	return &GrsaiDeliveryRequest{
		Mode:         mode,
		OriginalBody: original,
		UpstreamBody: upstream,
		Model:        model,
		ImageCount:   imageCount,
		ImageSize:    imageSize,
	}, nil
}

func decodeGrsaiDeliveryObject(body []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	var fields map[string]json.RawMessage
	if err := decoder.Decode(&fields); err != nil || fields == nil {
		return nil, fmt.Errorf("%w: body must be one non-null JSON object", ErrGrsaiInvalidRequest)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("%w: body must contain one JSON object", ErrGrsaiInvalidRequest)
	}
	return fields, nil
}

func parseGrsaiDeliveryMode(raw json.RawMessage) (GrsaiDeliveryMode, error) {
	if len(raw) == 0 {
		return GrsaiDeliveryJSON, nil
	}
	var replyType string
	if err := json.Unmarshal(raw, &replyType); err != nil {
		return "", fmt.Errorf("%w: replyType must be a string", ErrGrsaiInvalidReplyType)
	}
	switch GrsaiDeliveryMode(replyType) {
	case GrsaiDeliveryJSON, GrsaiDeliveryStream, GrsaiDeliveryAsync:
		return GrsaiDeliveryMode(replyType), nil
	default:
		return "", fmt.Errorf("%w: %q", ErrGrsaiInvalidReplyType, replyType)
	}
}

func grsaiDeliveryBillingFields(fields map[string]json.RawMessage) (string, int, string) {
	var model string
	_ = json.Unmarshal(fields["model"], &model)
	model = strings.TrimSpace(model)

	imageCount := 1
	for _, key := range []string{"n", "numImages", "num_images", "imageCount", "image_count"} {
		var count int
		if raw, exists := fields[key]; exists && json.Unmarshal(raw, &count) == nil && count > 0 {
			imageCount = count
			break
		}
	}
	var imageSize string
	_ = json.Unmarshal(fields["imageSize"], &imageSize)
	return model, imageCount, NormalizeImageBillingTierOrDefault(imageSize)
}
