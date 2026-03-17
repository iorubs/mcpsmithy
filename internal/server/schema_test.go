package server

import (
	"testing"

	"github.com/operator-assistant/mcpsmithy/internal/config"
)

func TestBuildJSONSchema(t *testing.T) {
	f64 := func(v float64) *float64 { return &v }

	tests := []struct {
		name   string
		params []config.ToolParam
		check  func(*testing.T, map[string]any)
	}{
		{
			"empty params produces object type",
			nil,
			func(t *testing.T, s map[string]any) {
				if s["type"] != "object" {
					t.Errorf("type = %v; want object", s["type"])
				}
			},
		},
		{
			"required string param",
			[]config.ToolParam{
				{Name: "query", Type: config.ParamTypeString, Required: true, Description: "Search query"},
			},
			func(t *testing.T, s map[string]any) {
				props := s["properties"].(map[string]any)
				q := props["query"].(map[string]any)
				if q["type"] != "string" {
					t.Errorf("type = %v; want string", q["type"])
				}
				req := s["required"].([]string)
				if len(req) != 1 || req[0] != "query" {
					t.Errorf("required = %v; want [query]", req)
				}
			},
		},
		{
			"enum constraint",
			[]config.ToolParam{
				{Name: "env", Type: config.ParamTypeString, Constraints: &config.ParamConstraints{
					Enum: []any{"dev", "staging", "prod"},
				}},
			},
			func(t *testing.T, s map[string]any) {
				props := s["properties"].(map[string]any)
				env := props["env"].(map[string]any)
				enum, ok := env["enum"].([]any)
				if !ok {
					t.Fatal("expected enum key")
				}
				if len(enum) != 3 || enum[0] != "dev" || enum[1] != "staging" || enum[2] != "prod" {
					t.Errorf("enum = %v; want [dev staging prod]", enum)
				}
			},
		},
		{
			"min and max constraint",
			[]config.ToolParam{
				{Name: "count", Type: config.ParamTypeNumber, Constraints: &config.ParamConstraints{
					Min: f64(1), Max: f64(100),
				}},
			},
			func(t *testing.T, s map[string]any) {
				props := s["properties"].(map[string]any)
				count := props["count"].(map[string]any)
				if count["type"] != "number" {
					t.Errorf("type = %v; want number", count["type"])
				}
				if v, ok := count["minimum"].(float64); !ok || v != 1.0 {
					t.Errorf("minimum = %v; want 1", count["minimum"])
				}
				if v, ok := count["maximum"].(float64); !ok || v != 100.0 {
					t.Errorf("maximum = %v; want 100", count["maximum"])
				}
			},
		},
		{
			"min only — no maximum key",
			[]config.ToolParam{
				{Name: "offset", Type: config.ParamTypeNumber, Constraints: &config.ParamConstraints{
					Min: f64(0),
				}},
			},
			func(t *testing.T, s map[string]any) {
				props := s["properties"].(map[string]any)
				offset := props["offset"].(map[string]any)
				if _, ok := offset["minimum"]; !ok {
					t.Error("expected minimum key")
				}
				if _, ok := offset["maximum"]; ok {
					t.Error("unexpected maximum key")
				}
			},
		},
		{
			"no constraints — no enum/min/max keys",
			[]config.ToolParam{
				{Name: "q", Type: config.ParamTypeString},
			},
			func(t *testing.T, s map[string]any) {
				props := s["properties"].(map[string]any)
				q := props["q"].(map[string]any)
				for _, key := range []string{"enum", "minimum", "maximum"} {
					if _, ok := q[key]; ok {
						t.Errorf("unexpected key %q in schema property", key)
					}
				}
			},
		},
		{
			"string default",
			[]config.ToolParam{{Name: "name", Type: config.ParamTypeString, Default: "world"}},
			func(t *testing.T, s map[string]any) {
				p := s["properties"].(map[string]any)["name"].(map[string]any)
				if p["default"] != "world" {
					t.Errorf("default = %v; want world", p["default"])
				}
			},
		},
		{
			"int default",
			[]config.ToolParam{{Name: "count", Type: config.ParamTypeNumber, Default: 42}},
			func(t *testing.T, s map[string]any) {
				p := s["properties"].(map[string]any)["count"].(map[string]any)
				if p["default"] != 42 {
					t.Errorf("default = %v (%T); want 42 (int)", p["default"], p["default"])
				}
			},
		},
		{
			"bool default",
			[]config.ToolParam{{Name: "verbose", Type: config.ParamTypeBool, Default: true}},
			func(t *testing.T, s map[string]any) {
				p := s["properties"].(map[string]any)["verbose"].(map[string]any)
				if p["default"] != true {
					t.Errorf("default = %v (%T); want true", p["default"], p["default"])
				}
			},
		},
		{
			"nil default — no default key",
			[]config.ToolParam{{Name: "q", Type: config.ParamTypeString}},
			func(t *testing.T, s map[string]any) {
				p := s["properties"].(map[string]any)["q"].(map[string]any)
				if _, ok := p["default"]; ok {
					t.Error("nil Default should not produce a default key")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, buildJSONSchema(tt.params))
		})
	}
}
