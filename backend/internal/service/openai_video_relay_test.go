//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func openAIVideoTestAccount() *Account {
	a := grokMediaContentTestAccount()
	a.Extra = map[string]any{"grok_video_protocol": "openai"}
	return a
}

func TestOpenAIVideoRelayCreatePreservesSecondsAndUsesVideosEndpoint(t *testing.T) {
	for _, seconds := range []string{`4`, `"4"`} {
		t.Run(seconds, func(t *testing.T) {
			upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(`{"id":"video_1","status":"queued"}`)}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			c, _ := grokMediaContentTestContext(http.MethodPost, "https://api.example/v1/videos", nil)
			body := []byte(`{"model":"seedance-2.5","prompt":"blue ceramic cup","seconds":` + seconds + `,"resolution":"480p"}`)
			result, err := svc.ForwardGrokMedia(context.Background(), c, openAIVideoTestAccount(), GrokMediaEndpointVideosGenerations, "", body, "application/json")
			require.NoError(t, err)
			require.Equal(t, "https://relay.example/v1/videos", upstream.request.URL.String())
			sent, err := io.ReadAll(upstream.request.Body)
			require.NoError(t, err)
			require.JSONEq(t, string(body), string(sent))
			require.Equal(t, 4, result.VideoDurationSeconds)
			require.Equal(t, "480p", result.VideoResolution)
			require.Equal(t, "video_1", result.ResponseID)
			require.Zero(t, result.VideoCount)
		})
	}
}

func TestOpenAIVideoRelayCompletionRequiresExplicitProtocol(t *testing.T) {
	for _, protocol := range []string{"", "openai"} {
		for _, status := range []string{"queued", "in_progress", "completed", "failed"} {
			t.Run(protocol+"/"+status, func(t *testing.T) {
				a := grokMediaContentTestAccount()
				a.Extra = map[string]any{"grok_video_protocol": protocol}
				upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(`{"id":"video_1","status":"` + status + `","model":"seedance-2.5","seconds":"4","resolution":"720p"}`)}
				svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
				c, _ := grokMediaContentTestContext(http.MethodGet, "https://api.example/v1/videos/video_1", nil)
				result, err := svc.ForwardGrokMedia(context.Background(), c, a, GrokMediaEndpointVideoStatus, "video_1", nil, "")
				require.NoError(t, err)
				if protocol == "openai" && status == "completed" {
					require.Equal(t, 1, result.VideoCount)
					require.Equal(t, 4, result.VideoDurationSeconds)
					require.Equal(t, "720p", result.VideoResolution)
					require.Equal(t, "seedance-2.5", result.Model)
					require.Equal(t, "video_1", result.ResponseID)
				} else {
					require.Zero(t, result.VideoCount)
				}
			})
		}
	}
}

func TestOpenAIVideoRelayContentBillsCompletionAndUsesAuthenticatedEndpoint(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{responses: []*http.Response{
		grokMediaContentStatusResponse(`{"id":"video_1","status":"completed","model":"seedance-2.5","seconds":4,"resolution":"480p","video":{"url":"https://cdn.example/video.mp4"}}`),
		{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"video/mp4"}}, Body: io.NopCloser(strings.NewReader("video-data"))},
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	c, r := grokMediaContentTestContext(http.MethodGet, "https://api.example/v1/videos/video_1/content", nil)
	result, err := svc.ForwardGrokMedia(context.Background(), c, openAIVideoTestAccount(), GrokMediaEndpointVideoContent, "video_1", nil, "")
	require.NoError(t, err)
	require.Equal(t, "video-data", r.Body.String())
	require.Equal(t, "https://relay.example/v1/videos/video_1/content", upstream.requests[1].URL.String())
	require.Equal(t, "Bearer upstream-key", upstream.requests[1].Header.Get("Authorization"))
	require.Equal(t, 1, result.VideoCount)
	require.Equal(t, 4, result.VideoDurationSeconds)
}

func TestOpenAIVideoRelayRejectsUnsupportedOperationsBeforeSubmitting(t *testing.T) {
	for _, endpoint := range []GrokMediaEndpoint{GrokMediaEndpointVideosEdits, GrokMediaEndpointVideosExtensions} {
		_, err := buildGrokMediaURL(openAIVideoTestAccount(), &config.Config{}, endpoint, "")
		require.Error(t, err)
	}
}

func TestOpenAIVideoRelayRejectsUnpricedOrAmbiguousRequests(t *testing.T) {
	for _, body := range []string{
		`{"model":"seedance-2.5","seconds":4,"resolution":"480p","reference_videos":["video.mp4"]}`,
		`{"model":"seedance-2.5","seconds":4,"resolution":"480p","content":[{"type":"video_url","url":"data:video/mp4;base64,AAAA"}]}`,
		`{"model":"seedance-2.5","seconds":4,"resolution":"480p","duration":8}`,
		`{"model":"seedance-2.5","seconds":4}`,
		`{"model":"seedance-2.5","seconds":4.5,"resolution":"480p"}`,
	} {
		t.Run(body, func(t *testing.T) {
			upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(`{"id":"unexpected"}`)}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			c, r := grokMediaContentTestContext(http.MethodPost, "https://api.example/v1/videos", nil)
			_, err := svc.ForwardGrokMedia(context.Background(), c, openAIVideoTestAccount(), GrokMediaEndpointVideosGenerations, "", []byte(body), "application/json")
			require.Error(t, err)
			require.Equal(t, http.StatusBadRequest, r.Code)
			require.Empty(t, upstream.requests)
		})
	}
}

func TestOpenAIVideoRelaySubmissionFailureDoesNotFailover(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{response: &http.Response{StatusCode: 503, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":{"message":"temporarily unavailable"}}`))}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	c, r := grokMediaContentTestContext(http.MethodPost, "https://api.example/v1/videos", nil)
	_, err := svc.ForwardGrokMedia(context.Background(), c, openAIVideoTestAccount(), GrokMediaEndpointVideosGenerations, "", []byte(`{"model":"seedance-2.5","seconds":4,"resolution":"480p"}`), "application/json")
	require.Error(t, err)
	var failover *UpstreamFailoverError
	require.False(t, errors.As(err, &failover))
	require.Equal(t, 503, r.Code)
	require.Len(t, upstream.requests, 1)
}
