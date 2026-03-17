# Config Design

Developer guide for the `.mcpsmithy.yaml` config format — schema
design, versioning, and how to extend it. For the user-facing field
reference, see the auto-generated docs in
[`docs/user/reference/config/`](../user/reference/config/README.md).

## Single Source of Truth: the `mcpsmithy` Struct Tag

Every yaml-tagged config field carries an
`mcpsmithy` struct tag that declares validation rules and defaults.
Fields without `required` are implicitly optional:

```go
// When to fetch external sources: always, ifNotPresent, or never.
PullPolicy PullPolicy `yaml:"pullPolicy,omitempty" mcpsmithy:"default=ifNotPresent"`
```

Tag format:

| Tag value                              | Meaning                         |
|----------------------------------------|---------------------------------|
| `mcpsmithy:"required"`                 | Field must be present           |
| `mcpsmithy:"default=VALUE"`            | Omitted → defaulted to VALUE    |
| `mcpsmithy:"oneof=GROUP"`              | Exactly one field per GROUP must be set |
| `mcpsmithy:"default=20,min=0"`         | Integer must be >= N            |

Two consumers read this tag:

| Consumer | What it does |
|----------|--------------|
| **schema.Process** | Single call that applies defaults, validates required fields, enum values, oneof groups, min bounds, and ref constraints — recurses into nested structs, maps, and slices automatically |
| **Doc generator** | Parses tags via go/ast to build the reference tables |

This eliminates drift between documentation, validation, and
defaulting — change the tag once, all consumers update automatically.

### Enum types and the `Valuer` interface

Named string types with a fixed set of valid values implement the
`schema.Valuer` interface:

```go
func (PullPolicy) Values() []string {
    return []string{string(PullPolicyAlways), string(PullPolicyIfNotPresent), string(PullPolicyNever)}
}
```

`schema.Process` checks non-zero fields whose type implements
`Valuer` — no per-field validation code needed. Adding a new enum
value means updating the const block and the `Values()` method in
one place.

### Mutual exclusivity and the `oneof` tag

Fields that form a mutually-exclusive group are tagged
`oneof=GROUP`, where GROUP is an arbitrary label that ties them
together. Fields sharing the same group name on the same struct are
checked by `ValidateOneOf()`: exactly one must be non-zero.

```go
Enum []any    `yaml:"enum,omitempty" mcpsmithy:"oneof?=no_enum_with_min"`
Min  *float64 `yaml:"min,omitempty"  mcpsmithy:"oneof?=no_enum_with_min"`
```

Two error cases:
- Neither set → allowed (both are optional)
- Both set → `"enum and min are mutually exclusive"`

The group name is a free-form label — pick something descriptive.
The `?` suffix makes the group optional (zero fields is OK).
The doc generator renders these fields as **oneof** in the Required
column.

## Versioning

Each config schema version lives in its own sub-package with types
and a parser. Each versioned parser uses strict field validation —
unknown keys are errors. Forward compatibility is handled by version
routing: each versioned `Parse()` method returns the latest `*Config`
type, converting if needed.

### Parse → Default → Validate

1. The caller reads the YAML file and passes raw bytes to `config.Parse()`.
2. `config.Parse()` unmarshals just the `version` field, then
   dispatches to the correct versioned parser (e.g., `v1.Parse()`).
3. The versioned parser YAML-decodes with strict mode, calls
   `schema.Process()`, and returns `*Config` already in the latest type
   (for v1 this is direct — v1 **is** the latest).
4. The rest of the codebase operates on the latest types via type
   aliases in `config.go`.

## Design Decisions

### Type aliases

`config.Config` is an alias for the latest version's `Config`. All
downstream consumers import `"internal/config"` without change —
adding a new version only touches the config packages.

### Self-contained versions

Types, parser, validation, and helpers all live together in the
versioned package. The shared `schema` package is version-agnostic;
each vN calls it on its own types.

### Strict per-version parsing

Each versioned parser rejects unknown keys as errors
(`yaml.WithKnownFields()`). Forward compatibility comes from version
routing, not lenient parsing.

### Config version ≠ protocol version

The config schema version (`version: "1"`) is independent of the MCP
protocol version.

### Tools

Every tool requires a `template:` field — a Go `text/template` string.

### Built-in Template Functions

Template functions are registered in `funcMap()`. User-facing docs are
generated automatically from the `BuiltinFunc` consts and their doc comments.

Typed stubs with matching signatures live in `Tool.Validate()`, which performs a dry-run
template execution at config-load time to catch syntax errors, arity
mismatches, and undeclared parameter references.

## Adding a New Config Field

1. Add the field to the struct in `internal/config/v1/types.go`
   with an above-field comment and an `mcpsmithy:"..."` tag.
2. If the field has a non-zero default, set the tag to
   `mcpsmithy:"default=VALUE"`. `schema.Process` handles
   top-level structs, map values, and slices automatically.
3. If the field is required, set `mcpsmithy:"required"`.
4. Add any semantic validation in `internal/config/v1/types.go` (e.g. a
   `Validate() error` method) or `internal/config/schema/process.go`.
5. Run `go run ./cmd/gen-docs/` — the user reference updates
   automatically.

## Adding a New Built-in Function

1. Add a `BuiltinFunc` const in `internal/config/v1/types.go` and a
   typed stub for it in the `template.FuncMap` inside `Tool.Validate()`.
2. Implement the function in `internal/tools/templating.go`.
3. Register it in `funcMap()` (same file).
4. Add a doc comment on the `BuiltinFunc` const — it's picked up automatically to generate the user reference docs.
5. Add tests alongside the implementation.
6. Add an example to `docs/user/examples/`.

## Adding a New Config Version

When the config schema needs breaking changes:

1. Create `internal/config/vN/` with its own `types.go`, `parse.go`,
   and `const Version = "N"`.
2. Add `mcpsmithy` tags to all yaml-tagged fields.
3. Implement `Parse()` — YAML-decode with strict mode, call `schema.Process`,
   convert the result to the **latest** `*Config` type, and return it.
   (For vN when vN is the new latest, conversion is direct.)
4. Update `parse.go` of the **previous** version to convert its decoded
   struct into vN's types before returning.
5. Update the type aliases in `config.go` to point to vN.
6. Add a `case vN.Version:` branch in `config.Parse()` (i.e. register
   `vN.Schema{}` in the `Versions` map).
7. Run `go run ./cmd/gen-docs/` — user-facing reference updates
   automatically.
