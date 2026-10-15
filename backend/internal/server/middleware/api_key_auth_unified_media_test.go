package middleware

import (
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

func TestUnifiedMediaTaskPollingSkipsBillingOnlyOnExactReadShape(t *testing.T) {
	for _, path := range []string{"/v1/images/generations/media_123", "/v1/videos/generations/media_123"} {
		require.True(t, isAsyncImageTaskRead(http.MethodGet, path))
		require.False(t, isAsyncImageTaskRead(http.MethodPost, path))
	}
	for _, path := range []string{"/v1/images/generations", "/v1/videos/generations/media_123/content", "/v1/videos/generations/other", "/images/generations/media_123"} {
		require.False(t, isAsyncImageTaskRead(http.MethodGet, path))
	}
}
