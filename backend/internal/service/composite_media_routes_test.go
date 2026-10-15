//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
	"github.com/stretchr/testify/require"
)

// AC-003 (route scope): mixed GRS/ToAPIs candidates preserve configured order; account failover is a separate layer.
func TestCompositeMediaResolverReturnsAllOrderedExplicitCandidates(t *testing.T) {
	resolver := NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{
		{ID: 3, GroupID: 7, PublicModel: "shared", MatchType: "exact", Endpoint: "images", TargetPlatform: "grsai", UpstreamModel: "grs-b", Priority: 20, Enabled: true},
		{ID: 1, GroupID: 7, PublicModel: "shared", MatchType: "exact", Endpoint: "images", TargetPlatform: "grsai", UpstreamModel: "grs-a", Priority: 10, Enabled: true},
		{ID: 2, GroupID: 7, PublicModel: "shared", MatchType: "exact", Endpoint: "images", TargetPlatform: "toapis", UpstreamModel: "toa", Priority: 10, Enabled: true},
		{ID: 4, GroupID: 7, PublicModel: "shared", MatchType: "exact", Endpoint: "videos", TargetPlatform: "toapis", Priority: 1, Enabled: true},
		{ID: 5, GroupID: 7, PublicModel: "shared", MatchType: "exact", Endpoint: "any", TargetPlatform: "grsai", Priority: 1, Enabled: true},
		{ID: 6, GroupID: 7, PublicModel: "shared", MatchType: "exact", Endpoint: "images", TargetPlatform: "openai", Priority: 1, Enabled: true},
		{ID: 7, GroupID: 7, PublicModel: "shared", MatchType: "exact", Endpoint: "images", TargetPlatform: "toapis", Priority: 1, Enabled: false},
	}})
	candidates, err := resolver.ResolveMediaCandidates(context.Background(), 7, "shared", "images")
	require.NoError(t, err)
	require.Len(t, candidates, 3)
	require.Equal(t, []int64{1, 2, 3}, []int64{candidates[0].Route.ID, candidates[1].Route.ID, candidates[2].Route.ID})
	require.Equal(t, "grs-a", candidates[0].UpstreamModel)
	has, err := resolver.HasMediaRoutes(context.Background(), 7, "images")
	require.NoError(t, err)
	require.True(t, has)
	candidates, err = resolver.ResolveMediaCandidates(context.Background(), 7, "shared", "videos")
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	_, err = resolver.ResolveMediaCandidates(context.Background(), 7, "shared", "responses")
	require.Error(t, err)
}

// AC-021, AC-022 (candidate scope): keep only the 4K-capable route and return 400 when all eligible capabilities disappear.
func TestCompositeMediaCandidatesFilterCompleteCapabilitiesAndSnapshot(t *testing.T) {
	publicJSON := `{"version":1,"fields":{"prompt":{"type":"string","required":true},"n":{"type":"integer","minimum":1,"maximum":8,"default":1},"metadata":{"type":"object","properties":{"resolution":{"type":"string","enum":["2K","4K"]}}}},"batch":{"native_max_results":1,"allow_split":true}}`
	grsJSON := `{"version":1,"fields":{"prompt":{"type":"string","required":true},"n":{"type":"integer","minimum":1,"maximum":8,"default":1},"metadata":{"type":"object","properties":{"resolution":{"type":"string","enum":["2K"]}}}},"batch":{"native_max_results":1,"allow_split":true}}`
	repo := &mediaCapabilityMemoryRepository{rows: map[int64]*MediaCapability{
		1: {ID: 1, PublicModel: "shared", MediaType: "image", Enabled: true, Revision: 1, SchemaVersion: 1, Schema: json.RawMessage(publicJSON)},
		2: {ID: 2, PublicModel: "grs/shared", MediaType: "image", Enabled: true, Revision: 2, SchemaVersion: 1, Schema: json.RawMessage(grsJSON)},
		3: {ID: 3, PublicModel: "toa/shared", MediaType: "image", Enabled: true, Revision: 3, SchemaVersion: 1, Schema: json.RawMessage(publicJSON)},
	}}
	resolver := NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{
		{ID: 1, GroupID: 7, PublicModel: "shared", MatchType: "exact", Endpoint: "images", TargetPlatform: "grsai", UpstreamModel: "provider-grs", CapabilityModel: "grs/shared", Priority: 1, Enabled: true},
		{ID: 2, GroupID: 7, PublicModel: "shared", MatchType: "exact", Endpoint: "images", TargetPlatform: "toapis", UpstreamModel: "provider-toa", CapabilityModel: "toa/shared", Priority: 2, Enabled: true},
	}})
	compiled, err := mediacapability.Compile("image", []byte(publicJSON))
	require.NoError(t, err)
	normalized, fields := compiled.Validate([]byte(`{"model":"shared","prompt":"p","n":3,"metadata":{"resolution":"4K"}}`))
	require.Empty(t, fields)
	svc := NewCompositeMediaRouteService(resolver, repo)
	candidates, err := svc.MediaCandidates(context.Background(), 7, "shared", "image", normalized)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, "toapis", candidates[0].Route.TargetPlatform)
	require.Equal(t, "provider-toa", candidates[0].Route.UpstreamModel)
	require.Equal(t, int64(3), candidates[0].Capability.ID)
	require.Equal(t, int64(3), candidates[0].Capability.Revision)
	require.True(t, candidates[0].Match.NeedsSplit)
	require.Equal(t, 1, candidates[0].Match.NativeBatchSize)
	require.NotEmpty(t, candidates[0].Capability.SchemaHash)
	repo.rows[3].Enabled = false
	_, err = svc.MediaCandidates(context.Background(), 7, "shared", "image", normalized)
	require.Equal(t, 400, infraerrors.Code(err))
	require.True(t, candidates[0].Match.Supported, "previously accepted snapshot must not mutate")
}

func TestCompositeMediaRouteConfigurationRequiresExplicitMediaEndpoint(t *testing.T) {
	for _, platform := range []string{"grsai", "toapis"} {
		route, err := compositeRouteFromInput(7, CompositeRouteInput{PublicModel: "shared", TargetPlatform: platform, Endpoint: "videos", CapabilityModel: "candidate-profile", Enabled: true})
		require.NoError(t, err)
		require.Equal(t, "videos", route.Endpoint)
		require.Equal(t, "candidate-profile", route.CapabilityModel)
		_, err = compositeRouteFromInput(7, CompositeRouteInput{PublicModel: "shared", TargetPlatform: platform, Endpoint: "any"})
		require.Error(t, err)
		_, err = compositeRouteFromInput(7, CompositeRouteInput{PublicModel: "shared", TargetPlatform: platform, Endpoint: "images", MatchType: "prefix"})
		require.Error(t, err)
	}
}
