package schema

import (
	"strconv"
	"strings"
)

// oneOfEntry represents a single oneof group membership.
type oneOfEntry struct {
	Group    string // group name
	Optional bool   // true for oneof?= (at-most-one); false for oneof= (exactly-one)
}

// tagInfo holds the parsed contents of a single mcpsmithy struct tag.
type tagInfo struct {
	Required    bool
	Default     string       // empty when no default is specified
	OneOfs      []oneOfEntry // all oneof/oneof? groups this field belongs to
	Min         *int         // minimum value for int fields (nil when absent)
	NotReserved bool         // value must not appear in ReservedContextKeys
	Refs        []string     // ref=path1|path2 — value must appear in keys of at least one navigated map
	TypedAs     string       // typed-as=fieldName — sibling field whose value (a TypeClassifier) governs type compatibility
}

// parseTag parses the struct tag value.
// Recognised directives (comma-separated, any order): required,
// default=VALUE, oneof=GROUP, oneof?=G1|G2 (at-most-one), min=N,
// notreserved, ref=path1|path2, typed-as=FIELD.
// A field may belong to multiple oneof groups via pipe-separated names in
// a single directive: oneof?=g1|g2 or oneof=g1|g2.
// Fields with no directives may omit the tag entirely.
func parseTag(raw string) tagInfo {
	var info tagInfo
	for p := range strings.SplitSeq(raw, ",") {
		switch {
		case p == "required":
			info.Required = true
		case strings.HasPrefix(p, "default="):
			info.Default = strings.TrimPrefix(p, "default=")
		case strings.HasPrefix(p, "oneof?="):
			for group := range strings.SplitSeq(strings.TrimPrefix(p, "oneof?="), "|") {
				info.OneOfs = append(info.OneOfs, oneOfEntry{Group: group, Optional: true})
			}
		case strings.HasPrefix(p, "oneof="):
			for group := range strings.SplitSeq(strings.TrimPrefix(p, "oneof="), "|") {
				info.OneOfs = append(info.OneOfs, oneOfEntry{Group: group, Optional: false})
			}
		case strings.HasPrefix(p, "min="):
			if n, err := strconv.Atoi(strings.TrimPrefix(p, "min=")); err == nil {
				info.Min = &n
			}
		case p == "notreserved":
			info.NotReserved = true
		case strings.HasPrefix(p, "ref="):
			info.Refs = strings.Split(strings.TrimPrefix(p, "ref="), "|")
		case strings.HasPrefix(p, "typed-as="):
			info.TypedAs = strings.TrimPrefix(p, "typed-as=")
		}
	}
	return info
}
