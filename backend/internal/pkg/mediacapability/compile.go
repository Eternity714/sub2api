package mediacapability

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var fieldNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var ratioPattern = regexp.MustCompile(`^[1-9][0-9]*:[1-9][0-9]*$`)

var imageFields = map[string]bool{"prompt": true, "size": true, "n": true, "image_urls": true, "metadata": true}
var videoFields = map[string]bool{"prompt": true, "duration": true, "aspect_ratio": true, "image_urls": true, "metadata": true}

func Compile(mediaType string, schemaJSON []byte) (*Compiled, error) {
	fail := func(errs ...FieldError) (*Compiled, error) { return nil, &CompileError{Fields: errs} }
	if mediaType != "image" && mediaType != "video" {
		return fail(fieldError("media_type", "invalid_media_type", "media type must be image or video"))
	}
	value, errs := parseJSON(schemaJSON, MaxSchemaBytes, 4*MaxDepth+8)
	if len(errs) != 0 {
		return nil, &CompileError{Fields: errs}
	}
	object, ok := value.(map[string]any)
	if !ok {
		return fail(fieldError("$", "invalid_schema", "schema must be an object"))
	}
	if errs = checkSchemaSyntax(object); len(errs) != 0 {
		return nil, &CompileError{Fields: errs}
	}
	var schema Schema
	decoder := json.NewDecoder(bytes.NewReader(schemaJSON))
	decoder.UseNumber()
	if err := decoder.Decode(&schema); err != nil {
		return fail(fieldError("$", "invalid_schema", "schema fields have invalid types"))
	}
	compiled := &Compiled{mediaType: mediaType, schema: schema}
	if schema.Version != 1 {
		errs = append(errs, fieldError("version", "unsupported_version", "only capability version 1 is supported"))
	}
	if schema.Fields == nil {
		errs = append(errs, fieldError("fields", "required", "fields must be an object"))
	}
	allowed := imageFields
	if mediaType == "video" {
		allowed = videoFields
	}
	count := 0
	for _, name := range sortedKeys(schema.Fields) {
		field := schema.Fields[name]
		if !allowed[name] {
			errs = append(errs, fieldError("fields."+name, "unknown_field", "field is not part of this media interface"))
		}
		errs = append(errs, validateFieldDefinition(field, "fields."+name, 1, &count)...)
		requiredType := map[string]string{"prompt": "string", "size": "string", "n": "integer", "image_urls": "array", "metadata": "object", "aspect_ratio": "string"}[name]
		if requiredType != "" && field.Type != requiredType {
			errs = append(errs, fieldError("fields."+name+".type", "type", "field type conflicts with the unified interface"))
		}
		if name == "duration" && field.Type != "integer" && field.Type != "number" {
			errs = append(errs, fieldError("fields.duration.type", "type", "duration must be numeric"))
		}
		if name == "image_urls" && field.Items != nil && (field.Items.Type != "string" || field.Items.Format != "http_url") {
			errs = append(errs, fieldError("fields.image_urls.items", "type", "image URLs must be http_url strings"))
		}
	}
	if metadata, exists := schema.Fields["metadata"]; exists {
		if _, exists := metadata.Properties["output_size"]; exists {
			errs = append(errs, fieldError("fields.metadata.properties.output_size", "reserved_field", "metadata.output_size is not supported"))
		}
	}
	if count > MaxFields {
		errs = append(errs, fieldError("fields", "size_limit", "schema has too many field nodes"))
	}
	if len(schema.Modes) > MaxModes {
		errs = append(errs, fieldError("modes", "size_limit", "too many modes"))
	}
	modeIDs := make(map[string]bool)
	for index, mode := range schema.Modes {
		path := fmt.Sprintf("modes[%d]", index)
		if mode.ID == "" || len(mode.ID) > 100 || modeIDs[mode.ID] {
			errs = append(errs, fieldError(path+".id", "invalid_mode", "mode ID must be nonempty and unique"))
		}
		modeIDs[mode.ID] = true
		errs = append(errs, compiled.validateConditions(mode.When, path+".when")...)
		errs = append(errs, compiled.validatePaths(mode.Required, path+".required")...)
		errs = append(errs, compiled.validatePaths(mode.Forbidden, path+".forbidden")...)
		for _, required := range mode.Required {
			for _, forbidden := range mode.Forbidden {
				if required == forbidden {
					errs = append(errs, fieldError(path, "invalid_mode", "one path cannot be both required and forbidden"))
				}
			}
		}
	}
	if len(schema.Rules) > MaxRules {
		errs = append(errs, fieldError("rules", "size_limit", "too many rules"))
	}
	for index, rule := range schema.Rules {
		path := fmt.Sprintf("rules[%d]", index)
		switch rule.Kind {
		case "at_least_one", "mutually_exclusive", "require_together", "requires":
		default:
			errs = append(errs, fieldError(path+".kind", "invalid_rule", "unsupported rule kind"))
		}
		minimum := 2
		if rule.Kind == "at_least_one" {
			minimum = 1
		}
		if len(rule.Paths) < minimum || len(rule.Paths) > MaxFields {
			errs = append(errs, fieldError(path+".paths", "invalid_rule", "invalid number of paths"))
		}
		errs = append(errs, compiled.validatePaths(rule.Paths, path+".paths")...)
		errs = append(errs, compiled.validateConditions(rule.When, path+".when")...)
	}
	if _, exists := object["batch"]; !exists {
		compiled.schema.Batch.NativeMaxResults = 1
	} else if schema.Batch.NativeMaxResults < 1 {
		errs = append(errs, fieldError("batch.native_max_results", "range", "native_max_results must be at least 1"))
	}
	if len(schema.ImageSpecs) > MaxFields {
		errs = append(errs, fieldError("image_specs", "size_limit", "too many image specifications"))
	}
	seenSpecs := make(map[string]bool)
	for index, spec := range schema.ImageSpecs {
		path := fmt.Sprintf("image_specs[%d]", index)
		if mediaType != "image" || !ratioPattern.MatchString(spec.AspectRatio) || spec.Resolution == "" || spec.Width < 1 || spec.Height < 1 {
			errs = append(errs, fieldError(path, "invalid_image_spec", "image specification must have a ratio, resolution and positive pixel dimensions"))
		}
		key := spec.AspectRatio + "\x00" + spec.Resolution
		if seenSpecs[key] {
			errs = append(errs, fieldError(path, "duplicate_image_spec", "ratio and resolution must identify one specification"))
		}
		seenSpecs[key] = true
	}
	if len(schema.ImageSpecs) > 0 {
		if _, ok := compiled.fieldAt("size"); !ok {
			errs = append(errs, fieldError("image_specs", "unknown_field", "image specifications require a size field"))
		}
		if field, ok := compiled.fieldAt("metadata.resolution"); !ok || field.Type != "string" {
			errs = append(errs, fieldError("image_specs", "unknown_field", "image specifications require string metadata.resolution"))
		}
	}
	if len(errs) != 0 {
		return nil, &CompileError{Fields: errs}
	}
	return compiled, nil
}

func validateFieldDefinition(field Field, path string, depth int, count *int) []FieldError {
	*count++
	var errs []FieldError
	if depth > MaxDepth {
		return []FieldError{fieldError(path, "depth_limit", "field nesting exceeds limit")}
	}
	switch field.Type {
	case "string", "number", "integer", "boolean", "object", "array":
	default:
		errs = append(errs, fieldError(path+".type", "type", "unsupported field type"))
	}
	if field.Minimum != nil && field.Maximum != nil && *field.Minimum > *field.Maximum {
		errs = append(errs, fieldError(path, "range", "minimum exceeds maximum"))
	}
	if field.Minimum != nil || field.Maximum != nil {
		if field.Type != "number" && field.Type != "integer" {
			errs = append(errs, fieldError(path, "type", "numeric limits require a numeric field"))
		}
	}
	for _, bounds := range []struct {
		name             string
		minimum, maximum *int
		fieldType        string
	}{{"length", field.MinLength, field.MaxLength, "string"}, {"items", field.MinItems, field.MaxItems, "array"}} {
		if bounds.minimum != nil && *bounds.minimum < 0 || bounds.maximum != nil && *bounds.maximum < 0 || bounds.minimum != nil && bounds.maximum != nil && *bounds.minimum > *bounds.maximum {
			errs = append(errs, fieldError(path, "range", "invalid "+bounds.name+" bounds"))
		}
		if (bounds.minimum != nil || bounds.maximum != nil) && field.Type != bounds.fieldType {
			errs = append(errs, fieldError(path, "type", bounds.name+" bounds conflict with type"))
		}
	}
	if field.Format != "" && (field.Format != "http_url" || field.Type != "string") {
		errs = append(errs, fieldError(path+".format", "format", "only string http_url format is supported"))
	}
	if len(field.Properties) > 0 && field.Type != "object" {
		errs = append(errs, fieldError(path+".properties", "type", "properties require object type"))
	}
	for _, name := range sortedKeys(field.Properties) {
		if !fieldNamePattern.MatchString(name) {
			errs = append(errs, fieldError(path+".properties."+name, "invalid_field_name", "property names must support unambiguous dot paths"))
		}
		errs = append(errs, validateFieldDefinition(field.Properties[name], path+".properties."+name, depth+1, count)...)
	}
	if field.Type == "array" && field.Items == nil {
		errs = append(errs, fieldError(path+".items", "required", "arrays must declare items"))
	}
	if field.Items != nil {
		if field.Type != "array" {
			errs = append(errs, fieldError(path+".items", "type", "items require array type"))
		}
		errs = append(errs, validateFieldDefinition(*field.Items, path+".items", depth+1, count)...)
	}
	if len(field.Enum) > MaxFields {
		errs = append(errs, fieldError(path+".enum", "size_limit", "too many enum values"))
	}
	enumField := field
	enumField.Enum = nil
	enumField.Default = nil
	for index, value := range field.Enum {
		if validation := validateValue(enumField, cloneJSON(value), path); len(validation) > 0 {
			errs = append(errs, fieldError(fmt.Sprintf("%s.enum[%d]", path, index), "invalid_enum", "enum value does not match field"))
		}
	}
	if len(field.Default) > 0 {
		value, parseErrs := parseJSON(field.Default, MaxMetadataBytes, MaxDepth)
		if len(parseErrs) > 0 {
			errs = append(errs, fieldError(path+".default", "invalid_default", "default must be valid bounded JSON"))
		} else {
			value = applyFieldDefaults(field, value)
			if validation := validateValue(field, value, path); len(validation) > 0 {
				errs = append(errs, fieldError(path+".default", "invalid_default", "default does not satisfy field constraints"))
			}
		}
	}
	return errs
}

func (c *Compiled) fieldAt(path string) (Field, bool) {
	if path == "model" {
		return Field{Type: "string", Required: true}, true
	}
	parts := strings.Split(path, ".")
	fields := c.schema.Fields
	var field Field
	for index, part := range parts {
		var exists bool
		field, exists = fields[part]
		if !exists {
			return Field{}, false
		}
		if index < len(parts)-1 && field.Type != "object" {
			return Field{}, false
		}
		fields = field.Properties
	}
	return field, true
}

func (c *Compiled) validatePaths(paths []string, path string) []FieldError {
	var errs []FieldError
	seen := make(map[string]bool)
	for index, ref := range paths {
		if _, exists := c.fieldAt(ref); !exists || seen[ref] {
			errs = append(errs, fieldError(fmt.Sprintf("%s[%d]", path, index), "unknown_path", "path must identify a unique declared field"))
		}
		seen[ref] = true
	}
	return errs
}

func (c *Compiled) validateConditions(conditions []Condition, path string) []FieldError {
	if len(conditions) > MaxFields {
		return []FieldError{fieldError(path, "size_limit", "too many conditions")}
	}
	var errs []FieldError
	for index, condition := range conditions {
		where := fmt.Sprintf("%s[%d]", path, index)
		field, exists := c.fieldAt(condition.Path)
		if !exists {
			errs = append(errs, fieldError(where+".path", "unknown_path", "condition references undeclared field"))
		}
		switch condition.Op {
		case "present", "absent", "empty", "not_empty":
		case "equals":
			if exists && len(validateValue(field, cloneJSON(condition.Value), where+".value")) > 0 {
				errs = append(errs, fieldError(where+".value", "type", "comparison value must match its declared field"))
			}
		default:
			errs = append(errs, fieldError(where+".op", "invalid_condition", "unsupported condition operator"))
		}
	}
	return errs
}

func checkSchemaSyntax(object map[string]any) []FieldError {
	var errs []FieldError
	check := func(value any, path string, allowed string) map[string]any {
		obj, ok := value.(map[string]any)
		if !ok {
			errs = append(errs, fieldError(path, "type", "configuration element must be an object"))
			return nil
		}
		for _, key := range sortedKeys(obj) {
			if obj[key] == nil && key != "default" && key != "value" {
				errs = append(errs, fieldError(childPath(path, key), "type", "typed configuration fields cannot be null"))
			}
			if !strings.Contains(" "+allowed+" ", " "+key+" ") {
				errs = append(errs, fieldError(childPath(path, key), "unknown_field", "unknown configuration field"))
			}
		}
		return obj
	}
	check(object, "$", "version fields modes rules image_specs batch")
	var fieldSyntax func(any, string, int)
	fieldSyntax = func(value any, path string, depth int) {
		if depth > MaxDepth {
			errs = append(errs, fieldError(path, "depth_limit", "field nesting exceeds limit"))
			return
		}
		obj := check(value, path, "type required nullable enum minimum maximum min_length max_length min_items max_items default properties items format")
		if properties, exists := obj["properties"]; exists {
			if fields, ok := properties.(map[string]any); ok {
				for _, name := range sortedKeys(fields) {
					fieldSyntax(fields[name], path+".properties."+name, depth+1)
				}
			} else {
				errs = append(errs, fieldError(path+".properties", "type", "properties must be an object"))
			}
		}
		if items, exists := obj["items"]; exists {
			fieldSyntax(items, path+".items", depth+1)
		}
	}
	fields, ok := object["fields"].(map[string]any)
	if !ok {
		errs = append(errs, fieldError("fields", "type", "fields must be an object"))
	}
	for _, name := range sortedKeys(fields) {
		fieldSyntax(fields[name], "fields."+name, 1)
	}
	conditions := func(value any, path string) {
		array, ok := value.([]any)
		if !ok {
			errs = append(errs, fieldError(path, "type", "when must be an array"))
			return
		}
		for index, entry := range array {
			where := fmt.Sprintf("%s[%d]", path, index)
			obj := check(entry, where, "path op value")
			if obj["op"] == "equals" {
				if _, exists := obj["value"]; !exists {
					errs = append(errs, fieldError(where+".value", "required", "equals requires an explicit value"))
				}
			}
		}
	}
	for _, section := range []struct{ name, keys string }{{"modes", "id when required forbidden"}, {"rules", "kind paths when"}, {"image_specs", "aspect_ratio resolution width height"}} {
		if value, exists := object[section.name]; exists {
			array, ok := value.([]any)
			if !ok {
				errs = append(errs, fieldError(section.name, "type", "configuration section must be an array"))
				continue
			}
			for index, entry := range array {
				path := fmt.Sprintf("%s[%d]", section.name, index)
				obj := check(entry, path, section.keys)
				if when, exists := obj["when"]; exists {
					conditions(when, path+".when")
				}
			}
		}
	}
	if value, exists := object["batch"]; exists {
		check(value, "batch", "native_max_results allow_split semantically_linked")
	}
	return errs
}
