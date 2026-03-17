package server

import "github.com/operator-assistant/mcpsmithy/internal/config"

// buildJSONSchema turns param configs into a JSON Schema object.
func buildJSONSchema(params []config.ToolParam) map[string]any {
	if len(params) == 0 {
		return map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
	}
	props := map[string]any{}
	var req []string
	for _, p := range params {
		prop := map[string]any{
			"type":        mapType(p.Type),
			"description": p.Description,
		}
		if p.Default != nil {
			prop["default"] = p.Default
		}

		// Emit constraint keywords into JSON Schema.
		if c := p.Constraints; c != nil {
			if len(c.Enum) > 0 {
				prop["enum"] = c.Enum
			}
			if c.Min != nil {
				prop["minimum"] = *c.Min
			}
			if c.Max != nil {
				prop["maximum"] = *c.Max
			}
		}

		props[p.Name] = prop
		if p.Required {
			req = append(req, p.Name)
		}
	}
	s := map[string]any{"type": "object", "properties": props}
	if len(req) > 0 {
		s["required"] = req
	}
	return s
}

// mapType converts a ParamType to its JSON Schema type name.
func mapType(t config.ParamType) string {
	switch t {
	case config.ParamTypeNumber:
		return "number"
	case config.ParamTypeBool:
		return "boolean"
	case config.ParamTypeArray:
		return "array"
	default:
		return "string"
	}
}
