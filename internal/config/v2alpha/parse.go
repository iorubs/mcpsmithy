package v2alpha

import (
	_ "embed"
	"errors"
	"fmt"

	"github.com/operator-assistant/mcpsmithy/internal/config/schema"
	"go.yaml.in/yaml/v4"
)

// typesSource is the raw Go source of types.go, embedded at compile time.
//
//go:embed types.go
var typesSource string

// TypesSources returns all Go source files needed for doc generation.
// Includes both the v1 types and the shared schema types (e.g. ParamType).
var TypesSources = []string{typesSource, schema.ParamTypeSource}

// Schema satisfies the config.VersionSchema interface for v1.
// Since v2alpha is the latest version, Parse returns directly without conversion.
type Schema struct{}

// Parse parses raw YAML bytes into a v1 Config, applies defaults, and
// runs tag-driven validation. Returns a joined error containing all
// validation errors, or nil if the config is valid.
func (Schema) Parse(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Load(data, &cfg, yaml.WithKnownFields()); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	errs := schema.Process(&cfg)

	return &cfg, errors.Join(errs...)
}

// RootType returns the zero value of the v1 root config type.
func (Schema) RootType() any { return Config{} }

// TypesSources returns the raw Go sources for this version's types.
func (Schema) TypesSources() []string { return TypesSources }
