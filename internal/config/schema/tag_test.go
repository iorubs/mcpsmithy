package schema

import (
	"testing"
)

func TestParseTag(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		wantReq     bool
		wantDef     string
		wantOneOfs  []oneOfEntry
		wantMin     *int
		wantNotRes  bool
		wantRefs    []string
		wantTypedAs string
	}{
		{"required", "required", true, "", nil, nil, false, nil, ""},
		{"bare optional", "-", false, "", nil, nil, false, nil, ""},
		{"default", "default=foo", false, "foo", nil, nil, false, nil, ""},
		{"oneof", "oneof=mode", false, "", []oneOfEntry{{Group: "mode", Optional: false}}, nil, false, nil, ""},
		{"oneof?", "oneof?=grp", false, "", []oneOfEntry{{Group: "grp", Optional: true}}, nil, false, nil, ""},
		{"oneof? multi", "oneof?=g1|g2", false, "", []oneOfEntry{{Group: "g1", Optional: true}, {Group: "g2", Optional: true}}, nil, false, nil, ""},
		{"oneof multi", "oneof=g1|g2", false, "", []oneOfEntry{{Group: "g1", Optional: false}, {Group: "g2", Optional: false}}, nil, false, nil, ""},
		{"default+min", "default=20,min=0", false, "20", nil, new(0), false, nil, ""},
		{"notreserved", "required,notreserved", true, "", nil, nil, true, nil, ""},
		{"ref single", "required,ref=conventions", true, "", nil, nil, false, []string{"conventions"}, ""},
		{"ref multi", "required,ref=sources.local|sources.scrape", true, "", nil, nil, false, []string{"sources.local", "sources.scrape"}, ""},
		{"typed-as", "typed-as=type", false, "", nil, nil, false, nil, "type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := parseTag(tt.raw)
			if info.Required != tt.wantReq {
				t.Errorf("required = %v; want %v", info.Required, tt.wantReq)
			}
			if info.Default != tt.wantDef {
				t.Errorf("default = %q; want %q", info.Default, tt.wantDef)
			}
			if len(info.OneOfs) != len(tt.wantOneOfs) {
				t.Errorf("oneOfs = %v; want %v", info.OneOfs, tt.wantOneOfs)
			} else {
				for i := range info.OneOfs {
					if info.OneOfs[i] != tt.wantOneOfs[i] {
						t.Errorf("oneOfs[%d] = %v; want %v", i, info.OneOfs[i], tt.wantOneOfs[i])
					}
				}
			}
			switch {
			case info.Min == nil && tt.wantMin == nil:
			case info.Min != nil && tt.wantMin != nil && *info.Min == *tt.wantMin:
			default:
				t.Errorf("min = %v; want %v", info.Min, tt.wantMin)
			}
			if info.NotReserved != tt.wantNotRes {
				t.Errorf("notReserved = %v; want %v", info.NotReserved, tt.wantNotRes)
			}
			if len(info.Refs) != len(tt.wantRefs) {
				t.Errorf("refs = %v; want %v", info.Refs, tt.wantRefs)
			} else {
				for i := range info.Refs {
					if info.Refs[i] != tt.wantRefs[i] {
						t.Errorf("refs[%d] = %q; want %q", i, info.Refs[i], tt.wantRefs[i])
					}
				}
			}
			if info.TypedAs != tt.wantTypedAs {
				t.Errorf("typedAs = %q; want %q", info.TypedAs, tt.wantTypedAs)
			}
		})
	}
}
