package mediacapability

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
)

// Nominal ratios for GRS documented pixel spellings whose rounded dimensions
// do not reduce to the display ratio. This is input syntax, not an output grid:
// providers still check the exact retained width and height before acceptance.
var grsPixelRatios = map[[2]int]string{
	{1672, 941}: "16:9", {941, 1672}: "9:16",
	{1443, 1090}: "4:3", {1090, 1443}: "3:4",
	{1408, 1120}: "5:4", {1120, 1408}: "4:5",
	{1920, 832}: "21:9", {832, 1920}: "9:21",
	{1456, 624}: "21:9", {624, 1456}: "9:21",
	{2912, 1248}: "21:9", {1248, 2912}: "9:21",
	{3840, 1648}: "21:9", {1648, 3840}: "9:21",
	{2048, 1360}: "3:2", {1360, 2048}: "2:3",
	{688, 2048}: "1:3", {2048, 688}: "3:1",
}

// GRS names form the public contract. The persisted worker protocol remains
// canonical so previously accepted requests and in-flight tasks stay readable.
func canonicalParameterPath(path, mediaType string) string {
	parts := strings.SplitN(path, ".", 2)
	name := parts[0]
	switch name {
	case "images":
		name = "image_urls"
	case "aspectRatio":
		name = "size"
		if mediaType == "video" {
			name = "aspect_ratio"
		}
	case "quality":
		name = "metadata.quality"
	case "imageSize", "resolution":
		name = "metadata.resolution"
	case "audios":
		name = "metadata.audio_urls"
	}
	if len(parts) == 2 {
		name += "." + parts[1]
	}
	return name
}

func canonicalSchema(schema *Schema, mediaType string) []FieldError {
	var errs []FieldError
	for _, key := range sortedKeys(schema.Fields) {
		path := canonicalParameterPath(key, mediaType)
		if path == key {
			continue
		}
		field := schema.Fields[key]
		delete(schema.Fields, key)
		parts := strings.Split(path, ".")
		fields := schema.Fields
		if len(parts) == 2 {
			metadata, exists := fields["metadata"]
			if exists && metadata.Type != "object" {
				errs = append(errs, fieldError("fields.metadata", "type", "metadata must be an object"))
				continue
			}
			metadata.Type = "object"
			if metadata.Properties == nil {
				metadata.Properties = make(map[string]Field)
			}
			if len(metadata.Default) == 0 {
				metadata.Default = json.RawMessage(`{}`)
			}
			fields["metadata"] = metadata
			fields = metadata.Properties
		}
		name := parts[len(parts)-1]
		if _, exists := fields[name]; exists {
			errs = append(errs, fieldError("fields."+key, "conflicting_alias", "declare only one spelling of a parameter"))
			continue
		}
		fields[name] = field
	}
	conditions := func(items []Condition) {
		for i := range items {
			items[i].Path = canonicalParameterPath(items[i].Path, mediaType)
		}
	}
	paths := func(items []string) {
		for i := range items {
			items[i] = canonicalParameterPath(items[i], mediaType)
		}
	}
	for i := range schema.Modes {
		conditions(schema.Modes[i].When)
		paths(schema.Modes[i].Required)
		paths(schema.Modes[i].Forbidden)
	}
	for i := range schema.Rules {
		conditions(schema.Rules[i].When)
		paths(schema.Rules[i].Paths)
	}
	return errs
}

func canonicalInput(values map[string]any, mediaType string) []FieldError {
	var errs []FieldError
	for _, key := range sortedKeys(values) {
		path := canonicalParameterPath(key, mediaType)
		if path == key {
			continue
		}
		value := values[key]
		delete(values, key)
		parts := strings.Split(path, ".")
		object := values
		if len(parts) == 2 {
			metadata, exists := values["metadata"]
			if !exists {
				metadata = map[string]any{}
				values["metadata"] = metadata
			}
			var ok bool
			object, ok = metadata.(map[string]any)
			if !ok {
				errs = append(errs, fieldError(key, "conflicting_alias", "parameter cannot be combined with non-object metadata"))
				continue
			}
		}
		name := parts[len(parts)-1]
		if previous, exists := object[name]; exists && !reflect.DeepEqual(previous, value) {
			errs = append(errs, fieldError(key, "conflicting_alias", "parameter spellings have conflicting values"))
			continue
		}
		object[name] = value
	}
	return errs
}

// Pixel dimensions retain the caller's exact geometry, independently of a
// provider's output grid. Both dimensions are bounded before multiplication.
func requestedPixels(values map[string]any, schema Schema) (*ImageSpec, []FieldError) {
	text, ok := values["aspectRatio"].(string)
	if !ok || !strings.Contains(text, "x") {
		return nil, nil
	}
	parts := strings.Split(text, "x")
	if len(parts) != 2 {
		return nil, []FieldError{fieldError("aspectRatio", "format", "pixel dimensions must be WIDTHxHEIGHT")}
	}
	w, e1 := strconv.Atoi(parts[0])
	h, e2 := strconv.Atoi(parts[1])
	if e1 != nil || e2 != nil || w < 1 || h < 1 || w > 8192 || h > 8192 {
		return nil, []FieldError{fieldError("aspectRatio", "range", "pixel dimensions must be between 1 and 8192")}
	}
	a, b := w, h
	for b != 0 {
		a, b = b, a%b
	}
	ratio := strconv.Itoa(w/a) + ":" + strconv.Itoa(h/a)
	if nominal, exists := grsPixelRatios[[2]int{w, h}]; exists {
		ratio = nominal
	}
	resolution := "1K"
	if w*h > 2*1024*1024 {
		resolution = "2K"
	}
	if w*h > 5*1024*1024 {
		resolution = "4K"
	}
	for _, spec := range schema.ImageSpecs {
		if spec.Width == w && spec.Height == h {
			resolution = spec.Resolution
			ratio = spec.AspectRatio
			break
		}
	}
	values["aspectRatio"] = ratio
	if explicit, exists := values["imageSize"]; exists && explicit != resolution {
		return nil, []FieldError{fieldError("imageSize", "conflicting_alias", "resolution conflicts with requested pixel dimensions")}
	}
	if explicit, exists := values["resolution"]; exists && explicit != resolution {
		return nil, []FieldError{fieldError("resolution", "conflicting_alias", "resolution conflicts with requested pixel dimensions")}
	}
	values["imageSize"] = resolution
	return &ImageSpec{AspectRatio: ratio, Resolution: resolution, Width: w, Height: h}, nil
}
