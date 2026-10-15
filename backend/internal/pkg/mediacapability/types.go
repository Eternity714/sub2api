// Package mediacapability compiles and validates the bounded media capability v1 contract.
// It performs no network, persistence, account selection, or billing operations.
package mediacapability

import "encoding/json"

const (
	MaxRequestBytes  = 1 << 20
	MaxMetadataBytes = 256 << 10
	MaxSchemaBytes   = 512 << 10
	MaxDepth         = 8
	MaxFields        = 256
	MaxModes         = 32
	MaxRules         = 128
)

type Schema struct {
	Version    int              `json:"version"`
	Fields     map[string]Field `json:"fields"`
	Modes      []Mode           `json:"modes,omitempty"`
	Rules      []Rule           `json:"rules,omitempty"`
	ImageSpecs []ImageSpec      `json:"image_specs,omitempty"`
	Batch      Batch            `json:"batch,omitempty"`
}

type Field struct {
	Type       string           `json:"type"`
	Required   bool             `json:"required,omitempty"`
	Nullable   bool             `json:"nullable,omitempty"`
	Enum       []any            `json:"enum,omitempty"`
	Minimum    *float64         `json:"minimum,omitempty"`
	Maximum    *float64         `json:"maximum,omitempty"`
	MinLength  *int             `json:"min_length,omitempty"`
	MaxLength  *int             `json:"max_length,omitempty"`
	MinItems   *int             `json:"min_items,omitempty"`
	MaxItems   *int             `json:"max_items,omitempty"`
	Default    json.RawMessage  `json:"default,omitempty"`
	Properties map[string]Field `json:"properties,omitempty"`
	Items      *Field           `json:"items,omitempty"`
	Format     string           `json:"format,omitempty"`
}

type Mode struct {
	ID        string      `json:"id"`
	When      []Condition `json:"when,omitempty"`
	Required  []string    `json:"required,omitempty"`
	Forbidden []string    `json:"forbidden,omitempty"`
}

type Condition struct {
	Path  string `json:"path"`
	Op    string `json:"op"`
	Value any    `json:"value,omitempty"`
}

type Rule struct {
	Kind  string      `json:"kind"`
	Paths []string    `json:"paths"`
	When  []Condition `json:"when,omitempty"`
}

type ImageSpec struct {
	AspectRatio string `json:"aspect_ratio"`
	Resolution  string `json:"resolution"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

type Batch struct {
	NativeMaxResults   int  `json:"native_max_results"`
	AllowSplit         bool `json:"allow_split"`
	SemanticallyLinked bool `json:"semantically_linked"`
}

type FieldError struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type CompileError struct {
	Fields []FieldError `json:"fields"`
}

func (e *CompileError) Error() string {
	if len(e.Fields) == 0 {
		return "invalid media capability schema"
	}
	return "invalid media capability schema: " + e.Fields[0].Path + ": " + e.Fields[0].Message
}

type NormalizedRequest struct {
	Values         map[string]any `json:"values"`
	Mode           string         `json:"mode"`
	ProvidedFields []string       `json:"provided_fields"`
	ImageSpec      *ImageSpec     `json:"image_spec,omitempty"`
	RequestedCount int            `json:"requested_count"`
	mediaType      string
}

type CandidateMatch struct {
	Supported       bool         `json:"supported"`
	NeedsSplit      bool         `json:"needs_split"`
	NativeBatchSize int          `json:"native_batch_size"`
	Errors          []FieldError `json:"errors"`
}

type Compiled struct {
	mediaType string
	schema    Schema
}

func fieldError(path, code, message string) FieldError {
	return FieldError{Path: path, Code: code, Message: message}
}

func childPath(parent, key string) string {
	if parent == "" || parent == "$" {
		return key
	}
	return parent + "." + key
}
