# STRUCTURE
_Last updated: 2026-04-25_

## Summary
AgentJail is a single Go binary in `cli/`. All source lives in `package main`; there are no sub-packages. Templates are bundled via `embed.go` and extracted at runtime into `.agentjail/` inside the host project directory.

## Directory Layout

```
agentjail/
├── cli/                        # Go module root (all source)
│   ├── go.mod / go.sum
│   ├── main.go                 # Orchestrator: flags, image build, docker run assembly
│   ├── agent.go                # Agent enumeration, command generation, interactive picker
│   ├── config.go               # GlobalConfig struct, YAML load/save, update-config subcommand
│   ├── docker.go               # imageExists, getContainerForDirectory, createTempDockerfile
│   ├── filesystem.go           # createAgentJailFolder, copyTemplateConfigs, updateGitignore
│   ├── metadata.go             # AgentJailMetadata JSON save/load, version check
│   ├── zellij.go               # writeZellijFiles, plugin handling, layout/config rendering
│   ├── noninteractive.go       # -N mode: exec args, NI lock, adaptRunArgsForNonInteractive
│   ├── logging.go              # logrus setup, initLogger, enableVerboseLogging
│   ├── embed.go                # //go:embed templates/* — bundles templates into binary
│   ├── terminal_other.go       # POSIX: no-op saveConsoleCP / restoreConsoleCP
│   ├── terminal_windows.go     # Windows: console code page save/restore via WinAPI
│   ├── *_test.go               # Tests alongside source files
│   └── templates/
│       ├── Dockerfile          # Ubuntu 24.04 image definition (build-arg controlled)
│       ├── .zshrc              # Container shell config, aliases, MOTD
│       └── configs/
│           ├── copilot/        # config.json, mcp.json
│           ├── opencode/       # opencode.json
│           ├── rovr/           # config.toml, pins.json, style.tcss
│           ├── claude/         # settings.json
│           └── zellij/         # config.kdl, layout.kdl (Go text/template)
├── dist/                       # Build output (gitignored)
│   ├── agentjail               # Linux binary
│   └── agentjail.exe           # Windows binary
├── docs/                       # Documentation
├── config_schema.yaml          # Human-readable schema for config.yaml
├── mise.toml                   # Build/test/run tasks
└── CLAUDE.md                   # AI assistant instructions
```

## Runtime Layout (per project)

```
<project>/
└── .agentjail/                 # Created on first run; gitignored
    ├── metadata.json           # Container state: name, volumes, timestamps
    ├── bash_history            # Shell history (persists across container runs)
    ├── zsh_history
    ├── ni.lock                 # Non-interactive lock file (transient)
    ├── rovr/                   # Rovr file browser config
    ├── opencode/               # OpenCode config (when enabled)
    ├── copilot/                # Copilot config (when enabled)
    └── zellij/                 # Zellij layout/config + tabs/ scripts
        ├── config.kdl
        ├── layout.kdl
        ├── plugins/            # Cached .wasm plugin files
        └── tabs/
            ├── agent.sh        # Tab 1: launches preferred agent
            ├── files.sh        # Tab 3: launches file browser
            └── hints.sh        # Bottom bar: keybind hints
```

## Module Boundaries

All code is `package main`. Logical grouping by file:

| File | Responsibility |
|------|---------------|
| `main.go` | Orchestration only — no business logic |
| `config.go` | All config I/O and the `update-config` subcommand |
| `agent.go` | Agent names, commands, interactive selection |
| `docker.go` | Docker queries (image/container existence) |
| `filesystem.go` | `.agentjail/` directory + template extraction |
| `metadata.go` | Metadata JSON persistence |
| `zellij.go` | Zellij file generation + plugin management |
| `noninteractive.go` | `-N` mode: lock, exec args, container coordination |
| `logging.go` | Logger singleton |
| `embed.go` | Embedded FS declaration |

## Where to Add New Code

- **New agent framework**: `config.go` (add field to `AgentFrameworksConfig`), `agent.go` (add to `enabledAgents` + `agentCommand`), `main.go` (add mount/env block), `filesystem.go` (`copyTemplateConfigs`), add template configs under `cli/templates/configs/<name>/`
- **New CLI flag**: `main.go` flag block
- **New config field**: `GlobalConfig` struct in `config.go`, `runConfigUpdateFromPath` defaults, `printCleanConfig` docs
- **New zellij feature**: `zellij.go`
- **New Docker behavior**: `docker.go` or inline in `main.go`
