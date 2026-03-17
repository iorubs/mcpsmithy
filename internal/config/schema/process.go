package schema

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// Process applies tag-driven defaults and validates v in a single traversal.
//
// Both operations share walkLeafFields: for each leaf field it first applies
// any default= value (via the set callback, which handles map write-back
// internally), then runs all field-level validators on the (now-defaulted)
// value. OneOf groups require struct-scope visibility so they run in a
// second pass via walkStructNodes.
//
// All validation errors are returned (required fields, enum, min, ref, oneof, notreserved, template syntax, typed-as).
func Process(v any) []error {
	var errs []error
	root := reflect.ValueOf(v)
	for root.Kind() == reflect.Pointer {
		if root.IsNil() {
			return nil
		}
		root = root.Elem()
	}

	walkLeafFields(v, "", func(fv reflect.Value, set func(reflect.Value), info tagInfo, path string) {
		// 1. Apply default before any check so validators see the final value.
		if info.Default != "" && isZero(fv) {
			if nv, ok := makeDefault(fv.Type(), info.Default); ok {
				set(nv)
				fv = nv
			}
		}

		// 2. Required — nothing else to check on a zero value.
		if isZero(fv) {
			if info.Required {
				errs = append(errs, errors.New(path+" is required"))
			}
			return
		}

		// 3. Enum values.
		if fv.Type().Implements(valuerType) {
			errs = append(errs, checkValuer(fv, path)...)
		}

		// 4. Custom field validation (e.g. template syntax check).
		if fv.Type().Implements(validatorType) {
			if err := fv.Interface().(validator).Validate(); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", path, err))
			}
		}

		// 5. Minimum bound.
		if info.Min != nil && (fv.Kind() == reflect.Int || fv.Kind() == reflect.Int64) {
			if int(fv.Int()) < *info.Min {
				errs = append(errs, fmt.Errorf("%s: must be >= %d, got %d", path, *info.Min, fv.Int()))
			}
		}

		// 6. Reserved name.
		if info.NotReserved && fv.Kind() == reflect.String {
			val := fv.String()
			if slices.Contains(reservedContextKeys, val) {
				errs = append(errs, fmt.Errorf("%s: %q is a reserved name (must not be one of: %s)",
					path, val, strings.Join(reservedContextKeys, ", ")))
			}
		}

		// 7. Ref — value must appear as a key in at least one referenced map.
		if len(info.Refs) > 0 && fv.Kind() == reflect.String {
			var validKeys []string
			for _, refPath := range info.Refs {
				validKeys = append(validKeys, resolveMapKeys(root, refPath)...)
			}
			if len(validKeys) > 0 {
				val := fv.String()
				if slices.Contains(validKeys, val) {
					return
				}
				errs = append(errs, fmt.Errorf("%s: %q does not match any declared key", path, val))
			}
		}
	})

	// Struct-level validation: oneof groups, typed-as constraints, and the
	// validator interface. Combined into a single pass to avoid redundant
	// tree traversals.
	walkStructNodes(v, "", func(rv reflect.Value, path string) {
		errs = append(errs, oneOfErrors(rv, path)...)
		errs = append(errs, typedAsErrors(rv, path)...)
		if rv.Type().Implements(validatorType) {
			if err := rv.Interface().(validator).Validate(); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", path, err))
			}
		}
	})
	return errs
}

// makeDefault converts a default string to a reflect.Value of the target type.
// Supports string (and named string types), int/int64, bool, and *bool.
func makeDefault(t reflect.Type, def string) (reflect.Value, bool) {
	switch t.Kind() {
	case reflect.String:
		v := reflect.New(t).Elem()
		v.SetString(def)
		return v, true
	case reflect.Int, reflect.Int64:
		if n, err := strconv.ParseInt(def, 10, 64); err == nil {
			v := reflect.New(t).Elem()
			v.SetInt(n)
			return v, true
		}
	case reflect.Bool:
		if b, err := strconv.ParseBool(def); err == nil {
			v := reflect.New(t).Elem()
			v.SetBool(b)
			return v, true
		}
	case reflect.Float32, reflect.Float64:
		if f, err := strconv.ParseFloat(def, 64); err == nil {
			v := reflect.New(t).Elem()
			v.SetFloat(f)
			return v, true
		}
	case reflect.Pointer:
		if t.Elem().Kind() == reflect.Bool {
			if b, err := strconv.ParseBool(def); err == nil {
				bv := reflect.New(t.Elem())
				bv.Elem().SetBool(b)
				return bv, true
			}
		}
	}
	return reflect.Value{}, false
}

// checkValuer verifies a field value is in the type's allowed set.
func checkValuer(fv reflect.Value, path string) []error {
	v := fv.Interface().(valuer)
	allowed := v.Values()
	actual := fmt.Sprint(fv.Interface())
	if slices.Contains(allowed, actual) {
		return nil
	}
	return []error{fmt.Errorf("%s: must be one of [%s], got %q", path, strings.Join(allowed, ", "), actual)}
}

// resolveMapKeys navigates root by dotted yaml-field path and returns the
// string keys of the map at the destination. Returns nil if any segment is
// missing, a nil pointer, or the destination is not a map.
func resolveMapKeys(root reflect.Value, dotPath string) []string {
	cur := root
	for seg := range strings.SplitSeq(dotPath, ".") {
		for cur.Kind() == reflect.Pointer {
			if cur.IsNil() {
				return nil
			}
			cur = cur.Elem()
		}
		if cur.Kind() != reflect.Struct {
			return nil
		}
		rt := cur.Type()
		found := false
		for i := 0; i < rt.NumField(); i++ {
			if yamlFieldName(rt.Field(i)) == seg {
				cur = cur.Field(i)
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}
	for cur.Kind() == reflect.Pointer {
		if cur.IsNil() {
			return nil
		}
		cur = cur.Elem()
	}
	if cur.Kind() != reflect.Map || cur.IsNil() {
		return nil
	}
	var keys []string
	for _, k := range cur.MapKeys() {
		if k.Kind() == reflect.String {
			keys = append(keys, k.String())
		}
	}
	return keys
}

// ── Struct-level validators (oneof, typed-as) ─────────────────────────────────

// oneOfErrors checks mutual-exclusivity groups within a single struct node rv.
// Called once per struct node from Process's combined struct-level pass.
//
// Groups tagged oneof= require exactly one member set (error on 0 or 2+).
// Groups tagged oneof?= require at most one member set (error on 2+, 0 is OK).
// A field may belong to multiple groups via repeated oneof/oneof? directives.
func oneOfErrors(rv reflect.Value, structPath string) []error {
	rt := rv.Type()
	type member struct {
		yamlName string
		set      bool
	}
	type groupInfo struct {
		optional bool // true when any member uses oneof?= for this group
		members  []member
	}
	groups := map[string]*groupInfo{}

	for i := 0; i < rt.NumField(); i++ {
		field, fv := rt.Field(i), rv.Field(i)
		// Skip struct containers the walker recurses into (struct, ptr-to-struct,
		// map). Slices are NOT skipped: value slices (e.g. []any) are leaf values
		// that can participate in oneof groups.
		if isNestedStruct(fv) {
			continue
		}
		yamlName := yamlFieldName(field)
		if yamlName == "" || yamlName == "-" {
			continue
		}
		raw := field.Tag.Get(tagName)
		if raw == "" {
			continue
		}
		info := parseTag(raw)
		for _, oe := range info.OneOfs {
			g, ok := groups[oe.Group]
			if !ok {
				g = &groupInfo{}
				groups[oe.Group] = g
			}
			if oe.Optional {
				g.optional = true
			}
			g.members = append(g.members, member{yamlName: yamlName, set: !isZero(fv)})
		}
	}

	var errs []error
	for _, name := range slices.Sorted(maps.Keys(groups)) {
		g := groups[name]
		var setNames, allNames []string
		for _, m := range g.members {
			allNames = append(allNames, m.yamlName)
			if m.set {
				setNames = append(setNames, m.yamlName)
			}
		}
		switch len(setNames) {
		case 1:
			// exactly one — valid for both oneof and oneof?
		case 0:
			if g.optional {
				continue // at-most-one: zero set is fine
			}
			p := structPath
			if p == "" {
				p = strings.Join(allNames, "/")
			}
			errs = append(errs, fmt.Errorf("%s: must set one of [%s]", p, strings.Join(allNames, ", ")))
		default:
			p := structPath
			if p == "" {
				p = strings.Join(setNames, "/")
			}
			errs = append(errs, fmt.Errorf("%s: %s are mutually exclusive", p, strings.Join(setNames, " and ")))
		}
	}
	return errs
}

// typedAsErrors checks typed-as cross-field constraints within a single struct
// node rv. Called once per struct node from Process's combined struct-level pass.
func typedAsErrors(rv reflect.Value, structPath string) []error {
	var errs []error
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		fv := rv.Field(i)
		if isZero(fv) {
			continue
		}
		raw := field.Tag.Get(tagName)
		if raw == "" {
			continue
		}
		info := parseTag(raw)
		if info.TypedAs == "" {
			continue
		}
		yamlName := yamlFieldName(field)
		fieldPath := childPathFor(structPath, yamlName)

		// Resolve the sibling field by its yaml name.
		sibling, ok := findFieldByYAMLName(rv, rt, info.TypedAs)
		if !ok || isZero(sibling) {
			continue
		}
		if !sibling.Type().Implements(typeClassifierType) {
			continue
		}
		tc := sibling.Interface().(typeClassifier)
		errs = append(errs, checkAgainstTypeClassifier(fv, tc, fieldPath)...)
	}
	return errs
}

// checkAgainstTypeClassifier validates a field value against a typeClassifier.
// Handles interface{}/any fields (Go type check) and struct/ptr-to-struct
// fields (constraint sub-field compatibility check).
func checkAgainstTypeClassifier(fv reflect.Value, tc typeClassifier, path string) []error {
	// Dereference pointer chain.
	actual := fv
	for actual.Kind() == reflect.Pointer {
		if actual.IsNil() {
			return nil
		}
		actual = actual.Elem()
	}

	switch actual.Kind() {
	case reflect.Interface:
		// any / interface{} field — validate the Go type of the stored value.
		if actual.IsNil() {
			return nil
		}
		if err := tc.Compatible(actual.Elem().Interface()); err != nil {
			return []error{fmt.Errorf("%s: %w", path, err)}
		}
	case reflect.Struct:
		return checkConstraintsStruct(actual, tc, path)
	}
	return nil
}

// checkConstraintsStruct inspects a constraints-like struct for min, max, and
// enum fields and validates their presence and values against the typeClassifier.
func checkConstraintsStruct(rv reflect.Value, tc typeClassifier, path string) []error {
	var errs []error
	rt := rv.Type()

	var minFloat, maxFloat *float64
	var enumSlice reflect.Value

	for i := 0; i < rt.NumField(); i++ {
		name := yamlFieldName(rt.Field(i))
		fv := rv.Field(i)
		switch name {
		case "min":
			if fv.Kind() == reflect.Pointer && !fv.IsNil() {
				f := fv.Elem().Float()
				minFloat = &f
			}
		case "max":
			if fv.Kind() == reflect.Pointer && !fv.IsNil() {
				f := fv.Elem().Float()
				maxFloat = &f
			}
		case "enum":
			if fv.Kind() == reflect.Slice && fv.Len() > 0 {
				enumSlice = fv
			}
		}
	}

	// min/max are only valid for numeric param types.
	if minFloat != nil && !tc.IsNumeric() {
		errs = append(errs, fmt.Errorf("%s.min: only valid for numeric types (int, integer, number, float)", path))
	}
	if maxFloat != nil && !tc.IsNumeric() {
		errs = append(errs, fmt.Errorf("%s.max: only valid for numeric types (int, integer, number, float)", path))
	}
	// min must be <= max.
	if minFloat != nil && maxFloat != nil && *minFloat > *maxFloat {
		errs = append(errs, fmt.Errorf("%s: min (%g) must be <= max (%g)", path, *minFloat, *maxFloat))
	}
	// Enum value Go types must match the declared param type.
	if enumSlice.IsValid() {
		for j := 0; j < enumSlice.Len(); j++ {
			val := enumSlice.Index(j)
			var v any
			if val.Kind() == reflect.Interface {
				if val.IsNil() {
					continue
				}
				v = val.Elem().Interface()
			} else {
				v = val.Interface()
			}
			if err := tc.Compatible(v); err != nil {
				errs = append(errs, fmt.Errorf("%s.enum[%d]: %w", path, j, err))
			}
		}
	}
	return errs
}
