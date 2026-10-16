//go:build unit

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// AC-029: user filters expand the neutral identity without changing source inputs.
func TestMediaMonitorFilterExpandsBothProvidersWithoutDuplicates(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?platform=media", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	h := &ChannelMonitorV2Handler{apiKeyService: &channelMonitorV2GroupAuthorizerStub{groups: []service.Group{{ID: 7}}}}
	platforms := []string{"media", "grsai", "openai"}
	models := []string{"media\x00shared", "grsai\x00shared", "openai\x00shared"}
	filter := service.ChannelMonitorV2Filter{Platforms: platforms, Models: models}
	require.True(t, h.scopeFilter(c, &filter, false))
	require.Equal(t, []string{"grsai", "toapis", "openai"}, filter.Platforms)
	require.Equal(t, []string{"grsai\x00shared", "toapis\x00shared", "openai\x00shared"}, filter.Models)
	require.Equal(t, []string{"media", "grsai", "openai"}, platforms)
	require.Equal(t, []string{"media\x00shared", "grsai\x00shared", "openai\x00shared"}, models)
}

// AC-029: user dimension identities merge both media providers and retain their volume.
func TestMediaMonitorDimensionsMergeProviderIdentities(t *testing.T) {
	src := &service.ChannelMonitorV2Dimensions{
		Platforms: []service.ChannelMonitorV2Dimension{
			{Value: "grsai", Label: "GRS.AI", Platform: "grsai", RequestCount: 2},
			{Value: "toapis", Label: "ToAPIs", Platform: "toapis", RequestCount: 7},
			{Value: "openai", Label: "OpenAI", Platform: "openai", RequestCount: 3},
		},
		Groups: []service.ChannelMonitorV2GroupDimension{{ID: 7, Platform: "toapis"}},
		Models: []service.ChannelMonitorV2Dimension{
			{Value: "grsai\x00shared", Label: "grsai\x00shared", Platform: "grsai", RequestCount: 2},
			{Value: "toapis\x00shared", Label: "ToAPIs shared", Platform: "toapis", RequestCount: 7},
			{Value: "toapis\x00__other__", Label: "ToAPIs Other", Platform: "toapis", RequestCount: 4},
		},
	}
	out := userMonitorV2Dimensions(src)
	require.Equal(t, []service.ChannelMonitorV2Dimension{
		{Value: "media", Label: "Media API", Platform: "media", RequestCount: 9},
		{Value: "openai", Label: "OpenAI", Platform: "openai", RequestCount: 3},
	}, out.Platforms)
	require.Equal(t, []service.ChannelMonitorV2Dimension{
		{Value: "media\x00shared", Label: "shared", Platform: "media", RequestCount: 9},
		{Value: "media\x00__other__", Label: "Other models", Platform: "media", RequestCount: 4},
	}, out.Models)
	require.Equal(t, "media", out.Groups[0].Platform)
	require.Equal(t, "toapis", src.Platforms[1].Value)
	require.Equal(t, "ToAPIs shared", src.Models[1].Label)
	require.Equal(t, "toapis", src.Groups[0].Platform)
}

// AC-029: public configuration has one neutral platform and keeps operator config private.
func TestMediaMonitorSnapshotMergesPublicPlatformConfiguration(t *testing.T) {
	repo := &mediaMonitorRepo{config: service.ChannelMonitorV2Config{Enabled: true, Platforms: []service.ChannelMonitorV2PlatformConfig{
		{Platform: "grsai", Enabled: false, Models: []string{"private-grs"}},
		{Platform: "toapis", Enabled: true, Models: []string{"private-toapis"}},
		{Platform: "openai", Enabled: true},
	}}}
	h := &ChannelMonitorV2Handler{service: service.NewChannelMonitorV2Service(repo), apiKeyService: &channelMonitorV2GroupAuthorizerStub{groups: []service.Group{{ID: 7}}}}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/channel-monitor-v2/snapshot", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	h.Snapshot(c)
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Data service.ChannelMonitorV2Snapshot `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, []service.ChannelMonitorV2PlatformConfig{
		{Platform: "media", Enabled: true}, {Platform: "openai", Enabled: true},
	}, body.Data.Config.Platforms)
	require.NotContains(t, w.Body.String(), "toapis")
	require.NotContains(t, w.Body.String(), "grsai")
	require.Equal(t, "grsai", repo.config.Platforms[0].Platform)
	require.Equal(t, "toapis", repo.config.Platforms[1].Platform)
}
