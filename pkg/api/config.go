package api

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/iorubs/mcpsmithy/internal/auth"
	"github.com/iorubs/mcpsmithy/internal/config"
)

// LoadConfig  reads and parses the config file at a path and returns the parsed config
// along with the resolved project root (the dir containing the file)
func LoadConfig(path string) (*config.Config, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("config: %w", err)
	}

	cfg, err := config.Parse(data)
	if err != nil {
		return nil, "", fmt.Errorf("config: %w", err)
	}

	root, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return nil, "", fmt.Errorf("resolving project root: %w", err)
	}

	return cfg, root, nil
}

// LoadCredentials reads the credentials file at path and returns a store for
// authenticating outbound HTTP requests. An empty path uses
// [auth.DefaultCredentialsPath].
//
// A missing file is not an error; hosts without an entry fall back to ~/.netrc.
// When no credentials file is present but a netrc is, this logs a warning once,
// since netrc is deprecated and cannot express vendor auth schemes.
func LoadCredentials(ctx context.Context, path string) (*auth.Store, error) {
	if path == "" {
		path = auth.DefaultCredentialsPath
	}
	store, err := auth.Load(path)
	if err != nil {
		return nil, fmt.Errorf("credentials: %w", err)
	}
	if !store.HasCredentials() && auth.NetrcExists() {
		slog.WarnContext(ctx, "no credentials file found; using ~/.netrc, which is deprecated and cannot express custom auth schemes or header names",
			"credentials", path)
	}
	return store, nil
}
