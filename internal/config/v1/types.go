// Package v1 defines the v1 config schema types for .mcpsmithy.yaml.
//
// This is currently the latest (and only) config version. When v2 is
// introduced, create internal/config/v2/ with its own types and update
// Schema.Parse here to convert v1 → v2 via the new version's converter.
package v1

import (
	"fmt"
	"maps"
	"strings"
	"text/template"

	"github.com/operator-assistant/mcpsmithy/internal/config/schema"
)

// Version is the schema version this package handles.
const Version = "1"

// Config is the root of .mcpsmithy.yaml. It has four top-level sections:
// project identifies the codebase and declares content sources;
// conventions map file-path patterns to docs and rules;
// tools define what the AI can call at runtime.
type Config struct {
	// Config schema version. Must be "1".
	Version string `yaml:"version" mcpsmithy:"required"`
	// Project identity, description, and content sources.
	Project Project `yaml:"project" mcpsmithy:"required"`
	// Rules and docs the AI should follow, keyed by name.
	Conventions map[string]Convention `yaml:"conventions" mcpsmithy:"required"`
	// Tools the AI can call, keyed by tool name (the name the AI sees).
	Tools map[string]Tool `yaml:"tools" mcpsmithy:"required"`
}

// Project tells the AI what this codebase is. Name and description appear
// in tool responses (e.g. project_info). Sources declare where code, docs,
// and other content live — the AI uses them for search and orientation.
type Project struct {
	// Human-readable project name shown in tool responses.
	Name string `yaml:"name" mcpsmithy:"required"`
	// Brief summary of what this project does. Shown alongside the name.
	Description string `yaml:"description" mcpsmithy:"required"`
	// Content sources that tell the AI where things live.
	// Sources with index: true (default) are searchable via search_for;
	// sources with index: false describe project structure without being searchable.
	Sources *ProjectSources `yaml:"sources,omitempty"`
}

// ProjectSources groups content the AI can search or reference.
// Each source type (local, scrape, git, http) is a named map — the
// names are used in convention docs entries to link conventions to
// specific content.
//
// Sources serve two purposes:
//   - Structure — sources with index: false describe where things live.
//   - Search — sources with index: true (default) are indexed for full-text search.
type ProjectSources struct {
	// Global pull policy for all sources. Per-source values override this.
	PullPolicy PullPolicy `yaml:"pullPolicy,omitempty" mcpsmithy:"default=ifNotPresent"`
	// Background sync interval in minutes.
	// When > 0, the server starts immediately after indexing local sources;
	// remote sources load in the background. Re-fetch runs every N minutes.
	SyncInterval int `yaml:"syncInterval,omitempty" mcpsmithy:"default=0,min=0"`
	// Files on disk, keyed by name. Reference these names in convention docs.
	Local map[string]LocalSource `yaml:"local,omitempty"`
	// Web pages to fetch and optionally follow links from, keyed by name.
	Scrape map[string]ScrapeSource `yaml:"scrape,omitempty"`
	// Git repositories to clone, keyed by name.
	Git map[string]GitSource `yaml:"git,omitempty"`
	// HTTP endpoints to fetch (archives, APIs, artifact stores), keyed by name.
	HTTP map[string]HTTPSource `yaml:"http,omitempty"`
}

// PullPolicy controls when remote sources are fetched.
type PullPolicy string

const (
	// Fetch sources on every server start.
	PullPolicyAlways PullPolicy = "always"
	// Fetch only when the local cache is missing.
	PullPolicyIfNotPresent PullPolicy = "ifNotPresent"
	// Never fetch — only use sources already on disk.
	PullPolicyNever PullPolicy = "never"
)

// Values returns the set of valid PullPolicy values.
func (PullPolicy) Values() []string {
	return []string{string(PullPolicyAlways), string(PullPolicyIfNotPresent), string(PullPolicyNever)}
}

// LocalSource points to files on disk relative to the project root.
// Commonly used for source code, docs, configs, and test files.
// Indexed for search by default; set index: false to expose them
// as structure without indexing.
type LocalSource struct {
	// Glob patterns relative to project root.
	Paths []string `yaml:"paths" mcpsmithy:"required"`
	// Explains what these files are. Shown to the AI in project overviews.
	Description string `yaml:"description,omitempty"`
	// When true (default), content is searchable via search_for.
	// Set to false for sources that describe structure only.
	Index *bool `yaml:"index,omitempty" mcpsmithy:"default=true"`
	// Per-source pull policy override (falls back to global when unset).
	PullPolicy PullPolicy `yaml:"pullPolicy,omitempty"`
}

// ScrapeSource fetches web pages by URL. Useful for external API docs,
// wikis, or any HTML content. Set maxDepth > 0 to follow links from
// seed URLs and build a wider search corpus.
type ScrapeSource struct {
	// HTTP(S) URLs to fetch.
	URLs []string `yaml:"urls" mcpsmithy:"required"`
	// Explains what these pages are. Shown to the AI in project overviews.
	Description string `yaml:"description,omitempty"`
	// When true (default), content is searchable via search_for.
	// Set to false for sources that describe structure only.
	Index *bool `yaml:"index,omitempty" mcpsmithy:"default=true"`
	// Max link-following depth from seed URLs.
	MaxDepth int `yaml:"maxDepth,omitempty" mcpsmithy:"default=0,min=0"`
	// Max KB per page.
	MaxPageSize int `yaml:"maxPageSize,omitempty" mcpsmithy:"default=2048,min=0"`
	// Max URLs to fetch (seeds + discovered pages combined).
	MaxPages int `yaml:"maxPages,omitempty" mcpsmithy:"default=20,min=0"`
	// Per-source pull policy override (falls back to global when unset).
	PullPolicy PullPolicy `yaml:"pullPolicy,omitempty"`
}

// GitSource specifies a git repository to clone and optionally index.
// Requires the git binary on PATH. Credentials come from the environment
// (SSH keys, git credential helpers, .netrc).
//
// For HTTP archive downloads without the git binary, use an HTTPSource
// with the forge's archive URL instead.
type GitSource struct {
	// Git repository URL (HTTPS or SSH).
	Repo string `yaml:"repo" mcpsmithy:"required"`
	// Branch or tag to fetch.
	Ref string `yaml:"ref,omitempty"`
	// Glob patterns within the repo.
	Paths []string `yaml:"paths" mcpsmithy:"required"`
	// Explains what this repo provides. Shown to the AI in project overviews.
	Description string `yaml:"description,omitempty"`
	// Git clone depth. Use 1 (default) for content only; increase if history is needed.
	Depth int `yaml:"depth,omitempty" mcpsmithy:"default=1,min=1"`
	// When true (default), content is searchable via search_for.
	// Set to false for sources that describe structure only.
	Index *bool `yaml:"index,omitempty" mcpsmithy:"default=true"`
	// Per-source pull policy override (falls back to global when unset).
	PullPolicy PullPolicy `yaml:"pullPolicy,omitempty"`
}

// HTTPSource fetches content from a URL and optionally extracts tar.gz
// archives. Works with any authenticated HTTP(S) endpoint: forge archive
// URLs, private APIs, artifact stores, etc.
//
// Authentication is automatic via .netrc; custom headers can be added for
// bearer tokens or API keys.
type HTTPSource struct {
	// HTTP(S) URL to fetch.
	URL string `yaml:"url" mcpsmithy:"required"`
	// Custom HTTP headers (e.g. Authorization).
	// Applied alongside .netrc credentials; explicit headers take precedence.
	Headers map[string]string `yaml:"headers,omitempty"`
	// Glob patterns within the fetched content.
	// For archives these match extracted files; for single files they match the saved filename.
	Paths []string `yaml:"paths" mcpsmithy:"required"`
	// Explains what this endpoint provides. Shown to the AI in project overviews.
	Description string `yaml:"description,omitempty"`
	// Extract tar.gz archives.
	// Auto-detected from Content-Type or URL suffix (.tar.gz/.tgz) when unset.
	Extract *bool `yaml:"extract,omitempty"`
	// When true (default), content is searchable via search_for.
	// Set to false for sources that describe structure only.
	Index *bool `yaml:"index,omitempty" mcpsmithy:"default=true"`
	// Per-source pull policy override (falls back to global when unset).
	PullPolicy PullPolicy `yaml:"pullPolicy,omitempty"`
}

// Convention maps a file-path pattern to documentation and rules.
// When the AI calls conventions_for with a file path, matching conventions
// return their description, linked docs, tags, and relations — giving
// the AI the context it needs before generating or reviewing code.
//
// The map key is the convention's unique name, used in relations and
// tool responses.
type Convention struct {
	// Glob pattern that matches file paths this convention applies to.
	// When the AI calls conventions_for with a file path, only conventions
	// whose scope matches are returned.
	//
	// Supported patterns:
	//   *        — matches any path (use as a global/catch-all convention)
	//   **       — matches any number of path segments within a larger pattern
	//   src/**   — all files under src/
	//   *.go     — Go files in the root only
	//   **/*.go  — all Go files at any depth
	//
	// Omit scope entirely to make a convention search-only: it will never be
	// matched by file path but remains findable via search_for.
	Scope string `yaml:"scope,omitempty"`
	// Documentation the AI should read when this convention is matched.
	// Each entry references a declared source by name, optionally narrowed to specific paths.
	Docs []DocRef `yaml:"docs,omitempty"`
	// Inline summary returned to the AI alongside matched docs.
	// Also used as the search corpus when the AI searches conventions by keyword.
	Description string `yaml:"description" mcpsmithy:"required"`
	// Keywords that improve search relevance when the AI searches for conventions.
	Tags []string `yaml:"tags,omitempty"`
	// Links to related conventions, so the AI discovers that changing one area
	// may require reading or following rules from another.
	Relations []ConventionRelations `yaml:"relations,omitempty"`
}

// ConventionRelations links two conventions so the AI discovers that
// working in one area may require awareness of another. For example,
// an API convention might relate to a testing convention because every
// endpoint needs integration tests.
type ConventionRelations struct {
	// Convention name (must match a conventions map key).
	Target string `yaml:"target" mcpsmithy:"required,ref=conventions"`
	// How this convention relates to the target.
	Description string `yaml:"description,omitempty"`
}

// DocRef links a convention to content from a declared source.
// The source field must match a key under project.sources (local, scrape,
// git, or http). Paths optionally narrow to specific files within that source.
type DocRef struct {
	// Source name — must match a key in project.sources (local, scrape, git, or http).
	Source string `yaml:"source" mcpsmithy:"required,ref=project.sources.local|project.sources.scrape|project.sources.git|project.sources.http"`
	// Optional paths within the source to specific files.
	Paths []string `yaml:"paths,omitempty"`
}

// Tool defines an operation the AI can invoke. Each tool has a template
// that produces text output, optional input parameters the AI fills in
// per call, and optional static options the config author controls.
// The map key becomes the tool name the AI sees in tool listings.
type Tool struct {
	// Description the AI sees when listing available tools.
	// Write what the tool does and when to call it, not just what it returns.
	Description string `yaml:"description" mcpsmithy:"required"`
	// Template body producing the tool's output.
	Template TemplateString `yaml:"template" mcpsmithy:"required"`
	// Input parameters the LLM varies per call — queries, file paths, search terms.
	Params []ToolParam `yaml:"params,omitempty"`
	// Static key-value pairs set by the config author, invisible to the LLM.
	// Injected into the template context alongside params at runtime.
	//
	// Use options for values fixed per tool: token budgets, result limits, base URLs, feature flags.
	// For example, wire maxResults from options (e.g. `{{ search_for .query .maxResults }}`)
	// so the config author controls the budget, not the LLM.
	Options map[string]any `yaml:"options,omitempty"`
	// When false params are not logged at DEBUG level. Default (nil/unset) logs params.
	LogParams *bool `yaml:"log_params,omitempty"`
	// Maximum output size for this tool in KB. Defaults to 1024 (1 MB).
	MaxOutputKB int `yaml:"maxOutputKB,omitempty" mcpsmithy:"default=1024,min=1"`
}

// Validate performs a dry-run execution of the tool's template against a
// zero-value context built from its declared params and options.
// This catches syntax errors, arity mismatches, and undeclared variable
// references in a single pass. Called by schema.Process at parse time.
func (t Tool) Validate() error {
	if t.Template == "" {
		return nil
	}
	ctx := map[string]any{
		schema.ReservedContextKey: struct {
			Project struct {
				Project
				Root string
			}
			Conventions map[string]Convention
			Tools       map[string]Tool
		}{},
	}
	for _, p := range t.Params {
		ctx[p.Name] = zeroForParamType(p.Type)
	}
	maps.Copy(ctx, t.Options)
	parsed, err := template.New("validate").Funcs(template.FuncMap{
		string(BuiltinFuncConventionsFor): func(string) string { return "" },
		string(BuiltinFuncSearchFor):      func(string, ...any) string { return "" },
		string(BuiltinFuncFileRead):       func(string, ...any) string { return "" },
		string(BuiltinFuncHTTPGet):        func(string, ...any) (string, error) { return "", nil },
		string(BuiltinFuncGrep):           func(string, float64, float64, string) string { return "" },
	}).Option("missingkey=error").Parse(string(t.Template))
	if err != nil {
		return err
	}
	var sb strings.Builder
	if err := parsed.Execute(&sb, ctx); err != nil {
		return fmt.Errorf("template dry-run: %w", err)
	}
	return nil
}

// ToolParam declares an input the AI provides when calling a tool.
// Params appear in the tool's input schema — the AI sees names,
// types, and descriptions before deciding what values to pass.
type ToolParam struct {
	// Parameter name.
	Name string `yaml:"name" mcpsmithy:"required,notreserved"`
	// Parameter type.
	Type ParamType `yaml:"type" mcpsmithy:"required"`
	// Whether the parameter is mandatory.
	Required bool `yaml:"required" mcpsmithy:"default=false"`
	// Shown to the AI. Use it to explain the expected format or how to
	// transform what the user provides into what the template expects.
	Description string `yaml:"description"`
	// Default value when not supplied.
	// The YAML type must match the param type (e.g. integer for int, boolean for bool).
	Default any `yaml:"default" mcpsmithy:"typed-as=type"`
	// Constraints on the parameter value (enum or min/max).
	Constraints *ParamConstraints `yaml:"constraints,omitempty" mcpsmithy:"typed-as=type"`
}

// ParamType names the valid parameter types for tool definitions.
type ParamType = schema.ParamType

const (
	// String value.
	ParamTypeString = schema.ParamTypeString
	// Numeric value (float64).
	ParamTypeNumber = schema.ParamTypeNumber
	// Boolean value.
	ParamTypeBool = schema.ParamTypeBool
	// Array value.
	ParamTypeArray = schema.ParamTypeArray
	// File path validated against the project sandbox.
	ParamTypeProjectFilePath = schema.ParamTypeProjectFilePath
)

// ParamConstraints limits what the AI can supply for a parameter.
// Use enum for a fixed set of valid values, or min/max for numeric
// bounds. Constraints are enforced server-side and surfaced in the
// tool's input schema so the AI knows the valid range upfront.
type ParamConstraints struct {
	// Fixed set of valid values.
	// Compatible with string, integer, and number param types.
	Enum []any `yaml:"enum,omitempty" mcpsmithy:"oneof?=no_enum_with_min|no_enum_with_max"`
	// Minimum value (inclusive).
	// Applies to integer and number param types only.
	Min *float64 `yaml:"min,omitempty" mcpsmithy:"oneof?=no_enum_with_min"`
	// Maximum value (inclusive).
	// Applies to integer and number param types only.
	Max *float64 `yaml:"max,omitempty" mcpsmithy:"oneof?=no_enum_with_max"`
}

// TemplateString is the template body for a tool. It uses Go text/template
// syntax with access to the project context via `{{ .mcpsmithy }}`, declared
// params and options as `{{ .paramName }}`, and built-in functions like
// conventions_for, search_for, file_read, http_get, and grep.
type TemplateString string

// BuiltinFunc names a template function available inside tool templates.
// These are the only functions callable from template bodies — they cover
// convention lookup, content search, file reading, HTTP fetching, and
// text filtering.
type BuiltinFunc string

const (
	// func(path string) string — Returns conventions matching a file path, including description and linked docs.
	BuiltinFuncConventionsFor BuiltinFunc = "conventions_for"
	// func(query string, [maxResults int], [maxResultSize int]) string — Full-text search over indexed sources.
	BuiltinFuncSearchFor BuiltinFunc = "search_for"
	// func(path string, [maxFileSize int]) string — Reads local files matching a glob pattern within the project sandbox.
	BuiltinFuncFileRead BuiltinFunc = "file_read"
	// func(url string, [maxReadKB int]) (string, error) — HTTP GET with .netrc auth and ANSI stripping. Caps the response body at maxReadKB KB (default 10240).
	BuiltinFuncHTTPGet BuiltinFunc = "http_get"
	// func(pattern string, before float64, after float64, input string) string — Filters input by regex pattern with context lines.
	BuiltinFuncGrep BuiltinFunc = "grep"
)

// zeroForParamType delegates to schema.ZeroForParamType.
var zeroForParamType = schema.ZeroForParamType
