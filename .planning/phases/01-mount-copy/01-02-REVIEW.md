---
phase: 01-mount-copy
reviewed: 2026-04-26T00:00:00Z
depth: standard
files_reviewed: 3
files_reviewed_list:
  - cli/noninteractive.go
  - cli/noninteractive_test.go
  - cli/main.go
findings:
  critical: 0
  warning: 3
  info: 2
  total: 5
status: issues_found
---

# Phase 01-02: Code Review Report

**Reviewed:** 2026-04-26
**Depth:** standard
**Files Reviewed:** 3
**Status:** issues_found

## Summary

This review covers the gap-closure change for MOUNT-02: the `buildNIClaudeArgs` function added to `cli/noninteractive.go` and its call site in `cli/main.go`. The function solves the correct problem — injecting the `dockerSetup` snippet before the `claude` invocation in non-interactive mode via an `sh -c` wrapper with the system prompt passed through an environment variable to avoid shell quoting hazards.

Three warnings and two info items were found. No critical issues.

---

## Warnings

### WR-01: `buildClaudeMountArgs("", "")` produces the setup snippet unconditionally regardless of whether a real mount exists

**File:** `cli/main.go:710`
**Issue:** The Claude Code setup snippet is appended to `dockerSetup` by calling `buildClaudeMountArgs("", "")` and discarding the volume/env return values. This always appends the `cp -r /tmp/.claude /root/.claude` snippet — even when no `~/.claude` directory was found on the host and therefore no `/tmp/.claude` volume was mounted. The snippet has an `[ -d /tmp/.claude ]` guard so it is safe at runtime, but the pattern is misleading: the outer `if globalConfig.AgentFrameworks.ClaudeCode.Enabled` block at line 517 only mounts `/tmp/.claude` when `~/.claude` actually exists on the host, yet the setup snippet is added unconditionally at line 710 regardless. This divergence means the non-interactive path always adds the copy snippet while the volume it depends on may be absent. The bug is latent — the guard prevents incorrect behaviour — but the inconsistency is a logic error waiting to become a real failure if the guard is ever changed.

**Fix:** Capture the return value of the real `buildClaudeMountArgs` call (line 524) and reuse the snippet there, or introduce a boolean `claudeMounted` that gates the snippet append at line 710:

```go
// In the ClaudeCode enabled block (around line 523):
volArgs, envArgs, claudeSetup := buildClaudeMountArgs(hostClaudePath, usr.HomeDir)
runArgs = append(runArgs, volArgs...)
runArgs = append(runArgs, envArgs...)
// ... volume tracking ...
dockerSetup += claudeSetup  // moved here, inside the "path exists" branch

// Line 709-712 becomes:
// (delete the second buildClaudeMountArgs call entirely)
```

---

### WR-02: `extraArgs` values are joined with space and embedded raw into the shell script — shell-injection risk for arguments containing spaces or shell metacharacters

**File:** `cli/noninteractive.go:92-96`
**Issue:** When `dockerSetup` is non-empty, `buildNIClaudeArgs` joins `extraArgs` with a plain space and concatenates them into the `sh -c` script string:

```go
extra = " " + strings.Join(extraArgs, " ")
script := dockerSetup + `exec claude --append-system-prompt "$_AGENTJAIL_SYS"` + extra
```

If any element of `extraArgs` contains a space, a single-quote, a semicolon, or a subshell sequence, it is passed verbatim into the shell script. For example, `--model claude; rm -rf /project` would be interpreted as two shell commands. `extraArgs` comes from `flag.Args()` (user-supplied CLI arguments passed through to `claude`), so the input is controlled by the operator rather than an end-user, but it is still unsanitized shell injection at the script level.

**Fix:** Shell-quote each argument before joining. The standard approach in Go is to wrap each argument in single quotes and escape embedded single quotes:

```go
quoted := make([]string, len(extraArgs))
for i, a := range extraArgs {
    quoted[i] = "'" + strings.ReplaceAll(a, "'", "'\\''") + "'"
}
extra = " " + strings.Join(quoted, " ")
```

Alternatively, restructure the invocation so `extraArgs` are passed as positional parameters to `sh` rather than embedded in the script string (e.g. `sh -c 'setup; exec claude "$@"' -- arg1 arg2`), which avoids quoting entirely.

---

### WR-03: Lock file release goroutine has no shutdown path if the container never starts

**File:** `cli/main.go:759-767`
**Issue:** When the process wins the NI lock, a goroutine polls `isContainerRunning(niName)` for up to 15 seconds and then releases the lock. If the subsequent `docker run` fails before the container becomes visible (e.g. image pull failure, port conflict), the goroutine exhausts all 75 iterations and releases the lock — but `niLockFile` is already set. Later, the `runCmd.Run()` failure handler at line 835 calls `releaseNILock(niLockFile)` again on the already-released (closed + removed) file handle. `f.Close()` on an already-closed file returns an error that is ignored, and `os.Remove` on a non-existent file is a no-op, so there is no crash — but the double-release is a logic error and the error from `f.Close()` is silently swallowed.

**Fix:** Nil out `niLockFile` after the goroutine releases it, or use a `sync.Once` to ensure `releaseNILock` runs at most once:

```go
var releaseOnce sync.Once
go func() {
    for i := 0; i < 75; i++ {
        time.Sleep(200 * time.Millisecond)
        if isContainerRunning(niName) {
            break
        }
    }
    releaseOnce.Do(func() { releaseNILock(lockFile) })
}()
// ...
// In the error handler:
releaseOnce.Do(func() { releaseNILock(niLockFile) })
```

---

## Info

### IN-01: `niPrompt` construction does not include `claudeExtraContext` when it is empty — minor inconsistency with how the interactive path assembles the prompt

**File:** `cli/main.go:781-784`
**Issue:** The non-interactive prompt is assembled as:

```go
niPrompt := claudeSystemPrompt
if claudeExtraContext != "" {
    niPrompt += "\n\n" + claudeExtraContext
}
```

This is correct, but the interactive path delegates to `agentCommand` / `resolveClaudeContext` which does the same concatenation. The two paths are slightly asymmetric: the interactive path uses the fully resolved `claudeExtraContext` string that already merges the config value and the `--claude-context` flag value; the non-interactive path re-does the concatenation manually. If `resolveClaudeContext` logic changes in the future the non-interactive path may diverge. Consider extracting the full prompt construction into a shared helper.

**Fix:** Low priority. No immediate action required; note for future refactoring.

---

### IN-02: Missing test for shell-quoting behaviour of `extraArgs` in `buildNIClaudeArgs`

**File:** `cli/noninteractive_test.go`
**Issue:** `TestBuildNIClaudeArgs_WithDockerSetup_WithExtraArgs` (line 189) only checks that `--model opus` appears in the script string. It does not test arguments that contain spaces or shell metacharacters, which is where WR-02 would surface. Adding a test case like `[]string{"--model", "claude opus 4"}` would have caught the quoting gap.

**Fix:** Add a test case with a space-containing argument and assert the resulting script is properly quoted (or refactor the implementation per WR-02 first).

---

_Reviewed: 2026-04-26_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
