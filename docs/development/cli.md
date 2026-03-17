# CLI Design

## Overview

mcpsmithy uses [Kong](https://github.com/alecthomas/kong) for CLI
parsing. Commands are declared as Go structs with Kong struct tags —
no imperative flag registration. The struct layout is the source of
truth; `gen-docs` reads it to produce the user-facing reference docs
in `docs/user/reference/server/`.

Do not document flags or command behaviour here — that lives in the
generated reference docs and must stay in sync with the code
automatically.

## Package Layout

```
cmd/mcpsmithy/main.go  → Entry point: kong.Parse() only, no logic
internal/commands/        → One file per subcommand; CLI root struct in commands.go
```

Each subcommand is a struct with a `Run(*CLI) error` method. Global
state (config path, log level) lives on `CLI` in `commands.go` and is
accessed via helper methods (`LoadConfig`, `Logger`, `ProjectRoot`).

## Conventions

- **stdout is the MCP protocol channel.** Never write to stdout in
  any command. All output goes to stderr via the `slog` logger.
- **No startup banners or plain `fmt` writes.** Use structured `slog`
  calls so output is consistent and filterable.
- **`LoadConfig` is the standard entry point** for subcommands that
  need config. It loads, validates, logs warnings/errors, and
  resolves the project root in one call.
- **Kong struct tags define the CLI surface.** Help text, defaults,
  enums, and short flags are all declared in the tag — not in `Run`.

## Adding a New Command

1. Create `internal/commands/<name>.go`.
2. Define a struct with a `Run(*CLI) error` method.
3. Add the struct as a field on `CLI` in `commands.go` with `cmd:""` and `help:""` tags.
4. Run `go run ./cmd/gen-docs` to regenerate the reference docs.
