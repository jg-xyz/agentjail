# Architecture
_Last updated: 2026-04-25_

## Summary

AgentJail is a single Go binary CLI tool that provisions isolated Docker containers for AI coding agents. There is no server, no database, and no separate frontend — all logic lives in `cli/` and executes as a short-lived host process that ultimately replaces itself with a `docker run` or `docker exec` invocation.

## Pattern Overview

**Overall:** Imperative orchestrator with embedded assets

**Key Characteristics:**
- No persistent daemon; every invocation is a one-shot process
- `main.go` is the sole orchestrator — it calls helpers sequentially and produces a `docker run` command
- All configuration files and templates are compiled into the binary via `//go:embed`
- The binary exits as soon as the container session ends (exec handoff)

## Layers

**Configuration layer:**
- Purpose: Load and validate user preferences; apply env overrides
- Location: `cli/config.go`
- Contains: `GlobalConfig` struct, `loadGlobalConfig()`, `saveGlobalConfig()`, `runConfigUpdate()`, `printCleanConfig()`
- Depends on: `gopkg.in/yaml.v3`, host filesystem at `~/.config/agentjail/config.yaml`
- Used by: `main.go` (always first step)

**Docker interaction layer:**
- Purpose: Check image/container state; build image from embedded Dockerfile
- Location: `cli/docker.go`
- Contains: `imageExists()`, `getContainerForDirectory()`, `isContainerRunning()`, `createTempDockerfile()`
- Depends on: `docker` CLI (invoked via `os/exec`), `embed.go` for Dockerfile bytes
- Used by: `main.go`

**Agent resolution layer:**
- Purpose: Translate agent names to shell commands; build Claude system prompt
- Location: `cli/agent.go`
- Contains: `enabledAgents()`, `agentCommand()`, `chooseEnabledAgent()`, `resolveClaudeContext()`
- Depends on: `GlobalConfig`
- Used by: `main.go` (for both `-A` flag and zellij entrypoint)

**Filesystem setup layer:**
- Purpose: Initialize `.agentjail/` in the project directory; copy template configs
- Location: `cli/filesystem.go`
- Contains: `createAgentJailFolder()`, `updateGitignore()`, `copyTemplateConfigs()`, `ensureFileFromTemplate()`
- Depends on: `embed.go` for template content
- Used by: `main.go` before container launch

**Metadata layer:**
- Purpose: Persist container session info to `.agentjail/metadata.json`
- Location: `cli/metadata.go`
- Contains: `AgentJailMetadata` struct, `saveMetadata()`, `loadMetadata()`, `checkVersionUpdate()`
- Depends on: host filesystem
- Used by: `main.go`

**Zellij layer:**
- Purpose: Generate KDL layout/config files and tab launcher scripts for the zellij multiplexer
- Location: `cli/zellij.go`
- Contains: `writeZellijFiles()`, `buildZellijEntrypoint()`, `buildBottomBar()`, `copyPlugins()`, `downloadPlugin()`
- Depends on: `embed.go` for KDL templates, `net/http` for URL-based plugin downloads
- Used by: `main.go` (when `ZellijEnabled()` is true)

**Non-interactive layer:**
- Purpose: Handle VS Code process-wrapper mode (no TTY); coordinate concurrent container starts
- Location: `cli/noninteractive.go`
- Contains: `nonInteractiveExecArgs()`, `adaptRunArgsForNonInteractive()`, `tryNILock()`, `releaseNILock()`, `niContainerNameForPrefix()`
- Depends on: host filesystem for lock file at `.agentjail/ni.lock`
- Used by: `main.go` when `-N` / `--noninteractive` flag is set

**Terminal layer:**
- Purpose: Save/restore terminal state around `docker exec`
- Location: `cli/terminal_other.go` (POSIX no-ops), `cli/terminal_windows.go` (Windows API)
- Contains: `saveConsoleCP()`, `restoreConsoleCP()`
- Depends on: `golang.org/x/term`, Windows API (Windows build only)
- Used by: `main.go` via `runWithTerminalRestore()`

**Embedded assets:**
- Purpose: Bundle all templates into the binary so no external files are needed at runtime
- Location: `cli/embed.go`
- Contains: `//go:embed templates/*` directive exposing `templatesFS embed.FS`
- Consumed by: `docker.go`, `filesystem.go`, `zellij.go`

## Entry Point Flow

**Standard interactive launch (`main.go`):**

1. `initLogger()` — configure logrus
2. Pre-scan `os.Args` for `--config` (optional path or print-mode)
3. Handle subcommands (`update-config`)
4. `loadGlobalConfig()` — read `~/.config/agentjail/config.yaml` (create defaults if absent)
5. Parse flags (`-d`, `-C`, `-E`, `-D`, `-e`, `-s`, `-n`, `-b`, `-A`, `-N`, `-v`, `-p`, etc.)
6. **Auto-exec path**: if no args, call `getContainerForDirectory(cwd)` — if a container is found, `docker exec -it` into it and exit
7. Resolve image: `imageExists()` → build from embedded Dockerfile if missing or `-b` requested
8. `createAgentJailFolder()` — create `<project>/.agentjail/`
9. `updateGitignore()` — add `.agentjail/` to `.gitignore`
10. `copyTemplateConfigs()` — seed agent config files from embedded templates
11. `writeZellijFiles()` — render KDL layout + tab scripts (when zellij enabled)
12. Assemble `docker run` args: volumes, env vars, port mappings, credential mounts
13. `saveMetadata()` — write `.agentjail/metadata.json`
14. Determine entrypoint: zellij / plain shell / non-interactive claude
15. `runWithTerminalRestore(docker run ...)` — exec container (blocks until session ends)

## Data Flow

**Configuration resolution:**
```
~/.config/agentjail/config.yaml
  → GlobalConfig struct (loadGlobalConfig)
    → applyEnvOverrides (AGENTJAIL_* vars)
      → flags override specific fields
        → docker run args assembled in main.go
```

**Agent credential flow:**
```
Host ~/.config/github-copilot  →  bind-mount → /root/.config/github-copilot
Host ~/.config/gh               →  bind-mount → /root/.config/gh
Host ~/.claude / ~/.claude.json →  bind-mount → /root/.claude[.json]
GH_TOKEN / GITHUB_TOKEN env    →  -e flag    → container env
ANTHROPIC_API_KEY env          →  -e flag    → container env (valueless, inherits parent env)
```

**Zellij tab auto-launch:**
```
writeZellijFiles() writes:
  .agentjail/zellij/layout.kdl       (3-tab layout: agent, shell, files)
  .agentjail/zellij/config.kdl       (theme + keybinds)
  .agentjail/zellij/tabs/agent.sh    (sets AGENTJAIL_TAB_CMD → shell rc hook launches agent)
  .agentjail/zellij/tabs/files.sh    (sets AGENTJAIL_TAB_CMD → shell rc hook launches file browser)
  .agentjail/zellij/tabs/hints.sh    (static keybind hint bar)
```

**Non-interactive (VS Code process wrapper) flow:**
```
agentjail -N [claude args]
  → getContainerForDirectory() → found? → docker exec -i <container> claude [args]
  → not found: tryNILock(.agentjail/ni.lock)
      won lock? → docker run -i agentjail-ni.<prefix> claude [args]
                → background goroutine releases lock when container starts
      lost lock? → poll isContainerRunning() up to 10s → docker exec into peer's container
```

## Key Abstractions

**GlobalConfig:**
- Purpose: Typed representation of `~/.config/agentjail/config.yaml`
- Location: `cli/config.go` — `type GlobalConfig struct`
- Pattern: Plain struct with YAML tags; helper methods `ZellijEnabled()`, `FileBrowserCmd()`, `ZellijThemeOrDefault()`

**AgentJailMetadata:**
- Purpose: Record what container was started and with which mounts/env vars
- Location: `cli/metadata.go` — `type AgentJailMetadata struct`
- Pattern: JSON file at `<project>/.agentjail/metadata.json`; read on version-check, written each launch

**templatesFS:**
- Purpose: Single embed.FS holding all files under `cli/templates/`
- Location: `cli/embed.go`
- Pattern: Other files call `templatesFS.ReadFile("templates/...")` directly; no abstraction layer

**ZellijPlugin:**
- Purpose: Represent a single `.wasm` plugin for the zellij status bar
- Location: `cli/config.go` — `type ZellijPlugin struct`
- Pattern: Either `Path` (host file, copied each launch) or `URL` (downloaded and cached on first use)

## Error Handling

**Strategy:** Fail-fast for unrecoverable errors; warn-and-continue for optional features

**Patterns:**
- Fatal errors: `log.Fatalf(...)` — terminates the process immediately
- Non-fatal errors (e.g. failing to update `.gitignore`, write metadata): `log.Warnf(...)` — execution continues
- Template/config creation failures that block agent functionality: fatal
- Credential mount failures: warn only (agent may fail later)

## Cross-Cutting Concerns

**Logging:** `cli/logging.go` — package-level `logrus.Logger` (`var log`). Info level by default; debug enabled with `--verbose`. All files use `log.*` directly (no logger injection).

**Shell escaping:** `shellEscape()` in `cli/zellij.go` — used whenever user-supplied strings (agent commands, prompts) are embedded in shell invocations passed to `docker run ... sh -c "..."`.

**Platform abstraction:** File-naming build tags (`_windows.go` / `_other.go`) for console code page save/restore; no other platform-specific code.

**Container naming:** `agentjail.<first5chars-of-project-dir>` for interactive; `agentjail-ni.<first5chars>` for non-interactive. Collisions across similarly-named projects are a known limitation.

---

*Architecture analysis: 2026-04-25*
