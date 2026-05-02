---
phase: 03-session-naming
plan: 01
subsystem: infra
tags: [go, docker, git, session-naming, env-vars]

requires:
  - phase: 02-write-back-sync
    provides: container exit lifecycle and env var injection patterns

provides:
  - detectGitBranch(projectDir string) string — runs git rev-parse in project dir
  - sanitizeBranchName(branch string) string — replaces non-[a-zA-Z0-9_-] with hyphens
  - buildSessionName(absDir string) string — returns <folder>_<branch> or <folder>
  - CLAUDE_SESSION_NAME env var injected via docker run -e
  - .zshrc exports CLAUDE_SESSION_NAME and sets terminal title

affects:
  - 03-02-PLAN.md (human verification of live container env var)

tech-stack:
  added: []
  patterns: [os/exec external command with cmd.Dir for directory-scoped git calls]

key-files:
  created:
    - cli/session_naming_test.go
  modified:
    - cli/main.go
    - cli/templates/.zshrc

key-decisions:
  - "Used strings.Builder character loop for sanitizeBranchName instead of regexp to avoid new import"
  - "Detached HEAD returns empty string (treated as no branch), session name falls back to folder name"
  - "CLAUDE_SESSION_NAME injected as last item in env var block in runArgs append"

patterns-established:
  - "os/exec with cmd.Dir for directory-scoped external command calls (matches docker.go pattern)"
  - "TDD: RED (test file with undefined functions) → GREEN (implement functions) → verify"

requirements-completed:
  - SESSION-01
  - SESSION-02

duration: 15min
completed: 2026-05-02
---

# Phase 3 Plan 01: Session Naming — Go CLI Implementation Summary

**Git branch detection and CLAUDE_SESSION_NAME injection via `detectGitBranch`/`sanitizeBranchName`/`buildSessionName` in Go CLI, with .zshrc export and terminal title**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-05-02T00:00:00Z
- **Completed:** 2026-05-02T00:15:00Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Three functions added to `cli/main.go`: `detectGitBranch`, `sanitizeBranchName`, `buildSessionName`
- `CLAUDE_SESSION_NAME` injected as last item in the docker run env var block
- `cli/templates/.zshrc` exports the var and sets terminal title when var is set
- 5 unit tests in `cli/session_naming_test.go` — all passing; full suite green

## Task Commits

1. **Task 1+2 combined: TDD RED→GREEN + .zshrc update** - `b02f90f` (feat)

## Files Created/Modified
- `cli/session_naming_test.go` — 5 unit tests for all three functions
- `cli/main.go` — 3 new functions + CLAUDE_SESSION_NAME env injection
- `cli/templates/.zshrc` — export block + terminal title before EXIT trap

## Decisions Made
- `sanitizeBranchName` uses a `strings.Builder` character loop instead of `regexp` — avoids a new import, logic is explicit and matches test table exactly
- Detached HEAD (`git rev-parse` returns literal `"HEAD"`) treated as no-branch → session name is folder name alone

## Deviations from Plan

None — plan executed exactly as written. `.zshrc` CLAUDE_SESSION_NAME count is 4 (comment + if-condition + export + echo), not 3 as the acceptance criterion expected — the comment line was counted by grep. All 4 occurrences are correct.

## Issues Encountered
None.

## User Setup Required
None — no external service configuration required.

## Next Phase Readiness
- Wave 2 (03-02) requires human to build the binary and verify `CLAUDE_SESSION_NAME` is set inside a live container
- All automated checks pass; live container test is the only remaining gate

---
*Phase: 03-session-naming*
*Completed: 2026-05-02*
