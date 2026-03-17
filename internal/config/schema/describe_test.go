package schema

import (
	"reflect"
	"testing"
)

// descTestConfig is a minimal struct mirroring the shape of a real config,
// used to verify Describe walks the type tree correctly.
type descTestConfig struct {
	Version string         `yaml:"version" mcpsmithy:"required"`
	Project descTestProj   `yaml:"project" mcpsmithy:"required"`
	Items   []descTestItem `yaml:"items"`
}

type descTestProj struct {
	Name string       `yaml:"name" mcpsmithy:"required"`
	Mode descTestEnum `yaml:"mode" mcpsmithy:"default=fast"`
}

type descTestItem struct {
	Label string `yaml:"label" mcpsmithy:"required,notreserved"`
	Count int    `yaml:"count" mcpsmithy:"default=1,min=0"`
}

type descTestEnum string

const (
	descTestEnumFast descTestEnum = "fast"
	descTestEnumSlow descTestEnum = "slow"
)

func (descTestEnum) Values() []string {
	return []string{string(descTestEnumFast), string(descTestEnumSlow)}
}

// descTestToken is a named non-enum string type — for SchemaDoc.Types coverage.
type descTestToken string

// descTokenHolder is a root config with a single named-string field.
type descTokenHolder struct {
	Token descTestToken `yaml:"token"`
}

// descOneOfHolder tests FieldDoc.OneOfGroups.
type descOneOfHolder struct {
	Function string `yaml:"function" mcpsmithy:"oneof=action"`
	Template string `yaml:"template" mcpsmithy:"oneof=action"`
}

// descRefHolder tests FieldDoc.Refs.
type descRefHolder struct {
	Items   map[string]descTestItem `yaml:"items"`
	ItemRef string                  `yaml:"itemRef" mcpsmithy:"ref=items"`
}

// descTestDocs is a hand-crafted DocProvider for the test types above.
var descTestDocs = DocProvider{
	Types: map[string]string{
		"descTestConfig": "Root config.",
		"descTestProj":   "Project metadata block.",
		"descTestItem":   "A single item.",
		"descTestEnum":   "descTestEnum doc.",
	},
	Fields: map[string]map[string]string{
		"descTestConfig": {"Version": "Schema version.", "Project": "Project block.", "Items": "List of items."},
		"descTestProj":   {"Name": "Project name.", "Mode": "Build mode."},
		"descTestItem":   {"Label": "Item label.", "Count": "Item count."},
	},
	Values: map[string]map[string]string{
		"descTestEnum": {"fast": "Fast mode.", "slow": "Slow mode."},
	},
}

func TestDescribe(t *testing.T) {
	doc := Describe(descTestConfig{}, "test", descTestDocs)

	t.Run("struct count", func(t *testing.T) {
		if got := len(doc.Structs); got != 3 {
			t.Fatalf("structs = %d; want 3", got)
		}
	})

	t.Run("enum count", func(t *testing.T) {
		if got := len(doc.Enums); got != 1 {
			t.Fatalf("enums = %d; want 1", got)
		}
	})

	t.Run("BFS order", func(t *testing.T) {
		if doc.Structs[0].Name != "descTestConfig" {
			t.Errorf("first struct = %q; want descTestConfig", doc.Structs[0].Name)
		}
	})

	t.Run("struct doc", func(t *testing.T) {
		if doc.Structs[0].Doc != "Root config." {
			t.Errorf("doc = %q; want %q", doc.Structs[0].Doc, "Root config.")
		}
	})

	t.Run("field metadata", func(t *testing.T) {
		var item *StructDoc
		for i := range doc.Structs {
			if doc.Structs[i].Name == "descTestItem" {
				item = &doc.Structs[i]
				break
			}
		}
		if item == nil {
			t.Fatal("descTestItem struct not found")
		}
		label := item.Fields[0]
		if label.YAMLName != "label" {
			t.Fatalf("field 0 name = %q; want label", label.YAMLName)
		}
		if label.Required != "yes" {
			t.Errorf("label.Required = %q; want yes", label.Required)
		}
		if !label.NotReserved {
			t.Error("label.NotReserved = false; want true")
		}
		if label.Description != "Item label." {
			t.Errorf("label.Description = %q; want %q", label.Description, "Item label.")
		}
		count := item.Fields[1]
		if count.Default != "1" {
			t.Errorf("count.Default = %q; want 1", count.Default)
		}
		if count.Min != "0" {
			t.Errorf("count.Min = %q; want 0", count.Min)
		}
	})

	t.Run("enum values", func(t *testing.T) {
		e := doc.Enums[0]
		if e.Name != "descTestEnum" {
			t.Fatalf("enum name = %q; want descTestEnum", e.Name)
		}
		if e.Doc != "descTestEnum doc." {
			t.Errorf("enum doc = %q; want %q", e.Doc, "descTestEnum doc.")
		}
		if len(e.Values) != 2 {
			t.Fatalf("enum values = %d; want 2", len(e.Values))
		}
		if e.Values[0].Label != "fast" || e.Values[0].Doc != "Fast mode." {
			t.Errorf("value[0] = %+v", e.Values[0])
		}
	})

	t.Run("type cross-links", func(t *testing.T) {
		projField := doc.Structs[0].Fields[1]
		if projField.Type != "descTestProj" {
			t.Errorf("project type = %q; want %q", projField.Type, "descTestProj")
		}
		if projField.Description != "Project block." {
			t.Errorf("project desc = %q; want %q", projField.Description, "Project block.")
		}
		itemsField := doc.Structs[0].Fields[2]
		if itemsField.Type != "descTestItem[]" {
			t.Errorf("items type = %q; want %q", itemsField.Type, "descTestItem[]")
		}
		if itemsField.Description != "List of items." {
			t.Errorf("items desc = %q; want %q", itemsField.Description, "List of items.")
		}
	})

	t.Run("enum field description", func(t *testing.T) {
		var proj *StructDoc
		for i := range doc.Structs {
			if doc.Structs[i].Name == "descTestProj" {
				proj = &doc.Structs[i]
				break
			}
		}
		if proj == nil {
			t.Fatal("descTestProj struct not found")
		}
		modeField := proj.Fields[1]
		if modeField.Description != "Build mode." {
			t.Errorf("mode desc = %q; want %q", modeField.Description, "Build mode.")
		}
	})
}

func TestFilterTypes(t *testing.T) {
	doc := Describe(descTestConfig{}, "test", descTestDocs)
	filtered := FilterTypes(doc, map[string]bool{"descTestItem": true, "descTestEnum": true})
	if len(filtered.Structs) != 1 || filtered.Structs[0].Name != "descTestItem" {
		t.Errorf("filtered structs = %v; want [descTestItem]", filtered.Structs)
	}
	if len(filtered.Enums) != 1 || filtered.Enums[0].Name != "descTestEnum" {
		t.Errorf("filtered enums = %v; want [descTestEnum]", filtered.Enums)
	}
}

func TestKnownTypes(t *testing.T) {
	doc := Describe(descTestConfig{}, "test", descTestDocs)
	known := doc.KnownTypes()
	for _, name := range []string{"descTestConfig", "descTestProj", "descTestItem", "descTestEnum"} {
		if !known[name] {
			t.Errorf("KnownTypes missing %q", name)
		}
	}
}

func TestDescribe_OneOfGroups(t *testing.T) {
	doc := Describe(descOneOfHolder{}, "test", DocProvider{})
	if len(doc.Structs) != 1 {
		t.Fatalf("structs = %d; want 1", len(doc.Structs))
	}
	s := doc.Structs[0]
	if len(s.Fields) != 2 {
		t.Fatalf("fields = %d; want 2", len(s.Fields))
	}
	fn := s.Fields[0]
	if fn.YAMLName != "function" {
		t.Fatalf("field[0] = %q; want function", fn.YAMLName)
	}
	if len(fn.OneOfGroups) != 1 {
		t.Fatalf("OneOfGroups = %d; want 1", len(fn.OneOfGroups))
	}
	g := fn.OneOfGroups[0]
	if g.Group != "action" {
		t.Errorf("group name = %q; want action", g.Group)
	}
	if g.Optional {
		t.Error("optional = true; want false")
	}
	if len(g.Peers) != 1 || g.Peers[0] != "template" {
		t.Errorf("peers = %v; want [template]", g.Peers)
	}
	if fn.Required != "oneof" {
		t.Errorf("required = %q; want oneof", fn.Required)
	}
}

func TestDescribe_Refs(t *testing.T) {
	doc := Describe(descRefHolder{}, "test", DocProvider{})
	var holder *StructDoc
	for i := range doc.Structs {
		if doc.Structs[i].Name == "descRefHolder" {
			holder = &doc.Structs[i]
			break
		}
	}
	if holder == nil {
		t.Fatal("descRefHolder not found")
	}
	var itemRefField *FieldDoc
	for i := range holder.Fields {
		if holder.Fields[i].YAMLName == "itemRef" {
			itemRefField = &holder.Fields[i]
			break
		}
	}
	if itemRefField == nil {
		t.Fatal("itemRef field not found")
	}
	if len(itemRefField.Refs) != 1 || itemRefField.Refs[0] != "items" {
		t.Errorf("Refs = %v; want [items]", itemRefField.Refs)
	}
}

func TestDescribe_NamedTypes(t *testing.T) {
	doc := Describe(descTokenHolder{}, "test", DocProvider{
		Types: map[string]string{"descTestToken": "A named token string."},
	})
	if len(doc.Types) != 1 {
		t.Fatalf("types = %d; want 1", len(doc.Types))
	}
	if doc.Types[0].Name != "descTestToken" {
		t.Errorf("type name = %q; want descTestToken", doc.Types[0].Name)
	}
	if doc.Types[0].Doc != "A named token string." {
		t.Errorf("type doc = %q; want %q", doc.Types[0].Doc, "A named token string.")
	}
}

func TestFriendlyTypeName(t *testing.T) {
	tests := []struct {
		t    reflect.Type
		want string
	}{
		{reflect.TypeFor[string](), "string"},
		{reflect.TypeFor[descTestEnum](), "descTestEnum"},
		{reflect.TypeFor[int](), "integer"},
		{reflect.TypeFor[float64](), "number"},
		{reflect.TypeFor[float32](), "number"},
		{reflect.TypeFor[bool](), "boolean"},
		{reflect.TypeFor[*string](), "string"},
		{reflect.TypeFor[[]string](), "string[]"},
		{reflect.TypeFor[[]descTestItem](), "descTestItem[]"},
		{reflect.TypeFor[map[string]int](), "map[string]integer"},
		{reflect.TypeFor[any](), "any"},
		{reflect.TypeFor[descTestItem](), "descTestItem"},
		{reflect.TypeFor[struct{}](), "object"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := friendlyTypeName(tt.t)
			if got != tt.want {
				t.Errorf("friendlyTypeName(%v) = %q; want %q", tt.t, got, tt.want)
			}
		})
	}
}
