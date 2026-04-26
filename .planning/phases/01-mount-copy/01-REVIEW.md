---
phase: 01-mount-copy
reviewed: 2026-04-26T00:53:52Z
depth: standard
files_reviewed: 3
files_reviewed_list:
  - cli/claude_mount_test.go
  - cli/main.go
  - cli/templates/Dockerfile
findings:
  critical: 1
  warning: 3
  info: 3
  total: 7
status: issues_found
---

# Phase 1: Code Review Report

**Reviewed:** 2026-04-26T00:53:52Z
**Depth:** standard
**Files Reviewed:** 3
**Status:** issues_found

## Summary

This review covers the Phase 1 claude-mount changes: the new `buildClaudeMountArgs()` helper in `main.go`, its five unit tests in `claude_mount_test.go`, and the `agentjail-init-claude` startup script plus updated CMD in `cli/templates/Dockerfile`.

The mount strategy (read-only `/tmp/.claude` bind, copy-on-start, path substitution) is architecturally sound. The `grep -qI` binary guard is a reasonable choice for Ubuntu 24.04. However there is one security issue with how `HOST_HOME` is interpolated into a `sed` regex, one incorrect operator precedence in the Dockerfile CMD, one hollow test that provides no coverage, and a handful of lesser issues.

## Critical Issues

### CR-01: HOST_HOME not sanitized before sed regex interpolation

**File:** `cli/main.go:33`
**Issue:** `HOST_HOME` is injected directly as the left-hand side of a `sed -i "s|...|...|g"` expression inside a shell string without escaping. The `|` character is chosen as the sed delimiter precisely because paths "shouldn't" contain it — but nothing prevents a user whose home directory contains shell-significant or sed-significant characters (`[`, `*`, `.`, `\`, `&`) from triggering misbehavior. For example, a home path of `/home/user.name` would make the `.` a regex wildcard, matching more than intended. A path containing `\` would cause sed to interpret escape sequences in the replacement.

The same snippet is emitted verbatim into the `agentjail-init-claude` Dockerfile script (line 400), compounding the exposure.

**Fix:** Escape the value at the point the snippet is generated in Go so sed treats it as a literal string. In `buildClaudeMountArgs`, replace the sed expression with one that escapes both the search and replace strings:

```go
// Escape any sed-special characters in hostHome so the substitution is literal.
escapedHome := strings.NewReplacer(
    `\`, `\\`,
    `.`, `\.`,
    `*`, `\*`,
    `[`, `\[`,
    `^`, `\^`,
    `$`, `\$`,
    `/`, `\/`,
    `&`, `\&`,
).Replace(hostHome)

setupSnippet = `if [ -d /tmp/.claude ] && [ ! -d /root/.claude ]; then ` +
    `cp -r /tmp/.claude /root/.claude && ` +
    `find /root/.claude -type f | while IFS= read -r f; do ` +
    `if grep -qI "" "$f" 2>/dev/null; then ` +
    fmt.Sprintf(`sed -i 's|%s/.claude|/root/.claude|g' "$f" 2>/dev/null || true; `, escapedHome) +
    `fi; ` +
    `done; ` +
    `fi; `
```

Apply the same escaping to the static copy embedded in `agentjail-init-claude`. Since that script uses `${HOST_HOME}` at runtime, an alternative is to escape inside the shell script itself using `printf '%s\n' "${HOST_HOME}" | sed 's/[[\.*^$&/]/\\&/g'`.

## Warnings

### WR-01: TestClaudeMountDisabled provides no coverage of the guarded behavior

**File:** `cli/claude_mount_test.go:31-40`
**Issue:** The test body asserts that a zero-value `GlobalConfig{}` has `ClaudeCode.Enabled == false`. This is a tautology — it tests Go's zero-value initialization, not the `buildClaudeMountArgs` gate in `main.go`. The comment acknowledges this ("confirms the function is gated on config in the integration") but the assertion provides no signal if someone accidentally calls `buildClaudeMountArgs` when `Enabled == false`, or if the gating code in `main.go` is removed.

**Fix:** Either delete this test (it gives false confidence) or replace it with a test that actually exercises the gate:

```go
func TestClaudeMountDisabledSkipsArgs(t *testing.T) {
    // When ClaudeCode is disabled, main.go must not call buildClaudeMountArgs.
    // Simulate the gate directly.
    cfg := &GlobalConfig{}
    cfg.AgentFrameworks.ClaudeCode.Enabled = false

    var volArgs []string
    if cfg.AgentFrameworks.ClaudeCode.Enabled {
        volArgs, _, _ = buildClaudeMountArgs("/some/.claude", "/some")
    }
    if len(volArgs) > 0 {
        t.Errorf("expected no volume args when ClaudeCode disabled, got %v", volArgs)
    }
}
```

### WR-02: Dockerfile CMD — `cd /project` is dead code after interactive shell exits

**File:** `cli/templates/Dockerfile:424`
**Issue:** The CMD ends with `&& ${SHELL} && cd /project`. The `cd /project` runs only after the interactive shell (`${SHELL}`) exits, which is the moment the container is already terminating. It has no effect.

More importantly, if the interactive shell exits with a non-zero code (e.g., user typed `exit 1`), the `&&` short-circuits and `cd /project` is skipped anyway — which is also harmless but reinforces that the instruction does nothing.

**Fix:** Remove `&& cd /project` from the CMD:

```dockerfile
CMD /usr/local/bin/agentjail-init-claude; /usr/local/bin/agentjail-install-browser || true && mise trust --yes && mise install || rich "No mise file detected" --print --style 'dark_goldenrod' && ${SHELL}
```

### WR-03: `grep -qI ""` binary detection is a GNU-only extension with no fallback

**File:** `cli/main.go:32`, `cli/templates/Dockerfile:399`
**Issue:** `grep -I` (treat binary files as not matching) is a GNU grep extension, not POSIX. The image currently uses Ubuntu 24.04 which ships GNU grep, so this works today. However:
- The flag is not documented in the script as GNU-specific, making portability assumptions implicit.
- A future Alpine-based Dockerfile variant would use BusyBox grep which does not support `-I`, causing `grep -qI "" "$f"` to fail with an error. The `2>/dev/null` silencer would eat the error, and because the exit code is non-zero the `if` guard would treat every file as binary and skip all path substitution.

**Fix:** Add a comment to both the Go snippet and the Dockerfile script making the GNU grep dependency explicit. If Alpine portability is desired, use a file-magic check instead:

```sh
# GNU grep -I: treat binary files as non-matching; requires GNU grep (Ubuntu/Debian).
if grep -qI "" "$f" 2>/dev/null; then
```

Alternatively, replace with `file --mime-encoding "$f" | grep -q text` which is portable but slower.

## Info

### IN-01: sed pattern matches path prefix — may over-substitute sibling paths

**File:** `cli/main.go:33`, `cli/templates/Dockerfile:400`
**Issue:** The substitution `s|${HOST_HOME}/.claude|/root/.claude|g` will replace any occurrence of `${HOST_HOME}/.claude` regardless of what follows. If a Claude config file stores a reference to a sibling path like `${HOST_HOME}/.claude_cache` or `${HOST_HOME}/.claude-sessions`, those would be incorrectly rewritten to `/root/.claude_cache` (which does not exist in the container).

In practice Claude Code's config files are unlikely to reference sibling `.claude*` paths, but this is a latent correctness risk.

**Fix:** Tighten the pattern to match only the `.claude` directory itself (followed by `/` or end-of-token):

```sh
sed -i "s|${HOST_HOME}/.claude/|/root/.claude/|g; s|${HOST_HOME}/.claude$|/root/.claude|g" "$f"
```

Or use a word-boundary variant if the target files are JSON:

```sh
sed -i "s|\"${HOST_HOME}/.claude\"|\"\/root\/.claude\"|g" "$f"
```

### IN-02: Missing test for the case where ~/.claude does not exist

**File:** `cli/claude_mount_test.go`
**Issue:** There is no test covering the scenario where ClaudeCode is enabled in config but `~/.claude` does not exist on the host. In `main.go` (line 523), the `os.Stat` gate prevents `buildClaudeMountArgs` from being called, so no volume or env arg is emitted. The `dockerSetup` snippet is still appended (line 710) via `buildClaudeMountArgs("", "")`, but the snippet's own `[ -d /tmp/.claude ]` guard makes it a no-op at runtime.

This chain is safe, but a test documenting the expected "no volume args when dir absent" behavior would protect against a future refactor that accidentally drops the `os.Stat` guard.

**Fix:** Add a test:

```go
func TestClaudeMountNoVolumesWhenDirAbsent(t *testing.T) {
    absent := filepath.Join(t.TempDir(), ".claude") // does not exist
    // Simulate the main.go gate: only call buildClaudeMountArgs if dir exists.
    if _, err := os.Stat(absent); os.IsNotExist(err) {
        return // correctly skipped
    }
    t.Error("expected buildClaudeMountArgs to be skipped when .claude does not exist")
}
```

### IN-03: `agentjail-init-claude` script uses `/bin/sh` but assumes `sed -i` GNU behavior

**File:** `cli/templates/Dockerfile:393`
**Issue:** The script declares `#!/bin/sh` but uses `sed -i "..."` with double-quoted strings. On Ubuntu 24.04 `/bin/sh` is `dash`, and `sed -i ""` (with empty suffix) is a BSD-ism. GNU sed accepts `sed -i` without a suffix argument, which is what this script does — but the mismatch between `sh` shebang and GNU sed assumptions makes the portability intent unclear.

This is not a bug on Ubuntu 24.04 (GNU sed accepts this), but the inconsistency is a maintenance signal.

**Fix:** Either change the shebang to `#!/bin/bash` to make the GNU dependency explicit, or add a comment:

```sh
#!/bin/sh
# Requires GNU sed (available on Ubuntu 24.04 via /usr/bin/sed).
```

---

_Reviewed: 2026-04-26T00:53:52Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
