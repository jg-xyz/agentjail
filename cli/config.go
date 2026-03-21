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
	AgentFrameworks      AgentFrameworksConfig `yaml:"agent_frameworks"`
	ContainerEnvVars     map[string]string     `yaml:"container_env_vars"`
	PortMappings         []string              `yaml:"port_mappings"`
}

type AgentFrameworksConfig struct {
	OpenCode   FrameworkConfig `yaml:"opencode"`
	Copilot    FrameworkConfig `yaml:"copilot"`
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
		config := &GlobalConfig{
			DefaultEditor:        "micro",
			DefaultShell:         "zsh",
			MountSystemGitconfig: true,
			MountGhConfig:        true,
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
