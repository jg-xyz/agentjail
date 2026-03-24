# [AGENTJAIL]
a cli for containerized agentic environments

AgentJail launches AI coding agents (GitHub Copilot, OpenCode, Claude Code) inside isolated Docker containers. Your project directory is mounted into the container, and a persistent `.agentjail/` folder keeps shell history, tool configs, and credentials across sessions.

## Requirements

- Docker
- Go 1.21+ (to build from source) or download a pre-built binary from `dist/`

## Installation

```sh
# build from source
cd cli && go build -o ../dist/agentjail .
```

Or with mise (from the repo root):

```sh
mise build
```

## Usage

```
agentjail [options]
```

Run from inside a project directory. With no arguments, AgentJail detects an existing container for the current directory and re-enters it. If none is found, it starts a new one.

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-d <dir>` | `.` | Project directory to mount |
| `-e <editor>` | config | Editor binary (`micro`, `vim`, `nano`) |
| `-s <shell>` | config | Shell (`zsh`, `bash`) |
| `-n <network>` | — | Docker network to connect to |
| `-v <mount>` | — | Extra volume mount, repeatable (`/host:/container`) |
| `-p <port>` | — | Port mapping, repeatable (`8080:8080`) |
| `-A` | — | Auto-start the preferred agent |
| `-b` / `-build` | — | Rebuild the image (uses cache) |
| `-build-no-cache` | — | Rebuild the image without cache |
| `-P` | — | Privileged mode — exposes the host Docker socket |
| `-C <path>` | `opencode.json` | Path to an OpenCode config file |
| `-E <path>` | — | Editor config file to mount at `/root/<filename>` |
| `-D <path>` | — | Custom Dockerfile to use |
| `--config` | — | Print a clean config template to stdout and exit |
| `--config <path>` | — | Load config from a specific file instead of the default |
| `--verbose` | — | Enable debug logging |

### Examples

### Subcommands

| Subcommand | Description |
|------------|-------------|
| `agentjail update-config` | Migrate an existing config file by adding any missing fields with their default values, preserving existing comments and formatting |

```sh
# start a container for the current project
agentjail

# start and immediately launch the preferred agent
agentjail -A

# forward a port and add an extra volume
agentjail -p 3000:3000 -v /tmp/data:/data

# rebuild the image after changing agent config, then start
agentjail -b

# print a starter config file
agentjail --config

# save it to the default location
agentjail --config > ~/.config/agentjail/config.yaml
```

## Configuration

The global config lives at `~/.config/agentjail/config.yaml`. It is created with defaults on first run. To see all available options with comments:

```sh
agentjail --config
```

### Full reference

```yaml
# Editor binary used inside the container.
# Sets $EDITOR and the "edit" shell alias.
# Values: micro | vim | nano
default_editor: micro

# Shell used inside the container.
# Values: zsh | bash
default_shell: zsh

# Mount ~/.gitconfig from the host so git identity is available.
mount_system_gitconfig: true

# Mount ~/.config/gh from the host.
# Required for gh CLI auth and the Copilot extension.
mount_gh_config_dir: true

# GitHub personal access token to inject as GH_TOKEN.
# Fallback chain (first non-empty wins):
#   1. github_token (this field)
#   2. GH_TOKEN host env var
#   3. GITHUB_TOKEN host env var
github_token: ""

# When true, also injects GITHUB_TOKEN into the container.
# Uses the same fallback chain as github_token, with an additional
# final fallback to `gh auth token` (the gh CLI).
# Has no effect when GITHUB_TOKEN is set via container_env_vars.
inject_gh_auth_token: false

# Anthropic API key for Claude Code.
# Fallback: ANTHROPIC_API_KEY host env var.
anthropic_api_key: ""

# Agent to auto-launch with -A.
# Must match an enabled framework name: copilot | opencode | claude
# Leave empty to be prompted when using -A.
preferred_agent: ""

# Launch the container inside a Zellij multiplexer.
# See "Zellij" section below.
use_zellij: true

# Zellij color theme. Any built-in theme name works, e.g.:
#   tokyo-night-storm (default), catppuccin-mocha, gruvbox-dark, nord
zellij_theme: tokyo-night-storm

# Command launched in the files tab.
# Values: rovr | <any terminal file manager in the image>
file_browser: rovr

# Zellij plugins to display in the bottom status bar (right side).
# See "Zellij plugins" section below.
zellij_plugins: []

# Which agent CLIs to install in the image.
# Changing enabled flags requires rebuilding: agentjail -b
agent_frameworks:
  copilot:
    enabled: true
  opencode:
    enabled: false
  claude:
    enabled: false

# Environment variables injected into the container via docker run -e.
# See "Forwarding environment variables" below.
container_env_vars: {}

# Ports to publish, merged with any -p flags on the command line.
# Uses Docker's -p format: [hostIP:]hostPort:containerPort[/protocol]
port_mappings: []
```

### Forwarding environment variables

`container_env_vars` supports two value formats:

**Literal value** — the string is injected as-is:

```yaml
container_env_vars:
  MY_TEAM: acme
  DEBUG: "true"
```

**Host env var reference** — reads the variable from the host at launch time using the `env:` prefix:

```yaml
container_env_vars:
  ANTHROPIC_API_KEY: env:ANTHROPIC_API_KEY
  OPENAI_API_KEY: env:OPENAI_API_KEY
  AWS_PROFILE: env:AWS_PROFILE
```

If the referenced host variable is unset, the entry is silently skipped — the variable is not passed to the container at all. This prevents accidentally overriding credential files with an empty value.

You can mix both formats freely:

```yaml
container_env_vars:
  ENVIRONMENT: production
  DATABASE_URL: env:DATABASE_URL
  STRIPE_KEY: env:STRIPE_SECRET_KEY
```

### Port mappings

```yaml
port_mappings:
  - "3000:3000"             # host 3000 -> container 3000
  - "127.0.0.1:5432:5432"  # bind only on loopback
  - "8080:80/tcp"           # explicit protocol
```

These are merged with any `-p` flags given on the command line.

## Zellij

When `use_zellij: true` (the default), the container opens inside a [Zellij](https://zellij.dev) terminal multiplexer with three tabs:

| Tab | Contents |
|-----|----------|
| agent | Preferred agent — auto-launches on first prompt |
| terminal | Plain shell |
| files | File browser (`rovr` by default) — auto-launches on first prompt |

Keybinds (start in locked mode — all other keys pass through to the active pane):

| Key | Action |
|-----|--------|
| `Alt+T` | New tab |
| `Alt+W` | Close tab |
| `Alt+[` / `Alt+]` | Cycle tabs |
| `Alt+Q` | Quit |

Set `use_zellij: false` to drop directly into a shell instead.

### Zellij plugins

Plugins are `.wasm` files that appear on the right side of the bottom status bar, alongside the keybind hints. Two installation methods are supported:

**Local path** — copied from the host on every launch:

```yaml
zellij_plugins:
  - path: "~/projects/my-plugin/dist/my-plugin.wasm"
```

**URL** — downloaded on first launch and cached in `.agentjail/zellij/plugins/`:

```yaml
zellij_plugins:
  - url: "https://github.com/user/repo/releases/latest/download/my-plugin.wasm"
```

To force a re-download of a cached plugin, delete `.agentjail/zellij/plugins/<filename>`.

Missing files and failed downloads are skipped with a warning — they will not prevent the container from starting.

## Agents

### GitHub Copilot

Requires a GitHub account with Copilot access. On first use, run `gh auth login` inside the container. Auth is persisted via the `~/.config/gh` mount.

Enable in config:

```yaml
agent_frameworks:
  copilot:
    enabled: true
mount_gh_config_dir: true
```

### OpenCode

Enable in config:

```yaml
agent_frameworks:
  opencode:
    enabled: true
```

Auth is persisted by mounting `~/.config/opencode` from the host. A project-level `opencode.json` config can be pointed to with `-C`.

### Claude Code

Requires an Anthropic API key. Auth and settings are persisted by mounting `~/.claude` and `~/.claude.json` from the host.

Enable in config:

```yaml
agent_frameworks:
  claude:
    enabled: true

# Option 1: set the key directly in config
anthropic_api_key: "sk-ant-..."

# Option 2: forward from the host environment
container_env_vars:
  ANTHROPIC_API_KEY: env:ANTHROPIC_API_KEY
```

If `anthropic_api_key` is empty and `ANTHROPIC_API_KEY` is not in `container_env_vars`, AgentJail will automatically forward the host's `ANTHROPIC_API_KEY` env var if it is set.

After enabling any agent, rebuild the image:

```sh
agentjail -b
```

## Container details

- Project directory → `/project` (working directory)
- `.agentjail/` in the project root → `/root/.agentjail` (shell history, tool configs)
- `.agentjail/` is automatically added to `.gitignore`
- Container name is derived from the project directory name (`agentjail.<prefix>`)
- Running `agentjail` with no args re-enters an existing container for the current directory

### What's in the image

Ubuntu 24.04 with: `gh`, `git`, `micro`, `vim`, `nano`, `zsh`, `bash`, `node` (via mise), `python3`, `uv`, `pip`, `aws-cli`, `ripgrep`, `fd`, `fzf`, `eza`, `yq`, `television`, `zellij`, `rovr`, `rich-cli`, starship prompt.

Shell aliases: `files` → file browser, `edit` → configured editor, `/exit` → exit.
