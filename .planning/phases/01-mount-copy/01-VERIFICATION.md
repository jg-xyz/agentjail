---
phase: 01-mount-copy
verified: 2026-04-25T21:30:00Z
status: gaps_found
score: 5/6 must-haves verified
overrides_applied: 0
gaps:
  - truth: "Non-interactive mode also gets the copy+translate via Dockerfile init script"
    status: failed
    reason: >
      In non-interactive mode, docker run is called with "claude --append-system-prompt ..."
      appended to runArgs (main.go line 785). When arguments follow the image name in
      docker run, Docker treats them as the command to execute and skips the CMD instruction
      entirely. agentjail-init-claude lives in the CMD line (Dockerfile line 424), so it
      never runs in non-interactive mode. The dockerSetup snippet (which contains the copy
      logic) is also not applied in this path — the non-interactive block does not wrap
      runArgs in "sh -c dockerSetup+...". The result: when agentjail -N is used as a
      VS Code process wrapper, /root/.claude is never populated from /tmp/.claude, so
      Claude Code cannot access host skills, plugins, or settings.
    artifacts:
      - path: "cli/main.go"
        issue: >
          Lines 780-786: non-interactive runArgs append "claude" directly without
          prepending dockerSetup or agentjail-init-claude. Lines 699-712 build
          claudeSetup into dockerSetup but lines 795-818 show dockerSetup is only
          consumed in the zellij and plain-shell branches, never in the nonInteractivePtr branch.
      - path: "cli/templates/Dockerfile"
        issue: >
          Line 424: agentjail-init-claude is invoked in CMD, but CMD is overridden
          whenever docker run receives an explicit command argument.
    missing:
      - >
        In the non-interactive branch of main.go (around line 780-786), prepend
        the agentjail-init-claude call before claude. Either wrap in sh -c (e.g.,
        runArgs = append(runArgs, "sh", "-c", "agentjail-init-claude; exec claude ..."))
        or add the copy snippet inline before appending "claude" as the command.
---

# Phase 1: Mount & Copy Verification Report

**Phase Goal:** Claude Code inside the container can load skills, plugins, and settings that were authored on the host
**Verified:** 2026-04-25T21:30:00Z
**Status:** gaps_found
**Re-verification:** No - initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `~/.claude` is mounted read-only at `/tmp/.claude`, NOT directly at `/root/.claude` | VERIFIED | `buildClaudeMountArgs` returns `%s:/tmp/.claude:ro`; old `%s:/root/.claude` format absent from mount block |
| 2 | `HOST_HOME` env var is injected into the container when ClaudeCode is enabled | VERIFIED | `buildClaudeMountArgs` returns `HOST_HOME=%s` env arg; wired at main.go line 526 |
| 3 | dockerSetup copies `/tmp/.claude` to `/root/.claude` and rewrites host paths in text files | VERIFIED | Lines 709-712: `claudeSetup` appended to `dockerSetup` when ClaudeCode enabled; snippet confirmed by `TestDockerSetupClaudeSnippet` |
| 4 | Binary files (`.wasm`, `.png`, `.jpg`) are copied without text substitution | VERIFIED | `grep -qI "" "$f"` guard present in both `buildClaudeMountArgs` snippet and Dockerfile script; confirmed by `TestDockerSetupBinarySkip` |
| 5 | Non-interactive mode also gets the copy+translate via Dockerfile init script | FAILED | CMD is bypassed when `docker run` receives an explicit command; non-interactive path appends `claude` directly, overriding CMD entirely |
| 6 | Claude Code resolves hook and plugin paths at `/root/.claude/...` (not host home dir) inside the container | VERIFIED | `sed -i "s|${HOST_HOME}/.claude|/root/.claude|g"` in both dockerSetup snippet and Dockerfile script rewrites all text-file references |

**Score:** 5/6 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cli/main.go` | Volume mount change + HOST_HOME injection + dockerSetup snippet | VERIFIED | `buildClaudeMountArgs` function at lines 16-39; wired at lines 522-534 (mount) and 709-712 (dockerSetup) |
| `cli/claude_mount_test.go` | 5 unit tests for mount arg assembly and dockerSetup | VERIFIED | 5 test functions present and passing: `TestClaudeMountReadOnly`, `TestClaudeMountDisabled`, `TestHostHomeEnvVar`, `TestDockerSetupClaudeSnippet`, `TestDockerSetupBinarySkip` |
| `cli/templates/Dockerfile` | `agentjail-init-claude` script baked into image | VERIFIED | Script at lines 392-404; chmod at line 405; CMD invocation at line 424 |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `cli/main.go` volume block | `cli/main.go` dockerSetup block | `HOST_HOME` env var consumed by `sed` in dockerSetup | VERIFIED | `HOST_HOME` injected at line 526; consumed in claudeSetup snippet via `${HOST_HOME}` shell variable at runtime |
| `cli/main.go` dockerSetup | `cli/templates/Dockerfile` agentjail-init-claude | Both implement same copy+translate logic; Dockerfile covers non-interactive path | PARTIAL | Dockerfile script exists and implements identical logic; however, it is NOT invoked in the actual non-interactive path because CMD is overridden by the explicit `claude` command |

### Data-Flow Trace (Level 4)

Not applicable - this phase produces infrastructure (volume mounts, startup scripts), not components rendering dynamic data.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `buildClaudeMountArgs` returns `:ro` mount | `go test -run TestClaudeMountReadOnly ./...` | PASS | PASS |
| `HOST_HOME` env arg included | `go test -run TestHostHomeEnvVar ./...` | PASS | PASS |
| dockerSetup snippet has `cp -r` and `sed -i` | `go test -run TestDockerSetupClaudeSnippet ./...` | PASS | PASS |
| Binary skip guard present | `go test -run TestDockerSetupBinarySkip ./...` | PASS | PASS |
| Full test suite clean | `go test ./...` | ok agentjail-cli | PASS |
| Binary compiles | `go build -o /dev/null .` | exit 0 | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| MOUNT-01 | 01-01-PLAN.md | Container mounts `~/.claude` read-only at `/tmp/.claude` instead of directly at `/root/.claude` | SATISFIED | `buildClaudeMountArgs` generates `%s:/tmp/.claude:ro`; old direct mount format absent; `TestClaudeMountReadOnly` confirms |
| MOUNT-02 | 01-01-PLAN.md | Container startup script copies `/tmp/.claude` to `/root/.claude` with path substitution | PARTIALLY SATISFIED | dockerSetup covers interactive/zellij paths; Dockerfile CMD covers plain no-args path; non-interactive path (`-N` flag) receives neither |
| MOUNT-03 | 01-01-PLAN.md | Path substitution skips binary files | SATISFIED | `grep -qI "" "$f"` guard in both locations; `TestDockerSetupBinarySkip` confirms |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `cli/main.go` | 710 | `buildClaudeMountArgs("", "")` — called with empty strings to extract only the snippet | Info | Intentional design: snippet uses `${HOST_HOME}` as a shell variable resolved at runtime, not Go-time. Function is pure and testable. No impact. |

No stub patterns, TODO markers, placeholder returns, or hardcoded empty data found in modified files.

### Human Verification Required

None for automated checks. The non-interactive path failure is mechanically verified by reading the code path.

### Gaps Summary

One gap blocks full goal achievement in the non-interactive container path.

**Root cause:** The plan's strategy for non-interactive coverage — using a Dockerfile CMD init script — does not work because Docker replaces CMD with the explicit command argument supplied by `docker run`. When agentjail's non-interactive mode calls `docker run ... agentjail claude --append-system-prompt ...`, Docker runs `claude` directly and never invokes `agentjail-init-claude`.

**Impact:** Claude Code launched via the `-N` process wrapper (VS Code integration) starts with `/root/.claude` empty. Host skills, plugins, and settings are inaccessible. MOUNT-02 is not satisfied for this path.

**Affected code:** `cli/main.go` lines 780-786 (where `claude` is appended to `runArgs` without prior copy step). The fix belongs here, not in the Dockerfile.

**Interactive and zellij paths are correct:** The `dockerSetup` string containing the copy+translate snippet is properly consumed in the zellij entrypoint (line 795) and the plain-shell `-A`/dockerSetup branches (lines 806-818). Only the non-interactive branch at lines 715-786 is missing the copy step.

---

_Verified: 2026-04-25T21:30:00Z_
_Verifier: Claude (gsd-verifier)_
