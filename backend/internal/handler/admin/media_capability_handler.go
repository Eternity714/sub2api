package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type MediaCapabilityHandler struct {
	service *service.MediaCapabilityService
}

func NewMediaCapabilityHandler(svc *service.MediaCapabilityService) *MediaCapabilityHandler {
	return &MediaCapabilityHandler{service: svc}
}

var mediaCapabilityWriteFields = []string{"public_model", "media_type", "enabled", "schema", "template_id", "template_version", "notes"}

func mediaCapabilityHandlerError(c *gin.Context, path, code, message string) {
	fields, _ := json.Marshal([]mediacapability.FieldError{{Path: path, Code: code, Message: message}})
	response.ErrorFrom(c, infraerrors.BadRequest("INVALID_MEDIA_CAPABILITY", message).WithMetadata(map[string]string{"fields": string(fields)}))
}

// This decoder is local to the new admin API. Existing platform parsers keep their behavior.
func decodeMediaCapabilityBody(c *gin.Context, dst any, allowed []string) bool {
	const maxAdminBody = 8 << 20 // request_json can contain JSON-escaped request bytes.
	raw, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxAdminBody))
	if err != nil {
		response.Error(c, http.StatusRequestEntityTooLarge, "request body exceeds size limit")
		return false
	}
	if !utf8.Valid(raw) {
		mediaCapabilityHandlerError(c, "$", "invalid_json", "body must be valid UTF-8")
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := readMediaCapabilityAdminJSON(decoder, "$", 0)
	if err != nil {
		mediaCapabilityHandlerError(c, "$", "invalid_json", err.Error())
		return false
	}
	if _, err = decoder.Token(); err != io.EOF {
		mediaCapabilityHandlerError(c, "$", "invalid_json", "expected one JSON object and end of input")
		return false
	}
	object, ok := value.(map[string]any)
	if !ok {
		mediaCapabilityHandlerError(c, "$", "invalid_type", "body must be a JSON object")
		return false
	}
	fields := make(map[string]bool, len(allowed))
	for _, key := range allowed {
		fields[key] = true
	}
	for key, value := range object {
		if !fields[key] {
			mediaCapabilityHandlerError(c, key, "unknown_field", "unknown field "+key)
			return false
		}
		if value == nil && key != "template_id" && key != "template_version" {
			mediaCapabilityHandlerError(c, key, "invalid_type", key+" cannot be null")
			return false
		}
	}
	decoder = json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err = decoder.Decode(dst); err != nil {
		mediaCapabilityHandlerError(c, "$", "invalid_type", "request fields have invalid types")
		return false
	}
	return true
}
func readMediaCapabilityAdminJSON(decoder *json.Decoder, path string, depth int) (any, error) {
	if depth > 24 {
		return nil, fmt.Errorf("JSON nesting exceeds limit")
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("malformed JSON")
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}
	switch delim {
	case '{':
		values := make(map[string]any)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, fmt.Errorf("malformed JSON")
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, fmt.Errorf("object key must be a string")
			}
			if _, duplicate := values[key]; duplicate {
				return nil, fmt.Errorf("duplicate JSON field at %s.%s", path, key)
			}
			value, err := readMediaCapabilityAdminJSON(decoder, path+"."+key, depth+1)
			if err != nil {
				return nil, err
			}
			values[key] = value
		}
		if _, err := decoder.Token(); err != nil {
			return nil, fmt.Errorf("malformed JSON")
		}
		return values, nil
	case '[':
		values := make([]any, 0)
		for decoder.More() {
			value, err := readMediaCapabilityAdminJSON(decoder, path, depth+1)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		if _, err := decoder.Token(); err != nil {
			return nil, fmt.Errorf("malformed JSON")
		}
		return values, nil
	default:
		return nil, fmt.Errorf("malformed JSON")
	}
}
func mediaCapabilityPositiveQuery(c *gin.Context, key string, fallback int) (int, bool) {
	raw := c.Query(key)
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		mediaCapabilityHandlerError(c, key, "invalid_value", key+" must be a positive integer")
		return 0, false
	}
	return value, true
}
func (h *MediaCapabilityHandler) List(c *gin.Context) {
	page, ok := mediaCapabilityPositiveQuery(c, "page", 1)
	if !ok {
		return
	}
	pageSize, ok := mediaCapabilityPositiveQuery(c, "page_size", 20)
	if !ok {
		return
	}
	if pageSize > 100 {
		pageSize = 100
	}
	if page > math.MaxInt/pageSize {
		mediaCapabilityHandlerError(c, "page", "invalid_value", "page is too large")
		return
	}
	filter := service.MediaCapabilityFilter{Page: page, PageSize: pageSize, Search: c.Query("search"), MediaType: c.Query("media_type")}
	if raw := c.Query("enabled"); raw != "" {
		if raw != "true" && raw != "false" {
			mediaCapabilityHandlerError(c, "enabled", "invalid_value", "enabled must be true or false")
			return
		}
		value := raw == "true"
		filter.Enabled = &value
	}
	rows, total, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": rows, "total": total, "page": page, "page_size": pageSize})
}
func (h *MediaCapabilityHandler) Get(c *gin.Context) {
	id, ok := parsePositiveIDParam(c, "id")
	if !ok {
		return
	}
	row, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, row)
}
func (h *MediaCapabilityHandler) Create(c *gin.Context) {
	var input service.MediaCapabilityInput
	if !decodeMediaCapabilityBody(c, &input, mediaCapabilityWriteFields) {
		return
	}
	row, err := h.service.Create(c.Request.Context(), input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, row)
}
func (h *MediaCapabilityHandler) Update(c *gin.Context) {
	id, ok := parsePositiveIDParam(c, "id")
	if !ok {
		return
	}
	var input struct {
		service.MediaCapabilityInput
		Revision int64 `json:"revision"`
	}
	allowed := append(append([]string(nil), mediaCapabilityWriteFields...), "revision")
	if !decodeMediaCapabilityBody(c, &input, allowed) {
		return
	}
	row, err := h.service.Update(c.Request.Context(), id, input.Revision, input.MediaCapabilityInput)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, row)
}
func (h *MediaCapabilityHandler) Delete(c *gin.Context) {
	id, ok := parsePositiveIDParam(c, "id")
	if !ok {
		return
	}
	revision, err := strconv.ParseInt(c.Query("revision"), 10, 64)
	if err != nil || revision < 1 {
		mediaCapabilityHandlerError(c, "revision", "invalid_value", "revision is required and must be a positive integer")
		return
	}
	if err = h.service.Delete(c.Request.Context(), id, revision); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "media capability deleted"})
}
func (h *MediaCapabilityHandler) Templates(c *gin.Context) {
	response.Success(c, gin.H{"items": h.service.Templates()})
}
func (h *MediaCapabilityHandler) Validate(c *gin.Context) {
	var input struct {
		PublicModel string          `json:"public_model"`
		MediaType   string          `json:"media_type"`
		Schema      json.RawMessage `json:"schema"`
		RequestJSON string          `json:"request_json"`
	}
	if !decodeMediaCapabilityBody(c, &input, []string{"public_model", "media_type", "schema", "request_json"}) {
		return
	}
	out, err := h.service.Validate(c.Request.Context(), service.MediaCapabilityInput{PublicModel: input.PublicModel, MediaType: input.MediaType, Schema: input.Schema}, input.RequestJSON)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}
