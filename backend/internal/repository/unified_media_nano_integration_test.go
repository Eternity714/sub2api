//go:build integration

package repository

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUnifiedMediaRepositoryNanoSnapshotRestoreAndFlatSettlement(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "media-api-catalog.json"))
	require.NoError(t, err)
	var catalog struct {
		Models []struct {
			Model  string          `json:"model"`
			Schema json.RawMessage `json:"schema"`
		} `json:"models"`
	}
	require.NoError(t, json.Unmarshal(raw, &catalog))
	for _, provider := range []struct{ platform, public, upstream string }{
		{service.PlatformGrsai, "nano-banana-pro", "nano-banana-pro"},
		{service.PlatformGrsai, "nano-banana-2.1", "nano-banana-2.1"},
		{service.PlatformCentOS, "nano-banana-pro", "gemini-3-pro-image-preview"},
		{service.PlatformCentOS, "nano-banana-2.1", "gemini-3.1-flash-image-preview"},
	} {
		upstreamModel := provider.upstream
		price := map[string]float64{"nano-banana-pro": .2, "nano-banana-2.1": .15}[provider.public]
		var schema json.RawMessage
		for _, entry := range catalog.Models {
			if entry.Model == provider.public {
				schema = entry.Schema
			}
		}
		require.NotEmpty(t, schema)
		for _, inputTier := range []string{"1k", "2k", "4K"} {
			t.Run(provider.platform+"/"+upstreamModel+"/"+inputTier, func(t *testing.T) {
				ctx := context.Background()
				tier := strings.ToUpper(inputTier)
				repo, fixture, accountID := unifiedMediaIntegrationFixture(t)
				_, err := integrationDB.ExecContext(ctx, `UPDATE accounts SET platform=$1 WHERE id=$2`, provider.platform, accountID)
				require.NoError(t, err)
				model := "nano-public-" + uuid.NewString()
				capsRepo := NewMediaCapabilityRepository(integrationDB)
				caps := service.NewMediaCapabilityService(capsRepo)
				capability, err := caps.Create(ctx, service.MediaCapabilityInput{PublicModel: model, MediaType: "image", Enabled: true, Schema: schema})
				require.NoError(t, err)
				t.Cleanup(func() {
					_, _ = integrationDB.ExecContext(ctx, `DELETE FROM media_model_capabilities WHERE id=$1`, capability.ID)
				})
				routesRepo := NewCompositeModelRouteRepository(integrationEntClient)
				route := &service.CompositeModelRoute{GroupID: fixture.GroupID, PublicModel: model, MatchType: "exact", TargetPlatform: provider.platform, UpstreamModel: upstreamModel, Endpoint: "images", Priority: 10, Enabled: true}
				require.NoError(t, routesRepo.Create(ctx, route))
				t.Cleanup(func() {
					_, _ = integrationDB.ExecContext(ctx, `DELETE FROM composite_model_routes WHERE id=$1`, route.ID)
				})
				dimensions := map[string][2]int{"1K": {672, 1584}, "2K": {1344, 3168}, "4K": {2688, 6336}}[tier]
				if provider.platform == service.PlatformCentOS {
					dimensions = [2]int{2816, 1536}
				}
				var pngData bytes.Buffer
				require.NoError(t, png.Encode(&pngData, image.NewGray(image.Rect(0, 0, dimensions[0], dimensions[1]))))
				inline := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngData.Bytes())
				posts := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					posts++
					var body map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					if provider.platform == service.PlatformCentOS {
						require.Equal(t, "/v1/chat/completions", r.URL.Path)
						require.Equal(t, upstreamModel, body["model"])
						require.Equal(t, false, body["stream"])
						require.NotContains(t, body, "imageSize")
						require.NotContains(t, body, "aspectRatio")
						require.NotContains(t, body, "image_config")
						_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "![image](" + inline + ")"}, "finish_reason": "stop"}}})
						return
					}
					require.Equal(t, "/v1/api/generate", r.URL.Path)
					ratio := "9:21"
					if tier == "1K" {
						ratio = "auto"
					}
					require.Equal(t, upstreamModel, body["model"])
					require.Equal(t, ratio, body["aspectRatio"])
					require.Equal(t, tier, body["imageSize"])
					require.Equal(t, "async", body["replyType"])
					require.NotContains(t, body, "quality")
					require.NotContains(t, body, "n")
					if tier == "4K" {
						require.Equal(t, []any{"https://example.com/ref.png"}, body["images"])
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"id": "nano-upstream-task", "status": "succeeded", "results": []any{map[string]any{"url": inline}}})
				}))
				defer server.Close()
				account := service.Account{ID: accountID, Platform: provider.platform, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"api_key": "synthetic-key", "base_url": server.URL}}
				storage := &unifiedMediaIntegrationStorage{}
				uploader := service.NewImageResultUploader(storage, "media/", 0, nil)
				channels := service.NewChannelService(NewChannelRepository(integrationDB), nil, nil, nil, nil)
				channel, err := channels.Create(ctx, &service.CreateChannelInput{Name: "nano-flat-" + uuid.NewString(), GroupIDs: []int64{fixture.GroupID}, ModelPricing: []service.ChannelModelPricing{{Platform: provider.platform, Models: []string{model}, BillingMode: service.BillingModeImage, PerRequestPrice: seriesPricePointer(price)}}})
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, channels.Delete(ctx, channel.ID)) })
				svc := service.NewUnifiedMediaTaskService(repo, caps, service.NewCompositeMediaRouteService(service.NewCompositeRouteResolver(routesRepo), capsRepo), nil, &service.UnifiedMediaPricingResolver{Resolver: service.NewModelPricingResolver(channels, nil)}, nil, unifiedMediaIntegrationPersister{uploader: uploader})
				group := &service.Group{ID: fixture.GroupID, Platform: service.PlatformComposite, RateMultiplier: 1}
				key := &service.APIKey{ID: fixture.APIKeyID, UserID: fixture.UserID, GroupID: &fixture.GroupID, Group: group}
				request := map[string]any{"model": model, "prompt": "a ceramic vase"}
				if tier != "1K" {
					request["imageSize"], request["aspectRatio"] = inputTier, "9:21"
				}
				if tier == "4K" && provider.platform == service.PlatformGrsai {
					request["images"] = []string{"https://example.com/ref.png"}
				}
				body, _ := json.Marshal(request)
				task, err := svc.Create(ctx, service.UnifiedMediaTaskCreateInput{APIKey: key, MediaType: "image", Body: body})
				require.NoError(t, err)
				require.Equal(t, price, task.HeldAmount)
				stored, err := repo.(*unifiedMediaTaskRepository).scan(integrationDB.QueryRowContext(ctx, `SELECT `+unifiedMediaTaskColumns+` FROM unified_media_tasks WHERE id=$1`, task.ID), true)
				require.NoError(t, err)
				require.Nil(t, stored.Payload.Normalized.ImageSpec)
				require.Equal(t, inputTier, stored.Payload.Normalized.Values["metadata"].(map[string]any)["resolution"])
				require.True(t, stored.Payload.Candidates[0].AdapterProfile.ResolutionUppercase)
				require.Equal(t, provider.platform == service.PlatformCentOS, stored.Payload.Candidates[0].AdapterProfile.DefaultImageGeometry)
				require.NoError(t, caps.Delete(ctx, capability.ID, capability.Revision))
				worker := service.NewUnifiedMediaTaskService(repo, nil, nil, &unifiedMediaIntegrationAccounts{account: account}, nil, service.NewUnifiedMediaUpstream(server.Client()), unifiedMediaIntegrationPersister{uploader: uploader})
				require.NoError(t, worker.RunOnce(ctx))
				result, err := worker.GetOwned(ctx, fixture.UserID, fixture.APIKeyID, "image", task.ID)
				require.NoError(t, err)
				require.Equal(t, "completed", result.Status)
				require.Empty(t, result.PublicView().Error)
				require.Len(t, result.PublicView().Result.Data, 1)
				require.Contains(t, result.PublicView().Result.Data[0], "https://s3.example/")
				require.Equal(t, price, service.UnifiedMediaTaskCharge(result))
				var balance, frozen, charged float64
				var usageTier string
				require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance,frozen_balance FROM users WHERE id=$1`, fixture.UserID).Scan(&balance, &frozen))
				require.InDelta(t, 10-price, balance, 1e-9)
				require.Zero(t, frozen)
				require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT actual_cost,image_size FROM usage_logs WHERE request_id=$1`, fmt.Sprintf("media_task:%s:0", task.ID)).Scan(&charged, &usageTier))
				require.Equal(t, price, charged)
				require.Equal(t, tier, usageTier)
				require.NoError(t, worker.RunOnce(ctx))
				require.Equal(t, 1, posts)
				require.Equal(t, 1, storage.saved)
			})
		}
	}
}
