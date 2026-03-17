package tools

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// sandbox restricts file operations to a project root.
type sandbox struct {
	root string
	fsys fs.FS
}

// newSandbox creates a sandbox rooted at dir.
func newSandbox(dir string) (*sandbox, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("abs: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			resolved = abs
		} else {
			return nil, err
		}
	}
	return &sandbox{root: resolved, fsys: os.DirFS(resolved)}, nil
}

func (s *sandbox) Root() string { return s.root }

func (s *sandbox) Resolve(path string) (string, error) {
	var abs string
	if filepath.IsAbs(path) {
		abs = path
	} else {
		abs = filepath.Join(s.root, path)
	}
	abs = filepath.Clean(abs)

	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			resolved = abs
		} else {
			return "", err
		}
	}

	if resolved != s.root && !strings.HasPrefix(resolved, s.root+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside project root %q", path, s.root)
	}
	return resolved, nil
}

func (s *sandbox) ValidateFilePath(path string) error {
	_, err := s.Resolve(path)
	if err != nil {
		return err
	}
	// Check existence via the sandbox's fs.FS using the relative path.
	rel := path
	if filepath.IsAbs(path) {
		rel, err = filepath.Rel(s.root, filepath.Clean(path))
		if err != nil {
			return err
		}
	}
	if _, err := fs.Stat(s.fsys, rel); err != nil {
		return fmt.Errorf("path %q: %w", path, err)
	}
	return nil
}

// globFS returns relative paths matching pattern against the given fs.FS.
// Supports ** patterns via directory walk.
func globFS(fsys fs.FS, pattern string) ([]string, error) {
	if strings.Contains(pattern, "**") {
		return expandGlobFS(fsys, pattern), nil
	}
	return fs.Glob(fsys, pattern)
}

// expandGlobFS handles ** patterns by walking an fs.FS.
func expandGlobFS(fsys fs.FS, pattern string) []string {
	parts := strings.SplitN(pattern, "**", 2)
	prefix := strings.TrimSuffix(parts[0], "/")
	suffix := ""
	if len(parts) > 1 {
		suffix = strings.TrimPrefix(parts[1], "/")
	}

	walkRoot := "."
	if prefix != "" {
		walkRoot = prefix
	}

	var matches []string
	// Walk error is non-fatal: root may not exist when the pattern
	// doesn't match any directory. Return whatever matches we found.
	_ = fs.WalkDir(fsys, walkRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}
		if d.IsDir() {
			return nil
		}
		if suffix == "" {
			matches = append(matches, path)
		} else {
			// Compute relative path from walkRoot so multi-segment suffixes
			// like "v1/*.go" match correctly (filepath.Base would strip dirs).
			rel := path
			if walkRoot != "." {
				rel = strings.TrimPrefix(path, walkRoot+"/")
			}
			n := strings.Count(suffix, "/") + 1
			parts := strings.Split(rel, "/")
			if len(parts) >= n {
				tail := strings.Join(parts[len(parts)-n:], "/")
				if ok, _ := filepath.Match(suffix, tail); ok {
					matches = append(matches, path)
				}
			}
		}
		return nil
	})
	return matches
}
