package schema

import (
	"fmt"
	"reflect"
)

// isNestedStruct reports whether fv is a struct, ptr-to-struct, or map —
// i.e. a container the walker recurses into. Slices are excluded so that
// value slices ([]any, []string, etc.) can participate in oneof groups as
// leaf values.
func isNestedStruct(fv reflect.Value) bool {
	switch fv.Kind() {
	case reflect.Struct, reflect.Map:
		return true
	case reflect.Pointer:
		return fv.Type().Elem().Kind() == reflect.Struct
	}
	return false
}

// walkStructNodes visits every struct value reachable from v, calling fn
// once per struct with its reflect.Value and dotted yaml path prefix.
func walkStructNodes(v any, prefix string, fn func(rv reflect.Value, path string)) {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return
		}
		rv = rv.Elem()
	}
	walk(rv, prefix, fn, nil)
}

// walkLeafFields calls fn for every non-structlike leaf field reachable from v.
//
// fn receives:
//   - fv: the current field value
//   - set: a function to write a new value back (handles map write-back internally)
//   - info: parsed tag directives for this field
//   - path: dotted yaml path to this field
func walkLeafFields(v any, prefix string, fn func(fv reflect.Value, set func(reflect.Value), info tagInfo, path string)) {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return
		}
		rv = rv.Elem()
	}
	walk(rv, prefix, nil, fn)
}

// walk is the unified recursive traversal. onStruct is called once per struct
// node; onLeaf is called for every non-container leaf field that carries an
// mcpsmithy tag. Either callback may be nil.
func walk(rv reflect.Value, prefix string, onStruct func(reflect.Value, string), onLeaf func(reflect.Value, func(reflect.Value), tagInfo, string)) {
	if rv.Kind() != reflect.Struct {
		return
	}
	if onStruct != nil {
		onStruct(rv, prefix)
	}
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		fv := rv.Field(i)
		yamlName := yamlFieldName(field)
		if yamlName == "" || yamlName == "-" {
			continue
		}
		childPath := childPathFor(prefix, yamlName)
		switch {
		case fv.Kind() == reflect.Struct:
			walk(fv, childPath, onStruct, onLeaf)
		case fv.Kind() == reflect.Pointer && !fv.IsNil() && fv.Elem().Kind() == reflect.Struct:
			walk(fv.Elem(), childPath, onStruct, onLeaf)
		case fv.Kind() == reflect.Map && !fv.IsNil():
			walkMap(fv, childPath, onStruct, onLeaf)
		case fv.Kind() == reflect.Slice:
			walkSlice(fv, childPath, onStruct, onLeaf)
		default:
			if onLeaf == nil {
				continue
			}
			raw := field.Tag.Get(tagName)
			if raw == "" {
				continue
			}
			info := parseTag(raw)
			idx := i
			onLeaf(fv, func(nv reflect.Value) { rv.Field(idx).Set(nv) }, info, childPath)
		}
	}
}

// walkMap traverses map[K]Struct or map[K]*Struct values.
// When onLeaf is set, struct values are copied before walking so that set
// closures can mutate the copy; the updated copy is written back afterwards.
func walkMap(m reflect.Value, prefix string, onStruct func(reflect.Value, string), onLeaf func(reflect.Value, func(reflect.Value), tagInfo, string)) {
	elemType := m.Type().Elem()
	isStruct := elemType.Kind() == reflect.Struct
	isPtrStruct := elemType.Kind() == reflect.Pointer && elemType.Elem().Kind() == reflect.Struct
	if !isStruct && !isPtrStruct {
		return
	}
	for _, key := range m.MapKeys() {
		path := fmt.Sprintf("%s[%s]", prefix, key)
		val := m.MapIndex(key)
		if isStruct {
			if onLeaf != nil {
				// Copy → walk (set closures target the copy) → write back.
				cp := reflect.New(elemType).Elem()
				cp.Set(val)
				walk(cp, path, onStruct, onLeaf)
				m.SetMapIndex(key, cp)
			} else {
				walk(val, path, onStruct, nil)
			}
		} else if isPtrStruct && !val.IsNil() {
			walk(val.Elem(), path, onStruct, onLeaf)
		}
	}
}

// walkSlice traverses []Struct or []*Struct values.
func walkSlice(s reflect.Value, prefix string, onStruct func(reflect.Value, string), onLeaf func(reflect.Value, func(reflect.Value), tagInfo, string)) {
	elemType := s.Type().Elem()
	isStruct := elemType.Kind() == reflect.Struct
	isPtrStruct := elemType.Kind() == reflect.Pointer && elemType.Elem().Kind() == reflect.Struct
	if !isStruct && !isPtrStruct {
		return
	}
	for i := 0; i < s.Len(); i++ {
		path := fmt.Sprintf("%s[%d]", prefix, i)
		elem := s.Index(i)
		if isStruct {
			walk(elem, path, onStruct, onLeaf)
		} else if isPtrStruct && !elem.IsNil() {
			walk(elem.Elem(), path, onStruct, onLeaf)
		}
	}
}

// childPathFor builds a dotted yaml path from a prefix and a field name.
func childPathFor(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}
