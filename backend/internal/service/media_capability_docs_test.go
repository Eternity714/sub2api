//go:build unit

package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
	"github.com/stretchr/testify/require"
)

// AC-025 (builtin-template documentation): every documented field/default/constraint,
// input mode, combination rule and example agrees with the checked-in catalog.
// Administrator-modified database capabilities still require a matching published document.
func TestMediaCatalogDocumentationMatchesSchemasAndExamples(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "UNIFIED_MEDIA_MODEL_PARAMETERS.md"))
	require.NoError(t, err, "mount repository docs at /workspace/docs:ro in the backend test container")
	api, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "UNIFIED_MEDIA_API.md"))
	require.NoError(t, err)
	for _, path := range []string{"/v1/images/generations", "/v1/images/generations/{task_id}", "/v1/videos/generations", "/v1/videos/generations/{task_id}"} {
		require.Contains(t, string(api), path)
	}
	require.Contains(t, string(api), "UNIFIED_MEDIA_MODEL_PARAMETERS.md", "the public protocol must link its model-specific reference")
	sections := mediaDocsSections(t, string(raw))
	catalog := MediaCapabilityCatalog()
	require.Len(t, sections, len(catalog), "missing or stale documented catalog entry")
	for _, entry := range catalog {
		key := entry.Platform + " / " + entry.UpstreamModel + " / " + entry.MediaType
		t.Run(key, func(t *testing.T) {
			section, exists := sections[key]
			require.True(t, exists, "missing model section")
			require.Contains(t, section, entry.Source)
			if len(entry.Schema) == 0 {
				require.Contains(t, section, "未提供可启用 schema")
				require.NotContains(t, section, "```json", "pending models must not advertise a generation example")
				return
			}
			var schema mediacapability.Schema
			require.NoError(t, json.Unmarshal(entry.Schema, &schema))
			rows := mediaDocsFieldRows(t, section)
			require.Equal(t, []string{"string", "是", "—", "已配置的公开模型别名"}, rows["model"])
			fields := map[string]mediacapability.Field{}
			mediaDocsFlattenFields(schema.Fields, "", fields)
			require.Len(t, rows, len(fields)+1, "unknown or missing documented fields")
			for path, field := range fields {
				row, ok := rows[path]
				require.True(t, ok, "missing field %s", path)
				fieldType := field.Type
				if field.Type == "array" && field.Items != nil {
					fieldType += "<" + field.Items.Type + ">"
				}
				require.Equal(t, fieldType, row[0], "%s type", path)
				required := "否"
				if field.Required {
					required = "是"
				}
				require.Equal(t, required, row[1], "%s required", path)
				if len(field.Default) == 0 {
					require.Equal(t, "—", row[2], "%s default", path)
				} else {
					require.JSONEq(t, string(field.Default), strings.Trim(row[2], "`"), "%s default", path)
				}
				if len(field.Enum) > 0 {
					matches := regexp.MustCompile("枚举 `([^`]*)`").FindStringSubmatch(row[3])
					require.Len(t, matches, 2, "%s enum", path)
					enum, marshalErr := json.Marshal(field.Enum)
					require.NoError(t, marshalErr)
					require.JSONEq(t, string(enum), matches[1], "%s enum", path)
				}
				for _, bound := range []struct {
					label string
					value any
				}{
					{"最小", field.Minimum}, {"最大", field.Maximum},
					{"最短字符", field.MinLength}, {"最长字符", field.MaxLength},
					{"最少项", field.MinItems}, {"最多项", field.MaxItems},
				} {
					switch value := bound.value.(type) {
					case *float64:
						if value != nil {
							require.Contains(t, row[3], fmt.Sprintf("%s %s", bound.label, strconv.FormatFloat(*value, 'f', -1, 64)), "%s bound", path)
						}
					case *int:
						if value != nil {
							require.Contains(t, row[3], fmt.Sprintf("%s %d", bound.label, *value), "%s bound", path)
						}
					}
				}
				if field.Format != "" {
					require.Contains(t, row[3], "格式 "+field.Format, path)
				}
			}
			for _, mode := range schema.Modes {
				line := mediaDocsListLine(t, section, mode.ID)
				for _, condition := range mode.When {
					clause := condition.Path + " " + condition.Op
					if condition.Op == "equals" {
						value, marshalErr := json.Marshal(condition.Value)
						require.NoError(t, marshalErr)
						clause += " " + string(value)
					}
					require.Contains(t, line, "`"+clause+"`", mode.ID)
				}
				if len(mode.Required) > 0 {
					require.Contains(t, line, "必填 "+mediaDocsQuotedPaths(mode.Required), mode.ID)
				}
				if len(mode.Forbidden) > 0 {
					require.Contains(t, line, "禁止 "+mediaDocsQuotedPaths(mode.Forbidden), mode.ID)
				}
			}
			for _, rule := range schema.Rules {
				prefix := "- `" + rule.Kind + "`：" + mediaDocsQuotedPaths(rule.Paths)
				require.Contains(t, section, prefix, "combination rule")
				if len(rule.When) > 0 {
					conditions, marshalErr := json.Marshal(rule.When)
					require.NoError(t, marshalErr)
					require.Contains(t, section, prefix+"，适用条件 `"+string(conditions)+"`", "conditional rule")
				}
			}
			for _, spec := range schema.ImageSpecs {
				require.Contains(t, section, fmt.Sprintf("| `%s` | `%s` | %d | %d |", spec.AspectRatio, spec.Resolution, spec.Width, spec.Height), "documented pixel mapping")
			}
			examples := regexp.MustCompile("(?s)```json\\s*\\n(.*?)\\n```").FindAllStringSubmatch(section, -1)
			require.Len(t, examples, 1, "exactly one public request example is required")
			require.JSONEq(t, string(entry.Example), examples[0][1])
			compiled, compileErr := mediacapability.Compile(entry.MediaType, entry.Schema)
			require.NoError(t, compileErr)
			_, fieldsErrors := compiled.Validate([]byte(examples[0][1]))
			require.Empty(t, fieldsErrors, "published request must be constructible without dynamic discovery")
		})
	}
}

func mediaDocsSections(t *testing.T, document string) map[string]string {
	t.Helper()
	sections := map[string]string{}
	for _, part := range strings.Split(document, "\n## ")[1:] {
		heading, body, found := strings.Cut(part, "\n")
		heading = strings.TrimSpace(heading)
		if !found || !strings.Contains(heading, " / ") {
			continue
		}
		_, duplicate := sections[heading]
		require.False(t, duplicate, "duplicate documentation section %s", heading)
		sections[heading] = body
	}
	return sections
}

func mediaDocsFieldRows(t *testing.T, section string) map[string][]string {
	t.Helper()
	rows := map[string][]string{}
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) != 7 {
			continue // Pixel mapping rows have four columns and are checked separately.
		}
		path := strings.Trim(strings.TrimSpace(cells[1]), "`")
		_, duplicate := rows[path]
		require.False(t, duplicate, "duplicate field %s", path)
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		rows[path] = cells[2:6]
	}
	return rows
}

func mediaDocsFlattenFields(fields map[string]mediacapability.Field, prefix string, out map[string]mediacapability.Field) {
	for name, field := range fields {
		path := prefix + name
		out[path] = field
		mediaDocsFlattenFields(field.Properties, path+".", out)
		if field.Items != nil && len(field.Items.Properties) > 0 {
			mediaDocsFlattenFields(field.Items.Properties, path+"[].", out)
		}
	}
}

func mediaDocsListLine(t *testing.T, section, id string) string {
	t.Helper()
	for _, line := range strings.Split(section, "\n") {
		if strings.HasPrefix(line, "- `"+id+"`：") {
			return line
		}
	}
	t.Fatalf("missing documented input mode %s", id)
	return ""
}

func mediaDocsQuotedPaths(paths []string) string { return "`" + strings.Join(paths, "`、`") + "`" }
