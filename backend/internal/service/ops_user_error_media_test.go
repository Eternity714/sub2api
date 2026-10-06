package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserMediaErrorProjectionPreservesOpsLog(t *testing.T) {
	src := &OpsErrorLog{Platform: "grsai"}
	require.Equal(t, "media", ToUserErrorRequest(src).Platform)
	detail := &OpsErrorLogDetail{OpsErrorLog: *src}
	require.Equal(t, "media", ToUserErrorRequestDetail(detail).Platform)
	require.Equal(t, "grsai", src.Platform)
	require.Equal(t, "grsai", detail.Platform)
	require.Equal(t, "openai", ToUserErrorRequest(&OpsErrorLog{Platform: "openai"}).Platform)
}

func TestUserMediaHistoricalErrorMessageHidesProviderAndUpstreamURL(t *testing.T) {
	for _, message := range []string{
		`GRS.AI request failed: Post "https://grsai.example/v1/draw?key=hidden": context deadline exceeded`,
		`grsai API 调用失败: https://127.0.0.1:8080/v1/draw returned HTTP 503`,
	} {
		t.Run(message, func(t *testing.T) {
			src := &OpsErrorLog{Platform: "grsai", Message: message, Phase: "upstream", Type: "api_error"}
			out := ToUserErrorRequest(src)
			require.Contains(t, out.Message, "Media API")
			require.NotContains(t, out.Message, "GRS.AI")
			require.NotContains(t, out.Message, "grsai")
			require.NotContains(t, out.Message, "https://")
			require.NotContains(t, out.Message, "127.0.0.1")
			require.NotContains(t, out.Message, "hidden")
			require.Equal(t, "upstream", out.Category)
			require.Equal(t, message, src.Message)
			if message[0] == 'G' {
				require.Contains(t, out.Message, "context deadline exceeded")
			} else {
				require.Contains(t, out.Message, "HTTP 503")
			}
			detail := &OpsErrorLogDetail{OpsErrorLog: *src, ErrorBody: `{"message":"user content"}`}
			require.Equal(t, out.Message, ToUserErrorRequestDetail(detail).Message)
			require.Equal(t, detail.ErrorBody, ToUserErrorRequestDetail(detail).ErrorBody)
		})
	}
	other := &OpsErrorLog{Platform: "openai", Message: "request failed at https://proxy.example/v1/chat"}
	require.Equal(t, other.Message, ToUserErrorRequest(other).Message)
}

func TestUserMediaHistoricalErrorBodyHidesProviderAndPreservesRawDetail(t *testing.T) {
	body := `{"error":{"message":"GRS.AI: quota exhausted at https://grsai.example/v1/draw?key=hidden","type":"billing_error","id":9007199254740993}}`
	src := &OpsErrorLogDetail{OpsErrorLog: OpsErrorLog{Platform: "grsai"}, ErrorBody: body}
	out := ToUserErrorRequestDetail(src)
	require.Contains(t, out.ErrorBody, "Media API")
	require.Contains(t, out.ErrorBody, "quota exhausted")
	require.Contains(t, out.ErrorBody, "billing_error")
	require.Contains(t, out.ErrorBody, "9007199254740993")
	require.NotContains(t, out.ErrorBody, "GRS.AI")
	require.NotContains(t, out.ErrorBody, "grsai")
	require.NotContains(t, out.ErrorBody, "https://")
	require.NotContains(t, out.ErrorBody, "hidden")
	require.True(t, json.Valid([]byte(out.ErrorBody)))
	require.Equal(t, body, src.ErrorBody)
	raw, err := json.Marshal(src)
	require.NoError(t, err)
	require.Contains(t, string(raw), "GRS.AI")
	require.Contains(t, string(raw), "grsai.example")
	src.Platform = "openai"
	require.Equal(t, body, ToUserErrorRequestDetail(src).ErrorBody)
	escaped := &OpsErrorLogDetail{OpsErrorLog: OpsErrorLog{Platform: "grsai"}, ErrorBody: `{"error":{"message":"GRS.AI failed at https:\/\/grsai.example\/draw?key=hidden"}}`}
	escapedOut := ToUserErrorRequestDetail(escaped)
	require.True(t, json.Valid([]byte(escapedOut.ErrorBody)))
	require.Contains(t, escapedOut.ErrorBody, "Media API failed at upstream service")
	require.NotContains(t, escapedOut.ErrorBody, "hidden")
}
