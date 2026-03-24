package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// GlobalConfig structure for ~/.config/agentjail/config.yaml
type GlobalConfig struct {
	DefaultEditor        string                `yaml:"default_editor"`
	DefaultShell         string                `yaml:"default_shell"`
	MountSystemGitconfig bool                  `yaml:"mount_system_gitconfig"`
	MountGhConfig        bool                  `yaml:"mount_gh_config"`
	GithubToken          string                `yaml:"github_token"`
	InjectGhAuthToken    bool                  `yaml:"inject_gh_auth_token"`
	AnthropicApiKey      string                `yaml:"anthropic_api_key"`
	PreferredAgent       string                `yaml:"preferred_agent"`
	UseZellij            *bool                 `yaml:"use_zellij"`
	ZellijTheme          string                `yaml:"zellij_theme"`
	FileBrowser          string                `yaml:"file_browser"`
	ZellijPlugins        []ZellijPlugin        `yaml:"zellij_plugins"`
	AgentFrameworks      AgentFrameworksConfig `yaml:"agent_frameworks"`
	ContainerEnvVars     map[string]string     `yaml:"container_env_vars"`
	PortMappings         []string              `yaml:"port_mappings"`
}

// ZellijEnabled reports whether zellij should be used as the multiplexer.
// Defaults to true when use_zellij is absent from the config file.
func (c *GlobalConfig) ZellijEnabled() bool {
	return c.UseZellij == nil || *c.UseZellij
}

// ZellijThemeOrDefault returns the configured zellij theme, defaulting to
// "tokyo-night-storm" when zellij_theme is absent from the config file.
func (c *GlobalConfig) ZellijThemeOrDefault() string {
	if c.ZellijTheme != "" {
		return c.ZellijTheme
	}
	return "tokyo-night-storm"
}

// FileBrowserCmd returns the command used to launch the file browser tab.
// Defaults to "rovr" when file_browser is absent from the config file.
func (c *GlobalConfig) FileBrowserCmd() string {
	if c.FileBrowser != "" {
		return c.FileBrowser
	}
	return "rovr"
}

// ZellijPlugin represents a single Zellij plugin .wasm file to load in the
// bottom status bar. Exactly one of Path or URL should be set.
type ZellijPlugin struct {
	Path string `yaml:"path"` // absolute or ~/... path on host to the .wasm file
	URL  string `yaml:"url"`  // direct URL to download the .wasm from (cached after first download)
}

type AgentFrameworksConfig struct {
	OpenCode FrameworkConfig `yaml:"opencode"`
	Copilot  FrameworkConfig `yaml:"copilot"`
	// ClaudeCode was previously keyed as "claude_code" in YAML. It was renamed
	// to "claude" to match the binary name. Users with claude_code: in their
	// config should rename the key to claude:.
	ClaudeCode FrameworkConfig `yaml:"claude"`
}

type FrameworkConfig struct {
	Enabled bool     `yaml:"enabled"`
	Plugins []string `yaml:"plugins"`
}

func getGlobalConfigPath() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return filepath.Join(usr.HomeDir, ".config", "agentjail", "config.yaml"), nil
}

func loadGlobalConfig() (*GlobalConfig, error) {
	configPath, err := getGlobalConfigPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		trueVal := true
		config := &GlobalConfig{
			DefaultEditor:        "micro",
			DefaultShell:         "zsh",
			MountSystemGitconfig: true,
			MountGhConfig:        true,
			UseZellij:            &trueVal,
			AgentFrameworks: AgentFrameworksConfig{
				Copilot: FrameworkConfig{Enabled: true},
			},
		}
		if err := saveGlobalConfig(config); err != nil {
			return nil, err
		}
		return config, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config GlobalConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Warn if the old "claude_code" key is present; it was renamed to "claude".
	var rawMap map[string]interface{}
	if yaml.Unmarshal(data, &rawMap) == nil {
		if af, ok := rawMap["agent_frameworks"].(map[string]interface{}); ok {
			if _, hasOld := af["claude_code"]; hasOld {
				fmt.Println("Warning: 'claude_code' in agent_frameworks has been renamed to 'claude'. Please update your config.")
			}
		}
	}

	return &config, nil
}

func saveGlobalConfig(config *GlobalConfig) error {
	configPath, err := getGlobalConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// loadGlobalConfigFromPath loads a GlobalConfig from a specific file path.
func loadGlobalConfigFromPath(path string) (*GlobalConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}
	var config GlobalConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}
	return &config, nil
}

// runConfigUpdate reads the existing config file, detects any missing top-level
// (and agent_frameworks sub-) keys, fills them in with their default values, and
// writes the file back. It prints a summary of what was added.
func runConfigUpdate() error {
	configPath, err := getGlobalConfigPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println("Config file does not exist. Creating with defaults...")
		trueVal := true
		config := &GlobalConfig{
			DefaultEditor:        "micro",
			DefaultShell:         "zsh",
			MountSystemGitconfig: true,
			MountGhConfig:        true,
			UseZellij:            &trueVal,
			ZellijTheme:          "tokyo-night-storm",
			FileBrowser:          "rovr",
			AgentFrameworks: AgentFrameworksConfig{
				Copilot: FrameworkConfig{Enabled: true},
			},
		}
		if err := saveGlobalConfig(config); err != nil {
			return err
		}
		fmt.Printf("Created %s\n", configPath)
		return nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Parse as generic map to detect which keys are actually present in the file.
	var rawMap map[string]interface{}
	if err := yaml.Unmarshal(data, &rawMap); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}
	if rawMap == nil {
		rawMap = make(map[string]interface{})
	}

	// Parse into the typed struct for modification.
	var config GlobalConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	var added []string

	if _, ok := rawMap["default_editor"]; !ok {
		config.DefaultEditor = "micro"
		added = append(added, "default_editor: micro")
	}
	if _, ok := rawMap["default_shell"]; !ok {
		config.DefaultShell = "zsh"
		added = append(added, "default_shell: zsh")
	}
	if _, ok := rawMap["mount_system_gitconfig"]; !ok {
		config.MountSystemGitconfig = true
		added = append(added, "mount_system_gitconfig: true")
	}
	if _, ok := rawMap["mount_gh_config"]; !ok {
		config.MountGhConfig = true
		added = append(added, "mount_gh_config: true")
	}
	if _, ok := rawMap["use_zellij"]; !ok {
		trueVal := true
		config.UseZellij = &trueVal
		added = append(added, "use_zellij: true")
	}
	if _, ok := rawMap["zellij_theme"]; !ok {
		config.ZellijTheme = "tokyo-night-storm"
		added = append(added, "zellij_theme: tokyo-night-storm")
	}
	if _, ok := rawMap["file_browser"]; !ok {
		config.FileBrowser = "rovr"
		added = append(added, "file_browser: rovr")
	}
	if _, ok := rawMap["zellij_plugins"]; !ok {
		config.ZellijPlugins = []ZellijPlugin{}
		added = append(added, "zellij_plugins: []")
	}
	if _, ok := rawMap["inject_gh_auth_token"]; !ok {
		config.InjectGhAuthToken = false
		added = append(added, "inject_gh_auth_token: false")
	}
	if _, ok := rawMap["preferred_agent"]; !ok {
		config.PreferredAgent = ""
		added = append(added, "preferred_agent: \"\"")
	}
	if _, ok := rawMap["github_token"]; !ok {
		config.GithubToken = ""
		added = append(added, "github_token: \"\"")
	}
	if _, ok := rawMap["anthropic_api_key"]; !ok {
		config.AnthropicApiKey = ""
		added = append(added, "anthropic_api_key: \"\"")
	}
	if _, ok := rawMap["container_env_vars"]; !ok {
		if config.ContainerEnvVars == nil {
			config.ContainerEnvVars = map[string]string{}
		}
		added = append(added, "container_env_vars: {}")
	}
	if _, ok := rawMap["port_mappings"]; !ok {
		if config.PortMappings == nil {
			config.PortMappings = []string{}
		}
		added = append(added, "port_mappings: []")
	}

	// agent_frameworks: check top-level key and each sub-framework.
	if agentRaw, ok := rawMap["agent_frameworks"]; !ok {
		config.AgentFrameworks = AgentFrameworksConfig{
			Copilot: FrameworkConfig{Enabled: true},
		}
		added = append(added, "agent_frameworks.opencode.enabled: false")
		added = append(added, "agent_frameworks.copilot.enabled: true")
		added = append(added, "agent_frameworks.claude.enabled: false")
	} else if agentMap, ok := agentRaw.(map[string]interface{}); ok {
		if _, ok := agentMap["opencode"]; !ok {
			added = append(added, "agent_frameworks.opencode.enabled: false")
		}
		if _, ok := agentMap["copilot"]; !ok {
			config.AgentFrameworks.Copilot = FrameworkConfig{Enabled: true}
			added = append(added, "agent_frameworks.copilot.enabled: true")
		}
		if _, ok := agentMap["claude"]; !ok {
			added = append(added, "agent_frameworks.claude.enabled: false")
		}
	}

	if len(added) == 0 {
		fmt.Printf("Config is already up to date (%s). No changes needed.\n", configPath)
		return nil
	}

	if err := saveGlobalConfig(&config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Updated %s — added %d missing properties:\n", configPath, len(added))
	for _, a := range added {
		fmt.Printf("  + %s\n", a)
	}
	return nil
}

// printCleanConfig prints a clean, commented config template to stdout.
func printCleanConfig() {
	fmt.Print(`# AgentJail configuration file
# Default location: ~/.config/agentjail/config.yaml

# Default editor to use inside the container (e.g. micro, vim, nano)
default_editor: micro

# Default shell to use inside the container (bash or zsh)
default_shell: zsh

# Mount the host ~/.gitconfig into the container
mount_system_gitconfig: true

# Mount the host ~/.config/gh into the container (used for gh copilot auth)
mount_gh_config: true

# GitHub personal access token (optional; falls back to GH_TOKEN / GITHUB_TOKEN env vars)
github_token: ""

# When true, injects GITHUB_TOKEN into the container using the fallback chain:
# github_token config > GH_TOKEN env > GITHUB_TOKEN env > gh auth token (CLI).
# Has no effect if GITHUB_TOKEN is already set via container_env_vars.
inject_gh_auth_token: false

# Preferred agent to auto-start with -A. Must match an enabled agent framework name
# (e.g. "copilot" or "opencode"). Leave empty to be prompted when using -A.
preferred_agent: ""

# Launch the container inside a zellij multiplexer with three tabs:
#   tab 1 — preferred agent (auto-starts on first prompt)
#   tab 2 — plain terminal
#   tab 3 — file browser (auto-starts on first prompt)
# Set to false to drop directly into a shell instead.
use_zellij: true

# Zellij color theme. Any built-in zellij theme name is accepted, e.g.:
#   tokyo-night-storm (default), tokyo-night, tokyo-night-light,
#   catppuccin-mocha, gruvbox-dark, nord, one-half-dark
zellij_theme: tokyo-night-storm

# Command used for the file browser tab when use_zellij is true.
# Defaults to "rovr". Set to "yazi" or any other terminal file manager to swap it out.
file_browser: rovr

# Zellij plugins to load when use_zellij is true.
# Each entry configures a plugin via path (host file) or url (downloaded and cached).
zellij_plugins: []
#   - path: "~/projects/my-plugin/dist/my-plugin.wasm"
#   - url: "https://example.com/plugins/helper.wasm"

# Anthropic API key for Claude Code (optional; falls back to ANTHROPIC_API_KEY env var)
anthropic_api_key: ""

# Agent framework settings
agent_frameworks:
  opencode:
    enabled: false
    plugins: []
  copilot:
    enabled: true
    plugins: []
  claude:
    enabled: false
    plugins: []

# Environment variables to inject into the container.
# Supports two schemas:
#   CONT_VAR: value            # set to a literal value
#   CONT_VAR: env:HOST_VAR     # read from the host environment variable HOST_VAR
container_env_vars: {}
#   MY_TOKEN: env:MY_HOST_TOKEN
#   DEBUG: "true"

# Port mappings to publish from the container to the host.
# Uses the same format as docker run -p: [hostIP:]hostPort:containerPort[/protocol]
# or containerPort alone (Docker assigns a random host port).
port_mappings: []
#   - "8080:8080"         # map host 8080 -> container 8080
#   - "127.0.0.1:3000:3000"  # bind only on loopback
#   - "5432:5432/tcp"    # explicit protocol
`)
}
