// Package tools provides the config-driven tool execution engine.
package tools

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"text/template"
	"unicode/utf8"

	"github.com/operator-assistant/mcpsmithy/internal/config"
	"github.com/operator-assistant/mcpsmithy/internal/conventions"
	"github.com/operator-assistant/mcpsmithy/internal/project"
)

type handler func(ctx context.Context, params map[string]any) (string, error)

// Engine holds registered tool handlers built from config.
type Engine struct {
	cfg      *config.Config
	sb       *sandbox
	handlers map[string]handler
	tools    map[string]config.Tool
	tpl      *templateEngine
}

// New builds an engine from the loaded config.
func New(ctx context.Context, cfg *config.Config, root string) (*Engine, error) {
	sb, err := newSandbox(root)
	if err != nil {
		return nil, fmt.Errorf("sandbox: %w", err)
	}

	idxMgr, _ := project.Build(ctx, cfg, sb.Root(), project.BuildOptions{})
	convIdx := conventions.BuildIndex(ctx, cfg)
	tpl := newTemplateEngine(cfg, sb.Root(), cfg.Conventions, idxMgr, convIdx, sb.fsys)

	e := &Engine{
		cfg:      cfg,
		sb:       sb,
		handlers: make(map[string]handler),
		tools:    cfg.Tools,
		tpl:      tpl,
	}

	for name, tool := range cfg.Tools {
		h, err := e.templateHandler(name, tool)
		if err != nil {
			return nil, fmt.Errorf("tool %q: %w", name, err)
		}
		e.handlers[name] = h
	}

	return e, nil
}

// Tools returns all registered tool definitions.
func (e *Engine) Tools() map[string]config.Tool { return e.tools }

func (e *Engine) Execute(ctx context.Context, name string, params map[string]any) (string, error) {
	h, ok := e.handlers[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %q", name)
	}

	tc := e.tools[name]
	if params == nil {
		params = make(map[string]any)
	}
	for _, p := range tc.Params {
		if _, provided := params[p.Name]; !provided {
			if p.Required {
				return "", fmt.Errorf("missing required parameter: %q", p.Name)
			}
			if p.Default != nil {
				params[p.Name] = p.Default
			} else {
				// Set type-appropriate zero value so templates don't fail on missing keys.
				params[p.Name] = zeroForType(p.Type)
			}
		}
		if p.Type == config.ParamTypeProjectFilePath {
			if v, ok := params[p.Name].(string); ok && v != "" {
				if err := e.sb.ValidateFilePath(v); err != nil {
					return "", fmt.Errorf("parameter %q: %w", p.Name, err)
				}
			}
		}
		if p.Constraints != nil {
			if err := validateConstraint(p, params[p.Name]); err != nil {
				return "", fmt.Errorf("parameter %q: %w", p.Name, err)
			}
		}
	}
	// Inject options (config-author values, invisible to LLM).
	maps.Copy(params, tc.Options)

	out, err := h(ctx, params)
	if err != nil {
		return "", fmt.Errorf("tool %q: %w", name, err)
	}
	limit := tc.MaxOutputKB * 1024
	if limit <= 0 {
		limit = 1024 * 1024 // matches maxOutputKB default=1024
	}
	return truncateOutput(out, limit), nil
}

// truncateOutput caps output at limit bytes, ensuring valid UTF-8.
func truncateOutput(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	trunc := s[:limit]
	for !utf8.ValidString(trunc) && len(trunc) > 0 {
		trunc = trunc[:len(trunc)-1]
	}
	return trunc + fmt.Sprintf("\n... (truncated at %dKB)", limit/1024)
}

// validateConstraint checks a parameter value against its declared constraints.
// It returns an error if the value violates an enum or numeric range constraint.
func validateConstraint(p config.ToolParam, val any) error {
	c := p.Constraints
	if c == nil {
		return nil
	}

	if len(c.Enum) > 0 {
		for _, allowed := range c.Enum {
			if fmt.Sprint(val) == fmt.Sprint(allowed) {
				return nil
			}
		}
		return fmt.Errorf("value %v not in allowed values %v", val, c.Enum)
	}

	if c.Min != nil || c.Max != nil {
		n, ok := toFloat64(val)
		if !ok {
			return fmt.Errorf("expected numeric value for range check, got %T(%v)", val, val)
		}
		if c.Min != nil && n < *c.Min {
			return fmt.Errorf("value %g is below minimum %g", n, *c.Min)
		}
		if c.Max != nil && n > *c.Max {
			return fmt.Errorf("value %g exceeds maximum %g", n, *c.Max)
		}
	}
	return nil
}

// toFloat64 converts a numeric value to float64 for range validation.
// Handles float64 (from JSON/LLM) and int (from YAML default values).
func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	default:
		return 0, false
	}
}

// zeroForType returns a type-appropriate zero value for the given ParamType.
func zeroForType(t config.ParamType) any {
	switch t {
	case config.ParamTypeNumber:
		return 0.0
	case config.ParamTypeBool:
		return false
	case config.ParamTypeArray:
		return []any{}
	default:
		return ""
	}
}

func (e *Engine) templateHandler(name string, t config.Tool) (handler, error) {
	parsed, err := template.New(name).Funcs(e.tpl.funcMap()).Option("missingkey=error").Parse(string(t.Template))
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	return func(_ context.Context, params map[string]any) (string, error) {
		ctx := e.tpl.Context(params)
		var b strings.Builder
		if err := parsed.Execute(&b, ctx); err != nil {
			return "", err
		}
		return b.String(), nil
	}, nil
}
