package mediacapability

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

func (c *Compiled) Validate(raw []byte) (*NormalizedRequest, []FieldError) {
	if c == nil {
		return nil, []FieldError{fieldError("$", "invalid_schema", "capability is unavailable")}
	}
	value, errs := parseJSON(raw, MaxRequestBytes, MaxDepth)
	if len(errs) > 0 {
		return nil, errs
	}
	values, ok := value.(map[string]any)
	if !ok {
		return nil, []FieldError{fieldError("$", "type", "request must be a JSON object")}
	}
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawFields); err != nil {
		return nil, []FieldError{fieldError("$", "invalid_json", "request must be valid JSON")}
	}
	if len(rawFields["metadata"]) > MaxMetadataBytes {
		return nil, []FieldError{fieldError("metadata", "size_limit", "metadata exceeds size limit")}
	}
	provided := make([]string, 0)
	collectProvided(values, "", &provided)
	sort.Strings(provided)
	applyObjectDefaults(c.schema.Fields, values)
	model, exists := values["model"]
	if !exists {
		errs = append(errs, fieldError("model", "required", "model is required"))
	} else if text, ok := model.(string); !ok || strings.TrimSpace(text) == "" || utf8.RuneCountInString(text) > 200 {
		errs = append(errs, fieldError("model", "type", "model must be a nonempty string of at most 200 characters"))
	}
	for _, key := range sortedKeys(values) {
		if key == "model" {
			continue
		}
		field, exists := c.schema.Fields[key]
		if !exists {
			errs = append(errs, fieldError(key, "unknown_field", "field is not declared for this media capability"))
			continue
		}
		errs = append(errs, validateValue(field, values[key], key)...)
	}
	for _, key := range sortedKeys(c.schema.Fields) {
		if _, exists := values[key]; !exists && c.schema.Fields[key].Required {
			errs = append(errs, fieldError(key, "required", "field is required"))
		}
	}
	for _, key := range []string{"size", "aspect_ratio"} {
		if value, exists := values[key]; exists && value != nil {
			if text, ok := value.(string); ok && !ratioPattern.MatchString(text) {
				errs = append(errs, fieldError(key, "format", "aspect ratio must contain two positive integers separated by a colon"))
			}
		}
	}
	requestedCount := 1
	if n, exists := values["n"]; exists && n != nil {
		number, ok := n.(json.Number)
		if !ok { /* type failure already reported */
		} else if count, err := strconv.Atoi(number.String()); err != nil || count < 1 {
			errs = append(errs, fieldError("n", "range", "n must be a positive integer within the supported range"))
		} else {
			requestedCount = count
		}
	}
	if duration, exists := values["duration"]; exists && duration != nil {
		if number, ok := asRat(duration); ok && number.Sign() <= 0 {
			errs = append(errs, fieldError("duration", "range", "duration must be positive"))
		}
	}
	if mode, exists := getPath(values, "metadata.layer_decomposition"); exists && mode == true {
		errs = append(errs, fieldError("metadata.layer_decomposition", "unsupported_operation", "layer decomposition is not supported"))
	}
	if len(errs) > 0 {
		return nil, errs
	}
	modeID := "default"
	if len(c.schema.Modes) > 0 {
		matches := make([]Mode, 0, 1)
		for _, mode := range c.schema.Modes {
			if conditionsMatch(values, mode.When) {
				matches = append(matches, mode)
			}
		}
		if len(matches) != 1 {
			return nil, []FieldError{fieldError("$", "mode_mismatch", "request must match exactly one input mode")}
		}
		mode := matches[0]
		modeID = mode.ID
		for _, path := range mode.Required {
			if _, exists := getPath(values, path); !exists {
				errs = append(errs, fieldError(path, "required", "field is required by the selected input mode"))
			}
		}
		for _, path := range mode.Forbidden {
			if _, exists := getPath(values, path); exists {
				errs = append(errs, fieldError(path, "forbidden", "field is forbidden by the selected input mode"))
			}
		}
	}
	for _, rule := range c.schema.Rules {
		if conditionsMatch(values, rule.When) {
			errs = append(errs, validateRule(values, rule)...)
		}
	}
	var imageSpec *ImageSpec
	if len(c.schema.ImageSpecs) > 0 {
		size, _ := values["size"].(string)
		resolution, _ := getPath(values, "metadata.resolution")
		for _, spec := range c.schema.ImageSpecs {
			if spec.AspectRatio == size && spec.Resolution == resolution {
				copy := spec
				imageSpec = &copy
				break
			}
		}
		if imageSpec == nil {
			errs = append(errs, fieldError("metadata.resolution", "unsupported_image_spec", "size and resolution do not identify a supported image specification"))
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return &NormalizedRequest{Values: values, Mode: modeID, ProvidedFields: provided, ImageSpec: imageSpec, RequestedCount: requestedCount, mediaType: c.mediaType}, nil
}

func collectProvided(values any, path string, provided *[]string) {
	switch value := values.(type) {
	case map[string]any:
		for _, key := range sortedKeys(value) {
			next := childPath(path, key)
			*provided = append(*provided, next)
			collectProvided(value[key], next, provided)
		}
	case []any:
		for index, item := range value {
			next := fmt.Sprintf("%s[%d]", path, index)
			*provided = append(*provided, next)
			collectProvided(item, next, provided)
		}
	}
}

func applyObjectDefaults(fields map[string]Field, values map[string]any) {
	for _, key := range sortedKeys(fields) {
		field := fields[key]
		value, exists := values[key]
		if !exists && len(field.Default) > 0 {
			value, _ = parseJSON(field.Default, MaxMetadataBytes, MaxDepth)
			exists = true
		}
		if exists {
			values[key] = applyFieldDefaults(field, value)
		}
	}
}

func applyFieldDefaults(field Field, value any) any {
	if object, ok := value.(map[string]any); ok && field.Type == "object" {
		applyObjectDefaults(field.Properties, object)
	}
	if array, ok := value.([]any); ok && field.Type == "array" && field.Items != nil {
		for index, entry := range array {
			array[index] = applyFieldDefaults(*field.Items, entry)
		}
	}
	return value
}

func validateValue(field Field, value any, path string) []FieldError {
	if value == nil {
		if field.Nullable {
			return nil
		}
		return []FieldError{fieldError(path, "type", "null is not allowed")}
	}
	var errs []FieldError
	validType := true
	switch field.Type {
	case "string":
		text, ok := value.(string)
		validType = ok
		if ok {
			length := utf8.RuneCountInString(text)
			if field.MinLength != nil && length < *field.MinLength || field.MaxLength != nil && length > *field.MaxLength {
				errs = append(errs, fieldError(path, "length", "string length is outside the permitted range"))
			}
			if field.Format == "http_url" && !isHTTPURL(text) {
				errs = append(errs, fieldError(path, "format", "expected an absolute HTTP or HTTPS URL without credentials"))
			}
		}
	case "integer", "number":
		number, ok := value.(json.Number)
		validType = ok
		if ok {
			if field.Type == "integer" {
				if _, valid := new(big.Int).SetString(number.String(), 10); !valid {
					validType = false
				}
			}
			rational, valid := asRat(number)
			if !valid {
				validType = false
			} else {
				if field.Minimum != nil {
					minimum, _ := new(big.Rat).SetString(strconv.FormatFloat(*field.Minimum, 'g', -1, 64))
					if minimum != nil && rational.Cmp(minimum) < 0 {
						errs = append(errs, fieldError(path, "range", "number is below the minimum"))
					}
				}
				if field.Maximum != nil {
					maximum, _ := new(big.Rat).SetString(strconv.FormatFloat(*field.Maximum, 'g', -1, 64))
					if maximum != nil && rational.Cmp(maximum) > 0 {
						errs = append(errs, fieldError(path, "range", "number exceeds the maximum"))
					}
				}
			}
		}
	case "boolean":
		_, validType = value.(bool)
	case "array":
		array, ok := value.([]any)
		validType = ok
		if ok {
			if field.MinItems != nil && len(array) < *field.MinItems || field.MaxItems != nil && len(array) > *field.MaxItems {
				errs = append(errs, fieldError(path, "length", "array size is outside the permitted range"))
			}
			if field.Items != nil {
				for index, item := range array {
					errs = append(errs, validateValue(*field.Items, item, fmt.Sprintf("%s[%d]", path, index))...)
				}
			}
		}
	case "object":
		object, ok := value.(map[string]any)
		validType = ok
		if ok {
			for _, key := range sortedKeys(object) {
				child, exists := field.Properties[key]
				if !exists {
					errs = append(errs, fieldError(childPath(path, key), "unknown_field", "field is not declared by this capability"))
					continue
				}
				errs = append(errs, validateValue(child, object[key], childPath(path, key))...)
			}
			for _, key := range sortedKeys(field.Properties) {
				if _, exists := object[key]; !exists && field.Properties[key].Required {
					errs = append(errs, fieldError(childPath(path, key), "required", "field is required"))
				}
			}
		}
	default:
		validType = false
	}
	if !validType {
		return []FieldError{fieldError(path, "type", "value does not match the declared "+field.Type+" type")}
	}
	if len(field.Enum) > 0 {
		matched := false
		for _, allowed := range field.Enum {
			if jsonEqual(value, allowed) {
				matched = true
				break
			}
		}
		if !matched {
			errs = append(errs, fieldError(path, "enum", "value is not one of the allowed values"))
		}
	}
	return errs
}

func isHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (strings.EqualFold(parsed.Scheme, "http") || strings.EqualFold(parsed.Scheme, "https")) && parsed.Host != "" && parsed.Hostname() != "" && parsed.User == nil && parsed.Opaque == ""
}

func asRat(value any) (*big.Rat, bool) {
	number, ok := value.(json.Number)
	if !ok {
		return nil, false
	}
	rational, valid := new(big.Rat).SetString(number.String())
	return rational, valid
}

func jsonEqual(left, right any) bool {
	if leftNumber, ok := asRat(left); ok {
		rightNumber, rightOK := asRat(right)
		return rightOK && leftNumber.Cmp(rightNumber) == 0
	}
	switch value := left.(type) {
	case map[string]any:
		other, ok := right.(map[string]any)
		if !ok || len(value) != len(other) {
			return false
		}
		for key, entry := range value {
			otherEntry, exists := other[key]
			if !exists || !jsonEqual(entry, otherEntry) {
				return false
			}
		}
		return true
	case []any:
		other, ok := right.([]any)
		if !ok || len(value) != len(other) {
			return false
		}
		for index, entry := range value {
			if !jsonEqual(entry, other[index]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(left, right)
	}
}

func getPath(values map[string]any, path string) (any, bool) {
	var value any = values
	for _, part := range strings.Split(path, ".") {
		object, ok := value.(map[string]any)
		if !ok {
			return nil, false
		}
		var exists bool
		value, exists = object[part]
		if !exists {
			return nil, false
		}
	}
	return value, true
}

func isEmpty(value any, exists bool) bool {
	if !exists || value == nil {
		return true
	}
	switch value := value.(type) {
	case string:
		return value == ""
	case []any:
		return len(value) == 0
	case map[string]any:
		return len(value) == 0
	}
	return false
}

func conditionsMatch(values map[string]any, conditions []Condition) bool {
	for _, condition := range conditions {
		value, exists := getPath(values, condition.Path)
		switch condition.Op {
		case "present":
			if !exists {
				return false
			}
		case "absent":
			if exists {
				return false
			}
		case "empty":
			if !isEmpty(value, exists) {
				return false
			}
		case "not_empty":
			if !exists || isEmpty(value, exists) {
				return false
			}
		case "equals":
			if !exists || !jsonEqual(value, condition.Value) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func validateRule(values map[string]any, rule Rule) []FieldError {
	present := make([]bool, len(rule.Paths))
	count := 0
	for index, path := range rule.Paths {
		_, present[index] = getPath(values, path)
		if present[index] {
			count++
		}
	}
	var errs []FieldError
	missing := func(path string) {
		errs = append(errs, fieldError(path, "combination", "required field combination is incomplete"))
	}
	switch rule.Kind {
	case "at_least_one":
		if count == 0 {
			missing(rule.Paths[0])
		}
	case "mutually_exclusive":
		if count > 1 {
			first := true
			for index, path := range rule.Paths {
				if present[index] {
					if first {
						first = false
					} else {
						errs = append(errs, fieldError(path, "combination", "fields are mutually exclusive"))
					}
				}
			}
		}
	case "require_together":
		if count > 0 && count < len(rule.Paths) {
			for index, path := range rule.Paths {
				if !present[index] {
					missing(path)
				}
			}
		}
	case "requires":
		if len(present) > 0 && present[0] {
			for index := 1; index < len(rule.Paths); index++ {
				if !present[index] {
					missing(rule.Paths[index])
				}
			}
		}
	}
	return errs
}
