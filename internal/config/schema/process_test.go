package schema

import (
	"errors"
	"strings"
	"testing"
)

// ----- test types -----

// testEnum is a named string type implementing Valuer.
type testEnum string

const (
	testEnumA testEnum = "alpha"
	testEnumB testEnum = "beta"
)

func (testEnum) Values() []string {
	return []string{string(testEnumA), string(testEnumB)}
}

type inner struct {
	Port int `yaml:"port" mcpsmithy:"default=8080"`
}

type mapEntry struct {
	Size int    `yaml:"size" mcpsmithy:"default=2048"`
	Mode string `yaml:"mode" mcpsmithy:"default=archive"`
}

type sliceEntry struct {
	Enabled *bool `yaml:"enabled" mcpsmithy:"default=true"`
}

type mapRequired struct {
	Name string `yaml:"name" mcpsmithy:"required"`
	Tag  string `yaml:"tag"`
}

type enumHolder struct {
	Policy testEnum `yaml:"policy" mcpsmithy:"default=alpha"`
	Label  string   `yaml:"label"`
}

// sample covers defaults, required, and nested struct/map/slice traversal.
type sample struct {
	Name    string              `yaml:"name"    mcpsmithy:"required"`
	Mode    string              `yaml:"mode"    mcpsmithy:"default=archive"`
	Count   int                 `yaml:"count"   mcpsmithy:"default=3"`
	Verbose *bool               `yaml:"verbose" mcpsmithy:"default=true"`
	Nested  inner               `yaml:"nested"`
	Entries map[string]mapEntry `yaml:"entries,omitempty"`
	Items   []sliceEntry        `yaml:"items,omitempty"`
}

type sampleWithEnums struct {
	Name    string                `yaml:"name"    mcpsmithy:"required"`
	Policy  testEnum              `yaml:"policy"  mcpsmithy:"default=alpha"`
	Sources map[string]enumHolder `yaml:"sources,omitempty"`
	List    []enumHolder          `yaml:"list,omitempty"`
}

type sampleWithRequiredMap struct {
	Entries map[string]mapRequired `yaml:"entries,omitempty"`
	Items   []mapRequired          `yaml:"items,omitempty"`
}

// oneofEntry has two mutually-exclusive fields in the same group.
type oneofEntry struct {
	Desc     string `yaml:"desc"     mcpsmithy:"required"`
	Function string `yaml:"function" mcpsmithy:"oneof=mode"`
	Template string `yaml:"template" mcpsmithy:"oneof=mode"`
}

type sampleWithOneOf struct {
	Tools map[string]oneofEntry `yaml:"tools,omitempty"`
	Items []oneofEntry          `yaml:"items,omitempty"`
}

// minEntry has an int field with a minimum bound and a default.
type minEntry struct {
	MaxPages int    `yaml:"maxPages" mcpsmithy:"default=20,min=0"`
	Label    string `yaml:"label"`
}

type sampleWithMin struct {
	Sources map[string]minEntry `yaml:"sources,omitempty"`
	Items   []minEntry          `yaml:"items,omitempty"`
}

// oneofOptEntry has three fields across two oneof? groups.
// enum conflicts with min (no_enum_with_min) and max (no_enum_with_max).
// min and max share no group — they coexist freely.
type oneofOptEntry struct {
	Enum []any    `yaml:"enum,omitempty"  mcpsmithy:"oneof?=no_enum_with_min|no_enum_with_max"`
	Min  *float64 `yaml:"min,omitempty"   mcpsmithy:"oneof?=no_enum_with_min"`
	Max  *float64 `yaml:"max,omitempty"   mcpsmithy:"oneof?=no_enum_with_max"`
}

type sampleWithOneOfOpt struct {
	Constraints map[string]oneofOptEntry `yaml:"constraints,omitempty"`
}

// ── test type classifier (for typed-as= tests) ─────────────────────────────

// typedAsConstraints mirrors a constraints struct for typed-as testing.
type typedAsConstraints struct {
	Enum []any    `yaml:"enum,omitempty" mcpsmithy:"oneof?=no_enum_with_min|no_enum_with_max"`
	Min  *float64 `yaml:"min,omitempty"  mcpsmithy:"oneof?=no_enum_with_min"`
	Max  *float64 `yaml:"max,omitempty"  mcpsmithy:"oneof?=no_enum_with_max"`
}

// typedAsParam uses the real ParamType (now in this package) for typed-as testing.
type typedAsParam struct {
	Name        string              `yaml:"name"        mcpsmithy:"required"`
	Type        ParamType           `yaml:"type"        mcpsmithy:"required"`
	Default     any                 `yaml:"default"     mcpsmithy:"typed-as=type"`
	Constraints *typedAsConstraints `yaml:"constraints,omitempty" mcpsmithy:"typed-as=type"`
}

type sampleWithTypedAs struct {
	Params []typedAsParam `yaml:"params,omitempty"`
}

// hasMsg reports whether msg appears in the list.
func hasMsg(msgs []error, sub string) bool {
	for _, m := range msgs {
		if strings.Contains(m.Error(), sub) {
			return true
		}
	}
	return false
}

// ----- defaults -----

func TestProcess_Defaults(t *testing.T) {
	f := false
	tests := []struct {
		name     string
		input    sample
		wantMode string
		wantCnt  int
		wantVerb bool
		wantPort int
	}{
		{"fills zero values", sample{}, "archive", 3, true, 8080},
		{"preserves explicit values", sample{Mode: "clone", Count: 42, Verbose: &f, Nested: inner{Port: 9090}}, "clone", 42, false, 9090},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.input
			Process(&s)
			if s.Mode != tt.wantMode {
				t.Errorf("Mode = %q; want %q", s.Mode, tt.wantMode)
			}
			if s.Count != tt.wantCnt {
				t.Errorf("Count = %d; want %d", s.Count, tt.wantCnt)
			}
			if s.Verbose == nil || *s.Verbose != tt.wantVerb {
				t.Errorf("Verbose = %v; want %v", s.Verbose, tt.wantVerb)
			}
			if s.Nested.Port != tt.wantPort {
				t.Errorf("Nested.Port = %d; want %d", s.Nested.Port, tt.wantPort)
			}
		})
	}
}

func TestProcess_NilPointer(t *testing.T) {
	if errs := Process((*sample)(nil)); errs != nil {
		t.Errorf("nil pointer: got errors %v; want none", errs)
	}
}

func TestProcess_DefaultsInMapValues(t *testing.T) {
	s := sample{
		Entries: map[string]mapEntry{
			"a": {},
			"b": {Size: 512},
		},
	}
	Process(&s)

	a := s.Entries["a"]
	if a.Size != 2048 {
		t.Errorf("a.Size = %d; want 2048", a.Size)
	}
	if a.Mode != "archive" {
		t.Errorf("a.Mode = %q; want archive", a.Mode)
	}
	b := s.Entries["b"]
	if b.Size != 512 {
		t.Errorf("b.Size = %d; want 512 (explicit)", b.Size)
	}
}

func TestProcess_DefaultsInSliceElements(t *testing.T) {
	s := sample{Items: []sliceEntry{{}, {}}}
	Process(&s)

	for i, item := range s.Items {
		if item.Enabled == nil || !*item.Enabled {
			t.Errorf("Items[%d].Enabled should default to true", i)
		}
	}
}

func TestProcess_NilMapAndSliceUntouched(t *testing.T) {
	s := sample{}
	Process(&s)
	if s.Entries != nil {
		t.Error("nil map should stay nil")
	}
	if s.Items != nil {
		t.Error("nil slice should stay nil")
	}
}

// ----- required -----

func TestProcess_Required(t *testing.T) {
	tests := []struct {
		name      string
		val       any
		wantError string
	}{
		{"missing top-level", &sample{}, "name is required"},
		{"present top-level", &sample{Name: "ok"}, ""},
		{"missing in map entry", &sampleWithRequiredMap{
			Entries: map[string]mapRequired{"a": {Name: "ok"}, "b": {}},
		}, "entries[b].name is required"},
		{"missing in slice entry", &sampleWithRequiredMap{
			Items: []mapRequired{{Name: "ok"}, {}},
		}, "items[1].name is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Process(tt.val)
			if tt.wantError == "" {
				if len(errs) != 0 {
					t.Errorf("expected no errors; got %v", errs)
				}
				return
			}
			if !hasMsg(errs, tt.wantError) {
				t.Errorf("expected error %q; got %v", tt.wantError, errs)
			}
		})
	}
}

// ----- enum / Valuer -----

func TestProcess_EnumValidation(t *testing.T) {
	tests := []struct {
		name     string
		val      any
		wantErrs int
		wantMsg  string
	}{
		{"valid", &sampleWithEnums{Name: "ok", Policy: testEnumA}, 0, ""},
		{"invalid", &sampleWithEnums{Name: "ok", Policy: "bad"}, 1, `policy: must be one of [alpha, beta], got "bad"`},
		{"zero gets default — skipped", &sampleWithEnums{Name: "ok"}, 0, ""},
		{"invalid in map", &sampleWithEnums{Name: "ok", Sources: map[string]enumHolder{
			"a": {Policy: testEnumB}, "b": {Policy: "nope"},
		}}, 1, `sources[b].policy: must be one of [alpha, beta], got "nope"`},
		{"invalid in slice", &sampleWithEnums{Name: "ok", List: []enumHolder{
			{Policy: testEnumA}, {Policy: "wrong"},
		}}, 1, `list[1].policy: must be one of [alpha, beta], got "wrong"`},
		{"nil", (*sampleWithEnums)(nil), 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Process(tt.val)
			if len(errs) != tt.wantErrs {
				t.Fatalf("got %d errors; want %d: %v", len(errs), tt.wantErrs, errs)
			}
			if tt.wantMsg != "" && !hasMsg(errs, tt.wantMsg) {
				t.Errorf("expected %q; got %v", tt.wantMsg, errs)
			}
		})
	}
}

// ----- oneof (errors) -----

func TestProcess_OneOf(t *testing.T) {
	tests := []struct {
		name     string
		val      any
		wantErrs int
		wantMsg  string
	}{
		{"exactly one", &oneofEntry{Desc: "ok", Function: "search_for"}, 0, ""},
		{"neither", &oneofEntry{Desc: "ok"}, 1, "function/template: must set one of [function, template]"},
		{"both", &oneofEntry{Desc: "ok", Function: "f", Template: "t"}, 1, "function/template: function and template are mutually exclusive"},
		{"invalid in map", &sampleWithOneOf{Tools: map[string]oneofEntry{
			"good": {Desc: "ok", Function: "f"}, "bad": {Desc: "oops"},
		}}, 1, "tools[bad]: must set one of [function, template]"},
		{"invalid in slice", &sampleWithOneOf{Items: []oneofEntry{
			{Desc: "ok", Template: "t"}, {Desc: "oops"},
		}}, 1, "items[1]: must set one of [function, template]"},
		{"nil", (*sampleWithOneOf)(nil), 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Process(tt.val)
			if len(errs) != tt.wantErrs {
				t.Fatalf("got %d errors; want %d: %v", len(errs), tt.wantErrs, errs)
			}
			if tt.wantMsg != "" && !hasMsg(errs, tt.wantMsg) {
				t.Errorf("expected %q; got %v", tt.wantMsg, errs)
			}
		})
	}
}

// ----- min -----

func TestProcess_Min(t *testing.T) {
	tests := []struct {
		name     string
		val      any
		wantErrs int
		wantMsg  string
	}{
		{"valid", &minEntry{MaxPages: 10}, 0, ""},
		{"zero gets default 20 — valid", &minEntry{}, 0, ""},
		{"below min", &minEntry{MaxPages: -1}, 1, "maxPages: must be >= 0, got -1"},
		{"invalid in map", &sampleWithMin{Sources: map[string]minEntry{
			"good": {MaxPages: 5}, "bad": {MaxPages: -2},
		}}, 1, "sources[bad].maxPages: must be >= 0, got -2"},
		{"invalid in slice", &sampleWithMin{Items: []minEntry{
			{MaxPages: 10}, {MaxPages: -3},
		}}, 1, "items[1].maxPages: must be >= 0, got -3"},
		{"nil", (*sampleWithMin)(nil), 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Process(tt.val)
			if len(errs) != tt.wantErrs {
				t.Fatalf("got %d errors; want %d: %v", len(errs), tt.wantErrs, errs)
			}
			if tt.wantMsg != "" && !hasMsg(errs, tt.wantMsg) {
				t.Errorf("expected %q; got %v", tt.wantMsg, errs)
			}
		})
	}
}

// ----- oneof? (at-most-one, errors) -----

func TestProcess_OneOfOptional(t *testing.T) {
	f := func(v float64) *float64 { return &v }

	tests := []struct {
		name     string
		val      any
		wantErrs int
		wantMsg  string
	}{
		{"all empty — valid", &oneofOptEntry{}, 0, ""},
		{"enum only — valid", &oneofOptEntry{Enum: []any{"a", "b"}}, 0, ""},
		{"min only — valid", &oneofOptEntry{Min: f(1)}, 0, ""},
		{"max only — valid", &oneofOptEntry{Max: f(100)}, 0, ""},
		{"min+max — valid", &oneofOptEntry{Min: f(1), Max: f(100)}, 0, ""},
		{"enum+min — error", &oneofOptEntry{Enum: []any{"a"}, Min: f(1)}, 1, "enum and min are mutually exclusive"},
		{"enum+max — error", &oneofOptEntry{Enum: []any{"a"}, Max: f(100)}, 1, "enum and max are mutually exclusive"},
		{"enum+min+max — two errors", &oneofOptEntry{Enum: []any{"a"}, Min: f(1), Max: f(100)}, 2, ""},
		{"in map — valid", &sampleWithOneOfOpt{Constraints: map[string]oneofOptEntry{
			"ok": {Min: f(0), Max: f(10)},
		}}, 0, ""},
		{"in map — error", &sampleWithOneOfOpt{Constraints: map[string]oneofOptEntry{
			"bad": {Enum: []any{"x"}, Min: f(0)},
		}}, 1, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Process(tt.val)
			if len(errs) != tt.wantErrs {
				t.Fatalf("got %d errors; want %d: %v", len(errs), tt.wantErrs, errs)
			}
			if tt.wantMsg != "" && !hasMsg(errs, tt.wantMsg) {
				t.Errorf("expected %q; got %v", tt.wantMsg, errs)
			}
		})
	}
}

// ----- typed-as (cross-field type compatibility) -----

func TestProcess_TypedAs(t *testing.T) {
	f := func(v float64) *float64 { return &v }

	tests := []struct {
		name     string
		val      any
		wantErrs int
		wantMsg  string
	}{
		// ── default type compatibility ──
		{"valid string default", &sampleWithTypedAs{Params: []typedAsParam{
			{Name: "q", Type: ParamTypeString, Default: "hello"},
		}}, 0, ""},
		{"valid int default", &sampleWithTypedAs{Params: []typedAsParam{
			{Name: "n", Type: ParamTypeNumber, Default: 42},
		}}, 0, ""},
		{"valid bool default", &sampleWithTypedAs{Params: []typedAsParam{
			{Name: "v", Type: ParamTypeBool, Default: true},
		}}, 0, ""},
		{"string default for number", &sampleWithTypedAs{Params: []typedAsParam{
			{Name: "n", Type: ParamTypeNumber, Default: "42"},
		}}, 1, "default"},
		{"string default for bool", &sampleWithTypedAs{Params: []typedAsParam{
			{Name: "v", Type: ParamTypeBool, Default: "true"},
		}}, 1, "default"},
		{"int default for string", &sampleWithTypedAs{Params: []typedAsParam{
			{Name: "q", Type: ParamTypeString, Default: 42},
		}}, 1, "default"},
		// ── constraints type compatibility ──
		{"valid min/max on numeric", &sampleWithTypedAs{Params: []typedAsParam{
			{Name: "n", Type: ParamTypeNumber, Constraints: &typedAsConstraints{Min: f(1), Max: f(100)}},
		}}, 0, ""},
		{"min on non-numeric type", &sampleWithTypedAs{Params: []typedAsParam{
			{Name: "q", Type: ParamTypeString, Constraints: &typedAsConstraints{Min: f(1)}},
		}}, 1, "min"},
		{"min greater than max", &sampleWithTypedAs{Params: []typedAsParam{
			{Name: "n", Type: ParamTypeNumber, Constraints: &typedAsConstraints{Min: f(100), Max: f(1)}},
		}}, 1, "min"},
		{"string enum with non-string value", &sampleWithTypedAs{Params: []typedAsParam{
			{Name: "q", Type: ParamTypeString, Constraints: &typedAsConstraints{Enum: []any{"dev", 42}}},
		}}, 1, "enum"},
		{"number enum with float", &sampleWithTypedAs{Params: []typedAsParam{
			{Name: "n", Type: ParamTypeNumber, Constraints: &typedAsConstraints{Enum: []any{1, 2.5}}},
		}}, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Process(tt.val)
			if len(errs) != tt.wantErrs {
				t.Fatalf("got %d errors; want %d: %v", len(errs), tt.wantErrs, errs)
			}
			if tt.wantMsg != "" && !hasMsg(errs, tt.wantMsg) {
				t.Errorf("expected %q; got %v", tt.wantMsg, errs)
			}
		})
	}
}

// ----- notreserved -----

// notReservedEntry has a field that must not be a reserved context key name.
type notReservedEntry struct {
	Name string `yaml:"name" mcpsmithy:"notreserved"`
}

func TestProcess_NotReserved(t *testing.T) {
	tests := []struct {
		name     string
		val      any
		wantErrs int
		wantMsg  string
	}{
		{"non-reserved name", &notReservedEntry{Name: "myparam"}, 0, ""},
		{"reserved name", &notReservedEntry{Name: "mcpsmithy"}, 1, "reserved"},
		{"nil", (*notReservedEntry)(nil), 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Process(tt.val)
			if len(errs) != tt.wantErrs {
				t.Fatalf("got %d errors; want %d: %v", len(errs), tt.wantErrs, errs)
			}
			if tt.wantMsg != "" && !hasMsg(errs, tt.wantMsg) {
				t.Errorf("expected %q; got %v", tt.wantMsg, errs)
			}
		})
	}
}

// ----- ref= -----

// refTool is a minimal map-entry type for ref= testing.
type refTool struct {
	Name string `yaml:"name"`
}

// refHolder has a string field that must reference a key in its own tools map.
type refHolder struct {
	Tools   map[string]refTool `yaml:"tools"`
	ToolRef string             `yaml:"tool" mcpsmithy:"ref=tools"`
}

func TestProcess_Ref(t *testing.T) {
	tests := []struct {
		name     string
		val      any
		wantErrs int
		wantMsg  string
	}{
		{"valid ref", &refHolder{
			Tools:   map[string]refTool{"search_for": {}},
			ToolRef: "search_for",
		}, 0, ""},
		{"invalid ref", &refHolder{
			Tools:   map[string]refTool{"search_for": {}},
			ToolRef: "missing",
		}, 1, `"missing" does not match any declared key`},
		{"no keys to validate against", &refHolder{ToolRef: "anything"}, 0, ""},
		{"zero ref value skipped", &refHolder{
			Tools: map[string]refTool{"search_for": {}},
		}, 0, ""},
		{"nil", (*refHolder)(nil), 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Process(tt.val)
			if len(errs) != tt.wantErrs {
				t.Fatalf("got %d errors; want %d: %v", len(errs), tt.wantErrs, errs)
			}
			if tt.wantMsg != "" && !hasMsg(errs, tt.wantMsg) {
				t.Errorf("expected %q; got %v", tt.wantMsg, errs)
			}
		})
	}
}

// ----- Validator interface (struct-level) -----

// validatedEntry implements Validator to test cross-field invariant checking.
type validatedEntry struct {
	Name string `yaml:"name" mcpsmithy:"required"`
}

func (v validatedEntry) Validate() error {
	if v.Name == "bad" {
		return errors.New("name is bad")
	}
	return nil
}

type sampleWithValidator struct {
	Items []validatedEntry `yaml:"items"`
}

// validatedLeafField is a named string type whose value is validated at the
// leaf level (step 4 in Process), distinct from the struct-level Validate path.
type validatedLeafField string

func (v validatedLeafField) Validate() error {
	if v == "bad" {
		return errors.New("leaf value is bad")
	}
	return nil
}

type sampleWithLeafValidator struct {
	Token validatedLeafField `yaml:"token" mcpsmithy:"required"`
}

func TestProcess_ValidatorInterface(t *testing.T) {
	tests := []struct {
		name     string
		val      any
		wantErrs int
		wantMsg  string
	}{
		{"valid", &validatedEntry{Name: "good"}, 0, ""},
		{"invalid", &validatedEntry{Name: "bad"}, 1, "name is bad"},
		{"invalid in slice", &sampleWithValidator{
			Items: []validatedEntry{{Name: "good"}, {Name: "bad"}},
		}, 1, "items[1]"},
		{"nil", (*validatedEntry)(nil), 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Process(tt.val)
			if len(errs) != tt.wantErrs {
				t.Fatalf("got %d errors; want %d: %v", len(errs), tt.wantErrs, errs)
			}
			if tt.wantMsg != "" && !hasMsg(errs, tt.wantMsg) {
				t.Errorf("expected %q; got %v", tt.wantMsg, errs)
			}
		})
	}
}

func TestProcess_LeafValidatorInterface(t *testing.T) {
	tests := []struct {
		name     string
		val      any
		wantErrs int
		wantMsg  string
	}{
		{"valid leaf", &sampleWithLeafValidator{Token: "good"}, 0, ""},
		{"invalid leaf", &sampleWithLeafValidator{Token: "bad"}, 1, "leaf value is bad"},
		{"zero leaf — required fires, Validate skipped", &sampleWithLeafValidator{}, 1, "token is required"},
		{"nil", (*sampleWithLeafValidator)(nil), 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Process(tt.val)
			if len(errs) != tt.wantErrs {
				t.Fatalf("got %d errors; want %d: %v", len(errs), tt.wantErrs, errs)
			}
			if tt.wantMsg != "" && !hasMsg(errs, tt.wantMsg) {
				t.Errorf("expected %q; got %v", tt.wantMsg, errs)
			}
		})
	}
}
