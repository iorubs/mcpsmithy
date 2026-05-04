# Testing

## Approach

Tests use the Go standard `testing` package exclusively; no
third-party test frameworks or assertion libraries. This keeps the
test toolchain identical to the production toolchain.

## Running Tests

```bash
go test ./internal/...        # all tests
go test -cover ./internal/... # with coverage summary
```

## What to Test

Focus on the public API of the package under test, boundary
conditions, error paths, and parsing logic. Tests exercise the
*module they live in*: do not redundantly verify the behaviour of
the standard library or third-party dependencies (don't write
tests that prove `yaml.Unmarshal` works; test our wrapping logic).

End-to-end tests will cover integration paths that are hard to unit test.

## Conventions

- Prefer **table-driven tests** when a function has multiple
  input/output variations. Standalone tests are fine when the setup
  or assertions are unique enough that a table adds awkwardness.
- Combine happy-path and error cases in the same table using a
  `wantErr` field when the test structure is identical.
- **Never ignore errors** in test code. Every error from setup
  (`os.WriteFile`, `os.MkdirAll`, `filepath.Abs`, `json.Marshal`,
  constructors, etc.) must be checked with `t.Fatal(err)` or
  surfaced via `http.Error` inside an `httptest` handler. `_ = ...`
  and `x, _ := ...` are forbidden in tests.
- Use `t.Helper()` in custom assertion or setup helpers so failure
  lines point to the call site, not the helper.
- Keep helpers minimal. A helper that wraps a single stdlib call
  without adding error handling, table semantics, or shared state
  is just noise; inline it.
- **No real network in unit tests.** Anything that needs a remote
  endpoint goes behind `httptest.Server`, a fake provider, or a
  build tag (`//go:build integration`).
