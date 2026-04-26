---
phase: 01-mount-copy
verified: 2026-04-26T13:10:00Z
status: passed
score: 6/6 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 5/6
  gaps_closed:
    - "Non-interactive mode also gets the copy+translate via Dockerfile init script"
  gaps_remaining: []
  regressions: []
---

# Phase 1: Mount & Copy Verification Report

**Phase Goal:** Claude Code inside the container can load skills, plugins, and settings that were authored on the host
**Verified:** 2026-04-26T13:10:00Z
**Status:** passed
**Re-verification:** Yes — after gap closure (plan 01-02)

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `~/.claude` is mounted read-only at `/tmp/.claude`, NOT directly at `/root/.claude` | VERIFIED | `buildClaudeMountArgs` returns `%s:/tmp/.claude:ro`; old direct-mount format absent from mount block; `TestClaudeMountReadOnly` confirms |
| 2 | `HOST_HOME` env var is injected into the container when ClaudeCode is enabled | VERIFIED | `buildClaudeMountArgs` returns `HOST_HOME=%s` env arg; wired at main.go line 526; `TestHostHomeEnvVar` confirms |
| 3 | dockerSetup copies `/tmp/.claude` to `/root/.claude` and rewrites host paths in text files | VERIFIED | Lines 709-712 in main.go: `claudeSetup` appended to `dockerSetup` when ClaudeCode enabled; `TestDockerSetupClaudeSnippet` confirms copy+sed snippet |
| 4 | Binary files (`.wasm`, `.png`, `.jpg`) are copied without text substitution | VERIFIED | `grep -qI "" "$f"` guard present in both `buildClaudeMountArgs` snippet and Dockerfile script; `TestDockerSetupBinarySkip` confirms |
| 5 | Non-interactive mode also gets the copy+translate via Dockerfile init script | VERIFIED | `buildNIClaudeArgs` in `cli/noninteractive.go` wraps `claude` invocation in `sh -c dockerSetup+...` when dockerSetup is non-empty; wired at main.go line 785; 4 unit tests confirm |
| 6 | Claude Code resolves hook and plugin paths at `/root/.claude/...` (not host home dir) inside the container | VERIFIED | `sed -i "s|${HOST_HOME}/.claude|/root/.claude|g"` in both dockerSetup snippet and Dockerfile script rewrites all text-file references |

**Score:** 6/6 truths verified

### Re-verification: Gap Closure for Truth #5

**Previous failure:** The non-interactive branch in `main.go` appended `claude` directly to `runArgs` without prepending `dockerSetup`. Docker treats an explicit command argument as a replacement for CMD, so `agentjail-init-claude` (which lives in Dockerfile CMD) never ran.

**Fix applied (plan 01-02):**
- `buildNIClaudeArgs(dockerSetup, niPrompt, extraArgs)` pure helper added to `cli/noninteractive.go` (lines 86-97)
- When `dockerSetup` is non-empty, the function returns `["-e", "_AGENTJAIL_SYS=<prompt>", "sh", "-c", "<dockerSetup>exec claude --append-system-prompt \"$_AGENTJAIL_SYS\""]`
- Old two-line append at main.go ~785-786 replaced with single call: `runArgs = append(runArgs, buildNIClaudeArgs(dockerSetup, niPrompt, flag.Args())...)`
- 4 new unit tests in `cli/noninteractive_test.go` (lines 149-196) cover no-setup, no-setup+extra-args, with-setup, and with-setup+extra-args paths

**Verification of fix:**
- `buildNIClaudeArgs` defined at `cli/noninteractive.go:86`
- main.go line 785 calls `buildNIClaudeArgs(dockerSetup, niPrompt, flag.Args())`
- `dockerSetup` at this point contains the claude copy+translate snippet (lines 709-712) when ClaudeCode is enabled
- All 4 `TestBuildNIClaudeArgs_*` tests pass

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cli/main.go` | Volume mount change + HOST_HOME injection + dockerSetup snippet + NI wiring | VERIFIED | `buildClaudeMountArgs` at lines 20-39; wired at lines 522-534 (mount) and 709-712 (dockerSetup); `buildNIClaudeArgs` call at line 785 |
| `cli/claude_mount_test.go` | 5 unit tests for mount arg assembly and dockerSetup | VERIFIED | 5 test functions present and passing: `TestClaudeMountReadOnly`, `TestClaudeMountDisabled`, `TestHostHomeEnvVar`, `TestDockerSetupClaudeSnippet`, `TestDockerSetupBinarySkip` |
| `cli/templates/Dockerfile` | `agentjail-init-claude` script baked into image | VERIFIED | Script at lines 392-404; chmod at line 405; CMD invocation at line 424 (covers plain no-args container path) |
| `cli/noninteractive.go` | `buildNIClaudeArgs` helper | VERIFIED | Function defined at line 86; alongside other NI helpers (`nonInteractiveExecArgs`, `adaptRunArgsForNonInteractive`, `tryNILock`) |
| `cli/noninteractive_test.go` | 4 unit tests for `buildNIClaudeArgs` | VERIFIED | `TestBuildNIClaudeArgs_NoDockerSetup`, `TestBuildNIClaudeArgs_NoDockerSetup_WithExtraArgs`, `TestBuildNIClaudeArgs_WithDockerSetup`, `TestBuildNIClaudeArgs_WithDockerSetup_WithExtraArgs` all pass |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `cli/main.go` volume block | `cli/main.go` dockerSetup block | `HOST_HOME` env var consumed by `sed` in dockerSetup | VERIFIED | `HOST_HOME` injected at line 526; consumed in claudeSetup snippet via `${HOST_HOME}` shell variable at container runtime |
| `cli/main.go` dockerSetup | `cli/noninteractive.go` `buildNIClaudeArgs` | Non-interactive branch passes `dockerSetup` string to helper | VERIFIED | main.go line 785: `buildNIClaudeArgs(dockerSetup, niPrompt, flag.Args())`; helper wraps in `sh -c` when non-empty |
| `cli/main.go` dockerSetup | `cli/templates/Dockerfile` agentjail-init-claude | Dockerfile covers plain no-args path; dockerSetup covers interactive/zellij/NI paths | VERIFIED | All paths now covered: NI via `buildNIClaudeArgs`, interactive/zellij via inline `dockerSetup`, plain no-args via Dockerfile CMD |

### Data-Flow Trace (Level 4)

Not applicable — this phase produces infrastructure (volume mounts, startup scripts), not components rendering dynamic data.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `buildClaudeMountArgs` returns `:ro` mount | `go test -run TestClaudeMountReadOnly ./...` | PASS | PASS |
| `HOST_HOME` env arg included | `go test -run TestHostHomeEnvVar ./...` | PASS | PASS |
| dockerSetup snippet has `cp -r` and `sed -i` | `go test -run TestDockerSetupClaudeSnippet ./...` | PASS | PASS |
| Binary skip guard present | `go test -run TestDockerSetupBinarySkip ./...` | PASS | PASS |
| NI with dockerSetup wraps in `sh -c` | `go test -run TestBuildNIClaudeArgs_WithDockerSetup ./...` | PASS | PASS |
| NI without dockerSetup uses direct `claude` | `go test -run TestBuildNIClaudeArgs_NoDockerSetup ./...` | PASS | PASS |
| Full test suite clean | `go test ./...` | `ok agentjail-cli` | PASS |
| Binary compiles | `go build -o /dev/null .` | exit 0 | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| MOUNT-01 | 01-01-PLAN.md | Container mounts `~/.claude` read-only at `/tmp/.claude` instead of directly at `/root/.claude` | SATISFIED | `buildClaudeMountArgs` generates `%s:/tmp/.claude:ro`; old direct-mount format absent; `TestClaudeMountReadOnly` confirms |
| MOUNT-02 | 01-01-PLAN.md, 01-02-PLAN.md | Container startup script copies `/tmp/.claude` to `/root/.claude` with path substitution | SATISFIED | All paths covered: interactive/zellij via dockerSetup snippet; non-interactive via `buildNIClaudeArgs` wrapping in `sh -c dockerSetup+...`; plain CMD via Dockerfile `agentjail-init-claude` |
| MOUNT-03 | 01-01-PLAN.md | Path substitution skips binary files | SATISFIED | `grep -qI "" "$f"` guard in both dockerSetup snippet and Dockerfile script; `TestDockerSetupBinarySkip` confirms |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `cli/main.go` | 710 | `buildClaudeMountArgs("", "")` — called with empty strings to extract only the snippet | Info | Intentional design: snippet uses `${HOST_HOME}` as a shell variable resolved at container runtime, not Go-time. Function is pure and testable. No impact. |

No stub patterns, TODO markers, placeholder returns, or hardcoded empty data found in modified files.

### Human Verification Required

None. All truths verified mechanically against the codebase.

### Gaps Summary

No gaps. All 6 must-haves are verified. The single gap from the initial verification (MOUNT-02 non-interactive path) is now closed by plan 01-02:

- `buildNIClaudeArgs` in `cli/noninteractive.go` ensures the dockerSetup copy+translate snippet runs before `claude` starts in non-interactive mode
- The fix is mechanically verified: main.go line 785 passes `dockerSetup` (which contains the claude copy snippet when ClaudeCode is enabled) to `buildNIClaudeArgs`; 4 unit tests confirm correct behavior

---

_Verified: 2026-04-26T13:10:00Z_
_Verifier: Claude (gsd-verifier)_
