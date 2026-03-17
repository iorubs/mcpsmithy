package sources

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	"github.com/operator-assistant/mcpsmithy/internal/config"
)

func init() {
	DefaultRegistry.Register("local", func(name string, raw any, projectRoot, _ string, _ config.PullPolicy) (Source, SourceMeta, error) {
		src, ok := raw.(config.LocalSource)
		if !ok {
			return nil, SourceMeta{}, fmt.Errorf("local source %q: unexpected config type %T", name, raw)
		}
		return &LocalSource{fsys: os.DirFS(projectRoot)}, SourceMeta{
			NoIndex:   src.Index != nil && !*src.Index,
			ReadGlobs: src.Paths,
		}, nil
	})
}

// LocalSource serves files directly from an existing filesystem; Fetch is a no-op.
type LocalSource struct{ fsys fs.FS }

func (s *LocalSource) Fetch(_ context.Context) error { return nil }
func (s *LocalSource) Read(globs []string, prefix string) ([]RawDoc, error) {
	return readFS(s.fsys, globs, prefix)
}
