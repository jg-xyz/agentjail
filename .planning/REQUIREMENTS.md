# Requirements

_AgentJail — Claude Config Management_
_Milestone 1 — v1_

## v1 Requirements

### Mount & Copy

- [ ] **MOUNT-01**: Container mounts `~/.claude` read-only at `/tmp/.claude` instead of directly at `/root/.claude`
- [ ] **MOUNT-02**: Container startup script copies `/tmp/.claude` → `/root/.claude` with path substitution (host home dir → `/root`)
- [ ] **MOUNT-03**: Path substitution skips binary files (`.wasm`, `.png`, `.jpg`, etc.) — text files only

### Write-back Sync

- [ ] **SYNC-01**: On container exit, `/root/.claude` is synced back to host `~/.claude` with reverse path substitution (`/root` → host home dir)
- [ ] **SYNC-02**: Sync preserves host file ownership using existing `HOST_UID`/`HOST_GID` env var pattern
- [ ] **SYNC-03**: Files unmodified inside container are not needlessly overwritten on the host (use mtime or content comparison)

### Session Naming

- [ ] **SESSION-01**: Claude Code launched with a session name env var or flag set to `<project-folder>_<git-branch>` format
- [ ] **SESSION-02**: Git branch is read from the mounted project directory at container start; falls back to folder name only if not a git repo or no branch

## v2 (Deferred)

- Continuous sync while container runs (inotify/rsync daemon) — exit-time sync sufficient for now
- Per-project Claude config overlays — out of scope; host config is source of truth
- Windows support for this feature — Linux/macOS only

## Out of Scope

- Modifying host `~/.claude` files in-place at startup — too risky, path errors would corrupt host config
- Selective sync of specific directories (e.g. memories only) — full sync is simpler and safer
- Creating a new Claude session management UI — session naming via env var only

## Traceability

| Requirement | Phase |
|-------------|-------|
| MOUNT-01 | Phase 1 |
| MOUNT-02 | Phase 1 |
| MOUNT-03 | Phase 1 |
| SYNC-01 | Phase 2 |
| SYNC-02 | Phase 2 |
| SYNC-03 | Phase 2 |
| SESSION-01 | Phase 3 |
| SESSION-02 | Phase 3 |

---
*Last updated: 2026-04-25 after initialization*
