package mediacapability

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"unicode/utf8"
)

// parseJSON retains exact numeric lexemes and rejects duplicate object keys at every depth.
func parseJSON(raw []byte, maxBytes, maxDepth int) (any, []FieldError) {
	if len(raw) > maxBytes {
		return nil, []FieldError{fieldError("$", "size_limit", "JSON exceeds size limit")}
	}
	if !utf8.Valid(raw) {
		return nil, []FieldError{fieldError("$", "invalid_json", "JSON must be valid UTF-8")}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, errs := readJSONValue(decoder, "$", 0, maxDepth)
	if len(errs) != 0 {
		return nil, errs
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, []FieldError{fieldError("$", "invalid_json", "expected one JSON value and end of input")}
	}
	return value, nil
}

func readJSONValue(decoder *json.Decoder, path string, depth, maxDepth int) (any, []FieldError) {
	token, err := decoder.Token()
	if err != nil {
		return nil, []FieldError{fieldError(path, "invalid_json", "malformed JSON")}
	}
	delim, isDelim := token.(json.Delim)
	if !isDelim {
		return token, nil
	}
	if depth >= maxDepth {
		return nil, []FieldError{fieldError(path, "depth_limit", "JSON nesting exceeds limit")}
	}
	switch delim {
	case '{':
		object := make(map[string]any)
		for decoder.More() {
			keyToken, err := decoder.Token()
			key, ok := keyToken.(string)
			if err != nil || !ok {
				return nil, []FieldError{fieldError(path, "invalid_json", "object key must be a string")}
			}
			nextPath := childPath(path, key)
			if _, exists := object[key]; exists {
				return nil, []FieldError{fieldError(nextPath, "duplicate_field", "duplicate JSON field")}
			}
			value, errs := readJSONValue(decoder, nextPath, depth+1, maxDepth)
			if len(errs) != 0 {
				return nil, errs
			}
			object[key] = value
		}
		if _, err := decoder.Token(); err != nil {
			return nil, []FieldError{fieldError(path, "invalid_json", "unterminated object")}
		}
		return object, nil
	case '[':
		array := make([]any, 0)
		for decoder.More() {
			value, errs := readJSONValue(decoder, fmt.Sprintf("%s[%d]", path, len(array)), depth+1, maxDepth)
			if len(errs) != 0 {
				return nil, errs
			}
			array = append(array, value)
		}
		if _, err := decoder.Token(); err != nil {
			return nil, []FieldError{fieldError(path, "invalid_json", "unterminated array")}
		}
		return array, nil
	default:
		return nil, []FieldError{fieldError(path, "invalid_json", "unexpected closing delimiter")}
	}
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func cloneJSON(value any) any {
	switch value := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(value))
		for key, child := range value {
			result[key] = cloneJSON(child)
		}
		return result
	case []any:
		result := make([]any, len(value))
		for i, child := range value {
			result[i] = cloneJSON(child)
		}
		return result
	default:
		return value
	}
}
