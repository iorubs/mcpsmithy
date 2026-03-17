package schema

import (
	"fmt"
	"reflect"
)

// ParamType names the valid parameter types for tool definitions.
// Use one of these as the type: value in a tool parameter.
//
// The classification methods (IsNumeric, IsStringLike, IsBoolean) and
// Compatible implement TypeClassifier, enabling the schema engine to
// validate typed-as constraints without per-version logic.
type ParamType string

const (
	// JSON Schema string.
	ParamTypeString ParamType = "string"
	// JSON Schema number (float64).
	ParamTypeNumber ParamType = "number"
	// JSON Schema boolean.
	ParamTypeBool ParamType = "bool"
	// JSON Schema array.
	ParamTypeArray ParamType = "array"
	// String validated as a file path inside the project sandbox.
	ParamTypeProjectFilePath ParamType = "project_file_path"
)

// Values returns the set of valid ParamType values.
// Implements Valuer for automatic enum validation.
func (ParamType) Values() []string {
	return []string{
		string(ParamTypeString),
		string(ParamTypeNumber),
		string(ParamTypeBool),
		string(ParamTypeArray),
		string(ParamTypeProjectFilePath),
	}
}

// IsNumeric reports whether this param type accepts numeric constraints (min/max).
// Implements TypeClassifier.
func (p ParamType) IsNumeric() bool {
	return p == ParamTypeNumber
}

// IsStringLike reports whether this param type expects string Go values.
// Implements TypeClassifier.
func (p ParamType) IsStringLike() bool {
	return p == ParamTypeString || p == ParamTypeProjectFilePath
}

// IsBoolean reports whether this param type expects bool Go values.
// Implements TypeClassifier.
func (p ParamType) IsBoolean() bool {
	return p == ParamTypeBool
}

// Compatible reports whether the given Go value is compatible with this param
// type. YAML unmarshals integers as int, floats as float64, strings as string,
// and booleans as bool. A mismatch means the config author wrote the wrong
// literal type. Implements TypeClassifier.
func (p ParamType) Compatible(v any) error {
	if v == nil {
		return nil
	}
	switch {
	case p.IsStringLike():
		if _, ok := v.(string); !ok {
			return fmt.Errorf("expected string, got %T(%v)", v, v)
		}
	case p == ParamTypeNumber:
		switch v.(type) {
		case int, float64: // ok — YAML unmarshals whole numbers as int
		default:
			return fmt.Errorf("expected number, got %T(%v)", v, v)
		}
	case p.IsBoolean():
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T(%v)", v, v)
		}
	case p == ParamTypeArray:
		if reflect.TypeOf(v).Kind() != reflect.Slice {
			return fmt.Errorf("expected array, got %T(%v)", v, v)
		}
	}
	return nil
}

// ZeroForParamType returns a type-appropriate zero value for template
// validation dry-runs.
func ZeroForParamType(pt ParamType) any {
	switch pt {
	case ParamTypeNumber:
		return 0.0
	case ParamTypeBool:
		return false
	case ParamTypeArray:
		return []any{}
	default:
		return ""
	}
}
