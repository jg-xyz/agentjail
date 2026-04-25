# AgentJail — Claude Config Management

## What This Is

AgentJail is a CLI tool that launches AI coding agents (Claude Code, Copilot, OpenCode) inside isolated Docker containers. This milestone focuses on making the Claude Code integration first-class: properly translating `~/.claude` paths when entering the container, syncing changes back to the host on exit, and automatically naming Claude sessions after the project and git branch.

## Core Value

Claude Code works inside the container exactly as it does on the host — same config, same skills, same plugins, sessions named to match the project context.

## Requirements

### Validated

- ✓ Docker container lifecycle management (create, exec, exit) — existing
- ✓ Project directory mounted at `/project` — existing
- ✓ `.agentjail/` persistence directory mounted at `/root/.agentjail` — existing
- ✓ Host `~/.claude` mounted at `/root/.claude` for Claude Code auth — existing
- ✓ `ANTHROPIC_API_KEY` injected from host or config — existing
- ✓ Zellij multi-tab layout with agent, terminal, file browser tabs — existing
- ✓ gitconfig mount-and-copy pattern (RO at `/tmp/.gitconfig`, copied on start) — existing

### Active

- [ ] `~/.claude` mounted read-only at `/tmp/.claude` instead of directly at `/root/.claude`
- [ ] Container startup copies `/tmp/.claude` → `/root/.claude` with host-path → `/root` translation
- [ ] Container exit syncs `/root/.claude` back to host `~/.claude` with reverse path translation
- [ ] Claude Code sessions named `<project-folder>_<git-branch>` by default
- [ ] Path translation handles: settings.json plugin paths, skills SKILL.md paths, memory file path refs

### Out of Scope

- Continuous/real-time sync while container runs — exit-time sync is sufficient and simpler
- Separate per-project claude configs — host config is the single source of truth
- Windows support for this feature — Linux/macOS only for now

## Context

Currently `~/.claude` is bind-mounted directly to `/root/.claude`. Files inside contain hardcoded host paths (e.g., `/home/jg/.claude/plugins/...`, `/home/jg/.claude/skills/...`). Inside the container, the user is `root`, so those paths don't resolve — breaking skill loading, plugin loading, memory references, and CLI operations that try to write to config (add repos, install plugins).

The gitconfig pattern already in the codebase (`main.go:dockerSetup`) is the right model: mount RO at `/tmp`, copy to home dir on startup. For Claude, we need the same but with path rewriting AND a write-back on container exit.

Session naming: Claude Code stores sessions in `~/.claude/projects/<hash>/`. Naming them `<project>_<branch>` makes it easy to find sessions for a specific branch context. Branch is read from the mounted project at container start.

## Constraints

- **Compatibility**: Must work with existing `~/.claude` structure — no changes to host files except write-back sync
- **Performance**: Path rewriting must handle potentially large `~/.claude` directory efficiently (only rewrite text files, skip binaries/`.wasm`)
- **Safety**: Write-back must preserve host files if container modified nothing (no unnecessary overwrites)
- **Tech stack**: Go, Docker CLI, existing `main.go` + `zellij.go` pattern

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Copy-on-start + exit sync (not live bind mount) | Live mount can't do path translation; copy enables clean translation in both directions | — Pending |
| Exit-time sync (not continuous) | Simpler, no background process, fits container lifecycle model | — Pending |
| Session name format `<project>_<branch>` | Mirrors how developers think about their work context | — Pending |
| Skip binary files during path rewrite | `.wasm` and other binaries can't have text substitution applied safely | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-04-25 after initialization*
