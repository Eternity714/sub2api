//go:build unit

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func mediaProjectionPrice(value float64) *float64 { return &value }

// AC-029: both upstream providers have a neutral public identity; admin and source retain their identity.
func TestUserMediaToAPIsDTOProjectionPreservesAdminAndSource(t *testing.T) {
	for _, platform := range []string{service.PlatformGrsai, service.PlatformToAPIs} {
		t.Run(platform, func(t *testing.T) {
			group := &service.Group{ID: 22, Name: "media", Platform: platform}
			require.Equal(t, "media", dto.GroupFromService(group).Platform)
			require.Equal(t, platform, dto.GroupFromServiceAdmin(group).Platform)
			require.Equal(t, platform, group.Platform)
		})
	}
	require.Equal(t, service.PlatformOpenAI, dto.UserVisiblePlatform(service.PlatformOpenAI))
	require.Equal(t, service.PlatformComposite, dto.UserVisiblePlatform(service.PlatformComposite))
}

// AC-029: a mixed media channel is one section, with lossless pricing and unchanged public model IDs.
func TestUserMediaToAPIsChannelMergesSectionsWithoutLosingPrices(t *testing.T) {
	pricing := func(value float64) *service.ChannelModelPricing {
		return &service.ChannelModelPricing{BillingMode: service.BillingModeImage, PerRequestPrice: mediaProjectionPrice(value)}
	}
	channel := service.AvailableChannel{SupportedModels: []service.SupportedModel{
		{Name: "shared", Platform: service.PlatformGrsai, Pricing: pricing(1)},
		{Name: "shared", Platform: service.PlatformToAPIs, Pricing: pricing(1)},
		{Name: "shared", Platform: service.PlatformToAPIs, Pricing: pricing(2)},
		{Name: "toapis/alias-only", Platform: service.PlatformToAPIs, Pricing: pricing(3)},
		{Name: "chat", Platform: service.PlatformOpenAI, Pricing: pricing(4)},
	}}
	groups := []userAvailableGroup{{ID: 22, Name: "media", Platform: service.PlatformComposite}}
	sections := buildPlatformSections(channel, groups)
	require.Len(t, sections, 2)
	media := sections[0]
	require.Equal(t, "media", media.Platform)
	require.Len(t, media.Groups, 1)
	require.Equal(t, int64(22), media.Groups[0].ID)
	require.Len(t, media.SupportedModels, 3)
	gotPrices := make(map[string][]float64)
	for _, model := range media.SupportedModels {
		require.Equal(t, "media", model.Platform)
		gotPrices[model.Name] = append(gotPrices[model.Name], *model.Pricing.PerRequestPrice)
	}
	require.ElementsMatch(t, []float64{1, 2}, gotPrices["shared"])
	require.Equal(t, []float64{3}, gotPrices["toapis/alias-only"])
	require.Equal(t, service.PlatformOpenAI, sections[1].Platform)
	require.Equal(t, service.PlatformToAPIs, channel.SupportedModels[1].Platform)
	require.Equal(t, service.PlatformComposite, groups[0].Platform)
}

// AC-029: projection happens after visibility filtering, so native GRS groups cannot discover ToAPIs models.
func TestUserMediaToAPIsChannelKeepsNativePlatformVisibility(t *testing.T) {
	channel := service.AvailableChannel{SupportedModels: []service.SupportedModel{
		{Name: "allowed", Platform: service.PlatformGrsai},
		{Name: "not-allowed", Platform: service.PlatformToAPIs},
	}}
	sections := buildPlatformSections(channel, []userAvailableGroup{{ID: 1, Name: "native", Platform: service.PlatformGrsai}})
	require.Len(t, sections, 1)
	require.Equal(t, "media", sections[0].Platform)
	require.Len(t, sections[0].SupportedModels, 1)
	require.Equal(t, "allowed", sections[0].SupportedModels[0].Name)
}

// AC-029: public model-plaza entries share the neutral identity without silently dropping different prices.
func TestUserMediaToAPIsPlazaPreservesDistinctPricing(t *testing.T) {
	model := func(platform string, price float64) service.PlazaModel {
		return service.PlazaModel{Name: "public-image", Platform: platform,
			Pricing: &service.ChannelModelPricing{BillingMode: service.BillingModeImage, PerRequestPrice: mediaProjectionPrice(price)}}
	}
	group := service.PlazaGroup{ID: 22, Platform: service.PlatformComposite, Models: []service.PlazaModel{
		model(service.PlatformGrsai, 1), model(service.PlatformToAPIs, 1), model(service.PlatformToAPIs, 2),
	}}
	visible := toModelPlazaGroupDTO(&group, nil)
	require.Len(t, visible.Models, 2)
	var prices []float64
	for _, entry := range visible.Models {
		require.Equal(t, "media", entry.Platform)
		require.Equal(t, "public-image", entry.Name)
		prices = append(prices, *entry.Pricing.PerRequestPrice)
	}
	require.ElementsMatch(t, []float64{1, 2}, prices)
	require.Equal(t, service.PlatformToAPIs, group.Models[1].Platform)
}

// AC-029: merging public media buckets sums monetary and request/token totals without altering repository data.
func TestUserMediaToAPIsDashboardAggregatesBothProviders(t *testing.T) {
	repo := &mediaDashboardRepo{stats: usagestats.UserDashboardStats{ByPlatform: []usagestats.PlatformDashboardStats{
		{Platform: service.PlatformGrsai, TotalRequests: 3, TotalTokens: 30, TotalActualCost: 0.3, TodayRequests: 1, TodayTokens: 10, TodayActualCost: 0.1},
		{Platform: service.PlatformToAPIs, TotalRequests: 4, TotalTokens: 40, TotalActualCost: 0.4, TodayRequests: 2, TodayTokens: 20, TodayActualCost: 0.2},
		{Platform: service.PlatformOpenAI, TotalRequests: 5, TotalTokens: 50, TotalActualCost: 0.5},
	}}}
	h := &UsageHandler{usageService: service.NewUsageService(repo, nil, nil, nil)}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/usage/dashboard/stats", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	h.DashboardStats(c)
	require.Equal(t, http.StatusOK, w.Code)
	var envelope struct {
		Data usagestats.UserDashboardStats `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.ByPlatform, 2)
	media := envelope.Data.ByPlatform[0]
	require.Equal(t, "media", media.Platform)
	require.Equal(t, int64(7), media.TotalRequests)
	require.Equal(t, int64(70), media.TotalTokens)
	require.InDelta(t, 0.7, media.TotalActualCost, 1e-10)
	require.Equal(t, int64(3), media.TodayRequests)
	require.Equal(t, int64(30), media.TodayTokens)
	require.InDelta(t, 0.3, media.TodayActualCost, 1e-10)
	require.Equal(t, repo.stats.ByPlatform[2], envelope.Data.ByPlatform[1])
	require.Equal(t, service.PlatformGrsai, repo.stats.ByPlatform[0].Platform)
	require.Equal(t, service.PlatformToAPIs, repo.stats.ByPlatform[1].Platform)
}

// AC-029: hide a provider-prefixed discovery alias only when an equivalent visible public model exists.
func TestUserMediaToAPIsChannelHidesOnlyEquivalentVisibleAliases(t *testing.T) {
	pricing := func(value float64) *service.ChannelModelPricing {
		return &service.ChannelModelPricing{BillingMode: service.BillingModeImage, PerRequestPrice: mediaProjectionPrice(value)}
	}
	channel := service.AvailableChannel{SupportedModels: []service.SupportedModel{
		{Name: "Public-Image", Platform: service.PlatformToAPIs, Pricing: pricing(1)},
		{Name: "toapis/public-image", Platform: service.PlatformToAPIs, Pricing: pricing(1)},
		{Name: "grsai/PUBLIC-IMAGE", Platform: service.PlatformGrsai, Pricing: pricing(1)},
		{Name: "toapis/Public-Image", Platform: service.PlatformToAPIs, Pricing: pricing(2)},
		{Name: "toapis/alias-only", Platform: service.PlatformToAPIs, Pricing: pricing(3)},
	}}
	sections := buildPlatformSections(channel, []userAvailableGroup{{ID: 22, Platform: service.PlatformComposite}})
	require.Len(t, sections, 1)
	var names []string
	for _, model := range sections[0].SupportedModels {
		names = append(names, model.Name)
	}
	require.ElementsMatch(t, []string{"Public-Image", "toapis/Public-Image", "toapis/alias-only"}, names)
	require.Len(t, channel.SupportedModels, 5)

	// An inaccessible public candidate cannot hide an otherwise visible native alias.
	channel.SupportedModels = []service.SupportedModel{
		{Name: "public-image", Platform: service.PlatformToAPIs, Pricing: pricing(1)},
		{Name: "grsai/public-image", Platform: service.PlatformGrsai, Pricing: pricing(1)},
	}
	sections = buildPlatformSections(channel, []userAvailableGroup{{ID: 1, Platform: service.PlatformGrsai}})
	require.Len(t, sections, 1)
	require.Len(t, sections[0].SupportedModels, 1)
	require.Equal(t, "grsai/public-image", sections[0].SupportedModels[0].Name)
}

// AC-029: model plaza does not strip accepted IDs or hide differently priced aliases.
func TestUserMediaToAPIsPlazaHidesOnlyEquivalentAliases(t *testing.T) {
	model := func(name string, price float64) service.PlazaModel {
		return service.PlazaModel{Name: name, Platform: service.PlatformToAPIs,
			Pricing: &service.ChannelModelPricing{BillingMode: service.BillingModeImage, PerRequestPrice: mediaProjectionPrice(price)}}
	}
	group := service.PlazaGroup{Platform: service.PlatformComposite, Models: []service.PlazaModel{
		model("public-image", 1), model("toapis/PUBLIC-IMAGE", 1), model("toapis/public-image", 2), model("toapis/only-alias", 3),
	}}
	visible := toModelPlazaGroupDTO(&group, nil)
	var names []string
	for _, entry := range visible.Models {
		names = append(names, entry.Name)
	}
	require.ElementsMatch(t, []string{"public-image", "toapis/public-image", "toapis/only-alias"}, names)
	require.Equal(t, "toapis/PUBLIC-IMAGE", group.Models[1].Name)
}
