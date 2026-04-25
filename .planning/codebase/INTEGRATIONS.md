# External Integrations

_Last updated: 2026-04-25_

## Summary

AgentJail integrates with Docker as its sole runtime dependency. All AI agent integrations (GitHub Copilot, OpenCode, Claude Code) are optional and controlled by config flags that gate both image build-args and runtime volume/env mounts. Credentials are passed into the container via bind-mounts and environment variables; no secrets are stored by AgentJail itself.

## Docker

**Role:** Core runtime — the entire tool is a Go wrapper that builds and manages Docker containers.

**Usage:**
- `docker build` — builds the `agentjail` image from the embedded `Dockerfile` template (or a user-supplied `-D` path)
- `docker run` — launches the container with project directory and config mounts
- `docker exec` — re-attaches to existing containers (auto-exec mode and non-interactive mode)
- `docker ps` / container inspection — `cli/docker.go` queries running containers to find ones matching the current project directory

**Privileged mode (`-P` flag):**
- Passes `--privileged` and mounts `/var/run/docker.sock` into the container (Docker-in-Docker)
- Triggers lazy install of `docker-ce-cli` inside the container on first run via the Docker official APT repo (pre-configured in the image)

## AI Agent Integrations

All agents are optional. Each is gated by `agent_frameworks.<name>.enabled: true` in `~/.config/agentjail/config.yaml`.

### GitHub Copilot

**Build-arg:** `USE_COPILOT=true` — installs via `curl -fsSL https://gh.io/copilot-install | bash`

**Auth mounts (runtime, `cli/main.go`):**
- Host `~/.config/github-copilot` → container `/root/.config/github-copilot` (credential store)
- Host `~/.config/gh` → container `/root/.config/gh` (gh CLI auth, primary auth path)
- Project-local `.agentjail/copilot/config.json` and `mcp.json` are overlaid as targeted mounts

**Token injection:**
- `GH_TOKEN` env var passed into container when `github_token` config field or `GH_TOKEN`/`GITHUB_TOKEN` host env vars are set
- `inject_gh_auth_token: true` config option additionally resolves via `gh auth token` CLI fallback

**Config location:** `.agentjail/copilot/` (copied from `cli/templates/configs/copilot/` on first run)

### OpenCode

**Build-arg:** `USE_OPENCODE=true` — installs via `curl -LsSf https://opencode.com/install.sh | sh`

**Auth mounts (runtime, `cli/main.go`):**
- `.agentjail/opencode/opencode.json` → container `/root/.config/opencode/config.json`
- Host `~/.config/opencode` → container `/root/.local/share/opencode` (auth persistence, mounted only if directory exists)
- `opencode.json` project file → container `/project/opencode_config.json` (optional, `-C` flag)

**Config location:** `.agentjail/opencode/` (copied from `cli/templates/configs/opencode/` on first run)

### Claude Code (Anthropic)

**Build-arg:** `USE_CLAUDE_CODE=true` — installs via `mise x -- npm install -g @anthropic-ai/claude-code`

**Auth mounts (runtime, `cli/main.go`):**
- Host `~/.claude` → container `/root/.claude` (auth store, mounted if directory exists)
- Host `~/.claude.json` → container `/root/.claude.json` (auth JSON, mounted if file exists)

**Token injection:**
- `ANTHROPIC_API_KEY` passed into container via valueless `-e` flag (avoids exposing in process listing)
- Resolved from: `anthropic_api_key` config field → `ANTHROPIC_API_KEY` host env var
- Skipped when `ANTHROPIC_API_KEY` is already explicitly set in `container_env_vars`

**System prompt injection:**
- `claudeSystemPrompt` constant in `cli/agent.go` is always appended, listing available container tools
- Additional context via `claude_append_system_prompt` config field and/or `--claude-context` flag (merged with blank line separator)
- Non-interactive mode (`-N`) passes the merged prompt via `claude --append-system-prompt`

**Config location:** `.agentjail/claude/` (copied from `cli/templates/configs/claude/` on first run)

## GitHub CLI (`gh`)

**Role:** Auth token source for Copilot (`gh auth token` is the last-resort fallback in the `inject_gh_auth_token` chain)

**Mount:** `~/.config/gh` → `/root/.config/gh` (when `mount_gh_config_dir: true`, default on)

## GitHub Releases API

**Used at Dockerfile build time** (not at CLI runtime) to fetch latest release tags for optional tools:
- `https://api.github.com/repos/helix-editor/helix/releases/latest` — when `EDITOR=hx`
- `https://api.github.com/repos/sxyazi/yazi/releases/latest` — when `FILE_BROWSER=yazi` or runtime browser install
- `https://api.github.com/repos/yorukot/superfile/releases/latest` — when `FILE_BROWSER=spf`

## Zellij Plugin Downloads

**Runtime, `cli/zellij.go`:**
- Plugins configured via `zellij_plugins` config entries with a `url` field are fetched via `net/http` on first use
- Cached in `.agentjail/zellij/plugins/` after first download; subsequent runs skip the fetch
- 30-second HTTP timeout; failed downloads are logged and skipped (non-fatal)

## AWS CLI

**Installed in image** via `mise use -g aws-cli`. Credentials are not mounted by default; users must inject via `container_env_vars` or additional `-v` mounts.

## Git

**Host config mount:** `~/.gitconfig` → `/tmp/.gitconfig:ro` (when `mount_system_gitconfig: true`, default on). The container copies it to `~/.gitconfig` on startup so git operations use host identity without modifying the host file.

## Data Storage

**Persistence:** `.agentjail/` directory in each project root, bind-mounted to `/root/.agentjail` in the container. Contains shell history, tool configs, zellij layout files, metadata JSON, and plugin cache.

**No external database or file storage service is used.**

## Environment Configuration

**Required for operation:**
- Docker daemon accessible on host

**Optional env vars (host-side, read by CLI):**
- `AGENTJAIL_SHELL`, `AGENTJAIL_EDITOR`, `AGENTJAIL_FILE_BROWSER` — override config fields
- `GH_TOKEN` / `GITHUB_TOKEN` — GitHub token fallback for Copilot
- `ANTHROPIC_API_KEY` — Claude Code API key fallback

**Injected into container at runtime:**
- `EDITOR`, `VISUAL`, `FILE_BROWSER`, `SHELL`, `CONTAINER_ID`, `HISTFILE`, `AGENTJAIL_HOST_PATH`
- `HOST_UID`, `HOST_GID` — for ownership restoration on container exit
- Agent-specific tokens as described above
- `container_env_vars` config map (supports `env:HOST_VAR` reference syntax)

---

_Integration audit: 2026-04-25_
