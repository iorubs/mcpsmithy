# Testing

## Approach

Tests use the Go standard `testing` package exclusively — no
third-party test frameworks or assertion libraries. This keeps the
test toolchain identical to the production toolchain.

## Running Tests

```bash
go test ./internal/...        # all tests
go test -cover ./internal/... # with coverage summary
```

## What to Test

Focus on the public API, boundary conditions, error paths, and
parsing logic. End-to-end tests (planned) will cover integration
paths that are hard to unit test.

## Conventions

- Prefer **table-driven tests** when a function has multiple
  input/output variations. Standalone tests are fine when the setup
  or assertions are unique enough that a table adds awkwardness.
- Combine happy-path and error cases in the same table using a
  `wantErr` field when the test structure is identical.
