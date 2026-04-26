---
phase: 01-mount-copy
plan: 02
subsystem: noninteractive
tags: [go, docker, non-interactive, tdd, gap-closure]
gap_closure: true

requires: [01-01]
provides:
  - buildNIClaudeArgs helper function in cli/noninteractive.go
  - non-interactive branch wired to prepend dockerSetup before claude
  - 4 unit tests in cli/noninteractive_test.go

key-files:
  created:
    - cli/noninteractive_test.go (4 new tests added)
  modified:
    - cli/noninteractive.go
    - cli/main.go

key-decisions:
  - "buildNIClaudeArgs placed in noninteractive.go (alongside nonInteractiveExecArgs, adaptRunArgsForNonInteractive) — not main.go as plan suggested"
  - "Prompt passed via _AGENTJAIL_SYS env var to avoid shell quoting hazards with --append-system-prompt"
  - "Extra args joined with strings.Join and appended to sh -c script string"

requirements-completed: [MOUNT-02]

duration: 5min
completed: 2026-04-26
---

# Phase 01 Plan 02: MOUNT-02 Gap Closure Summary

**Non-interactive mode now prepends the dockerSetup copy+translate snippet before invoking claude, matching the zellij and plain-shell branches.**

## Performance

- **Duration:** ~5 min
- **Completed:** 2026-04-26
- **Tasks:** 1 (TDD: RED → GREEN → wire)
- **Files modified:** 3

## Accomplishments

- `buildNIClaudeArgs(dockerSetup, niPrompt, extraArgs)` pure helper added to `cli/noninteractive.go`
- Non-interactive branch in `cli/main.go` (formerly 2 `append` lines) replaced with single `buildNIClaudeArgs` call
- 4 unit tests added to `cli/noninteractive_test.go` covering: no-setup path, no-setup+extra-args, with-setup, with-setup+extra-args
- All existing tests continue to pass

## Task Commits

1. `788bf55` — feat(01-02): add buildNIClaudeArgs + wire non-interactive branch (MOUNT-02)

## Decisions Made

- Function placed in `noninteractive.go` rather than `main.go` — that is where the other NI helpers (`nonInteractiveExecArgs`, `adaptRunArgsForNonInteractive`, `tryNILock`) live, making it immediately discoverable
- `strings` import added to both `noninteractive.go` (implementation) and `noninteractive_test.go` (test assertions)

## Deviations from Plan

- Plan specified adding `buildNIClaudeArgs` to `cli/main.go`; placed in `cli/noninteractive.go` instead, where all other non-interactive helpers reside. Functionally identical — same package.

## Issues Encountered

None.

## Self-Check: PASSED

- `cli/noninteractive.go` contains `func buildNIClaudeArgs`
- `cli/main.go` non-interactive branch calls `buildNIClaudeArgs(dockerSetup, niPrompt, flag.Args())`
- `cli/noninteractive_test.go` contains `TestBuildNIClaudeArgs_NoDockerSetup`, `TestBuildNIClaudeArgs_NoDockerSetup_WithExtraArgs`, `TestBuildNIClaudeArgs_WithDockerSetup`, `TestBuildNIClaudeArgs_WithDockerSetup_WithExtraArgs`
- `cd cli && go test ./...` exits 0
- `cd cli && go build -o /dev/null .` exits 0
- Commit `788bf55` exists

## Next Phase Readiness

- MOUNT-02 gap is closed; Phase 1 verification can now pass fully
- No blockers
