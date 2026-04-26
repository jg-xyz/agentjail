# Phase 2: Write-back Sync - Context

**Gathered:** 2026-04-26
**Status:** Ready for planning

<domain>
## Phase Boundary

On container exit, sync `/root/.claude` back to the host `~/.claude` with reverse path translation (`/root` → `$HOST_HOME`) and correct file ownership. Skips unmodified files. Configurable deletion propagation.

</domain>

<decisions>
## Implementation Decisions

### Sync Trigger
- **D-01:** Exit trap inside container — `trap 'agentjail-sync-claude' EXIT` in `.zshrc`. Fires on ALL exit paths (normal exit, Ctrl-C, SIGTERM caught by shell).
- **D-02:** NOT post-exit Go CLI docker cp. The container handles its own sync while still alive.

### Writable Mount Design
- **D-03:** Two bind mounts for ~/.claude: existing `/tmp/.claude:ro` (read source for startup copy) PLUS new `/tmp/.claude-out:rw` (write destination for exit sync). Both mount the same host `~/.claude` directory.
- **D-04:** Sync script uses `cp + sed` pattern (same as Phase 1 startup copy). No rsync — not in container by default. Reverse path: `s|/root/.claude|${HOST_HOME}/.claude|g` applied to text files; binary files (`grep -qI` guard) copied as-is.
- **D-05:** After writing files, `chown $HOST_UID:$HOST_GID` the synced output (reuse existing HOST_UID/HOST_GID env var pattern from zellij chown fix).

### Changed-File Detection (SYNC-03)
- **D-06:** Content compare after translation — for each text file: apply reverse sed, compare result to existing `/tmp/.claude-out/$rel`. Only write if different. For binary files: `cmp -s` byte comparison.
- **D-07:** Deletions NOT synced by default. Configurable via `agent_frameworks.claude_code.sync_mode` in `~/.config/agentjail/config.yaml`:
  - `additions_only` (default) — new/modified files only, no deletion propagation
  - `full` — additions + deletions (removes host files deleted inside container)

### Failure Handling
- **D-08:** On sync failure: warn to stderr (`⚠ agentjail: claude sync failed: $err >&2`), then exit 0 — do NOT block container exit.
- **D-09:** Sync skippable via BOTH mechanisms:
  - `AGENTJAIL_NO_SYNC=1` env var set in container
  - `/root/.agentjail/nosync` file sentinel (user can `touch /root/.agentjail/nosync` inside container)
  - If either is set, trap prints skip notice and returns without syncing.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project & Requirements
- `.planning/REQUIREMENTS.md` — SYNC-01, SYNC-02, SYNC-03 definitions
- `.planning/PROJECT.md` — constraints (safety, no unnecessary overwrites, Linux/macOS only)
- `.planning/ROADMAP.md` §Phase 2 — success criteria

### Phase 1 Patterns (carry forward)
- `.planning/phases/01-mount-copy/01-01-SUMMARY.md` — established patterns: grep -qI binary guard, sed path translation, HOST_HOME env var, buildClaudeMountArgs shape
- `.planning/phases/01-mount-copy/01-01-PLAN.md` — interfaces: buildClaudeMountArgs signature, dockerSetup assembly pattern

### Source Files to Read
- `cli/main.go` lines 695-730 — dockerSetup assembly + mount arg construction (where to add /tmp/.claude-out:rw)
- `cli/main.go` lines 520-535 — where volArgs/envArgs from buildClaudeMountArgs are appended to runArgs
- `cli/config.go` — ClaudeCodeConfig struct (add sync_mode field here)
- `cli/templates/.zshrc` — where to add `trap 'agentjail-sync-claude' EXIT`
- `cli/templates/Dockerfile` — where agentjail-init-claude script lives (model for agentjail-sync-claude script)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `buildClaudeMountArgs(hostClaudePath, hostHome)` in `cli/main.go:16` — returns (volArgs, envArgs, setupSnippet). Add the `:rw` writeback mount here (new return value or extend volArgs).
- `HOST_UID`/`HOST_GID` env vars — already injected in zellij chown fix (`main.go` chownFix string). Reuse for exit sync chown.
- `HOST_HOME` env var — already injected when ClaudeCode enabled. Available inside container for reverse sed.
- `grep -qI "" "$f"` binary guard — established in Phase 1 copy snippet. Copy verbatim into sync script.
- `agentjail-init-claude` script in Dockerfile — structural model for `agentjail-sync-claude` exit script.

### Established Patterns
- `/tmp` RO mount + writable copy: the Phase 1 pattern. Phase 2 adds the reverse direction.
- Idempotent scripts with guards — sync script should handle empty /tmp/.claude-out gracefully.
- Dockerfile scripts baked into image + invoked from shell hooks — same model for sync script.

### Integration Points
- `cli/main.go` docker run assembly — add `-v ${hostClaudePath}:/tmp/.claude-out:rw` to volArgs
- `cli/templates/.zshrc` — add `trap 'agentjail-sync-claude' EXIT` near bottom
- `cli/templates/Dockerfile` — add `agentjail-sync-claude` script (alongside `agentjail-init-claude`)
- `cli/config.go` `ClaudeCodeConfig` struct — add `SyncMode string \`yaml:"sync_mode"\`` with default `additions_only`
- `config_schema.yaml` — document new sync_mode field

</code_context>

<specifics>
## Specific Ideas

- The sync script name should be `agentjail-sync-claude` (parallel to `agentjail-init-claude`)
- Skip sentinel path: `/root/.agentjail/nosync` (inside container, maps to `.agentjail/nosync` in project)
- Env var skip: `AGENTJAIL_NO_SYNC=1`
- Config values: `additions_only` | `full` (not booleans — leaves room for future modes)

</specifics>

<deferred>
## Deferred Ideas

- Continuous sync while container runs (inotify/rsync daemon) — already out of scope in REQUIREMENTS.md v2 Deferred
- Selective sync of specific subdirectories (e.g. memories only) — out of scope per REQUIREMENTS.md
- Windows support — out of scope per PROJECT.md constraints

</deferred>

---

*Phase: 2-Write-back Sync*
*Context gathered: 2026-04-26*
