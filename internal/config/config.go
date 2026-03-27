// Package config loads and version-routes .mcpsmithy.yaml configs.
//
// Each config schema version lives in its own sub-package (v1/, v2/, …)
// and satisfies the [VersionSchema] interface. The loader reads raw YAML,
// detects the "version" field, and delegates to the correct version.
//
// Adding a new version:
//  1. Create the vN/ sub-package satisfying [VersionSchema]
//  2. Register it in the versions map in this file
//
// Type aliases below re-export the latest version's types so that
// consumers can keep importing "internal/config" without change.
package config

import (
	"fmt"
	"maps"
	"slices"

	"github.com/operator-assistant/mcpsmithy/internal/config/schema"
	v1 "github.com/operator-assistant/mcpsmithy/internal/config/v1"
	"go.yaml.in/yaml/v4"
)

const (
	// ReservedContextKey is the single template context key owned by the
	// engine. User params with this name collide with the injected config namespace.
	// Canonical definition is in the schema package; this alias keeps
	// the existing export path stable.
	ReservedContextKey = schema.ReservedContextKey

	// Constant aliases — re-export values so consumers never import v1 directly.
	// PullPolicy values control when external sources are fetched.
	PullPolicyAlways       = v1.PullPolicyAlways
	PullPolicyIfNotPresent = v1.PullPolicyIfNotPresent
	PullPolicyNever        = v1.PullPolicyNever

	// ParamType values for tool parameter types.
	ParamTypeString          = v1.ParamTypeString
	ParamTypeNumber          = v1.ParamTypeNumber
	ParamTypeBool            = v1.ParamTypeBool
	ParamTypeArray           = v1.ParamTypeArray
	ParamTypeProjectFilePath = v1.ParamTypeProjectFilePath

	// BuiltinFunc names — the template functions available inside tool templates.
	BuiltinFuncConventionsFor = v1.BuiltinFuncConventionsFor
	BuiltinFuncSearchFor      = v1.BuiltinFuncSearchFor
	BuiltinFuncFileRead       = v1.BuiltinFuncFileRead
	BuiltinFuncHTTPGet        = v1.BuiltinFuncHTTPGet
	BuiltinFuncHTTPPost       = v1.BuiltinFuncHTTPPost
	BuiltinFuncHTTPPut        = v1.BuiltinFuncHTTPPut
	BuiltinFuncGrep           = v1.BuiltinFuncGrep
)

// Versions is the single source of truth for which schema versions are
// accepted. Each entry satisfies [VersionSchema].
var Versions = map[string]VersionSchema{
	v1.Version: v1.Schema{},
}

// VersionSchema is the contract each config version must satisfy.
// The Parse method must return the latest Config type (converting if needed).
type VersionSchema interface {
	Parse([]byte) (*Config, error)
	RootType() any
	TypesSources() []string
}

// TypesSources returns the raw Go source files for the latest version's types.
// Callers that only need the latest version can use this directly.
var TypesSources = v1.TypesSources

// Type aliases — always point to the latest version.
type (
	Config              = v1.Config
	Project             = v1.Project
	DocRef              = v1.DocRef
	Convention          = v1.Convention
	ConventionRelations = v1.ConventionRelations
	ProjectSources      = v1.ProjectSources
	LocalSource         = v1.LocalSource
	ScrapeSource        = v1.ScrapeSource
	GitSource           = v1.GitSource
	HTTPSource          = v1.HTTPSource
	Tool                = v1.Tool
	ToolParam           = v1.ToolParam
	ParamConstraints    = v1.ParamConstraints
	TemplateString      = v1.TemplateString
	PullPolicy          = v1.PullPolicy
	ParamType           = v1.ParamType
	BuiltinFunc         = v1.BuiltinFunc
)

// Parse parses raw YAML bytes, detects the version, delegates to the
// correct versioned parser, and converts the result to the latest
// Config type.
func Parse(data []byte) (*Config, error) {
	var header struct {
		Version string `yaml:"version"`
	}
	if err := yaml.Unmarshal(data, &header); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	e, ok := Versions[header.Version]
	if !ok {
		return nil, fmt.Errorf("unsupported config version %q (supported: %s)",
			header.Version, slices.Sorted(maps.Keys(Versions)))
	}
	return e.Parse(data)
}
