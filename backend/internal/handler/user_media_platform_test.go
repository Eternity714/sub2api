//go:build unit

package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestUserMediaChannelProjectionKeepsMatchingModels(t *testing.T) {
	price := 0.01
	channel := service.AvailableChannel{SupportedModels: []service.SupportedModel{
		{Name: "image-model", Platform: "grsai", Pricing: &service.ChannelModelPricing{BillingMode: "image", PerRequestPrice: &price}},
		{Name: "private-openai-model", Platform: "openai"},
	}}
	groups := []userAvailableGroup{{ID: 3, Name: "images", Platform: "grsai"}}
	sections := buildPlatformSections(channel, groups)
	require.Len(t, sections, 1)
	require.Equal(t, "media", sections[0].Platform)
	require.Equal(t, "media", sections[0].Groups[0].Platform)
	require.Len(t, sections[0].SupportedModels, 1)
	require.Equal(t, "image-model", sections[0].SupportedModels[0].Name)
	require.Equal(t, "media", sections[0].SupportedModels[0].Platform)
	require.Equal(t, 0.01, *sections[0].SupportedModels[0].Pricing.PerRequestPrice)
	require.Equal(t, "grsai", groups[0].Platform)
	require.Equal(t, "grsai", channel.SupportedModels[0].Platform)
}

type mediaMonitorRepo struct {
	service.ChannelMonitorV2Repository
	config     service.ChannelMonitorV2Config
	dimensions service.ChannelMonitorV2Dimensions
	models     service.ChannelMonitorV2List[service.ChannelMonitorV2ModelRow]
	matrix     service.ChannelMonitorV2Matrix
	filter     service.ChannelMonitorV2Filter
}

func (r *mediaMonitorRepo) GetConfig(context.Context) (*service.ChannelMonitorV2Config, error) {
	return &r.config, nil
}
func (r *mediaMonitorRepo) GetDimensions(_ context.Context, f service.ChannelMonitorV2Filter, _ service.ChannelMonitorV2Config) (*service.ChannelMonitorV2Dimensions, error) {
	r.filter = f
	return &r.dimensions, nil
}
func (r *mediaMonitorRepo) GetSnapshot(_ context.Context, f service.ChannelMonitorV2Filter, cfg service.ChannelMonitorV2Config, _ bool) (*service.ChannelMonitorV2Snapshot, error) {
	r.filter = f
	return &service.ChannelMonitorV2Snapshot{Config: cfg}, nil
}
func (r *mediaMonitorRepo) GetModels(_ context.Context, f service.ChannelMonitorV2Filter, _ service.ChannelMonitorV2Config, _ bool) (*service.ChannelMonitorV2List[service.ChannelMonitorV2ModelRow], error) {
	r.filter = f
	return &r.models, nil
}
func (r *mediaMonitorRepo) GetMatrix(_ context.Context, f service.ChannelMonitorV2Filter, _ service.ChannelMonitorV2Config, _ service.ChannelMonitorV2GroupBy, _ bool) (*service.ChannelMonitorV2Matrix, error) {
	r.filter = f
	return &r.matrix, nil
}

func TestUserMediaMonitorV2ResponsesPreserveInternalPlatforms(t *testing.T) {
	repo := &mediaMonitorRepo{
		config: service.ChannelMonitorV2Config{Enabled: true, Platforms: []service.ChannelMonitorV2PlatformConfig{{Platform: "grsai", Enabled: true}}},
		dimensions: service.ChannelMonitorV2Dimensions{Platforms: []service.ChannelMonitorV2Dimension{{Value: "grsai", Label: "GRS.AI", Platform: "grsai"}}, Groups: []service.ChannelMonitorV2GroupDimension{{ID: 3, Platform: "grsai"}}, Models: []service.ChannelMonitorV2Dimension{
			{Value: "grsai\x00image-model", Label: "grsai\x00image-model", Platform: "grsai"},
			{Value: "openai\x00image-model", Label: "openai\x00image-model", Platform: "openai"},
		}},
		models: service.ChannelMonitorV2List[service.ChannelMonitorV2ModelRow]{Items: []service.ChannelMonitorV2ModelRow{{Platform: "grsai", Model: "image-model"}}},
		matrix: service.ChannelMonitorV2Matrix{Items: []service.ChannelMonitorV2MatrixRow{{Platform: "grsai", Model: "image-model"}}},
	}
	h := &ChannelMonitorV2Handler{service: service.NewChannelMonitorV2Service(repo), apiKeyService: &channelMonitorV2GroupAuthorizerStub{groups: []service.Group{{ID: 3}}}}
	for _, call := range []struct {
		name        string
		user, admin func(*gin.Context)
	}{{"dimensions", h.Dimensions, h.Dimensions}, {"snapshot", h.Snapshot, h.AdminSnapshot}, {"models", h.Models, h.AdminModels}, {"matrix", h.Matrix, h.AdminMatrix}} {
		t.Run(call.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/?platform=media&model=media%00image-model&model=openai%00image-model", nil)
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
			call.user(c)
			require.Equal(t, http.StatusOK, w.Code)
			require.Contains(t, w.Body.String(), "media")
			require.NotContains(t, w.Body.String(), "grsai")
			require.NotContains(t, w.Body.String(), "GRS.AI")
			require.Equal(t, []string{"grsai"}, repo.filter.Platforms)
			require.Equal(t, []string{"grsai\x00image-model", "openai\x00image-model"}, repo.filter.Models)
			if call.name == "dimensions" {
				var body struct {
					Data service.ChannelMonitorV2Dimensions `json:"data"`
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				require.Equal(t, "media\x00image-model", body.Data.Models[0].Value)
				require.Equal(t, "image-model", body.Data.Models[0].Label)
				require.Equal(t, repo.dimensions.Models[1], body.Data.Models[1])
			}
			w = httptest.NewRecorder()
			c, _ = gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/?platform=grsai&model=grsai%00image-model", nil)
			c.Set(string(middleware.ContextKeyUserRole), service.RoleAdmin)
			call.admin(c)
			require.Contains(t, w.Body.String(), "grsai")
			require.NotContains(t, w.Body.String(), "media")
			require.Equal(t, []string{"grsai\x00image-model"}, repo.filter.Models)
		})
	}
	require.Equal(t, "grsai", repo.config.Platforms[0].Platform)
	require.Equal(t, "grsai", repo.dimensions.Platforms[0].Value)
	require.Equal(t, "grsai\x00image-model", repo.dimensions.Models[0].Value)
	require.Equal(t, "grsai\x00image-model", repo.dimensions.Models[0].Label)
	require.Equal(t, "grsai", repo.models.Items[0].Platform)
	require.Equal(t, "grsai", repo.matrix.Items[0].Platform)
}

type mediaDashboardRepo struct {
	service.UsageLogRepository
	stats usagestats.UserDashboardStats
}

func (r *mediaDashboardRepo) GetUserDashboardStats(context.Context, int64) (*usagestats.UserDashboardStats, error) {
	return &r.stats, nil
}

func TestUserMediaDashboardResponseKeepsSource(t *testing.T) {
	repo := &mediaDashboardRepo{stats: usagestats.UserDashboardStats{ByPlatform: []usagestats.PlatformDashboardStats{{Platform: "grsai", TotalActualCost: 0.01}}}}
	h := &UsageHandler{usageService: service.NewUsageService(repo, nil, nil, nil)}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/usage/dashboard/stats", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	h.DashboardStats(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"platform":"media"`)
	require.NotContains(t, w.Body.String(), "grsai")
	require.Equal(t, "grsai", repo.stats.ByPlatform[0].Platform)
	require.Equal(t, 0.01, repo.stats.ByPlatform[0].TotalActualCost)
}

func TestUserMediaPaymentPlansKeepInternalGroup(t *testing.T) {
	db, err := sql.Open("sqlite", "file:media_payment_plans?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(entsql.OpenDB(dialect.SQLite, db))))
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	group, err := client.Group.Create().SetName("images").SetPlatform("grsai").Save(ctx)
	require.NoError(t, err)
	_, err = client.SubscriptionPlan.Create().SetName("images").SetGroupID(group.ID).SetPrice(0.01).SetValidityDays(30).SetValidityUnit("day").SetForSale(true).Save(ctx)
	require.NoError(t, err)
	h := &PaymentHandler{configService: service.NewPaymentConfigService(client, nil, []byte("0123456789abcdef0123456789abcdef"))}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/payment/plans", nil)
	h.GetPlans(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"group_platform":"media"`)
	require.NotContains(t, w.Body.String(), "grsai")
	stored, err := client.Group.Get(ctx, group.ID)
	require.NoError(t, err)
	require.Equal(t, "grsai", stored.Platform)
}

func TestUserMediaPlazaProjectionPreservesPriceAndSource(t *testing.T) {
	price := 0.01
	group := service.PlazaGroup{ID: 3, Platform: "grsai", Models: []service.PlazaModel{{
		Name: "image-model", Platform: "grsai", Pricing: &service.ChannelModelPricing{BillingMode: "image", PerRequestPrice: &price},
	}}}
	visible := filterPlazaVisibleGroups([]service.PlazaGroup{group}, nil, false)
	require.Len(t, visible, 1)
	out := toModelPlazaGroupDTO(&visible[0], nil)
	require.Equal(t, "media", out.Platform)
	require.Equal(t, "media", out.Models[0].Platform)
	require.Equal(t, 0.01, *out.Models[0].Pricing.PerRequestPrice)
	require.Equal(t, "grsai", group.Platform)
	require.Equal(t, "grsai", group.Models[0].Platform)
}

func TestUserMediaQuotaProjectionDoesNotChangeRepositoryRecords(t *testing.T) {
	repo := &fakeQuotaRepoForUserHandler{records: []service.UserPlatformQuotaRecord{{UserID: 42, Platform: "grsai"}}}
	h := &UserHandler{userPlatformQuotaRepo: repo}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/user/platform-quotas", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	h.GetMyPlatformQuotas(c)
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Data struct {
			Quotas []struct {
				Platform string `json:"platform"`
			} `json:"platform_quotas"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Data.Quotas, 1)
	require.Equal(t, "media", body.Data.Quotas[0].Platform)
	require.Equal(t, "grsai", repo.records[0].Platform)
}

func TestUserMediaMonitorV1Projection(t *testing.T) {
	view := &service.UserMonitorView{ID: 1, Provider: "grsai"}
	detail := &service.UserMonitorDetail{ID: 1, Provider: "grsai"}
	require.Equal(t, "media", userMonitorViewToItem(view, false).Provider)
	require.Equal(t, "media", userMonitorDetailToResponse(detail).Provider)
	require.Equal(t, "grsai", view.Provider)
	require.Equal(t, "grsai", detail.Provider)
}

func TestUserMediaMonitorFilterUsesInternalPlatform(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/channel-monitor-v2/models?platform=media", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	h := &ChannelMonitorV2Handler{apiKeyService: &channelMonitorV2GroupAuthorizerStub{groups: []service.Group{{ID: 3}}}}
	models := []string{"media\x00image-model", "openai\x00image-model", "plain-model"}
	filter := service.ChannelMonitorV2Filter{Platforms: []string{"media", "openai"}, Models: models}
	require.True(t, h.scopeFilter(c, &filter, false))
	require.Equal(t, []string{"grsai", "openai"}, filter.Platforms)
	require.Equal(t, []string{"grsai\x00image-model", "openai\x00image-model", "plain-model"}, filter.Models)
	require.Equal(t, []string{"media\x00image-model", "openai\x00image-model", "plain-model"}, models)
	admin := service.ChannelMonitorV2Filter{Platforms: []string{"grsai"}, Models: []string{"grsai\x00image-model"}}
	require.True(t, h.scopeFilter(nil, &admin, true))
	require.Equal(t, []string{"grsai"}, admin.Platforms)
	require.Equal(t, []string{"grsai\x00image-model"}, admin.Models)
}
