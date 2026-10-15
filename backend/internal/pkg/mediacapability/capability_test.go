//go:build unit

package mediacapability

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

const imageSchema = `{"version":1,"fields":{"prompt":{"type":"string"},"size":{"type":"string","enum":["1:1","16:9"],"default":"1:1"},"n":{"type":"integer","minimum":1,"maximum":8,"default":1},"image_urls":{"type":"array","max_items":2,"items":{"type":"string","format":"http_url"}},"metadata":{"type":"object","default":{},"properties":{"resolution":{"type":"string","enum":["1K","2K"],"default":"1K"},"seed":{"type":"integer","minimum":0},"options":{"type":"object","properties":{"quality":{"type":"string","enum":["high","low"]}}}}}},"modes":[{"id":"text","when":[{"path":"image_urls","op":"empty"}],"required":["prompt"]},{"id":"reference","when":[{"path":"image_urls","op":"not_empty"}]}],"image_specs":[{"aspect_ratio":"1:1","resolution":"1K","width":1024,"height":1024},{"aspect_ratio":"16:9","resolution":"1K","width":1536,"height":864},{"aspect_ratio":"1:1","resolution":"2K","width":2048,"height":2048}],"batch":{"native_max_results":1,"allow_split":true}}`

func compileTest(t *testing.T, mediaType, raw string) *Compiled {
	t.Helper()
	compiled, err := Compile(mediaType, []byte(raw))
	if err != nil {
		t.Fatalf("compile valid fixture: %v", err)
	}
	return compiled
}

func requirePath(t *testing.T, errs []FieldError, path string) {
	t.Helper()
	for _, err := range errs {
		if err.Path == path {
			return
		}
	}
	t.Fatalf("expected error at %q, got %+v", path, errs)
}

func TestValidateNormalizesDeclaredDefaultsAndPreservesExplicitFields(t *testing.T) {
	compiled := compileTest(t, "image", imageSchema)
	got, errs := compiled.Validate([]byte(`{"model":"public-image","prompt":"portrait","metadata":{"seed":9007199254740993}}`))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}
	if got.Mode != "text" || got.RequestedCount != 1 {
		t.Fatalf("unexpected normalization: %+v", got)
	}
	if got.Values["size"] != "1:1" || got.ImageSpec == nil || got.ImageSpec.Width != 1024 {
		t.Fatalf("defaults/spec missing: %+v", got)
	}
	metadata := got.Values["metadata"].(map[string]any)
	if metadata["seed"] != json.Number("9007199254740993") || metadata["resolution"] != "1K" {
		t.Fatalf("numeric precision/default lost: %#v", metadata)
	}
	if strings.Join(got.ProvidedFields, ",") != "metadata,metadata.seed,model,prompt" {
		t.Fatalf("explicit field provenance wrong: %v", got.ProvidedFields)
	}
}

// AC-019, AC-020 (validator scope): reject foreign top-level fields and invalid nested metadata with field paths.
func TestValidateStrictJSONAndFieldRules(t *testing.T) {
	compiled := compileTest(t, "image", imageSchema)
	cases := []struct{ name, body, path string }{
		{"video duration on image", `{"model":"x","prompt":"p","duration":8}`, "duration"},
		{"case sensitive", `{"model":"x","Prompt":"p"}`, "Prompt"},
		{"unknown metadata", `{"model":"x","prompt":"p","metadata":{"unknown":true}}`, "metadata.unknown"},
		{"unknown nested", `{"model":"x","prompt":"p","metadata":{"options":{"secret":true}}}`, "metadata.options.secret"},
		{"metadata object", `{"model":"x","prompt":"p","metadata":[]}`, "metadata"},
		{"no coercion", `{"model":"x","prompt":"p","n":"3"}`, "n"},
		{"integer required", `{"model":"x","prompt":"p","n":1.5}`, "n"},
		{"range", `{"model":"x","prompt":"p","n":9}`, "n"},
		{"null disallowed", `{"model":"x","prompt":"p","n":null}`, "n"},
		{"enum", `{"model":"x","prompt":"p","size":"4:3"}`, "size"},
		{"reference count", `{"model":"x","image_urls":["https://a/x","https://a/y","https://a/z"]}`, "image_urls"},
		{"url scheme", `{"model":"x","image_urls":["file:///a"]}`, "image_urls[0]"},
		{"url credentials", `{"model":"x","image_urls":["https://user:secret@example.com/a"]}`, "image_urls[0]"},
		{"no prompt for text", `{"model":"x"}`, "prompt"},
		{"model required", `{"prompt":"p"}`, "model"},
		{"model empty", `{"model":"","prompt":"p"}`, "model"},
		{"duplicate root key", `{"model":"x","model":"y","prompt":"p"}`, "model"},
		{"duplicate nested key", `{"model":"x","prompt":"p","metadata":{"seed":1,"seed":2}}`, "metadata.seed"},
		{"extra JSON", `{"model":"x","prompt":"p"} {}`, "$"},
		{"JSON array", `[]`, "$"},
		{"pixel size escape", `{"model":"x","prompt":"p","metadata":{"output_size":"1024x1024"}}`, "metadata.output_size"},
		{"unmapped specification", `{"model":"x","prompt":"p","size":"16:9","metadata":{"resolution":"2K"}}`, "metadata.resolution"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, errs := compiled.Validate([]byte(tc.body))
			if got != nil {
				t.Fatalf("invalid input returned normalized request: %+v", got)
			}
			requirePath(t, errs, tc.path)
		})
	}
}

// AC-026 (validator scope): reference mode accepts an omitted prompt without inventing one.
func TestValidatePromptDependsOnInputModeAndVideoDoesNotAcceptImageFields(t *testing.T) {
	image := compileTest(t, "image", imageSchema)
	got, errs := image.Validate([]byte(`{"model":"public-image","image_urls":["https://example.com/ref.png"]}`))
	if len(errs) != 0 || got.Mode != "reference" {
		t.Fatalf("reference mode rejected: %+v %+v", got, errs)
	}
	if _, exists := got.Values["prompt"]; exists {
		t.Fatal("missing prompt was fabricated")
	}
	video := compileTest(t, "video", `{"version":1,"fields":{"prompt":{"type":"string"},"duration":{"type":"number","minimum":1,"maximum":20},"aspect_ratio":{"type":"string"}}}`)
	_, errs = video.Validate([]byte(`{"model":"public-video","size":"1:1","n":2}`))
	requirePath(t, errs, "size")
	requirePath(t, errs, "n")
}

func TestCompileRejectsInvalidOrUnsafeSchema(t *testing.T) {
	cases := []struct{ name, media, raw string }{
		{"unsupported media", "audio", `{"version":1,"fields":{}}`},
		{"version", "image", `{"version":2,"fields":{}}`},
		{"model reserved", "image", `{"version":1,"fields":{"model":{"type":"string"}}}`},
		{"wrong endpoint field", "image", `{"version":1,"fields":{"duration":{"type":"number"}}}`},
		{"unknown schema keyword", "image", `{"version":1,"fields":{},"additionalProperties":true}`},
		{"duplicate schema key", "image", `{"version":1,"version":1,"fields":{}}`},
		{"unknown type", "image", `{"version":1,"fields":{"prompt":{"type":"secret"}}}`},
		{"bad default", "image", `{"version":1,"fields":{"n":{"type":"integer","minimum":1,"default":0}}}`},
		{"unresolved reference", "image", `{"version":1,"fields":{},"rules":[{"kind":"requires","paths":["prompt","metadata.mask"]}]}`},
		{"unknown condition", "image", `{"version":1,"fields":{"prompt":{"type":"string"}},"modes":[{"id":"x","when":[{"path":"prompt","op":"execute"}]}]}`},
		{"unknown format", "image", `{"version":1,"fields":{"prompt":{"type":"string","format":"shell"}}}`},
		{"pixel escape declared", "image", `{"version":1,"fields":{"metadata":{"type":"object","properties":{"output_size":{"type":"string"}}}}}`},
		{"impossible range", "image", `{"version":1,"fields":{"n":{"type":"integer","minimum":5,"maximum":1}}}`},
		{"duplicate spec", "image", `{"version":1,"fields":{"size":{"type":"string"},"metadata":{"type":"object","default":{},"properties":{"resolution":{"type":"string"}}}},"image_specs":[{"aspect_ratio":"1:1","resolution":"1K","width":1024,"height":1024},{"aspect_ratio":"1:1","resolution":"1K","width":2048,"height":2048}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Compile(tc.media, []byte(tc.raw)); err == nil {
				t.Fatal("invalid schema accepted")
			}
		})
	}
}

func TestValidateModesRulesAndDefaultsAreRechecked(t *testing.T) {
	raw := `{"version":1,"fields":{"prompt":{"type":"string"},"metadata":{"type":"object","properties":{"mode":{"type":"string","default":"reference"},"start":{"type":"string","format":"http_url"},"end":{"type":"string","format":"http_url"},"a":{"type":"boolean"},"b":{"type":"boolean"},"c":{"type":"boolean"},"layer_decomposition":{"type":"boolean"}}}},"modes":[{"id":"reference","when":[{"path":"metadata.mode","op":"equals","value":"reference"}],"required":["metadata.start"],"forbidden":["prompt"]}],"rules":[{"kind":"require_together","paths":["metadata.start","metadata.end"]},{"kind":"mutually_exclusive","paths":["metadata.a","metadata.b"]},{"kind":"at_least_one","paths":["metadata.a","metadata.c"]},{"kind":"requires","paths":["metadata.a","metadata.c"]}]}`
	compiled := compileTest(t, "image", raw)
	valid := `{"model":"x","metadata":{"start":"https://a/x","end":"https://a/y","c":true}}`
	got, errs := compiled.Validate([]byte(valid))
	if len(errs) != 0 || got.Mode != "reference" {
		t.Fatalf("valid reference: %+v %+v", got, errs)
	}
	invalid := []struct{ body, path string }{
		{`{"model":"x","metadata":{"c":true}}`, "metadata.start"},
		{`{"model":"x","prompt":"p","metadata":{"start":"https://a/x","end":"https://a/y","c":true}}`, "prompt"},
		{`{"model":"x","metadata":{"start":"https://a/x","c":true}}`, "metadata.end"},
		{`{"model":"x","metadata":{"start":"https://a/x","end":"https://a/y","a":true,"b":true,"c":true}}`, "metadata.b"},
		{`{"model":"x","metadata":{"start":"https://a/x","end":"https://a/y","a":true}}`, "metadata.c"},
		{`{"model":"x","metadata":{"start":"https://a/x","end":"https://a/y","c":true,"layer_decomposition":true}}`, "metadata.layer_decomposition"},
	}
	for _, tc := range invalid {
		_, errs := compiled.Validate([]byte(tc.body))
		requirePath(t, errs, tc.path)
	}
	ambiguous := compileTest(t, "image", `{"version":1,"fields":{"prompt":{"type":"string"}},"modes":[{"id":"a"},{"id":"b"}]}`)
	_, errs = ambiguous.Validate([]byte(`{"model":"x"}`))
	requirePath(t, errs, "$")
}

// AC-023 (candidate scope): n=3 matches native_max_results=1 when semantic splitting is allowed; no HTTP/worker assertion here.
func TestMatchCandidateAllowsSemanticSplitWithoutWeakeningParameters(t *testing.T) {
	public := compileTest(t, "image", imageSchema)
	req, errs := public.Validate([]byte(`{"model":"x","prompt":"p","n":3,"metadata":{"resolution":"2K"}}`))
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	candidate := compileTest(t, "image", imageSchema)
	match := MatchCandidate(req, candidate)
	if !match.Supported || !match.NeedsSplit || match.NativeBatchSize != 1 {
		t.Fatalf("native=1 wrongly rejected: %+v", match)
	}
	incompatible := compileTest(t, "image", strings.ReplaceAll(imageSchema, `"enum":["1K","2K"]`, `"enum":["1K"]`))
	if MatchCandidate(req, incompatible).Supported {
		t.Fatal("candidate silently downgraded 2K")
	}
	lowerTotal := compileTest(t, "image", strings.ReplaceAll(imageSchema, `"maximum":8`, `"maximum":2`))
	if MatchCandidate(req, lowerTotal).Supported {
		t.Fatal("split bypassed total n cap")
	}
	linked := compileTest(t, "image", strings.ReplaceAll(imageSchema, `"allow_split":true`, `"allow_split":true,"semantically_linked":true`))
	if MatchCandidate(req, linked).Supported {
		t.Fatal("semantically linked group was independently split")
	}
	noSplit := compileTest(t, "image", strings.ReplaceAll(imageSchema, `"allow_split":true`, `"allow_split":false`))
	if MatchCandidate(req, noSplit).Supported {
		t.Fatal("unsupported splitting accepted")
	}
}

func TestValidateBoundsLargeInputsAndRetainsNoMutableState(t *testing.T) {
	compiled := compileTest(t, "image", `{"version":1,"fields":{"prompt":{"type":"string"},"metadata":{"type":"object","properties":{"note":{"type":"string"}}}}}`)
	_, errs := compiled.Validate([]byte(`{"model":"x","prompt":"` + strings.Repeat("x", 1024*1024) + `"}`))
	requirePath(t, errs, "$")
	_, errs = compiled.Validate([]byte(`{"model":"x","metadata":{"note":"` + strings.Repeat("x", 256*1024) + `"}}`))
	requirePath(t, errs, "metadata")
	first, errs := compiled.Validate([]byte(`{"model":"x","metadata":{"note":"a"}}`))
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	first.Values["metadata"].(map[string]any)["note"] = "changed"
	next, errs := compiled.Validate([]byte(`{"model":"x","metadata":{"note":"b"}}`))
	if len(errs) != 0 || next.Values["metadata"].(map[string]any)["note"] != "b" {
		t.Fatalf("compiled state mutated: %+v %+v", next, errs)
	}
}

func TestDefaultsPreserveExplicitNullFalseAndZero(t *testing.T) {
	compiled := compileTest(t, "image", `{"version":1,"fields":{"prompt":{"type":"string","nullable":true,"default":"fallback"},"metadata":{"type":"object","default":{},"properties":{"flag":{"type":"boolean","default":true},"seed":{"type":"integer","default":5}}}}}`)
	got, errs := compiled.Validate([]byte(`{"model":"x","prompt":null,"metadata":{"flag":false,"seed":0}}`))
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	metadata := got.Values["metadata"].(map[string]any)
	if got.Values["prompt"] != nil || metadata["flag"] != false || metadata["seed"] != json.Number("0") {
		t.Fatalf("explicit values replaced: %#v", got.Values)
	}
	if strings.Join(got.ProvidedFields, ",") != "metadata,metadata.flag,metadata.seed,model,prompt" {
		t.Fatalf("provenance lost: %+v", got.ProvidedFields)
	}
	defaults, errs := compiled.Validate([]byte(`{"model":"x"}`))
	if len(errs) != 0 || strings.Join(defaults.ProvidedFields, ",") != "model" || defaults.Values["prompt"] != "fallback" {
		t.Fatalf("default provenance/values wrong: %+v %+v", defaults, errs)
	}
}

func TestModeEmptyCoversMissingCollectionsButNotFalseOrZero(t *testing.T) {
	compiled := compileTest(t, "image", `{"version":1,"fields":{"metadata":{"type":"object","properties":{"value":{"type":"integer","nullable":true},"flag":{"type":"boolean"}}}},"modes":[{"id":"empty","when":[{"path":"metadata.value","op":"empty"}]},{"id":"value","when":[{"path":"metadata.value","op":"not_empty"}]}]}`)
	for _, tc := range []struct{ raw, mode string }{{`{"model":"x"}`, "empty"}, {`{"model":"x","metadata":{"value":null}}`, "empty"}, {`{"model":"x","metadata":{"value":0,"flag":false}}`, "value"}} {
		got, errs := compiled.Validate([]byte(tc.raw))
		if len(errs) != 0 || got.Mode != tc.mode {
			t.Fatalf("wrong empty mode: %+v %+v", got, errs)
		}
	}
	refs := compileTest(t, "image", imageSchema)
	got, errs := refs.Validate([]byte(`{"model":"x","prompt":"p","image_urls":[]}`))
	if len(errs) != 0 || got.Mode != "text" {
		t.Fatalf("empty reference array mode: %+v %+v", got, errs)
	}
	flag := compileTest(t, "image", `{"version":1,"fields":{"metadata":{"type":"object","properties":{"flag":{"type":"boolean"}}}},"modes":[{"id":"empty","when":[{"path":"metadata.flag","op":"empty"}]},{"id":"present","when":[{"path":"metadata.flag","op":"not_empty"}]}]}`)
	got, errs = flag.Validate([]byte(`{"model":"x","metadata":{"flag":false}}`))
	if len(errs) != 0 || got.Mode != "present" {
		t.Fatalf("false treated as empty: %+v %+v", got, errs)
	}
}

func TestMalformedRequestsAndResourceBoundsAreRejected(t *testing.T) {
	compiled := compileTest(t, "image", `{"version":1,"fields":{"metadata":{"type":"object","properties":{"seed":{"type":"number"}}}}}`)
	for _, body := range []string{`{"model":"x",}`, `{"model":"x","metadata":{"seed":1e999999999}}`, `{"model":"x","metadata":{"seed":1e-999999999}}`, `{"model":"x","metadata":` + strings.Repeat(`{"a":`, 9) + `null` + strings.Repeat(`}`, 9) + `}`} {
		if got, errs := compiled.Validate([]byte(body)); got != nil || len(errs) == 0 {
			t.Fatalf("malformed/unbounded request accepted: %+v %+v", got, errs)
		}
	}
	if _, err := Compile("image", []byte(strings.Repeat(" ", 512*1024)+`{}`)); err == nil {
		t.Fatal("oversized schema accepted")
	}
	fields := make(map[string]any)
	properties := make(map[string]any)
	for i := 0; i < 257; i++ {
		properties["p"+strconv.Itoa(i)] = map[string]any{"type": "string"}
	}
	fields["metadata"] = map[string]any{"type": "object", "properties": properties}
	raw, _ := json.Marshal(map[string]any{"version": 1, "fields": fields})
	if _, err := Compile("image", raw); err == nil {
		t.Fatal("unbounded field graph accepted")
	}
}

func TestCandidateMatchDoesNotMutateOrFillAcceptedParameters(t *testing.T) {
	public := compileTest(t, "image", `{"version":1,"fields":{"prompt":{"type":"string"}}}`)
	request, errs := public.Validate([]byte(`{"model":"x","prompt":"p"}`))
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	candidate := compileTest(t, "image", `{"version":1,"fields":{"prompt":{"type":"string"},"size":{"type":"string","default":"1:1"}}}`)
	if MatchCandidate(request, candidate).Supported {
		t.Fatal("candidate silently added default size")
	}
	if _, exists := request.Values["size"]; exists {
		t.Fatal("candidate mutated accepted values")
	}
	video := compileTest(t, "video", `{"version":1,"fields":{"prompt":{"type":"string"}}}`)
	if MatchCandidate(request, video).Supported {
		t.Fatal("image request matched video capability")
	}
	if MatchCandidate(nil, candidate).Supported || MatchCandidate(request, nil).Supported {
		t.Fatal("nil candidate input accepted")
	}
}

func TestCompileRejectsNullTypedConfiguration(t *testing.T) {
	for _, raw := range []string{
		`{"version":1,"fields":{"prompt":{"type":"string","required":null}}}`,
		`{"version":1,"fields":{"prompt":{"type":"string","enum":null}}}`,
		`{"version":1,"fields":{"metadata":{"type":"object","properties":null}}}`,
		`{"version":1,"fields":{},"batch":{"native_max_results":1,"allow_split":null}}`,
	} {
		if _, err := Compile("image", []byte(raw)); err == nil {
			t.Fatalf("null typed schema accepted: %s", raw)
		}
	}
}
