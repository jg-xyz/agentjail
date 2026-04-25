# CONCERNS
_Last updated: 2026-04-25_

## Summary
AgentJail works well for its purpose but has several structural gaps: `main()` is untestable due to its size, version management is hardcoded, and the container name derivation can produce collisions on short directory names. Security is reasonable for a local dev tool but credentials flow through env vars and CLI args.

## Technical Debt

- **Hardcoded version**: `currentVersion := "1.0.0"` in `main.go` — no build-time injection, version never actually changes
- **`main()` monolith**: ~400 lines handling flags, image build, volume assembly, env var injection, and exec. Impossible to unit test; changes require end-to-end verification
- **Container name collision risk**: name is `agentjail.<first-5-chars-of-dir>` — projects named `backend`, `backoffice`, `back-end` all collide on `agentjail.backe`
- **Config backup files accumulate**: `runConfigUpdateFromPath` creates `.bkup.<timestamp>` files on each run with changes; no cleanup mechanism
- **`copyTemplateConfigs` always overwrites rovr**: rovr config overwritten on every launch regardless of user edits
- **Deprecated `claude_code` key**: handled with a warning but the migration is manual; old configs silently use wrong defaults

## Security Considerations

- **Credentials in env vars**: `ANTHROPIC_API_KEY`, `GH_TOKEN`, `GITHUB_TOKEN` passed as `-e KEY=VALUE` args — visible in process listings (`ps aux`)
  - Partial mitigation for `ANTHROPIC_API_KEY`: uses valueless `-e ANTHROPIC_API_KEY` to inherit from process env, but only when not in `container_env_vars`
  - `GH_TOKEN` still uses `-e GH_TOKEN=<value>` form
- **Plugin download**: `downloadPlugin` uses `http.Client` with 30s timeout; no TLS certificate pinning, no checksum verification on downloaded `.wasm` files — supply chain risk if URL is compromised
- **`//nolint:gosec` on plugin download**: `client.Get(rawURL)` marked as user-supplied config, not attacker-controlled — reasonable for local dev tool
- **`--privileged` mode**: intentionally supported; mounts `/var/run/docker.sock` — full container escape capability. No warning shown to user
- **`container_env_vars` with `env:` prefix**: reads arbitrary host env vars into container — misconfiguration could leak sensitive vars
- **`.agentjail/` not encrypted**: contains metadata, shell history, and auth credential files (copilot config) in plaintext

## Missing Features / Gaps

- **No CI/CD**: zero GitHub Actions workflows; no automated test runs on PR
- **No binary versioning**: no `--version` flag, no build-time version injection via `ldflags`
- **No image version tracking**: image is always named `agentjail`; no tag strategy for upgrades — stale images silently persist
- **No `--dry-run`**: no way to preview the `docker run` command without executing it
- **No container cleanup**: no `agentjail stop` or `agentjail rm` subcommand; containers must be managed manually
- **Windows**: `terminal_windows.go` provides console CP save/restore but the tool hasn't been validated end-to-end on Windows (no CI)
- **No health check**: no verification that the container actually started and the agent is responsive

## Improvement Opportunities

- Extract `main()` into smaller functions to enable unit testing
- Build-time version injection: `go build -ldflags "-X main.version=$(git describe --tags)"`
- Container name collision: use hash of full path instead of first 5 chars
- Use `--env-file` for credential injection instead of `-e KEY=VALUE` to avoid process listing exposure
- Add `agentjail list` / `agentjail stop` / `agentjail rm` subcommands
- GitHub Actions workflow for `go test ./...` on push/PR

## Cross-cutting Concerns

- **Zellij coupling**: `main.go` has significant branching on `globalConfig.ZellijEnabled()` — disabling zellij changes launch behavior substantially
- **Agent framework coupling**: each new agent requires coordinated changes across `config.go`, `agent.go`, `main.go`, `filesystem.go`, and `templates/`
- **Host filesystem assumptions**: assumes `~/.config/gh`, `~/.config/github-copilot`, `~/.claude`, `~/.gitconfig` exist at standard locations; no fallback when paths are non-standard
- **Docker dependency**: entire tool is non-functional without Docker installed; no validation at startup
