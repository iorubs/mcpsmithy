package sources

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/operator-assistant/mcpsmithy/internal/config"
)

func TestGitSourceFetch_SkipPolicies(t *testing.T) {
	tests := []struct {
		name      string
		policy    config.PullPolicy
		dirExists bool
		wantSkip  bool
	}{
		{"ifNotPresent, dir exists → skip", config.PullPolicyIfNotPresent, true, true},
		{"never → always skip", config.PullPolicyNever, false, true},
		{"never, dir exists → skip", config.PullPolicyNever, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := t.TempDir()
			destDir := filepath.Join(base, "repo")
			if tt.dirExists {
				if err := os.MkdirAll(destDir, 0o755); err != nil {
					t.Fatal(err)
				}
				// Write a sentinel file so we can confirm it was not removed.
				if err := os.WriteFile(filepath.Join(destDir, "sentinel"), []byte("ok"), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			src := &GitSource{
				repo:    "https://github.com/example/does-not-exist",
				destDir: destDir,
				policy:  tt.policy,
			}

			if err := src.Fetch(context.Background()); err != nil {
				t.Fatalf("Fetch() returned unexpected error: %v", err)
			}

			if tt.dirExists {
				if _, err := os.Stat(filepath.Join(destDir, "sentinel")); err != nil {
					t.Error("sentinel file missing; Fetch must have cleared destDir despite skip policy")
				}
			}
		})
	}
}

func TestGitSourceFetch_InvalidRepo(t *testing.T) {
	destDir := t.TempDir()
	src := &GitSource{
		repo:    "https://invalid.example.invalid/no-such-repo.git",
		destDir: filepath.Join(destDir, "clone"),
		policy:  config.PullPolicyAlways,
	}
	if err := src.Fetch(context.Background()); err == nil {
		t.Error("expected error cloning non-existent repo, got nil")
	}
}
