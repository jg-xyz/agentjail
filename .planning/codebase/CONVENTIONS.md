# CONVENTIONS
_Last updated: 2026-04-25_

## Summary
AgentJail follows standard Go idioms: error wrapping with `%w`, `log.Fatalf` for unrecoverable CLI errors, and flat `package main` with no sub-packages. Code is organized by concern into separate files rather than by abstraction layers.

## Error Handling

- **Fatal errors** (unrecoverable): `log.Fatalf("%v", err)` — exits with message
- **Warnings** (recoverable): `log.Warnf("could not X: %v", err)` — continues execution
- **Error wrapping**: `fmt.Errorf("context: %w", err)` used consistently in helpers
- **os.IsNotExist checks**: used before creating files/dirs, never panic on missing optional resources
- Pattern: helpers return `error`; callers in `main.go` decide fatal vs warn

```go
if err := someOp(); err != nil {
    log.Warnf("could not do X: %v", err)  // non-fatal
}
if err := criticalOp(); err != nil {
    log.Fatalf("critical op: %v", err)    // fatal
}
```

## Naming Conventions

- **Functions**: `camelCase`, verb-first (`loadGlobalConfig`, `createAgentJailFolder`, `buildZellijEntrypoint`)
- **Types**: `PascalCase` (`GlobalConfig`, `AgentFrameworksConfig`, `AgentJailMetadata`)
- **Constants**: `camelCase` for unexported (`claudeSystemPrompt`, `containerPluginsDir`)
- **YAML tags**: `snake_case` matching config file keys
- **JSON tags**: `snake_case` for metadata, `omitempty` for optional fields
- **Test files**: `<file>_test.go` alongside source

## Logging

Uses `logrus` (package-level `var log = logrus.New()` in `logging.go`):
- `log.Info` / `log.Infof` — user-visible status messages
- `log.Debug` / `log.Debugf` — verbose mode only (`--verbose` flag)
- `log.Warn` / `log.Warnf` — non-fatal issues
- `log.Fatalf` — exits; used for unrecoverable errors in CLI path
- Timestamps disabled; level text padded for alignment

## Go Idioms Used

- **OS-specific files**: `terminal_windows.go` / `terminal_other.go` via filename-based build constraints (no `//go:build` tags)
- **Embedded FS**: `//go:embed templates/*` in `embed.go`; accessed via `templatesFS.ReadFile()`
- **`text/template`**: used for KDL config/layout rendering in `zellij.go`
- **`flag` package**: standard library flags; custom `arrayFlags` type for repeatable `-v`/`-p` flags
- **`exec.Command`**: used directly for all Docker and `gh` CLI invocations; no wrapper abstraction
- **`defer` for cleanup**: temp files, lock files, terminal state restore

## Docker Command Assembly

`runArgs` slice is built up incrementally in `main.go`:
```go
runArgs := []string{"run", "-it", "--rm", ...}
runArgs = append(runArgs, "-v", mount)
// ... many conditional appends
execCmd := exec.Command("docker", runArgs...)
```
No abstraction over `docker run`; flags are appended directly. This makes the full command visible in `--verbose` mode via `log.Debugf("exec: docker %v", runArgs)`.

## File Permissions

- Executable scripts (agent.sh, files.sh, hints.sh): `0755` via `writeExecutable()`
- Config/data files: `0644`
- Directories: `0755`

## Template Configs

Conditionally copied based on enabled agents in `copyTemplateConfigs()`. Templates are always bundled in the binary; copying is idempotent (overwrites on each run for rovr, skip-if-exists pattern not used).
