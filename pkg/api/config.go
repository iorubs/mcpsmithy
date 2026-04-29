package api

import (
	"fmt"
	"os"
	"path/filepath"

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
