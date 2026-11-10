//go:build unit

package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserKuaipaoChannelAndPlazaUseMediaIdentity(t *testing.T) {
	price := 0.066
	pricing := &service.ChannelModelPricing{BillingMode: service.BillingModeImage, PerRequestPrice: &price}
	channel := service.AvailableChannel{SupportedModels: []service.SupportedModel{
		{Name: "gpt-image-2.6", Platform: service.PlatformGrsai, Pricing: pricing},
		{Name: "gpt-image-2.6", Platform: "kuaipao", Pricing: pricing},
	}}
	sections := buildPlatformSections(channel, []userAvailableGroup{{ID: 22, Platform: service.PlatformComposite}})
	require.Len(t, sections, 1)
	require.Equal(t, "media", sections[0].Platform)
	require.Len(t, sections[0].SupportedModels, 1)
	require.Equal(t, "media", sections[0].SupportedModels[0].Platform)
	group := service.PlazaGroup{Platform: service.PlatformComposite, Models: []service.PlazaModel{
		{Name: "gpt-image-2.6", Platform: service.PlatformGrsai, Pricing: pricing},
		{Name: "gpt-image-2.6", Platform: "kuaipao", Pricing: pricing},
	}}
	visible := toModelPlazaGroupDTO(&group, nil)
	require.Len(t, visible.Models, 1)
	require.Equal(t, "media", visible.Models[0].Platform)
	require.Equal(t, 0.066, *visible.Models[0].Pricing.PerRequestPrice)
	require.Equal(t, "kuaipao", group.Models[1].Platform)
	require.Equal(t, service.PlatformOpenAI, dto.UserVisiblePlatform(service.PlatformOpenAI))
	require.Equal(t, "kuaipao", dto.GroupFromServiceAdmin(&service.Group{Platform: "kuaipao"}).Platform)
}
