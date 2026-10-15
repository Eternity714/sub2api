package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
)

func isMediaRouteEndpoint(endpoint string) bool {
	return endpoint == CompositeRouteEndpointImages || endpoint == CompositeRouteEndpointVideos
}
func isMediaRoutePlatform(platform string) bool {
	return platform == PlatformGrsai || platform == PlatformToAPIs
}

// ResolveMediaCandidates only resolves explicit media routes. It deliberately avoids
// the legacy detector, any endpoint, and ambiguous account-owned model inference.
func (r *CompositeRouteResolver) ResolveMediaCandidates(ctx context.Context, groupID int64, model, endpoint string) ([]CompositeRouteDecision, error) {
	if !isMediaRouteEndpoint(endpoint) {
		return nil, mediaCapabilityFieldError("endpoint", "invalid_endpoint", "media endpoint must be images or videos")
	}
	model = strings.TrimSpace(model)
	if groupID <= 0 || model == "" {
		return nil, mediaCapabilityFieldError("model", "invalid_route", "a group and public model are required")
	}
	routes, err := r.mediaRoutes(ctx, groupID, endpoint)
	if err != nil {
		return nil, err
	}
	result := make([]CompositeRouteDecision, 0, len(routes))
	for _, route := range routes {
		if route.PublicModel != model || route.MatchType != CompositeRouteMatchExact {
			continue
		}
		upstream := strings.TrimSpace(route.UpstreamModel)
		if upstream == "" {
			upstream = model
		}
		copy := route
		result = append(result, CompositeRouteDecision{Matched: true, Source: CompositeRouteSourceExplicit, GroupID: groupID, PublicModel: model, TargetPlatform: route.TargetPlatform, UpstreamModel: upstream, Endpoint: endpoint, Route: &copy})
	}
	return result, nil
}
func (r *CompositeRouteResolver) HasMediaRoutes(ctx context.Context, groupID int64, endpoint string) (bool, error) {
	if !isMediaRouteEndpoint(endpoint) {
		return false, mediaCapabilityFieldError("endpoint", "invalid_endpoint", "media endpoint must be images or videos")
	}
	routes, err := r.mediaRoutes(ctx, groupID, endpoint)
	return len(routes) > 0, err
}
func (r *CompositeRouteResolver) mediaRoutes(ctx context.Context, groupID int64, endpoint string) ([]CompositeModelRoute, error) {
	if r == nil || r.repo == nil || groupID <= 0 {
		return nil, nil
	}
	routes, err := r.repo.ListByGroup(ctx, groupID, false)
	if err != nil {
		return nil, err
	}
	result := make([]CompositeModelRoute, 0, len(routes))
	for _, route := range routes {
		if route.Enabled && route.GroupID == groupID && route.Endpoint == endpoint && route.MatchType == CompositeRouteMatchExact && isMediaRoutePlatform(route.TargetPlatform) {
			result = append(result, route)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority < result[j].Priority
		}
		return result[i].ID < result[j].ID
	})
	return result, nil
}

type MediaCandidate struct {
	Route      CompositeRouteDecision         `json:"route"`
	Capability MediaCapabilitySnapshot        `json:"capability_snapshot"`
	Compiled   *mediacapability.Compiled      `json:"-"`
	Match      mediacapability.CandidateMatch `json:"match"`
}

type CompositeMediaRouteService struct {
	resolver     *CompositeRouteResolver
	capabilities MediaCapabilityRepository
}

func NewCompositeMediaRouteService(resolver *CompositeRouteResolver, capabilities MediaCapabilityRepository) *CompositeMediaRouteService {
	return &CompositeMediaRouteService{resolver: resolver, capabilities: capabilities}
}
func (s *CompositeMediaRouteService) MediaCandidates(ctx context.Context, groupID int64, model, mediaType string, normalized *mediacapability.NormalizedRequest) ([]MediaCandidate, error) {
	endpoint := ""
	switch mediaType {
	case "image":
		endpoint = CompositeRouteEndpointImages
	case "video":
		endpoint = CompositeRouteEndpointVideos
	default:
		return nil, mediaCapabilityFieldError("media_type", "invalid_media_type", "media type must be image or video")
	}
	if normalized == nil || normalized.Values["model"] != model {
		return nil, mediaCapabilityFieldError("model", "model_mismatch", "normalized public model must match the requested model")
	}
	if s == nil || s.resolver == nil || s.capabilities == nil {
		return nil, mediaCapabilityFieldError("model", "capability_unavailable", "media routing is unavailable")
	}
	routes, err := s.resolver.ResolveMediaCandidates(ctx, groupID, model, endpoint)
	if err != nil {
		return nil, err
	}
	result := make([]MediaCandidate, 0, len(routes))
	failures := make([]mediacapability.FieldError, 0)
	for _, route := range routes {
		capabilityModel := route.Route.CapabilityModel
		if capabilityModel == "" {
			capabilityModel = model
		}
		row, err := s.capabilities.GetByModel(ctx, capabilityModel, mediaType)
		if errors.Is(err, ErrMediaCapabilityNotFound) || err == nil && (row == nil || !row.Enabled) {
			continue
		}
		if err != nil {
			return nil, err
		}
		compiled, err := mediacapability.Compile(mediaType, row.Schema)
		if err != nil {
			continue
		}
		match := mediacapability.MatchCandidate(normalized, compiled)
		if !match.Supported {
			failures = append(failures, match.Errors...)
			continue
		}
		schema := append(json.RawMessage(nil), row.Schema...)
		digest := sha256.Sum256(schema)
		snapshot := MediaCapabilitySnapshot{ID: row.ID, Revision: row.Revision, PublicModel: row.PublicModel, MediaType: row.MediaType, SchemaVersion: row.SchemaVersion, SchemaHash: hex.EncodeToString(digest[:]), Schema: schema}
		result = append(result, MediaCandidate{Route: route, Capability: snapshot, Compiled: compiled, Match: match})
	}
	if len(result) == 0 {
		if len(failures) == 0 {
			failures = append(failures, mediacapability.FieldError{Path: "model", Code: "unsupported_parameters", Message: "no enabled media route capability supports the requested model and parameters"})
		}
		return nil, mediaCapabilityValidationError(failures)
	}
	return result, nil
}
