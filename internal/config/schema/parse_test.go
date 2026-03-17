package schema

import (
	"testing"
)

// testTypeSrc is a self-contained Go source string used across doc-parsing tests.
const testTypeSrc = `package p

// Config is the root config.
type Config struct {
	// Name is the server name.
	Name string
	// Port is the listen port.
	Port int
}

// Mode controls operation mode.
type Mode string

const (
	// ModeA activates mode A.
	ModeA Mode = "a"
	// ModeB activates mode B.
	ModeB Mode = "b"
)

type Undocumented struct {
	NoDoc string
}
`

func TestParseTypeDocs(t *testing.T) {
	docs := ParseTypeDocs(testTypeSrc)

	t.Run("type doc", func(t *testing.T) {
		if got := docs.Types["Config"]; got != "Config is the root config." {
			t.Errorf("Config doc = %q; want %q", got, "Config is the root config.")
		}
		if got := docs.Types["Mode"]; got != "Mode controls operation mode." {
			t.Errorf("Mode doc = %q; want %q", got, "Mode controls operation mode.")
		}
	})

	t.Run("undocumented type absent", func(t *testing.T) {
		if _, ok := docs.Types["Undocumented"]; ok {
			t.Error("undocumented type should not appear in Types map")
		}
	})

	t.Run("field doc", func(t *testing.T) {
		fields := docs.Fields["Config"]
		if fields == nil {
			t.Fatal("Config fields not found")
		}
		if got := fields["Name"]; got != "Name is the server name." {
			t.Errorf("Name field doc = %q; want %q", got, "Name is the server name.")
		}
		if got := fields["Port"]; got != "Port is the listen port." {
			t.Errorf("Port field doc = %q; want %q", got, "Port is the listen port.")
		}
	})

	t.Run("undocumented field absent", func(t *testing.T) {
		if f := docs.Fields["Undocumented"]; f != nil {
			t.Errorf("undocumented struct should have no field docs, got %v", f)
		}
	})

	t.Run("enum value doc", func(t *testing.T) {
		values := docs.Values["Mode"]
		if values == nil {
			t.Fatal("Mode values not found")
		}
		if got := values["a"]; got != "ModeA activates mode A." {
			t.Errorf("value 'a' doc = %q; want %q", got, "ModeA activates mode A.")
		}
		if got := values["b"]; got != "ModeB activates mode B." {
			t.Errorf("value 'b' doc = %q; want %q", got, "ModeB activates mode B.")
		}
	})

	t.Run("multi-source later wins", func(t *testing.T) {
		src1 := `package p

// Config is the original.
type Config struct{}
`
		src2 := `package p

// Config is the override.
type Config struct{}
`
		docs := ParseTypeDocs(src1, src2)
		if got := docs.Types["Config"]; got != "Config is the override." {
			t.Errorf("multi-source: later source should win, got %q", got)
		}
	})

	t.Run("multi-source merges distinct types", func(t *testing.T) {
		src1 := `package p

// Foo is from source 1.
type Foo struct{}
`
		src2 := `package p

// Bar is from source 2.
type Bar struct{}
`
		docs := ParseTypeDocs(src1, src2)
		if got := docs.Types["Foo"]; got != "Foo is from source 1." {
			t.Errorf("Foo doc = %q; want %q", got, "Foo is from source 1.")
		}
		if got := docs.Types["Bar"]; got != "Bar is from source 2." {
			t.Errorf("Bar doc = %q; want %q", got, "Bar is from source 2.")
		}
	})

	t.Run("comment without space", func(t *testing.T) {
		// The parser accepts both "// text" and "//text" comment styles.
		src := `package p

//CompactDoc no leading space.
type Compact struct{}
`
		docs := ParseTypeDocs(src)
		if got := docs.Types["Compact"]; got != "CompactDoc no leading space." {
			t.Errorf("compact comment = %q; want %q", got, "CompactDoc no leading space.")
		}
	})

	t.Run("embedded ParamType source", func(t *testing.T) {
		// ParamTypeSource is embedded at compile time and must parse without panic.
		docs := ParseTypeDocs(ParamTypeSource)
		if docs.Types["ParamType"] == "" {
			t.Error("ParamType doc should be present in the embedded source")
		}
		// The embedded source should also include enum value docs.
		if docs.Values["ParamType"] == nil {
			t.Error("ParamType enum values should be present in the embedded source")
		}
	})
}
