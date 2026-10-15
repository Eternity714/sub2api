package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
)

var (
	ErrMediaCapabilityNotFound = infraerrors.NotFound("MEDIA_CAPABILITY_NOT_FOUND", "media capability not found")
	ErrMediaCapabilityExists   = infraerrors.Conflict("MEDIA_CAPABILITY_EXISTS", "this model and media type already have a capability")
	ErrMediaCapabilityConflict = infraerrors.Conflict("MEDIA_CAPABILITY_REVISION_CONFLICT", "media capability changed; reload before saving")
)

type MediaCapability struct {
	ID              int64           `json:"id"`
	PublicModel     string          `json:"public_model"`
	MediaType       string          `json:"media_type"`
	SchemaVersion   int             `json:"schema_version"`
	Revision        int64           `json:"revision"`
	Enabled         bool            `json:"enabled"`
	Schema          json.RawMessage `json:"schema"`
	TemplateID      string          `json:"template_id,omitempty"`
	TemplateVersion *int            `json:"template_version,omitempty"`
	Notes           string          `json:"notes,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type MediaCapabilityInput struct {
	PublicModel     string          `json:"public_model"`
	MediaType       string          `json:"media_type"`
	Enabled         bool            `json:"enabled"`
	Schema          json.RawMessage `json:"schema"`
	TemplateID      string          `json:"template_id,omitempty"`
	TemplateVersion *int            `json:"template_version,omitempty"`
	Notes           string          `json:"notes,omitempty"`
}

type MediaCapabilityFilter struct {
	Search    string
	MediaType string
	Enabled   *bool
	Page      int
	PageSize  int
}

type MediaCapabilityRepository interface {
	List(context.Context, MediaCapabilityFilter) ([]MediaCapability, int64, error)
	Get(context.Context, int64) (*MediaCapability, error)
	GetByModel(context.Context, string, string) (*MediaCapability, error)
	Create(context.Context, *MediaCapability) error
	Update(context.Context, *MediaCapability, int64) error
	Delete(context.Context, int64, int64) error
}

type MediaCapabilityValidation struct {
	Valid      bool                               `json:"valid"`
	Errors     []mediacapability.FieldError       `json:"errors"`
	Normalized *mediacapability.NormalizedRequest `json:"normalized,omitempty"`
}

type MediaCapabilitySnapshot struct {
	ID            int64           `json:"id"`
	Revision      int64           `json:"revision"`
	PublicModel   string          `json:"public_model"`
	MediaType     string          `json:"media_type"`
	SchemaVersion int             `json:"schema_version"`
	SchemaHash    string          `json:"schema_hash"`
	Schema        json.RawMessage `json:"schema"`
}

type ValidatedMediaRequest struct {
	Normalized *mediacapability.NormalizedRequest `json:"normalized"`
	Capability MediaCapabilitySnapshot            `json:"capability_snapshot"`
}

type MediaCapabilityTemplate struct {
	ID            string          `json:"id"`
	Version       int             `json:"version"`
	Name          string          `json:"name"`
	MediaType     string          `json:"media_type"`
	Schema        json.RawMessage `json:"schema"`
	Platform      string          `json:"platform,omitempty"`
	UpstreamModel string          `json:"upstream_model,omitempty"`
	PublicModel   string          `json:"public_model,omitempty"`
	Source        string          `json:"source,omitempty"`
	Evidence      string          `json:"evidence,omitempty"`
	Notes         string          `json:"notes,omitempty"`
	Example       json.RawMessage `json:"example,omitempty"`
	Enabled       bool            `json:"enabled"`
}

type MediaCapabilityService struct{ repo MediaCapabilityRepository }

func NewMediaCapabilityService(repo MediaCapabilityRepository) *MediaCapabilityService {
	return &MediaCapabilityService{repo: repo}
}

func mediaCapabilityFieldError(path, code, message string) error {
	return mediaCapabilityValidationError([]mediacapability.FieldError{{Path: path, Code: code, Message: message}})
}
func mediaCapabilityValidationError(fields []mediacapability.FieldError) error {
	encoded, _ := json.Marshal(fields)
	return infraerrors.BadRequest("INVALID_MEDIA_CAPABILITY", "invalid media capability configuration or request").WithMetadata(map[string]string{"fields": string(encoded)})
}
func mediaCapabilityCompile(mediaType string, raw []byte) (*mediacapability.Compiled, error) {
	compiled, err := mediacapability.Compile(mediaType, raw)
	if err != nil {
		var ce *mediacapability.CompileError
		if errors.As(err, &ce) {
			return nil, mediaCapabilityValidationError(ce.Fields)
		}
		return nil, mediaCapabilityFieldError("schema", "invalid_schema", err.Error())
	}
	return compiled, nil
}
func normalizeMediaCapabilityInput(input MediaCapabilityInput) (*MediaCapability, error) {
	input.PublicModel = strings.TrimSpace(input.PublicModel)
	if input.PublicModel == "" || !utf8.ValidString(input.PublicModel) || utf8.RuneCountInString(input.PublicModel) > 200 {
		return nil, mediaCapabilityFieldError("public_model", "invalid_value", "public_model must contain 1 to 200 characters")
	}
	if input.MediaType != "image" && input.MediaType != "video" {
		return nil, mediaCapabilityFieldError("media_type", "invalid_value", "media_type must be image or video")
	}
	if utf8.RuneCountInString(input.Notes) > 2000 {
		return nil, mediaCapabilityFieldError("notes", "max_length", "notes must be at most 2000 characters")
	}
	if len(input.TemplateID) > 100 {
		return nil, mediaCapabilityFieldError("template_id", "max_length", "template_id must be at most 100 bytes")
	}
	if (input.TemplateID == "") != (input.TemplateVersion == nil) {
		return nil, mediaCapabilityFieldError("template_version", "required_together", "template_id and template_version must be supplied together")
	}
	if input.TemplateVersion != nil && *input.TemplateVersion < 1 {
		return nil, mediaCapabilityFieldError("template_version", "invalid_value", "template_version must be positive")
	}
	if _, err := mediaCapabilityCompile(input.MediaType, input.Schema); err != nil {
		return nil, err
	}
	// Canonical JSON keeps persisted schema digests independent of whitespace/key order.
	var value any
	decoder := json.NewDecoder(strings.NewReader(string(input.Schema)))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, mediaCapabilityFieldError("schema", "invalid_schema", "schema must be JSON")
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var version *int
	if input.TemplateVersion != nil {
		cp := *input.TemplateVersion
		version = &cp
	}
	return &MediaCapability{PublicModel: input.PublicModel, MediaType: input.MediaType, SchemaVersion: 1, Enabled: input.Enabled, Schema: canonical, TemplateID: input.TemplateID, TemplateVersion: version, Notes: input.Notes}, nil
}
func (s *MediaCapabilityService) Create(ctx context.Context, input MediaCapabilityInput) (*MediaCapability, error) {
	row, err := normalizeMediaCapabilityInput(input)
	if err != nil {
		return nil, err
	}
	if err = s.repo.Create(ctx, row); err != nil {
		return nil, err
	}
	return row, nil
}
func (s *MediaCapabilityService) Update(ctx context.Context, id, revision int64, input MediaCapabilityInput) (*MediaCapability, error) {
	if id <= 0 {
		return nil, ErrMediaCapabilityNotFound
	}
	if revision < 1 {
		return nil, mediaCapabilityFieldError("revision", "invalid_value", "revision must be positive")
	}
	row, err := normalizeMediaCapabilityInput(input)
	if err != nil {
		return nil, err
	}
	row.ID = id
	if err = s.repo.Update(ctx, row, revision); err != nil {
		return nil, err
	}
	return row, nil
}
func (s *MediaCapabilityService) Delete(ctx context.Context, id, revision int64) error {
	if id <= 0 {
		return ErrMediaCapabilityNotFound
	}
	if revision < 1 {
		return mediaCapabilityFieldError("revision", "invalid_value", "revision must be positive")
	}
	return s.repo.Delete(ctx, id, revision)
}
func (s *MediaCapabilityService) Get(ctx context.Context, id int64) (*MediaCapability, error) {
	if id <= 0 {
		return nil, ErrMediaCapabilityNotFound
	}
	return s.repo.Get(ctx, id)
}
func (s *MediaCapabilityService) List(ctx context.Context, filter MediaCapabilityFilter) ([]MediaCapability, int64, error) {
	filter.Search = strings.TrimSpace(filter.Search)
	if len(filter.Search) > 200 {
		return nil, 0, mediaCapabilityFieldError("search", "max_length", "search must be at most 200 bytes")
	}
	if filter.MediaType != "" && filter.MediaType != "image" && filter.MediaType != "video" {
		return nil, 0, mediaCapabilityFieldError("media_type", "invalid_value", "media_type must be image or video")
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	return s.repo.List(ctx, filter)
}
func (s *MediaCapabilityService) Validate(ctx context.Context, input MediaCapabilityInput, requestJSON string) (*MediaCapabilityValidation, error) {
	row, err := normalizeMediaCapabilityInput(input)
	if err != nil {
		return nil, err
	}
	compiled, err := mediaCapabilityCompile(row.MediaType, row.Schema)
	if err != nil {
		return nil, err
	}
	normalized, fields := compiled.Validate([]byte(requestJSON))
	if len(fields) == 0 && normalized.Values["model"] != row.PublicModel {
		fields = []mediacapability.FieldError{{Path: "model", Code: "model_mismatch", Message: "model must match public_model"}}
	}
	if len(fields) > 0 {
		return &MediaCapabilityValidation{Valid: false, Errors: fields}, nil
	}
	return &MediaCapabilityValidation{Valid: true, Errors: []mediacapability.FieldError{}, Normalized: normalized}, nil
}
func (s *MediaCapabilityService) ValidateActive(ctx context.Context, publicModel, mediaType string, raw []byte) (*ValidatedMediaRequest, error) {
	if mediaType != "image" && mediaType != "video" {
		return nil, mediaCapabilityFieldError("media_type", "invalid_value", "media_type must be image or video")
	}
	row, err := s.repo.GetByModel(ctx, publicModel, mediaType)
	if errors.Is(err, ErrMediaCapabilityNotFound) || err == nil && !row.Enabled {
		return nil, mediaCapabilityFieldError("model", "capability_unavailable", "no enabled capability for this model and media type")
	}
	if err != nil {
		return nil, err
	}
	compiled, err := mediaCapabilityCompile(row.MediaType, row.Schema)
	if err != nil {
		return nil, err
	}
	normalized, fields := compiled.Validate(raw)
	if len(fields) > 0 {
		return nil, mediaCapabilityValidationError(fields)
	}
	if normalized.Values["model"] != row.PublicModel {
		return nil, mediaCapabilityFieldError("model", "model_mismatch", "model must match public_model")
	}
	schema := append(json.RawMessage(nil), row.Schema...)
	digest := sha256.Sum256(schema)
	return &ValidatedMediaRequest{Normalized: normalized, Capability: MediaCapabilitySnapshot{ID: row.ID, Revision: row.Revision, PublicModel: row.PublicModel, MediaType: row.MediaType, SchemaVersion: row.SchemaVersion, SchemaHash: hex.EncodeToString(digest[:]), Schema: schema}}, nil
}

func (s *MediaCapabilityService) Templates() []MediaCapabilityTemplate {
	templates := []MediaCapabilityTemplate{
		{ID: "basic-image-v1", Version: 1, Name: "图片生成基础模板", MediaType: "image", Schema: json.RawMessage(`{"version":1,"fields":{"prompt":{"type":"string","required":true,"min_length":1,"max_length":32000},"size":{"type":"string","enum":["1:1","16:9","9:16"],"default":"1:1"},"n":{"type":"integer","minimum":1,"maximum":8,"default":1},"image_urls":{"type":"array","max_items":8,"items":{"type":"string","format":"http_url"}},"metadata":{"type":"object","properties":{"resolution":{"type":"string","enum":["1K","2K","4K"]}}}},"batch":{"native_max_results":1,"allow_split":true}}`)},
		{ID: "basic-video-v1", Version: 1, Name: "视频生成基础模板", MediaType: "video", Schema: json.RawMessage(`{"version":1,"fields":{"prompt":{"type":"string","required":true,"min_length":1,"max_length":32000},"duration":{"type":"number","minimum":1,"maximum":60},"aspect_ratio":{"type":"string","enum":["16:9","9:16","1:1"]},"image_urls":{"type":"array","max_items":2,"items":{"type":"string","format":"http_url"}},"metadata":{"type":"object","properties":{}}}}`)},
	}
	for _, entry := range MediaCapabilityCatalog() {
		if len(entry.Schema) == 0 {
			continue
		}
		templates = append(templates, MediaCapabilityTemplate{ID: entry.Platform + "-" + entry.MediaType + "-" + entry.UpstreamModel + "-v1", Version: 1, Name: entry.Platform + " / " + entry.UpstreamModel, MediaType: entry.MediaType, Schema: entry.Schema, Platform: entry.Platform, UpstreamModel: entry.UpstreamModel, PublicModel: entry.Platform + "/" + entry.UpstreamModel, Source: entry.Source, Evidence: entry.Evidence, Notes: entry.Notes, Example: entry.Example, Enabled: false})
	}
	return templates
}
