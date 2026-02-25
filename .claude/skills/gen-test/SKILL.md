---
name: gen-test
description: Generate Go tests for a package or function, matching project conventions
disable-model-invocation: true
---

# Generate Tests

Generate Go tests that match this project's established conventions.

## Arguments

- `$ARGUMENTS`: Package path or function name to generate tests for (e.g., `pkg/client.go`, `extractHost`)

## Conventions

Follow these patterns observed in the existing test suite:

1. **Package**: Use `package ripley` (same package, not `_test` suffix) for internal function access
2. **License header**: Include the GPLv3 license header at the top of every test file (copy from any existing `*_test.go`)
3. **Stdlib only**: Use only the standard `testing` package — no testify, no gomock
4. **Assertions**: Use `t.Errorf` for non-fatal checks, `t.Fatalf` for fatal setup failures
5. **Table-driven tests**: Use the `[]struct{ name, input, expected }` pattern with `t.Run` for parameterized cases (see `TestExtractHost` in `pkg/metrics_test.go` for reference)
6. **Simple tests**: For straightforward cases, use direct `TestFunctionName` functions without tables
7. **Error format**: `t.Errorf("got = %v; want %v", actual, expected)`
8. **Race safe**: All tests must pass with `-race` flag
9. **No external services**: Tests should be self-contained. If testing HTTP, use `net/http/httptest`

## Workflow

1. Read the source file(s) to understand the functions to test
2. Read existing `*_test.go` files in the same package for style reference
3. Generate tests covering:
   - Happy path
   - Edge cases and error conditions
   - Boundary values where applicable
4. Run `go test -race ./...` to verify tests pass
5. Run `golangci-lint run` to verify no lint issues
