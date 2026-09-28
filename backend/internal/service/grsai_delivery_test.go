package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseGrsaiDeliveryRequestBuildsIndependentAsyncBody(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		mode GrsaiDeliveryMode
	}{
		{"default", `{"model":"gpt-image-2","prompt":"private","numImages":2,"imageSize":"1K","future":{"n":9007199254740993}}`, GrsaiDeliveryJSON},
		{"json", `{"model":"gpt-image-2","replyType":"json","future":{"n":9007199254740993}}`, GrsaiDeliveryJSON},
		{"stream", `{"model":"gpt-image-2","replyType":"stream","future":{"n":9007199254740993}}`, GrsaiDeliveryStream},
		{"async", `{"model":"gpt-image-2","replyType":"async","future":{"n":9007199254740993}}`, GrsaiDeliveryAsync},
	} {
		t.Run(tt.name, func(t *testing.T) {
			original := []byte(tt.body)
			snapshot := bytes.Clone(original)
			request, err := ParseGrsaiDeliveryRequest(original)
			require.NoError(t, err)
			require.Equal(t, tt.mode, request.Mode)
			require.Equal(t, snapshot, request.OriginalBody)
			require.Equal(t, snapshot, original)
			require.Equal(t, "gpt-image-2", request.Model)
			var upstream map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(request.UpstreamBody, &upstream))
			require.JSONEq(t, `"async"`, string(upstream["replyType"]))
			require.JSONEq(t, `{"n":9007199254740993}`, string(upstream["future"]))
			original[0] = 'x'
			require.Equal(t, snapshot, request.OriginalBody)
		})
	}
}

func TestParseGrsaiDeliveryRequestRejectsControlConflicts(t *testing.T) {
	for _, body := range []string{
		`[]`, `null`, `{"replyType":"STREAM"}`, `{"replyType":false}`,
		`{"stream":true}`, `{"async":false}`, `{"stream":null}`,
		`{"replyType":"json","replyType":"async"}`, `{"replyType":"json"} {}`,
	} {
		t.Run(body, func(t *testing.T) {
			_, err := ParseGrsaiDeliveryRequest([]byte(body))
			require.ErrorIs(t, err, ErrGrsaiInvalidRequest)
		})
	}
	_, err := ParseGrsaiDeliveryRequest([]byte(`{"prompt":"private-secret","replyType":"bad"}`))
	require.True(t, errors.Is(err, ErrGrsaiInvalidRequest))
	require.NotContains(t, err.Error(), "private-secret")
}

func TestParseGrsaiDeliveryRequestSnapshotsBillingInputs(t *testing.T) {
	request, err := ParseGrsaiDeliveryRequest([]byte(`{"model":"  gpt-image-2  ","numImages":2,"imageSize":"1K"}`))
	require.NoError(t, err)
	require.Equal(t, "gpt-image-2", request.Model)
	require.Equal(t, 1, request.ImageCount)
	require.Equal(t, ImageBillingSize1K, request.ImageSize)

	defaults, err := ParseGrsaiDeliveryRequest([]byte(`{"model":"gpt-image-2","numImages":0}`))
	require.NoError(t, err)
	require.Equal(t, 1, defaults.ImageCount)
	require.Equal(t, ImageBillingSize2K, defaults.ImageSize)
}

func TestParseGrsaiVideoRequestStrictBillingAndPassthrough(t *testing.T) {
	req, err := ParseGrsaiDeliveryRequest([]byte(`{"model":"minimax-h3","duration":5,"resolution":"768p","images":["https://example.com/ref.png"],"audios":[{"url":"x"}],"seed":9007199254740993,"future":{"nested":true},"replyType":"stream"}`))
	require.NoError(t, err)
	require.Equal(t, 5, req.DurationSeconds)
	require.Equal(t, "768p", req.Resolution)
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(req.UpstreamBody, &body))
	require.JSONEq(t, `9007199254740993`, string(body["seed"]))
	require.JSONEq(t, `{"nested":true}`, string(body["future"]))
	require.JSONEq(t, `[{"url":"x"}]`, string(body["audios"]))
	require.JSONEq(t, `"async"`, string(body["replyType"]))
	for _, raw := range []string{`5.0`, `"5"`, `-1`, `0`, `9223372036854775808`, `null`, `true`} {
		req, err := ParseGrsaiDeliveryRequest([]byte(`{"model":"minimax-h3","duration":` + raw + `}`))
		require.NoError(t, err)
		require.ErrorIs(t, req.VideoFieldsError, ErrGrsaiInvalidRequest)
	}
	req, err = ParseGrsaiDeliveryRequest([]byte(`{"model":"minimax-h3","resolution":1}`))
	require.NoError(t, err)
	require.ErrorIs(t, req.VideoFieldsError, ErrGrsaiInvalidRequest)
}
