package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserDTOHidesMediaProviderWithoutChangingAdminOrSource(t *testing.T) {
	group := &service.Group{ID: 3, Platform: "grsai", Name: "images", RateMultiplier: 1}
	key := &service.APIKey{ID: 2, UserID: 1, Group: group}
	subscription := &service.UserSubscription{ID: 4, Group: group}
	user := &service.User{ID: 1, APIKeys: []service.APIKey{*key}, Subscriptions: []service.UserSubscription{*subscription}}
	usage := &service.UsageLog{ID: 5, Group: group, APIKey: key, Subscription: subscription}
	redeem := &service.RedeemCode{ID: 6, Group: group}
	for name, value := range map[string]any{
		"available group": GroupFromService(group),
		"key":             APIKeyFromService(key),
		"user":            UserFromService(user),
		"usage":           UsageLogFromService(usage),
		"subscription":    UserSubscriptionFromService(subscription),
		"redeem":          RedeemCodeFromService(redeem),
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := json.Marshal(value)
			require.NoError(t, err)
			require.Contains(t, string(raw), `"platform":"media"`)
			require.NotContains(t, string(raw), `"platform":"grsai"`)
		})
	}
	for name, value := range map[string]any{
		"group": GroupFromServiceAdmin(group), "user": UserFromServiceAdmin(user),
		"usage": UsageLogFromServiceAdmin(usage), "subscription": UserSubscriptionFromServiceAdmin(subscription),
		"redeem": RedeemCodeFromServiceAdmin(redeem),
	} {
		t.Run("admin "+name, func(t *testing.T) {
			raw, err := json.Marshal(value)
			require.NoError(t, err)
			require.Contains(t, string(raw), `"platform":"grsai"`)
			require.NotContains(t, string(raw), `"platform":"media"`)
		})
	}
	require.Equal(t, "grsai", group.Platform)
	require.Equal(t, "grsai", key.Group.Platform)
	require.Equal(t, "grsai", user.APIKeys[0].Group.Platform)
	require.Equal(t, "openai", GroupFromService(&service.Group{Platform: "openai"}).Platform)
}

func TestUserUsageRequestIDHidesMediaProvider(t *testing.T) {
	src := &service.UsageLog{RequestID: "grsai_task:123"}
	require.Equal(t, "media_task:123", UsageLogFromService(src).RequestID)
	require.Equal(t, "grsai_task:123", UsageLogFromServiceAdmin(src).RequestID)
	require.Equal(t, "grsai_task:123", src.RequestID)
}
