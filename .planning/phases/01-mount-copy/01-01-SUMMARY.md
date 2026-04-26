---
phase: 01-mount-copy
plan: 01
subsystem: infra
tags: [docker, go, claude-code, mount, path-translation, tdd]

# Dependency graph
requires: []
provides:
  - buildClaudeMountArgs helper function in cli/main.go
  - /tmp/.claude:ro read-only bind mount replacing direct /root/.claude mount
  - HOST_HOME env var injection for runtime sed path substitution
  - dockerSetup snippet that copies /tmp/.claude to /root/.claude with binary-safe path rewriting
  - agentjail-init-claude Dockerfile script covering non-interactive container path
  - 5 unit tests in cli/claude_mount_test.go
affects: [02-exit-sync, 03-session-naming]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Read-only /tmp mount + copy-on-start pattern (matches existing gitconfig pattern)"
    - "Binary-safe file rewriting via grep -qI guard before sed substitution"
    - "Idempotent init scripts: skip if destination already exists"
    - "TDD: test file before implementation, RED -> GREEN commit sequence"

key-files:
  created:
    - cli/claude_mount_test.go
  modified:
    - cli/main.go
    - cli/templates/Dockerfile

key-decisions:
  - "Mount ~/.claude read-only at /tmp/.claude (not /root/.claude) so container cannot modify host files"
  - "Use HOST_HOME env var (set by Go CLI from user.Current()) as sed delimiter source — not user-controlled input"
  - "agentjail-init-claude script in Dockerfile CMD covers non-interactive path that skips dockerSetup"
  - "grep -qI guard skips binary files (wasm, png, jpg) before sed to prevent binary corruption"
  - "buildClaudeMountArgs returns constant shell string using ${HOST_HOME} — a shell variable resolved at container runtime, not Go compile time"

patterns-established:
  - "Pattern: /tmp RO mount + startup copy = host credential sharing without write-back risk"
  - "Pattern: Dockerfile init script invoked from CMD for all container paths including non-interactive"

requirements-completed: [MOUNT-01, MOUNT-02, MOUNT-03]

# Metrics
duration: 12min
completed: 2026-04-25
---

# Phase 01 Plan 01: Mount & Copy Summary

**Read-only /tmp/.claude mount with binary-safe path rewriting via sed + HOST_HOME, covered by both dockerSetup snippet (interactive) and agentjail-init-claude Dockerfile script (non-interactive)**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-04-25T20:37:00Z
- **Completed:** 2026-04-25T20:49:00Z
- **Tasks:** 3
- **Files modified:** 3 (cli/main.go, cli/claude_mount_test.go, cli/templates/Dockerfile)

## Accomplishments
- Replaced direct /root/.claude bind-mount with read-only /tmp/.claude:ro mount (T-01-01 threat mitigated)
- HOST_HOME env var injected from user.Current().HomeDir for runtime sed path substitution
- dockerSetup snippet copies /tmp/.claude to /root/.claude with grep -qI binary skip guard
- agentjail-init-claude script baked into Dockerfile CMD covers non-interactive mode
- 5 unit tests passing in claude_mount_test.go covering all key behaviors
- Full test suite (all pre-existing tests) continues to pass

## Task Commits

Each task was committed atomically:

1. **Task 1: Extract testable helper and write unit tests** - `726d7b5` (feat — TDD RED+GREEN)
2. **Task 2: Wire helper into main.go and replace direct mount** - `36cc877` (feat)
3. **Task 3: Add agentjail-init-claude script to Dockerfile** - `0b7db7b` (feat)

_Note: Task 1 used TDD — test file created first (RED), then function added (GREEN) in a single commit._

## Files Created/Modified
- `cli/claude_mount_test.go` - 5 unit tests: read-only mount, disabled gate, HOST_HOME env, copy snippet, binary skip
- `cli/main.go` - buildClaudeMountArgs() helper; replaced direct mount; added dockerSetup claude snippet
- `cli/templates/Dockerfile` - agentjail-init-claude script + CMD invocation

## Decisions Made
- Used `grep -qI "" "$f"` binary detection (same pattern as established in research) — portable, handles all binary types without file-extension allowlists
- buildClaudeMountArgs() uses empty string args for the dockerSetup snippet call since the shell snippet uses `${HOST_HOME}` as a literal shell variable (not Go interpolation) — this keeps the function pure and testable
- Semicolon (not &&) after agentjail-init-claude in CMD so CMD continues even if .claude doesn't exist on host

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Threat Surface Scan

No new network endpoints, auth paths, or trust boundary changes introduced beyond what the plan's threat model covers. T-01-01 (write protection via :ro mount) is now implemented. T-01-02 through T-01-05 remain accepted per plan.

## Known Stubs

None - all functionality is wired end-to-end. The copy+translate runs at container creation; no placeholder data.

## Self-Check: PASSED

- `cli/claude_mount_test.go` exists with 5 test functions
- `cli/main.go` contains buildClaudeMountArgs, /tmp/.claude:ro, HOST_HOME=, claudeSetup
- `cli/templates/Dockerfile` contains agentjail-init-claude (3 occurrences), cp -r /tmp/.claude, grep -qI
- Commits 726d7b5, 36cc877, 0b7db7b exist in git log
- `cd cli && go test ./...` passes
- `cd cli && go build -o /dev/null .` succeeds

## Next Phase Readiness
- Phase 2 (exit sync) can now write back /root/.claude changes to host ~/.claude with reverse path translation
- HOST_HOME env var is available inside container for reverse sed (/.claude|/root/.claude -> ${HOST_HOME}/.claude)
- No blockers or concerns

---
*Phase: 01-mount-copy*
*Completed: 2026-04-25*
