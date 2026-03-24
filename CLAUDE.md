# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

AgentJail is a CLI tool that launches AI coding agents (GitHub Copilot, OpenCode) inside isolated Docker containers with project-specific configurations. It mounts the host project directory into the container and manages the full container lifecycle.

## Commands

All commands run from the `cli/` directory (where `go.mod` lives). Mise tasks handle the `cd` automatically.

```bash
mise build           # Build Linux binary → ../dist/agentjail
mise build_win       # Build Windows binary → ../dist/agentjail.exe
mise test            # go test ./... -v
mise run             # go run .
```

Running directly:
```bash
cd cli && go test ./...               # all tests
cd cli && go test -run TestFoo ./...  # single test
cd cli && go build -o ../dist/agentjail .
```

## Architecture

The CLI (`cli/`) is a single Go binary. There is no server, no database, and no separate frontend.

### Entry point flow (`main.go`)

1. Parse flags and load config (`~/.config/agentjail/config.yaml`)
2. If no args: detect an existing container for the current directory and re-exec into it (auto-exec)
3. Otherwise: check/build Docker image, create `.agentjail/` folder in the project, build the `docker run` command, exec into the container
4. Optionally auto-launch a configured agent (`-A` flag)

### Key files

| File | Role |
|------|------|
| `main.go` | Orchestrates everything: flag parsing, image build, `docker run` command assembly, exec |
| `config.go` | Loads `~/.config/agentjail/config.yaml`; provides typed `Config` struct with defaults |
| `docker.go` | Image existence check; finds existing containers by mount path |
| `agent.go` | Lists enabled agents; generates launch commands; interactive selection prompt |
| `filesystem.go` | Creates `.agentjail/` in the project; copies template configs; updates `.gitignore` |
| `metadata.go` | Saves/loads `.agentjail/metadata.json` tracking container name, image version, volumes |
| `embed.go` | Embeds `templates/` directory into the binary at compile time |
| `terminal_windows.go` / `terminal_other.go` | Platform-specific terminal state save/restore |

### Templates (`cli/templates/`)

Bundled via Go embed. Contains:
- `Dockerfile` — Ubuntu 24.04 image with gh CLI, micro/vim/nano editors, mise, Node.js, Python, starship prompt, and conditional agent installs (build args `USE_COPILOT`, `USE_OPENCODE`)
- `.zshrc` — Shell config with aliases (`files`, `edit`, `/exit`) and MOTD
- `configs/` — Default configs for copilot, opencode, and rovr, copied to `.agentjail/` on first run

### Container conventions

- Project directory mounts to `/project` (working dir)
- `.agentjail/` in the project root mounts to `/root/.agentjail` for persistence (shell history, tool configs)
- Container name is derived from the project directory path
- `.agentjail/` is automatically added to `.gitignore`

### Configuration

Global config at `~/.config/agentjail/config.yaml`. Schema documented in `config_schema.yaml`. Key fields:
- `agent_frameworks` — enable/configure Copilot and OpenCode
- `preferred_agent` — auto-selected with `-A`
- `mount_system_gitconfig`, `mount_gh_config_dir` — host credential sharing
- `inject_gh_auth_token` — inject `GITHUB_TOKEN` into container
- `container_env_vars` — custom env vars (supports `$ENV_VAR` references)
- `port_mappings` — forwarded ports

### Cross-platform

Windows support uses `terminal_windows.go` to save/restore console code pages via the Windows API. The `terminal_other.go` file provides no-op stubs for POSIX. Build tags are not used — file naming (`_windows`, `_other`) drives OS selection.
