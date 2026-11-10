//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
	"github.com/stretchr/testify/require"
)

func TestImageResolutionPrioritiesOrderOnlyCapableProviders(t *testing.T) {
	model := "gpt-image-2.6-flare"
	schema := seriesPublicSchemas(t)[model]
	caps := &mediaCapabilityMemoryRepository{rows: map[int64]*MediaCapability{1: {ID: 1, PublicModel: model, MediaType: "image", Enabled: true, Schema: schema}}}
	rows := []CompositeModelRoute{
		{ID: 1, GroupID: 22, PublicModel: model, MatchType: "exact", Endpoint: "images", TargetPlatform: PlatformCentOS, UpstreamModel: model, Priority: 10, Enabled: true},
		{ID: 2, GroupID: 22, PublicModel: model, MatchType: "exact", Endpoint: "images", TargetPlatform: PlatformKuaipao, UpstreamModel: model, Priority: 20, ResolutionPriorities: map[string]int{"4K": 30}, Enabled: true},
		{ID: 3, GroupID: 22, PublicModel: model, MatchType: "exact", Endpoint: "images", TargetPlatform: PlatformGrsai, UpstreamModel: model, Priority: 30, ResolutionPriorities: map[string]int{"4K": 20}, Enabled: true},
	}
	svc := NewCompositeMediaRouteService(NewCompositeRouteResolver(compositeRouteRepoStub{routes: rows}), caps)
	compiled, err := mediacapability.Compile("image", schema)
	require.NoError(t, err)
	for _, tier := range []string{"1K", "2K", "4K"} {
		for _, reference := range []bool{false, true} {
			rows[0].UpstreamModel = model
			body := map[string]any{"model": model, "prompt": "cup", "imageSize": tier, "aspectRatio": "1:1", "quality": "auto"}
			if reference {
				body["images"] = []string{"https://example.com/reference.png"}
				// An unverified candidate is skipped; capable backups retain the tier order.
				rows[0].UpstreamModel = "unverified-centos-model"
			}
			raw, _ := json.Marshal(body)
			n, fields := compiled.Validate(raw)
			require.Empty(t, fields)
			candidates, err := svc.MediaCandidates(context.Background(), 22, model, "image", n)
			require.NoError(t, err)
			var actual []string
			for _, candidate := range candidates {
				actual = append(actual, candidate.Route.TargetPlatform)
			}
			want := []string{PlatformKuaipao, PlatformGrsai}
			if tier == "4K" {
				want = []string{PlatformGrsai, PlatformKuaipao}
			}
			if !reference {
				want = append([]string{PlatformCentOS}, want...)
			}
			require.Equal(t, want, actual, fmt.Sprintf("%s references=%t", tier, reference))
		}
	}
}

func TestResolutionPriorityOverridesRequireExactImageRoutes(t *testing.T) {
	base := CompositeRouteInput{PublicModel: "image", TargetPlatform: PlatformCentOS, UpstreamModel: CentOSDefaultImageModel, Endpoint: "images", MatchType: "exact", Priority: 10, Enabled: true}
	for _, priorities := range []map[string]int{{"4K": 20}, {"1K": 1, "2K": 2, "4K": 3}} {
		input := base
		input.ResolutionPriorities = priorities
		route, err := compositeRouteFromInput(22, input)
		require.NoError(t, err)
		require.Equal(t, priorities, route.ResolutionPriorities)
		require.Equal(t, priorities["4K"], route.ResolutionPriority("4k"))
	}
	for _, priorities := range []map[string]int{{"8K": 20}, {"4k": 20}, {"4K": 0}, {"2K": -1}, {"1K": 1000001}} {
		input := base
		input.ResolutionPriorities = priorities
		_, err := compositeRouteFromInput(22, input)
		require.Error(t, err)
	}
	for _, endpoint := range []string{"videos", "responses"} {
		input := base
		input.TargetPlatform = PlatformToAPIs
		input.Endpoint = endpoint
		input.ResolutionPriorities = map[string]int{"4K": 20}
		_, err := compositeRouteFromInput(22, input)
		require.Error(t, err)
	}
	route := CompositeModelRoute{Endpoint: "responses", MatchType: "exact", Priority: 10, ResolutionPriorities: map[string]int{"4K": 20}}
	require.Equal(t, 10, route.ResolutionPriority("4K"))
}
