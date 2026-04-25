# Technology Stack

_Last updated: 2026-04-25_

## Summary

AgentJail is a single Go binary CLI tool with no server, database, or frontend. The host-side binary is written in Go; the container environment is Ubuntu 24.04 with a curated set of developer tools installed at image build time. Build tooling is managed via `mise`.

## Languages

**Primary:**
- Go 1.25.0 — entire CLI (`cli/`)

**Secondary:**
- Shell (Bash/Zsh) — container startup scripts, rc file fragments, tab launcher hooks (embedded in Dockerfile and written at runtime by `cli/zellij.go`)
- KDL — Zellij layout and config templates (`cli/templates/configs/zellij/`)

## Runtime

**Environment:**
- Go binary: Linux or Windows host (cross-platform via OS-specific file naming, no build tags)
- Container: Ubuntu 24.04 (`ubuntu:24.04`)

**Package Manager:**
- Go modules (`go.mod` / `go.sum` in `cli/`)
- `mise` — manages Node.js, AWS CLI, and all developer tools inside the container image

## Frameworks

**Core:**
- Standard library (`flag`, `os/exec`, `text/template`, `net/http`, `embed`) — flag parsing, Docker subprocess management, template rendering, plugin downloads

**Testing:**
- Go built-in `testing` package — all tests in `cli/*_test.go`

**Build/Dev:**
- `mise` — task runner (`mise build`, `mise test`, `mise run`); defined in `mise.toml`
- Docker — builds and runs the container image (`agentjail` image tag)

## Key Dependencies

**Critical:**
- `gopkg.in/yaml.v3 v3.0.1` — config file parsing/writing (`cli/config.go`); comment-preserving round-trip via `yaml.Node`
- `golang.org/x/term v0.41.0` — terminal state save/restore before/after `docker exec` (`cli/main.go`, `cli/terminal_other.go`)
- `golang.org/x/sys v0.42.0` — Windows console code-page save/restore (`cli/terminal_windows.go`)
- `github.com/sirupsen/logrus v1.9.4` — structured levelled logging (`cli/logging.go`)

## Configuration

**Host config:**
- `~/.config/agentjail/config.yaml` — global user config, loaded at startup
- Schema documented in `config_schema.yaml` (project root)
- `AGENTJAIL_SHELL`, `AGENTJAIL_EDITOR`, `AGENTJAIL_FILE_BROWSER` env vars override config fields per-invocation

**Build:**
- `cli/go.mod`, `cli/go.sum` — module dependencies
- `mise.toml` (project root) — task definitions
- `cli/embed.go` — embeds `cli/templates/` directory into the binary at compile time (`//go:embed templates`)

## Platform Requirements

**Development:**
- Go 1.25+
- Docker daemon running on host
- `mise` for task running

**Production:**
- Linux or Windows host with Docker
- No external services required; all runtime dependencies are resolved inside the Docker image at build time

## Container Image Tools (installed at image build time)

Managed via `mise use -g` in the Dockerfile:
- `node` — JavaScript runtime (required by Copilot, Claude Code, Fresh editor)
- `aws-cli` — AWS CLI
- `bat`, `eza`, `fd`, `fzf`, `ripgrep`, `television` — shell productivity tools
- `yq` — YAML processor
- `zellij` — terminal multiplexer (default session wrapper)
- `pipx:rich-cli` — rich terminal output for MOTD
- `pipx:rovr` — default file browser

APT packages always installed: `build-essential`, `curl`, `gh`, `git`, `jq`, `micro`, `nano`, `python3`, `python3-pip`, `python3-venv`, `sudo`, `unzip`, `vim`, `wget`, `zsh`

Optional APT packages (build-arg gated): `neovim` (`EDITOR=nvim`), `nnn` (`FILE_BROWSER=nnn`)

Additional tools installed via curl at build time:
- `uv` / `uvx` — Python package manager (from `astral.sh`)
- `starship` v1.24.2 — shell prompt

---

_Stack analysis: 2026-04-25_
