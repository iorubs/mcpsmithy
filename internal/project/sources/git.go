package sources

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/operator-assistant/mcpsmithy/internal/config"
)

func init() {
	DefaultRegistry.Register("git", func(name string, raw any, _, baseDir string, global config.PullPolicy) (Source, SourceMeta, error) {
		src, ok := raw.(config.GitSource)
		if !ok {
			return nil, SourceMeta{}, fmt.Errorf("git source %q: unexpected config type %T", name, raw)
		}
		destDir := filepath.Join(baseDir, "git", name)
		return &GitSource{
				repo:    src.Repo,
				ref:     src.Ref,
				depth:   src.Depth,
				destDir: destDir,
				policy:  resolvePolicy(src.PullPolicy, global),
			}, SourceMeta{
				NoIndex:    src.Index != nil && !*src.Index,
				ReadGlobs:  src.Paths,
				ReadPrefix: src.Repo,
			}, nil
	})
}

// GitSource clones a remote git repository to disk.
type GitSource struct {
	repo, ref string
	depth     int
	destDir   string
	policy    config.PullPolicy
}

// Fetch clones the git repository to disk.
// Credentials come from the environment (SSH keys, git credential
// helpers, .netrc). If destDir already exists it is removed first
// to ensure a clean clone.
func (s *GitSource) Fetch(ctx context.Context) error {
	if skipFetch(s.policy, s.destDir) {
		return nil
	}

	depth := s.depth
	if depth <= 0 {
		depth = 1
	}

	if err := os.MkdirAll(filepath.Dir(s.destDir), 0o755); err != nil {
		return err
	}

	if err := os.RemoveAll(s.destDir); err != nil {
		return err
	}

	args := []string{"clone", "--depth", strconv.Itoa(depth)}
	if s.ref != "" {
		args = append(args, "--branch", s.ref)
	}
	args = append(args, "--single-branch", s.repo, s.destDir)

	cmd := exec.CommandContext(ctx, "git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("git clone %q: %w\n%s", s.repo, err, msg)
		}
		return fmt.Errorf("git clone %q: %w", s.repo, err)
	}
	return nil
}

func (s *GitSource) Read(globs []string, prefix string) ([]RawDoc, error) {
	return readFS(os.DirFS(s.destDir), globs, prefix)
}
