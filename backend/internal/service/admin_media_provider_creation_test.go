//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateAccountPreservesGrsaiProviderAndCredentials(t *testing.T) {
	repo := &upstreamBillingProbeAccountRepo{}
	account, err := (&adminServiceImpl{accountRepo: repo}).CreateAccount(context.Background(), &CreateAccountInput{
		Name: "media", Platform: PlatformGrsai, Type: AccountTypeAPIKey,
		Credentials:          map[string]any{"api_key": "key", "base_url": "https://media.example.com"},
		SkipDefaultGroupBind: true,
	})
	require.NoError(t, err)
	require.Equal(t, PlatformGrsai, account.Platform)
	require.Equal(t, "https://media.example.com", account.GetCredential("base_url"))
	require.Equal(t, "key", account.GetCredential("api_key"))
	require.Same(t, account, repo.accounts[account.ID])
}

func TestCreateGroupPreservesGrsaiProvider(t *testing.T) {
	repo := &groupRepoStubForAdmin{}
	group, err := (&adminServiceImpl{groupRepo: repo}).CreateGroup(context.Background(), &CreateGroupInput{
		Name: "media", Platform: PlatformGrsai, RateMultiplier: 1,
	})
	require.NoError(t, err)
	require.Equal(t, PlatformGrsai, group.Platform)
	require.Same(t, group, repo.created)
}

func TestCreateChannelPreservesGrsaiPricingAndMappings(t *testing.T) {
	price := 0.1
	legacyPricing := []ChannelModelPricing{{Platform: PlatformGrsai, Models: []string{"image-model"}, BillingMode: BillingModeImage, PerRequestPrice: &price}}
	for name, input := range map[string]*CreateChannelInput{
		"pricing":               {Name: "media", ModelPricing: legacyPricing},
		"mapping":               {Name: "media", ModelMapping: map[string]map[string]string{PlatformGrsai: {"public": "upstream"}}},
		"account_stats_pricing": {Name: "media", AccountStatsPricingRules: []AccountStatsPricingRule{{Pricing: legacyPricing}}},
		"group":                 {Name: "media", GroupIDs: []int64{1}},
	} {
		t.Run(name, func(t *testing.T) {
			var created *Channel
			repo := &mockChannelRepository{
				getGroupPlatformsFn: func(context.Context, []int64) (map[int64]string, error) {
					return map[int64]string{1: PlatformGrsai}, nil
				},
				createFn:  func(_ context.Context, channel *Channel) error { created = channel; return nil },
				getByIDFn: func(context.Context, int64) (*Channel, error) { return created, nil },
			}
			channel, err := NewChannelService(repo, nil, nil, nil, nil).Create(context.Background(), input)
			require.NoError(t, err)
			require.NotNil(t, channel)
			require.Equal(t, input.ModelPricing, channel.ModelPricing)
			require.Equal(t, input.ModelMapping, channel.ModelMapping)
			require.Equal(t, input.AccountStatsPricingRules, channel.AccountStatsPricingRules)
			require.Equal(t, input.GroupIDs, channel.GroupIDs)
		})
	}
}

func TestDuplicateGroupPreservesGrsaiProvider(t *testing.T) {
	source := &Group{ID: 1, Name: "media", Platform: PlatformGrsai}
	repo := newDuplicateGroupRepoStub(source)
	svc := &adminServiceImpl{groupRepo: repo, groupDuplicateRepo: repo}
	group, err := svc.DuplicateGroup(context.Background(), source.ID, "admin:1", "")
	require.NoError(t, err)
	require.Equal(t, PlatformGrsai, group.Platform)
	require.NotEqual(t, source.ID, group.ID)
}

func TestDuplicateAccountBuilderPreservesGrsaiProvider(t *testing.T) {
	account, err := buildAccountForCreate(&CreateAccountInput{
		Name: "media-copy", Platform: PlatformGrsai, Type: AccountTypeAPIKey,
	}, nil)
	require.NoError(t, err)
	require.Equal(t, PlatformGrsai, account.Platform)
}

func TestCreateOpenAIAccountPreservesCustomMediaCredentials(t *testing.T) {
	repo := &upstreamBillingProbeAccountRepo{}
	account, err := (&adminServiceImpl{accountRepo: repo}).CreateAccount(context.Background(), &CreateAccountInput{
		Name: "media", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials:          map[string]any{"api_key": "key", "base_url": "https://media.example.com/custom"},
		SkipDefaultGroupBind: true,
	})
	require.NoError(t, err)
	require.Equal(t, "https://media.example.com/custom", account.GetCredential("base_url"))
	require.Equal(t, "key", account.GetCredential("api_key"))
}
