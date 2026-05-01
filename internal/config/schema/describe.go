package schema

import (
	"fmt"
	"maps"
	"reflect"
	"sort"
	"strings"
)

// Describe walks the type tree rooted at v (typically a zero-value config
// struct like v1.Config{}) and produces a SchemaDoc using reflection and
// struct tags. Documentation strings come from docs (populated from Go source comments via go/ast).
func Describe(v any, version string, docs DocProvider) SchemaDoc {
	rootType := reflect.TypeOf(v)
	for rootType.Kind() == reflect.Pointer {
		rootType = rootType.Elem()
	}

	// BFS from the root type to discover all reachable structs and enums.
	knownStructs := map[string]reflect.Type{}
	knownEnums := map[string]reflect.Type{}
	knownTypes := map[string]bool{} // named non-struct, non-enum types
	var structOrder []string
	var enumOrder []string
	var typeOrder []string
	visited := map[string]bool{}

	type queueItem struct {
		t reflect.Type
	}
	queue := []queueItem{{rootType}}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		t := item.t
		for t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct || visited[t.Name()] {
			continue
		}
		visited[t.Name()] = true
		knownStructs[t.Name()] = t
		structOrder = append(structOrder, t.Name())

		// Visit yaml-tagged fields to discover child types.
		for field := range t.Fields() {
			ft := resolveFieldType(field)
			if ft == nil {
				continue
			}
			// Enum: named string implementing valuer.
			if ft.Kind() == reflect.String && ft.Name() != "string" && ft.Implements(valuerType) {
				if !visited[ft.Name()] {
					visited[ft.Name()] = true
					knownEnums[ft.Name()] = ft
					enumOrder = append(enumOrder, ft.Name())
				}
				continue
			}
			// Named type: non-struct, non-enum (e.g. TemplateString).
			if ft.Kind() == reflect.String && ft.Name() != "string" {
				if !visited[ft.Name()] {
					visited[ft.Name()] = true
					knownTypes[ft.Name()] = true
					typeOrder = append(typeOrder, ft.Name())
				}
				continue
			}
			// Struct: recurse.
			if ft.Kind() == reflect.Struct && ft.Name() != "" {
				queue = append(queue, queueItem{ft})
			}
		}
	}

	// Build struct docs in BFS order.
	known := make(map[string]reflect.Type, len(knownStructs)+len(knownEnums))
	maps.Copy(known, knownStructs)
	maps.Copy(known, knownEnums)
	structs := make([]StructDoc, 0, len(structOrder))
	for _, name := range structOrder {
		structs = append(structs, describeStruct(knownStructs[name], known, docs))
	}

	// Build enum docs in discovery order.
	enums := make([]EnumDoc, 0, len(enumOrder))
	for _, name := range enumOrder {
		enums = append(enums, describeEnum(knownEnums[name], docs))
	}

	// Discover enums that weren't reachable via struct fields but have
	// documented const values in the source (e.g. BuiltinFunc).
	for typeName, valueDocs := range docs.Values {
		if visited[typeName] {
			continue
		}
		var vals []EnumValueDoc
		for label, vdoc := range valueDocs {
			vals = append(vals, EnumValueDoc{Label: label, Doc: vdoc})
		}
		sort.Slice(vals, func(i, j int) bool { return vals[i].Label < vals[j].Label })
		enums = append(enums, EnumDoc{
			Name:   typeName,
			Doc:    docs.Types[typeName],
			Values: vals,
		})
	}

	// Build type docs for named non-struct types.
	typeDocs := make([]TypeDoc, 0, len(typeOrder))
	for _, name := range typeOrder {
		typeDocs = append(typeDocs, TypeDoc{Name: name, Doc: docs.Types[name]})
	}

	return SchemaDoc{Version: version, Structs: structs, Enums: enums, Types: typeDocs}
}

// resolveFieldType returns the "leaf" type for a struct field, unwrapping
// pointers, slices, and map values. Returns nil if the field has no yaml tag.
func resolveFieldType(field reflect.StructField) reflect.Type {
	yamlTag := field.Tag.Get("yaml")
	if yamlTag == "" || yamlTag == "-" {
		return nil
	}
	return unwrapType(field.Type)
}

// describeStruct builds a StructDoc for one reflect.Type.
// known maps type names to their reflect.Type, used for TypeRef.
// docs supplies documentation strings from Go source comments.
func describeStruct(t reflect.Type, known map[string]reflect.Type, docs DocProvider) StructDoc {
	doc := docs.Types[t.Name()]
	fieldDocs := docs.Fields[t.Name()]

	var fields []FieldDoc
	for field := range t.Fields() {
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "" || yamlTag == "-" {
			continue
		}
		name, _, _ := strings.Cut(yamlTag, ",")

		mcpTag := field.Tag.Get(tagName)
		info := parseTag(mcpTag)

		req := "no"
		if info.Required {
			req = "yes"
		}
		if len(info.OneOfs) > 0 && !info.OneOfs[0].Optional {
			req = "oneof"
		}

		def := "—"
		if info.Default != "" {
			def = info.Default
		}

		minStr := ""
		if info.Min != nil {
			minStr = fmt.Sprintf("%d", *info.Min)
		}

		// Field description from Go doc comment; fall back to child type doc.
		desc := ""
		if fd, ok := fieldDocs[field.Name]; ok {
			desc = fd
		} else {
			leaf := unwrapType(field.Type)
			if _, ok := known[leaf.Name()]; ok {
				desc = docs.Types[leaf.Name()]
			}
		}
		if desc == "" {
			desc = "—"
		}

		var groups []OneOfGroup
		for _, oe := range info.OneOfs {
			groups = append(groups, OneOfGroup{Group: oe.Group, Optional: oe.Optional})
		}

		fields = append(fields, FieldDoc{
			YAMLName:    name,
			Type:        friendlyTypeName(field.Type),
			TypeRef:     leafTypeRef(field.Type, known),
			Required:    req,
			Default:     def,
			Description: desc,
			Min:         minStr,
			NotReserved: info.NotReserved,
			Refs:        info.Refs,
			OneOfGroups: groups,
		})
	}

	// Resolve oneof groups: replace internal group names with peer field yaml names.
	groupMembers := map[string][]string{} // group name → list of yaml field names
	for _, f := range fields {
		for _, g := range f.OneOfGroups {
			groupMembers[g.Group] = append(groupMembers[g.Group], f.YAMLName)
		}
	}
	for i := range fields {
		for j := range fields[i].OneOfGroups {
			g := &fields[i].OneOfGroups[j]
			for _, name := range groupMembers[g.Group] {
				if name != fields[i].YAMLName {
					g.Peers = append(g.Peers, name)
				}
			}
		}
	}

	return StructDoc{Name: t.Name(), Doc: doc, Fields: fields}
}

// describeEnum builds an EnumDoc for one reflect.Type implementing valuer.
func describeEnum(t reflect.Type, docs DocProvider) EnumDoc {
	v := reflect.New(t).Elem().Interface()
	doc := docs.Types[t.Name()]

	valuer := v.(valuer)
	values := valuer.Values()

	valueDocs := docs.Values[t.Name()]

	enumValues := make([]EnumValueDoc, 0, len(values))
	for _, val := range values {
		vdoc := "—"
		if d, found := valueDocs[val]; found {
			vdoc = d
		}
		enumValues = append(enumValues, EnumValueDoc{Label: val, Doc: vdoc})
	}

	return EnumDoc{Name: t.Name(), Doc: doc, Values: enumValues}
}

// leafTypeRef returns the Go name of the leaf type if it's a known schema
// type (struct or enum), or "" otherwise. It unwraps pointers, slices, and
// map values to find the leaf.
func leafTypeRef(t reflect.Type, known map[string]reflect.Type) string {
	t = unwrapType(t)
	if _, ok := known[t.Name()]; ok {
		return t.Name()
	}
	return ""
}

// friendlyTypeName renders a reflect.Type as a plain type string.
func friendlyTypeName(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		if t.Name() != "string" {
			return t.Name()
		}
		return "string"
	case reflect.Int, reflect.Int64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Pointer:
		return friendlyTypeName(t.Elem())
	case reflect.Slice:
		return friendlyTypeName(t.Elem()) + "[]"
	case reflect.Map:
		return "map[" + friendlyTypeName(t.Key()) + "]" + friendlyTypeName(t.Elem())
	case reflect.Struct:
		if t.Name() != "" {
			return t.Name()
		}
		return "object"
	case reflect.Interface:
		return "any"
	default:
		if t.Name() != "" {
			return t.Name()
		}
		return "unknown"
	}
}
