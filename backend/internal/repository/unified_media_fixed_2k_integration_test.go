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
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUnifiedMediaRepositoryFixed2KRestartAndInlineSettlement(t *testing.T) {
	var pngData bytes.Buffer
	require.NoError(t, png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 2048, 2048))))
	inline := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngData.Bytes())
	for _, version := range []string{"2.5", "2.6"} {
		for _, platform := range []string{service.PlatformGrsai, service.PlatformKuaipao} {
			t.Run(version+"/"+platform, func(t *testing.T) {
				ctx := context.Background()
				repo, fixture, accountID := unifiedMediaIntegrationFixture(t)
				_, err := integrationDB.ExecContext(ctx, `UPDATE accounts SET platform=$1 WHERE id=$2`, platform, accountID)
				require.NoError(t, err)
				model := "fixed-2k-" + uuid.NewString()
				upstreamModel := "gpt-image-" + version + "-2k"
				if platform == service.PlatformGrsai {
					upstreamModel = "gpt-image-" + version + "-flare"
				}
				schema := json.RawMessage(`{"version":1,"fixed_image_resolution":"2K","auto_image_aspect_ratio":"1:1","fields":{"prompt":{"type":"string","required":true},"aspectRatio":{"type":"string","enum":["auto","1:1"],"default":"1:1"},"quality":{"type":"string","enum":["auto","low","medium","high"],"default":"auto"},"n":{"type":"integer","minimum":1,"maximum":1,"default":1},"replyType":{"type":"string","enum":["async"],"default":"async"}},"image_specs":[{"aspect_ratio":"1:1","resolution":"2K","width":2048,"height":2048}]}`)
				capRepo := NewMediaCapabilityRepository(integrationDB)
				caps := service.NewMediaCapabilityService(capRepo)
				capability, err := caps.Create(ctx, service.MediaCapabilityInput{PublicModel: model, MediaType: "image", Enabled: true, Schema: schema})
				require.NoError(t, err)
				t.Cleanup(func() {
					_, _ = integrationDB.ExecContext(ctx, `DELETE FROM media_model_capabilities WHERE id=$1`, capability.ID)
				})
				routeRepo := NewCompositeModelRouteRepository(integrationEntClient)
				route := &service.CompositeModelRoute{GroupID: fixture.GroupID, PublicModel: model, MatchType: "exact", TargetPlatform: platform, UpstreamModel: upstreamModel, Endpoint: "images", Priority: 10, Enabled: true}
				require.NoError(t, routeRepo.Create(ctx, route))
				t.Cleanup(func() {
					_, _ = integrationDB.ExecContext(ctx, `DELETE FROM composite_model_routes WHERE id=$1`, route.ID)
				})
				posts := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					posts++
					var body map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					require.Equal(t, upstreamModel, body["model"])
					require.Equal(t, "high", body["quality"])
					require.NotContains(t, body, "imageSize")
					if platform == service.PlatformGrsai {
						require.Equal(t, "/v1/api/generate", r.URL.Path)
						require.Equal(t, "2048x2048", body["aspectRatio"])
						require.Equal(t, "async", body["replyType"])
						_ = json.NewEncoder(w).Encode(map[string]any{"id": "captured-grs-task", "status": "succeeded", "results": []any{map[string]any{"url": inline}}})
					} else {
						require.Equal(t, "/v2/images/generations", r.URL.Path)
						require.Equal(t, "1:1", body["aspect_ratio"])
						require.Equal(t, "2K", body["resolution"])
						_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"url": inline}}})
					}
				}))
				defer server.Close()
				base := server.URL
				if platform == service.PlatformKuaipao {
					base += "/v2"
				}
				account := service.Account{ID: accountID, Platform: platform, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"api_key": "synthetic-key", "base_url": base}}
				storage := &unifiedMediaIntegrationStorage{}
				uploader := service.NewImageResultUploader(storage, "media/", 0, nil)
				channels := service.NewChannelService(NewChannelRepository(integrationDB), nil, nil, nil, nil)
				price := .2
				channel, err := channels.Create(ctx, &service.CreateChannelInput{Name: "fixed-pricing-" + uuid.NewString(), GroupIDs: []int64{fixture.GroupID}, ModelPricing: []service.ChannelModelPricing{{Platform: platform, Models: []string{model}, BillingMode: service.BillingModeImage, Intervals: []service.PricingInterval{{TierLabel: "2K", PerRequestPrice: &price}}}}})
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, channels.Delete(ctx, channel.ID)) })
				pricing := &service.UnifiedMediaPricingResolver{Resolver: service.NewModelPricingResolver(channels, nil)}
				svc := service.NewUnifiedMediaTaskService(repo, caps, service.NewCompositeMediaRouteService(service.NewCompositeRouteResolver(routeRepo), capRepo), nil, pricing, nil, unifiedMediaIntegrationPersister{uploader: uploader})
				group := &service.Group{ID: fixture.GroupID, Platform: service.PlatformComposite, RateMultiplier: 1}
				key := &service.APIKey{ID: fixture.APIKeyID, UserID: fixture.UserID, GroupID: &fixture.GroupID, Group: group}
				task, err := svc.Create(ctx, service.UnifiedMediaTaskCreateInput{APIKey: key, MediaType: "image", Body: []byte(fmt.Sprintf(`{"model":%q,"prompt":"private cup","aspectRatio":"auto","quality":"high"}`, model))})
				require.NoError(t, err)
				require.Equal(t, .2, task.HeldAmount)
				storedTask, err := repo.(*unifiedMediaTaskRepository).scan(integrationDB.QueryRowContext(ctx, `SELECT `+unifiedMediaTaskColumns+` FROM unified_media_tasks WHERE id=$1`, task.ID), true)
				require.NoError(t, err)
				require.Equal(t, "2K", storedTask.Payload.Normalized.FixedImageResolution)
				require.Equal(t, 2048, storedTask.Payload.Normalized.ImageSpec.Width)
				require.NoError(t, caps.Delete(ctx, capability.ID, capability.Revision))
				// A fresh worker has no capability/routing/pricing services and must
				// complete solely from encrypted payload and frozen provider rules.
				worker := service.NewUnifiedMediaTaskService(repo, nil, nil, &unifiedMediaIntegrationAccounts{account: account}, nil, service.NewUnifiedMediaUpstream(server.Client()), unifiedMediaIntegrationPersister{uploader: uploader})
				require.NoError(t, worker.RunOnce(ctx))
				result, err := worker.GetOwned(ctx, fixture.UserID, fixture.APIKeyID, "image", task.ID)
				require.NoError(t, err)
				require.Equal(t, "completed", result.Status)
				require.Empty(t, result.PublicView().Error)
				require.Len(t, result.PublicView().Result.Data, 1)
				require.Contains(t, result.PublicView().Result.Data[0], "https://s3.example/")
				require.Empty(t, result.Units[0].URLs)
				require.Equal(t, .2, service.UnifiedMediaTaskCharge(result))
				var balance, frozen, charged float64
				var tier string
				require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance,frozen_balance FROM users WHERE id=$1`, fixture.UserID).Scan(&balance, &frozen))
				require.Equal(t, 9.8, balance)
				require.Zero(t, frozen)
				require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT actual_cost,image_size FROM usage_logs WHERE request_id=$1`, "media_task:"+task.ID+":0").Scan(&charged, &tier))
				require.Equal(t, .2, charged)
				require.Equal(t, "2K", tier)
				require.NoError(t, worker.RunOnce(ctx))
				require.Equal(t, 1, posts)
				require.Equal(t, 1, storage.saved)
			})
		}
	}
}
