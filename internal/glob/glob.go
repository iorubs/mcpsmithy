// Package glob provides shared glob-to-regexp compilation used by the
// sources and conventions packages.
package glob

import (
	"io/fs"
	"regexp"
	"strings"
)

// ToRegexp compiles a glob pattern into a regexp.
// ** matches any number of path segments; * matches within a single segment.
func ToRegexp(pattern string) *regexp.Regexp {
	r := regexp.QuoteMeta(pattern)
	r = strings.ReplaceAll(r, `\*\*`, `.*`)
	r = strings.ReplaceAll(r, `\*`, `[^/]*`)
	return regexp.MustCompile(`^` + r + `$`)
}

// WalkFS returns relative paths within fsys that match a ** glob pattern.
// ** matches any number of path segments; * matches within a single segment.
func WalkFS(fsys fs.FS, pattern string) ([]string, error) {
	re := ToRegexp(pattern)
	var matches []string
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if re.MatchString(path) {
			matches = append(matches, path)
		}
		return nil
	})
	return matches, err
}
