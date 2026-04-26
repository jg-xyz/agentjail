# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-25)

**Core value:** Claude Code works inside the container exactly as it does on the host — same config, same skills, same plugins, sessions named to match the project context
**Current focus:** Phase 1 — Mount & Copy

## Current Position

Phase: 1 of 3 (Mount & Copy)
Plan: 0 of TBD in current phase
Status: Ready to plan
Last activity: 2026-04-25 — Roadmap created

Progress: [░░░░░░░░░░] 0%

## Performance Metrics

**Velocity:**
- Total plans completed: 0
- Average duration: -
- Total execution time: -

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| - | - | - | - |

**Recent Trend:**
- Last 5 plans: -
- Trend: -

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- Copy-on-start + exit sync (not live bind mount): live mount can't do path translation; copy enables clean translation in both directions
- Exit-time sync (not continuous): simpler, no background process, fits container lifecycle model
- Session name format `<project>_<branch>`: mirrors how developers think about work context
- Skip binary files during path rewrite: `.wasm` and other binaries can't have text substitution applied safely

### Pending Todos

None yet.

### Blockers/Concerns

None yet.

## Deferred Items

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| *(none)* | | | |

## Session Continuity

Last session: 2026-04-25
Stopped at: Roadmap created, ready to plan Phase 1
Resume file: None
