# Phase 1: Mount & Copy - Pattern Map

**Mapped:** 2026-04-25
**Files analyzed:** 2 (cli/main.go modified, cli/templates/Dockerfile possibly modified)
**Analogs found:** 2 / 2

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `cli/main.go` | utility (container arg assembly) | request-response | `cli/main.go` lines 529-541 (gitconfig mount block) + lines 668-677 (dockerSetup block) | exact — same file, same pattern repeated |
| `cli/templates/Dockerfile` | config | event-driven (container CMD) | `cli/templates/Dockerfile` line 404 (CMD line) | role-match — CMD extension for non-interactive gap |
| `cli/claude_mount_test.go` | test | request-response | `cli/noninteractive_test.go` (runArgs assembly tests) | role-match |

---

## Pattern Assignments

### `cli/main.go` — Volume Mount Change (lines 491-510)

**Analog:** Same file, gitconfig mount block at lines 529-541.

**Pattern to replace** (lines 497-503, existing — REMOVE this):
```go
hostClaudePath := filepath.Join(usr.HomeDir, ".claude")
if _, err := os.Stat(hostClaudePath); err == nil {
    claudeMount := fmt.Sprintf("%s:/root/.claude", hostClaudePath)
    runArgs = append(runArgs, "-v", claudeMount)
    volumes = append(volumes, claudeMount)
    log.Info("mounting host ~/.claude for Claude Code auth")
}
```

**Pattern to copy from** (lines 529-541 — gitconfig block, exact analog):
```go
// Mount system gitconfig if enabled.
// The file is mounted read-only at /tmp/.gitconfig and copied to ~/.gitconfig
// via the dockerSetup startup string, so the container gets its own mutable
// copy rather than a direct bind-mount into the home directory.
if globalConfig.MountSystemGitconfig {
    usr, _ := user.Current()
    gitconfigPath := filepath.Join(usr.HomeDir, ".gitconfig")
    if _, err := os.Stat(gitconfigPath); err == nil {
        gitconfigMount := fmt.Sprintf("%s:/tmp/.gitconfig:ro", gitconfigPath)
        runArgs = append(runArgs, "-v", gitconfigMount)
        volumes = append(volumes, gitconfigMount)
    }
}
```

**New code follows this shape exactly** — replace the direct `/root/.claude` mount with:
```go
hostClaudePath := filepath.Join(usr.HomeDir, ".claude")
if _, err := os.Stat(hostClaudePath); err == nil {
    // Mount read-only at /tmp/.claude — copied to /root/.claude at startup
    // with path substitution (see dockerSetup block below).
    claudeTmpMount := fmt.Sprintf("%s:/tmp/.claude:ro", hostClaudePath)
    runArgs = append(runArgs, "-v", claudeTmpMount)
    volumes = append(volumes, claudeTmpMount)
    // HOST_HOME is consumed by the dockerSetup sed substitution.
    runArgs = append(runArgs, "-e", fmt.Sprintf("HOST_HOME=%s", usr.HomeDir))
    log.Info("mounting host ~/.claude read-only at /tmp/.claude for Claude Code")
}
```

**HOST_HOME env injection analog** (lines 411-417 — HOST_UID/HOST_GID pattern):
```go
// Inject host UID/GID so the container can restore file ownership on exit.
if hostUser, err := user.Current(); err == nil {
    runArgs = append(runArgs,
        "-e", fmt.Sprintf("HOST_UID=%s", hostUser.Uid),
        "-e", fmt.Sprintf("HOST_GID=%s", hostUser.Gid),
    )
}
```
The `HOST_HOME` injection follows the same `-e KEY=VALUE` append into `runArgs`.

---

### `cli/main.go` — dockerSetup String Extension (after line 677)

**Analog:** Lines 668-677 (existing dockerSetup block).

**Pattern to copy from** (lines 668-677):
```go
dockerSetup := ""
if *privilegedPtr {
    dockerSetup = "command -v docker >/dev/null 2>&1 || (sudo apt-get update -qq && sudo apt-get install -y -qq docker-ce-cli && sudo apt-get clean && sudo rm -rf /var/lib/apt/lists/*); "
    log.Info("privileged mode: will install Docker CLI on startup if not already present")
}
if globalConfig.MountSystemGitconfig {
    // Copy the read-only /tmp mount to the user's home so git operations
    // inside the container use the host identity without modifying the host file.
    dockerSetup += `[ -f /tmp/.gitconfig ] && cp /tmp/.gitconfig ~/.gitconfig 2>/dev/null; `
}
```

**New block appended after line 677** — follows the same `dockerSetup +=` pattern:
```go
if globalConfig.AgentFrameworks.ClaudeCode.Enabled {
    // Copy /tmp/.claude (read-only mount) to /root/.claude and rewrite all
    // host home-dir paths in text files so hooks and plugin installPaths
    // resolve correctly inside the container.
    dockerSetup += `if [ -d /tmp/.claude ] && [ ! -d /root/.claude ]; then ` +
        `cp -r /tmp/.claude /root/.claude && ` +
        `find /root/.claude -type f | while IFS= read -r f; do ` +
            `if grep -qI "" "$f" 2>/dev/null; then ` +
                `sed -i "s|${HOST_HOME}/.claude|/root/.claude|g" "$f" 2>/dev/null || true; ` +
            `fi; ` +
        `done; ` +
    `fi; `
}
```

**Where dockerSetup flows** (already in codebase — no changes needed to these call sites):
- Line 760: `runArgs = append(runArgs, "sh", "-c", dockerSetup+zellijEntrypoint+...)`
- Line 771: `initCmd := fmt.Sprintf("...%s...", dockerSetup, cmd, shell)`
- Line 775: `initCmd := fmt.Sprintf("...%smise trust...", dockerSetup, shell)`
- Line 782: `initCmd := fmt.Sprintf("...%smise trust...", dockerSetup, shell)`

**Non-interactive gap (lines 746-751):** The `-N` path appends `claude` directly to `runArgs` and never wraps with `sh -c dockerSetup`. The copy snippet must reach the non-interactive container. The planner must decide between:
1. Embedding a `/usr/local/bin/agentjail-init-claude` script in the Dockerfile CMD (runs at container creation regardless of path)
2. Prepending the copy as a CMD override in the non-interactive runArgs

---

### `cli/templates/Dockerfile` — CMD Extension (line 404)

**Current CMD** (line 404):
```dockerfile
CMD /usr/local/bin/agentjail-install-browser || true && mise trust --yes && mise install || rich "No mise file detected" --print --style 'dark_goldenrod' && ${SHELL} && cd /project
```

**Analog pattern:** The Dockerfile already contains helper scripts baked into the image (e.g., `agentjail-install-browser` referenced at line 404). A new `/usr/local/bin/agentjail-init-claude` helper script follows this same pattern: installed during the Docker build, invoked from CMD.

**New script to add** (baked into Dockerfile, placed before CMD):
```dockerfile
RUN printf '%s\n' \
    '#!/bin/sh' \
    '# Copy /tmp/.claude to /root/.claude with host-path substitution.' \
    '# Invoked from CMD so it runs at container creation, not on docker exec.' \
    'if [ -d /tmp/.claude ] && [ ! -d /root/.claude ]; then' \
    '  cp -r /tmp/.claude /root/.claude' \
    '  find /root/.claude -type f | while IFS= read -r f; do' \
    '    if grep -qI "" "$f" 2>/dev/null; then' \
    '      sed -i "s|${HOST_HOME}/.claude|/root/.claude|g" "$f" 2>/dev/null || true' \
    '    fi' \
    '  done' \
    'fi' \
    > /usr/local/bin/agentjail-init-claude && chmod +x /usr/local/bin/agentjail-init-claude
```

**Updated CMD**:
```dockerfile
CMD /usr/local/bin/agentjail-init-claude; /usr/local/bin/agentjail-install-browser || true && mise trust --yes && mise install || rich "No mise file detected" --print --style 'dark_goldenrod' && ${SHELL} && cd /project
```

This approach makes the copy idempotent for the non-interactive path without modifying the NI runArgs assembly.

---

### `cli/claude_mount_test.go` — New Test File

**Analog:** `cli/noninteractive_test.go` — tests that verify `runArgs` slice contents by calling helper functions directly.

**Package and import pattern** (noninteractive_test.go lines 1-9):
```go
package main

import (
    "os"
    "path/filepath"
    "reflect"
    "testing"
    "time"
)
```

**Test structure pattern** (noninteractive_test.go lines 11-17):
```go
func TestNonInteractiveExecArgs_Basic(t *testing.T) {
    got := nonInteractiveExecArgs("agentjail.myproj", nil)
    want := []string{"exec", "-i", "agentjail.myproj", "claude"}
    if !reflect.DeepEqual(got, want) {
        t.Errorf("got %v, want %v", got, want)
    }
}
```

**What to test in `claude_mount_test.go`:**

The `main.go` volume assembly logic is not currently extracted into a testable function. The research (line 330) recommends extracting it into a helper (e.g., `buildClaudeVolumes(config, user)`) similar to `nonInteractiveExecArgs`. Tests should then:

- `TestClaudeMountReadOnly` — ClaudeCode enabled + `~/.claude` exists on host → verify `runArgs` contains `/tmp/.claude:ro`, does NOT contain `/root/.claude`
- `TestClaudeMountDisabled` — ClaudeCode disabled → verify no `/tmp/.claude` volume added
- `TestHostHomeEnvVar` — ClaudeCode enabled → verify `runArgs` contains `HOST_HOME=<value>`
- `TestDockerSetupClaudeSnippet` — ClaudeCode enabled → verify `dockerSetup` string contains `cp -r /tmp/.claude /root/.claude`

**Temp dir pattern from docker_test.go** (lines 9-27):
```go
func TestCreateTempDockerfile_CreatesFile(t *testing.T) {
    path, err := createTempDockerfile()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    defer os.Remove(path)
    // ... assert on file contents ...
}
```

---

## Shared Patterns

### Volume Assembly: Stat-Guard + Append
**Source:** `cli/main.go` lines 440-453 (copilot block), lines 529-541 (gitconfig block)
**Apply to:** The new `~/.claude` mount block

All conditional mounts follow this exact three-step shape:
```go
usr, _ := user.Current()
hostXPath := filepath.Join(usr.HomeDir, ".X")
if _, err := os.Stat(hostXPath); err == nil {
    xMount := fmt.Sprintf("%s:/container/path:flag", hostXPath)
    runArgs = append(runArgs, "-v", xMount)
    volumes = append(volumes, xMount)
}
```

### dockerSetup String: Conditional Append
**Source:** `cli/main.go` lines 673-677
**Apply to:** The new claude copy+translate snippet

All startup snippets use `dockerSetup += \`shell; \`` — backtick raw string, semicolon-terminated, trailing space. Conditional on the relevant config boolean.

### Env Injection: `-e KEY=VALUE` Append
**Source:** `cli/main.go` lines 411-417 (HOST_UID/HOST_GID)
**Apply to:** `HOST_HOME` injection

```go
runArgs = append(runArgs,
    "-e", fmt.Sprintf("HOST_UID=%s", hostUser.Uid),
    "-e", fmt.Sprintf("HOST_GID=%s", hostUser.Gid),
)
```

### user.Current() Error Handling
**Source:** `cli/main.go` lines 493-496
**Apply to:** Any new `user.Current()` call in the ClaudeCode block

```go
usr, err := user.Current()
if err != nil {
    log.Warnf("could not determine current user, skipping Claude Code host mounts: %v", err)
} else {
    // ... mount logic
}
```
The existing ClaudeCode block already uses this pattern; the new volume mount belongs inside the same `else` branch.

---

## No Analog Found

All files in scope have close analogs in the codebase. No "no analog" entries.

---

## Metadata

**Analog search scope:** `cli/main.go` (full read), `cli/templates/Dockerfile` (CMD grep), `cli/noninteractive_test.go`, `cli/docker_test.go`
**Files scanned:** 5
**Pattern extraction date:** 2026-04-25
