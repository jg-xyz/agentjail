---
phase: 02-write-back-sync
plan: "01"
subsystem: cli
tags: [config, docker-mounts, sync, tdd]
dependency_graph:
  requires: []
  provides: [SYNC-01, SYNC-02, SYNC-03]
  affects: [cli/config.go, cli/main.go, cli/claude_mount_test.go]
tech_stack:
  added: []
  patterns: [TDD RED/GREEN, frameworkNodes YAML node builder, buildClaudeMountArgs volume pattern]
key_files:
  created: []
  modified:
    - cli/config.go
    - cli/main.go
    - cli/claude_mount_test.go
decisions:
  - "SyncMode defaults to additions_only via empty-string fallback at injection site — no sentinel value in config struct needed"
  - "SYNC_MODE injection placed inside os.Stat(hostClaudePath) block — env var only injected when ~/.claude exists on host"
metrics:
  duration: "~10 minutes"
  completed: "2026-05-01T20:39:03Z"
---

# Phase 2 Plan 01: Write-back Sync Infrastructure Summary

**One-liner:** Added /tmp/.claude-out:rw bind mount and SYNC_MODE env var injection with SyncMode config field defaulting to additions_only.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 (RED) | Failing tests for RW mount | f0a64b5 | cli/claude_mount_test.go |
| 1 (GREEN) | SyncMode config field + mounts + SYNC_MODE injection | 05ce52e | cli/config.go, cli/main.go |

## What Was Built

**cli/config.go:**
- Added `SyncMode string \`yaml:"sync_mode"\`` field to `FrameworkConfig` struct
- `frameworkNodes()` now emits `sync_mode: additions_only` as default YAML node for all frameworks written to new configs

**cli/main.go:**
- `buildClaudeMountArgs` now returns both `-v path:/tmp/.claude:ro` AND `-v path:/tmp/.claude-out:rw` in volumeArgs
- ClaudeCode enabled block injects `SYNC_MODE` env var using `globalConfig.AgentFrameworks.ClaudeCode.SyncMode`, falling back to `"additions_only"` when empty

**cli/claude_mount_test.go:**
- Added `TestClaudeOutMountReadWrite`: asserts /tmp/.claude-out:rw mount is present in volArgs
- Added `TestClaudeMountBothReadAndWrite`: asserts both :ro and :rw mounts are present simultaneously
- Full suite: 7 tests in claude_mount_test.go all pass

## Verification

```
go build -o /dev/null .     # clean build
go test ./... -v            # all tests pass (full suite)
```

Grep checks all pass:
- `SyncMode string \`yaml:"sync_mode"\`` in FrameworkConfig
- `sync_mode: additions_only` node in frameworkNodes()
- `/tmp/.claude-out:rw` in buildClaudeMountArgs
- `SYNC_MODE=` fmt.Sprintf in ClaudeCode block
- TestClaudeOutMountReadWrite and TestClaudeMountBothReadAndWrite in test file

## Deviations from Plan

None — plan executed exactly as written. TDD gate sequence followed: RED commit (f0a64b5) then GREEN commit (05ce52e).

## TDD Gate Compliance

- RED gate commit: f0a64b5 `test(02-01): add failing tests for /tmp/.claude-out:rw mount` — PASS
- GREEN gate commit: 05ce52e `feat(02-01): add SyncMode config field and /tmp/.claude-out:rw mount` — PASS
- REFACTOR gate: not needed, implementation was clean

## Known Stubs

None — all fields are wired: SyncMode reads from config YAML, SYNC_MODE is injected into container env, both volume mounts are added to runArgs.

## Threat Flags

No new network endpoints, auth paths, or trust boundary surfaces introduced beyond what the plan's threat model covers (T-02-01, T-02-02, T-02-03).

## Self-Check: PASSED

- cli/config.go exists and contains SyncMode field: FOUND
- cli/main.go exists and contains /tmp/.claude-out:rw: FOUND
- cli/claude_mount_test.go contains both new test functions: FOUND
- Commit f0a64b5 exists: FOUND
- Commit 05ce52e exists: FOUND
