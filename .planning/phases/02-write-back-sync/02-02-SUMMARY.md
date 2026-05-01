---
phase: 02-write-back-sync
plan: "02"
subsystem: container-templates
tags: [write-back-sync, dockerfile, zshrc, config-schema, exit-trap]
dependency_graph:
  requires: []
  provides: [agentjail-sync-claude-script, exit-trap-registration, sync-mode-docs]
  affects: [cli/templates/Dockerfile, cli/templates/.zshrc, config_schema.yaml]
tech_stack:
  added: []
  patterns: [exit-trap-sync, content-based-comparison, reverse-path-translation]
key_files:
  created: []
  modified:
    - cli/templates/Dockerfile
    - cli/templates/.zshrc
    - config_schema.yaml
decisions:
  - "EXIT trap only (no INT/TERM) to avoid zsh trap interaction issues"
  - "Content-based comparison (not mtime) to prevent unnecessary overwrites"
  - "grep -qI binary guard reuses exact pattern from agentjail-init-claude"
  - "additions_only as default SYNC_MODE to match safe default principle"
metrics:
  duration: "96s"
  completed_date: "2026-05-01"
  tasks_completed: 2
  files_modified: 3
---

# Phase 02 Plan 02: Container-Side Sync Script and EXIT Trap Summary

Container-side write-back sync implemented via agentjail-sync-claude shell script baked into the Dockerfile image, registered as a zsh EXIT trap, with sync_mode documented in config_schema.yaml.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add agentjail-sync-claude script to Dockerfile | d129eb2 | cli/templates/Dockerfile |
| 2 | Add EXIT trap to .zshrc and document sync_mode | 5622cbe | cli/templates/.zshrc, config_schema.yaml |

## What Was Built

### Task 1: agentjail-sync-claude script (Dockerfile)

Inserted `/usr/local/bin/agentjail-sync-claude` after the `agentjail-init-claude` block (line 405) in the Dockerfile. The script:

- Skips sync when `AGENTJAIL_NO_SYNC=1` env var is set or `/root/.agentjail/nosync` sentinel file exists
- Exits cleanly (exit 0) if `/root/.claude` was never created
- Creates `/tmp/.claude-out` output directory
- For text files: applies reverse path translation (`/root/.claude` → `${HOST_HOME}/.claude`) using sed, compares translated content against existing host file, writes only if changed (atomic write via `.tmp` + `mv`)
- For binary files: uses `cmp -s` byte comparison, copies only if different
- Binary guard (`grep -qI ""`) prevents sed from corrupting `.wasm` and other binary files — same pattern as `agentjail-init-claude`
- In `SYNC_MODE=full`: removes `/tmp/.claude-out` files that no longer exist in `/root/.claude`
- Restores host ownership: `chown -R ${HOST_UID}:${HOST_GID} /tmp/.claude-out 2>/dev/null || true`
- Exits 0 unconditionally to prevent blocking container shutdown

### Task 2: EXIT trap and config documentation

**cli/templates/.zshrc:** Appended `trap 'agentjail-sync-claude' EXIT` at end of file. Uses EXIT signal only — no INT or TERM traps — to avoid zsh trap interaction issues.

**config_schema.yaml:** Extended the `agent_frameworks.claude` section with full `sync_mode` documentation: describes `additions_only` (default, no deletions propagated) and `full` (deletions propagated) modes, notes content-based change detection, and documents both skip mechanisms (`AGENTJAIL_NO_SYNC=1` and `nosync` sentinel). Added `sync_mode: additions_only` field to the example config.

## Deviations from Plan

None — plan executed exactly as written.

## Threat Surface Scan

No new trust boundaries introduced beyond those already in the plan's threat model (T-02-04 through T-02-08). All mitigations from the threat register were implemented:
- T-02-04: Binary guard (`grep -qI ""`) present — prevents sed from corrupting binary files
- T-02-06: `chown || true` + `exit 0` — sync failures cannot block container exit

## Known Stubs

None. The script is complete and functional. The `/tmp/.claude-out` bind-mount to host `~/.claude` is handled by the Go CLI (separate plan 02-01).

## Self-Check

| Check | Result |
|-------|--------|
| cli/templates/Dockerfile exists | FOUND |
| cli/templates/.zshrc exists | FOUND |
| config_schema.yaml exists | FOUND |
| 02-02-SUMMARY.md exists | FOUND |
| Commit d129eb2 exists | FOUND |
| Commit 5622cbe exists | FOUND |

## Self-Check: PASSED
