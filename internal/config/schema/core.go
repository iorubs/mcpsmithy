// Package schema provides tag-driven defaults and validation for config structs.
package schema

import (
	"reflect"
	"strings"
)

// ── Constants ────────────────────────────────────────────────────────────────

// tagName is the struct tag key used by mcpsmithy for field metadata.
const tagName = "mcpsmithy"

// ReservedContextKey is the template context key injected by the engine.
// It is the canonical definition shared across all config versions.
const ReservedContextKey = "mcpsmithy"

// reservedContextKeys lists all template context keys injected by the engine.
// User-defined param names must not collide with any of these.
var reservedContextKeys = []string{ReservedContextKey}

// ── Interfaces ───────────────────────────────────────────────────────────────

// valuer is implemented by named string types that have a fixed set of
// valid values (enums). Process checks non-zero fields whose type
// implements this interface.
type valuer interface {
	Values() []string
}

var valuerType = reflect.TypeFor[valuer]()

// validator is implemented by named types that can validate their own value
// at config-load time. Process calls Validate on every non-zero field whose
// type implements this interface and treats any error as a hard config error.
type validator interface {
	Validate() error
}

var validatorType = reflect.TypeFor[validator]()

// DocProvider supplies documentation extracted from Go source comments.
// Populated by the AST walker in v1/docs.go and passed to Describe.
type DocProvider struct {
	Types  map[string]string            // type name → doc
	Fields map[string]map[string]string // type name → Go field name → doc
	Values map[string]map[string]string // type name → enum value → doc
}

// typeClassifier is implemented by field types that can classify values for
// type-compatibility checking. Used by the typed-as= tag directive.
//
// When a struct field is tagged with typed-as=<sibling>, schema.Process
// resolves the sibling field, checks that it implements typeClassifier, and
// validates the tagged field's value (or sub-fields) against it.
type typeClassifier interface {
	IsNumeric() bool
	IsStringLike() bool
	IsBoolean() bool
	// Compatible reports whether the given Go value is compatible with this
	// type. Returns a descriptive error if not.
	Compatible(v any) error
}

var typeClassifierType = reflect.TypeFor[typeClassifier]()

// ── Shared reflection helpers ────────────────────────────────────────────────

// isZero reports whether a value is the zero value for its type.
func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.Len() == 0
	case reflect.Slice, reflect.Map:
		return v.IsNil() || v.Len() == 0
	case reflect.Pointer, reflect.Interface:
		return v.IsNil()
	default:
		return v.IsZero()
	}
}

// yamlFieldName extracts the field name from the yaml struct tag.
func yamlFieldName(f reflect.StructField) string {
	tag := f.Tag.Get("yaml")
	if tag == "" {
		return ""
	}
	name, _, _ := strings.Cut(tag, ",")
	return name
}

// findFieldByYAMLName returns the reflect.Value of the struct field whose yaml
// tag name matches yamlName, and true if found.
func findFieldByYAMLName(rv reflect.Value, rt reflect.Type, yamlName string) (reflect.Value, bool) {
	for i := 0; i < rt.NumField(); i++ {
		if yamlFieldName(rt.Field(i)) == yamlName {
			return rv.Field(i), true
		}
	}
	return reflect.Value{}, false
}

// unwrapType resolves the leaf element type by peeling off pointers, slices,
// and map values.
func unwrapType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	if t.Kind() == reflect.Map {
		t = t.Elem()
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

// ── Output types ─────────────────────────────────────────────────────────────

// SchemaDoc holds the fully described schema for a config version.
type SchemaDoc struct {
	Version string
	Structs []StructDoc
	Enums   []EnumDoc
	Types   []TypeDoc // named non-struct, non-enum types (e.g. TemplateString)
}

// KnownTypes builds the set of type names present in a SchemaDoc.
// Callers can use this for cross-linking when rendering.
func (d SchemaDoc) KnownTypes() map[string]bool {
	m := make(map[string]bool, len(d.Structs)+len(d.Enums)+len(d.Types))
	for _, s := range d.Structs {
		m[s.Name] = true
	}
	for _, e := range d.Enums {
		m[e.Name] = true
	}
	for _, t := range d.Types {
		m[t.Name] = true
	}
	return m
}

// FilterTypes returns a new SchemaDoc containing only the structs and enums
// whose names appear in the names set. Used by setup to extract the types
// relevant to a specific config section.
func FilterTypes(doc SchemaDoc, names map[string]bool) SchemaDoc {
	var structs []StructDoc
	for _, s := range doc.Structs {
		if names[s.Name] {
			structs = append(structs, s)
		}
	}
	var enums []EnumDoc
	for _, e := range doc.Enums {
		if names[e.Name] {
			enums = append(enums, e)
		}
	}
	var types []TypeDoc
	for _, t := range doc.Types {
		if names[t.Name] {
			types = append(types, t)
		}
	}
	return SchemaDoc{Version: doc.Version, Structs: structs, Enums: enums, Types: types}
}

// StructDoc describes a config struct type.
type StructDoc struct {
	Name   string
	Doc    string
	Fields []FieldDoc
}

// FieldDoc describes a single struct field.
type FieldDoc struct {
	YAMLName    string
	Type        string // plain type string (e.g. "Tool", "string[]", "map[string]Convention")
	TypeRef     string // non-empty when the leaf type is a known schema type (e.g. "Convention")
	Required    string // "yes", "no", or "oneof"
	Default     string // "—" or the default value
	Description string
	Min         string // "" or the minimum value as a string
	NotReserved bool
	Refs        []string
	OneOfGroups []OneOfGroup // mutual-exclusivity groups this field belongs to
}

// OneOfGroup describes a mutual-exclusivity group membership for a field.
type OneOfGroup struct {
	Group    string   // group name (internal identifier from the tag)
	Optional bool     // true = at-most-one (oneof?=); false = exactly-one (oneof=)
	Peers    []string // yaml names of the other fields in this group
}

// TypeDoc describes a named non-struct type (e.g. a named string).
type TypeDoc struct {
	Name string
	Doc  string
}

// EnumDoc describes a named string type with const values.
type EnumDoc struct {
	Name   string
	Doc    string
	Values []EnumValueDoc
}

// EnumValueDoc is a single enum value with its label and doc.
type EnumValueDoc struct {
	Label string
	Doc   string
}
