# Roadmap: AgentJail — Claude Config Management

## Overview

This milestone makes the Claude Code integration first-class by solving three interconnected problems: the `~/.claude` directory must be copied-in with host paths translated so skills and plugins resolve inside the container; changes must sync back to the host on exit with reverse translation; and Claude sessions must be named after the project and git branch so they are easily found.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Mount & Copy** - Mount `~/.claude` read-only at `/tmp/.claude` and copy it into the container with host-path translation on startup
- [ ] **Phase 2: Write-back Sync** - Sync `/root/.claude` back to the host on container exit with reverse path translation and safe ownership handling
- [ ] **Phase 3: Session Naming** - Auto-name Claude Code sessions `<project-folder>_<git-branch>` by detecting the branch at container start

## Phase Details

### Phase 1: Mount & Copy
**Goal**: Claude Code inside the container can load skills, plugins, and settings that were authored on the host
**Depends on**: Nothing (first phase)
**Requirements**: MOUNT-01, MOUNT-02, MOUNT-03
**Success Criteria** (what must be TRUE):
  1. `~/.claude` is mounted read-only at `/tmp/.claude`; no direct bind to `/root/.claude`
  2. At container start, `/root/.claude` is populated as a writable copy of `/tmp/.claude`
  3. Paths inside copied text files reference `/root/...` instead of the host home dir
  4. Binary files (`.wasm`, `.png`, `.jpg`, etc.) are copied as-is without attempted text substitution
**Plans:** 2 plans

Plans:
- [x] 01-01-PLAN.md -- Read-only mount, copy-on-start with path translation, binary skip, Dockerfile init script
- [x] 01-02-PLAN.md -- Gap closure: buildNIClaudeArgs wires dockerSetup into non-interactive branch (MOUNT-02)

### Phase 2: Write-back Sync
**Goal**: Changes made to Claude config inside the container (memories, settings edits, new skills) are preserved on the host after the session ends
**Depends on**: Phase 1
**Requirements**: SYNC-01, SYNC-02, SYNC-03
**Success Criteria** (what must be TRUE):
  1. On container exit, `/root/.claude` is synced back to host `~/.claude` with `/root` translated back to the host home dir
  2. Synced files on the host are owned by the host user (correct `HOST_UID`/`HOST_GID`), not root
  3. Files that were not modified inside the container are not overwritten on the host
**Plans:** 2 plans

Plans:
- [ ] 02-01-PLAN.md -- Go CLI: /tmp/.claude-out:rw mount, SYNC_MODE env var injection, SyncMode config field, unit tests
- [ ] 02-02-PLAN.md -- Container: agentjail-sync-claude script, zsh EXIT trap, config_schema.yaml docs

### Phase 3: Session Naming
**Goal**: Claude Code sessions are named after the project and current git branch so the user can identify them immediately
**Depends on**: Phase 2
**Requirements**: SESSION-01, SESSION-02
**Success Criteria** (what must be TRUE):
  1. Claude Code launches with a session name in `<project-folder>_<git-branch>` format
  2. When the project directory is not a git repo (or has no branch), the session name falls back to the project folder name alone
**Plans**: TBD
**UI hint**: no

## Progress

**Execution Order:**
Phases execute in numeric order: 1 -> 2 -> 3

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Mount & Copy | 2/2 | Complete | 2026-04-26 |
| 2. Write-back Sync | 0/2 | Not started | - |
| 3. Session Naming | 0/TBD | Not started | - |
