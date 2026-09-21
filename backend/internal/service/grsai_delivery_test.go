package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseGrsaiDeliveryRequestRebuildsIndependentStreamBody(t *testing.T) {
	raw := []byte(`{"model":"nano-banana-2-lite","replyType":"async","size":{"width":1024},"extra":{"x":[1,true]}}`)
	before := append([]byte(nil), raw...)
	got, err := ParseGrsaiDeliveryRequest(raw)
	require.NoError(t, err)
	require.Equal(t, GrsaiDeliveryAsync, got.Mode)
	require.Equal(t, before, raw)
	var upstream map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(got.UpstreamBody, &upstream))
	require.JSONEq(t, `"nano-banana-2-lite"`, string(upstream["model"]))
	require.JSONEq(t, `{"width":1024}`, string(upstream["size"]))
	require.JSONEq(t, `{"x":[1,true]}`, string(upstream["extra"]))
	require.JSONEq(t, `"stream"`, string(upstream["replyType"]))
}

func TestParseGrsaiDeliveryRequestRejectsLegacyFlagsAndBadReplyType(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte(`{"model":"nano-banana-2-lite","stream":true}`),
		[]byte(`{"model":"nano-banana-2-lite","async":true}`),
		[]byte(`{"model":"nano-banana-2-lite","replyType":true}`),
		[]byte(`{"model":"nano-banana-2-lite","replyType":"provider_async"}`),
	} {
		_, err := ParseGrsaiDeliveryRequest(raw)
		require.Error(t, err)
	}
}

func TestParseGrsaiDeliveryRequestAcceptsEveryDeliveryMode(t *testing.T) {
	tests := []struct {
		name string
		body string
		want GrsaiDeliveryMode
	}{
		{name: "default json", body: `{"model":"nano-banana-2-lite"}`, want: GrsaiDeliveryJSON},
		{name: "explicit json", body: `{"model":"nano-banana-2-lite","replyType":"json"}`, want: GrsaiDeliveryJSON},
		{name: "stream", body: `{"model":"nano-banana-2-lite","replyType":"stream"}`, want: GrsaiDeliveryStream},
		{name: "async", body: `{"model":"nano-banana-2-lite","replyType":"async"}`, want: GrsaiDeliveryAsync},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseGrsaiDeliveryRequest([]byte(tt.body))
			require.NoError(t, err)
			require.Equal(t, tt.want, got.Mode)
			var upstream map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(got.UpstreamBody, &upstream))
			require.JSONEq(t, `"stream"`, string(upstream["replyType"]))
		})
	}
}
