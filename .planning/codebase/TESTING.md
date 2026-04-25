# TESTING
_Last updated: 2026-04-25_

## Summary
Test coverage is focused on pure logic functions (config parsing, metadata serialization, shell escaping, zellij template rendering). The main orchestration flow in `main.go` has no direct tests. No mocking framework used — tests rely on `t.TempDir()` and real filesystem operations.

## Test Files

| File | What It Tests |
|------|--------------|
| `config_test.go` | `loadGlobalConfigFromPath`, field defaults, missing files |
| `config_update_test.go` | `runConfigUpdateFromPath`: missing keys added, existing keys preserved, backup created |
| `agent_test.go` | `enabledAgents`, `agentCommand`, `resolveClaudeContext`, `chooseEnabledAgent` |
| `docker_test.go` | `isContainerRunning`, `imageExists`, `getContainerForDirectory` |
| `filesystem_test.go` | `createAgentJailFolder`, `updateGitignore`, `copyTemplateConfigs` |
| `metadata_test.go` | `saveMetadata`, `loadMetadata`, `checkVersionUpdate` round-trip |
| `zellij_test.go` | `shellEscape`, `sanitizeKDLString`, `parseZellijKeybinds`, `buildHintsLine`, `pluginNameFromURL`, `downloadPlugin`, `writeZellijFiles`, `buildZellijEntrypoint` |
| `noninteractive_test.go` | `adaptRunArgsForNonInteractive`, `nonInteractiveExecArgs`, `tryNILock`/`releaseNILock` |
| `dockerfile_test.go` | Embedded Dockerfile exists and has expected content |

## Test Patterns

**Table-driven tests** used throughout:
```go
cases := []struct{ input, want string }{ ... }
for _, c := range cases {
    got := fn(c.input)
    if got != c.want { t.Errorf(...) }
}
```

**TempDir for filesystem tests**:
```go
dir := t.TempDir()  // auto-cleaned after test
```

**Real filesystem operations**: no mocking — `saveMetadata`, `copyTemplateConfigs`, etc. run against temp directories.

**HTTP test server** for download tests in `zellij_test.go`:
```go
srv := httptest.NewServer(http.HandlerFunc(...))
defer srv.Close()
```

**No test helpers/fixtures files** — all test data is inline.

## What Is NOT Tested

- `main()` function — the entire orchestration/flag-parsing/docker-run assembly
- `loadGlobalConfig()` (only `loadGlobalConfigFromPath` is tested)
- `printCleanConfig()`
- `runWithTerminalRestore()`
- `chooseEnabledAgent()` interactive path (stdin not mocked)
- Container lifecycle (no Docker daemon in tests)
- The `-N` non-interactive container start path end-to-end
- Zellij plugin download caching behavior in real filesystem

## Test Infrastructure

- Run: `cd cli && go test ./...` or `mise test`
- No CI configuration found (no `.github/workflows/`)
- No test coverage reporting configured
- No benchmarks
- Build constraints: none (OS-specific files use filename conventions, not `//go:build`)

## Coverage Gaps

High-value untested areas:
1. `main()` orchestration — largest untested surface
2. `runConfigUpdateFromPath` backup+write path with real YAML comment preservation
3. Non-interactive lock contention race (concurrent `agentjail -N` processes)
4. `copyPlugins` with URL caching (only download path tested, not cache hit)
