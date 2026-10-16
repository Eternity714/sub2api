//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type gatewayMediaDiscoveryChannelRepo struct {
	service.ChannelRepository
	pricing []service.ChannelModelPricing
	err     error
}

func (r *gatewayMediaDiscoveryChannelRepo) ListAll(context.Context) ([]service.Channel, error) {
	return []service.Channel{{ID: 1, Status: service.StatusActive, BillingModelSource: service.BillingModelSourceRequested,
		GroupIDs: []int64{22}, ModelPricing: r.pricing}}, r.err
}

func (r *gatewayMediaDiscoveryChannelRepo) GetGroupPlatforms(context.Context, []int64) (map[int64]string, error) {
	return map[int64]string{22: service.PlatformComposite}, nil
}

func newGatewayMediaDiscoveryHandler(routes *gatewayModelsRouteRepoStub, pricing []service.ChannelModelPricing, channelError error) *GatewayHandler {
	channels := service.NewChannelService(&gatewayMediaDiscoveryChannelRepo{pricing: pricing, err: channelError}, nil, nil, nil, nil)
	return &GatewayHandler{gatewayService: service.NewGatewayService(
		&gatewayModelsAccountRepoStub{},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, channels, nil, service.NewCompositeRouteResolver(routes), nil, nil,
	)}
}

func mediaDiscoveryPrice(models []string, value float64) service.ChannelModelPricing {
	return service.ChannelModelPricing{Platform: service.PlatformToAPIs, Models: models, BillingMode: service.BillingModeImage, PerRequestPrice: &value}
}

// AC-029: discovery hides redundant provider aliases after allowlisting; accepted routes remain unchanged.
func TestGatewayMediaDiscoveryHidesOnlyRedundantVisibleAliases(t *testing.T) {
	const groupID int64 = 22
	routes := &gatewayModelsRouteRepoStub{routes: []service.CompositeModelRoute{
		{ID: 1, GroupID: groupID, PublicModel: "image-one", UpstreamModel: "image-one", TargetPlatform: service.PlatformToAPIs, Endpoint: service.CompositeRouteEndpointImages, MatchType: service.CompositeRouteMatchExact, Enabled: true},
		{ID: 2, GroupID: groupID, PublicModel: "toapis/image-one", UpstreamModel: "image-one", TargetPlatform: service.PlatformToAPIs, Endpoint: service.CompositeRouteEndpointImages, MatchType: service.CompositeRouteMatchExact, Enabled: true},
		{ID: 3, GroupID: groupID, PublicModel: "MiniMax-H3", UpstreamModel: "minimax-h3", TargetPlatform: service.PlatformGrsai, Endpoint: service.CompositeRouteEndpointVideos, MatchType: service.CompositeRouteMatchExact, Enabled: true},
		{ID: 4, GroupID: groupID, PublicModel: "grsai/minimax-h3", UpstreamModel: "minimax-h3", TargetPlatform: service.PlatformGrsai, Endpoint: service.CompositeRouteEndpointVideos, MatchType: service.CompositeRouteMatchExact, Enabled: true},
		{ID: 5, GroupID: groupID, PublicModel: "toapis/only-alias", UpstreamModel: "only-alias", TargetPlatform: service.PlatformToAPIs, Endpoint: service.CompositeRouteEndpointImages, MatchType: service.CompositeRouteMatchExact, Enabled: true},
	}}
	for i := range routes.routes {
		routes.routes[i].CapabilityModel = routes.routes[i].TargetPlatform + "/" + routes.routes[i].UpstreamModel
	}
	for _, tc := range []struct {
		name      string
		allowlist []string
		want      []string
	}{
		{"all", nil, []string{"image-one", "MiniMax-H3", "toapis/only-alias"}},
		{"alias only allowed", []string{"toapis/image-one"}, []string{"toapis/image-one"}},
		{"neutral only allowed", []string{"image-one"}, []string{"image-one"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newGatewayMediaDiscoveryHandler(routes, []service.ChannelModelPricing{
				mediaDiscoveryPrice([]string{"image-one", "toapis/image-one"}, 1),
				mediaDiscoveryPrice([]string{"MiniMax-H3", "grsai/minimax-h3"}, 2),
			}, nil)
			group := &service.Group{ID: groupID, Platform: service.PlatformComposite}
			if tc.allowlist != nil {
				group.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: tc.allowlist}
			}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
			c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{Group: group})
			h.Models(c)
			require.Equal(t, http.StatusOK, w.Code)
			var out gatewayModelsResponseForTest
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
			require.ElementsMatch(t, tc.want, modelIDsForTest(out.Data))
		})
	}
	resolved, err := service.NewCompositeRouteResolver(routes).ResolveMediaCandidates(context.Background(), groupID, "toapis/image-one", service.CompositeRouteEndpointImages)
	require.NoError(t, err)
	require.Len(t, resolved, 1)
	require.Equal(t, "image-one", resolved[0].UpstreamModel)
	require.Equal(t, "toapis/image-one", routes.routes[1].PublicModel)
}

// AC-029: a matching basename cannot hide different media, price or routing capabilities.
func TestGatewayMediaDiscoveryKeepsUnprovenAliases(t *testing.T) {
	for _, scenario := range []string{"different media", "different price", "different upstream", "different capability", "implicit capability", "missing alias candidate", "uncovered fallback", "missing price", "channel query failure", "missing channel dependency"} {
		t.Run(scenario, func(t *testing.T) {
			routes := &gatewayModelsRouteRepoStub{routes: []service.CompositeModelRoute{
				{ID: 1, GroupID: 22, PublicModel: "image", MatchType: service.CompositeRouteMatchExact, Endpoint: service.CompositeRouteEndpointImages, TargetPlatform: service.PlatformToAPIs, UpstreamModel: "same", CapabilityModel: "toapis/same", Enabled: true},
				{ID: 2, GroupID: 22, PublicModel: "toapis/image", MatchType: service.CompositeRouteMatchExact, Endpoint: service.CompositeRouteEndpointImages, TargetPlatform: service.PlatformToAPIs, UpstreamModel: "same", CapabilityModel: "toapis/same", Enabled: true},
			}}
			pricing := []service.ChannelModelPricing{mediaDiscoveryPrice([]string{"image", "toapis/image"}, 1)}
			var channelError error
			switch scenario {
			case "different media":
				routes.routes[1].Endpoint = service.CompositeRouteEndpointVideos
			case "different price":
				pricing = []service.ChannelModelPricing{mediaDiscoveryPrice([]string{"image"}, 1), mediaDiscoveryPrice([]string{"toapis/image"}, 2)}
			case "different upstream":
				routes.routes[1].UpstreamModel = "different"
			case "different capability":
				routes.routes[1].CapabilityModel = "toapis/different"
			case "implicit capability":
				routes.routes[0].CapabilityModel, routes.routes[1].CapabilityModel = "", ""
			case "missing alias candidate":
				routes.routes[1].TargetPlatform = service.PlatformOpenAI
			case "uncovered fallback":
				routes.routes = append(routes.routes, service.CompositeModelRoute{ID: 3, GroupID: 22, PublicModel: "toapis/image", MatchType: service.CompositeRouteMatchExact, Endpoint: service.CompositeRouteEndpointImages, TargetPlatform: service.PlatformGrsai, UpstreamModel: "fallback", CapabilityModel: "grsai/fallback", Priority: 50, Enabled: true})
			case "missing price":
				pricing = nil
			case "channel query failure":
				channelError = errors.New("unavailable")
			}
			h := newGatewayMediaDiscoveryHandler(routes, pricing, channelError)
			if scenario == "missing channel dependency" {
				h = newGatewayModelsHandlerForTest(&gatewayModelsAccountRepoStub{}, routes)
			}
			group := &service.Group{ID: 22, Platform: service.PlatformComposite}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
			c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{Group: group})
			h.Models(c)
			var out gatewayModelsResponseForTest
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
			require.ElementsMatch(t, []string{"image", "toapis/image"}, modelIDsForTest(out.Data))
		})
	}
}
