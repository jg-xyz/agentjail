package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadGlobalConfigFromPath_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
default_editor: vim
default_shell: bash
mount_system_gitconfig: false
mount_gh_config: true
github_token: "tok123"
preferred_agent: "copilot"
agent_frameworks:
  opencode:
    enabled: false
  copilot:
    enabled: true
    plugins:
      - my-plugin
container_env_vars:
  FOO: bar
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadGlobalConfigFromPath(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DefaultEditor != "vim" {
		t.Errorf("DefaultEditor: got %q, want %q", cfg.DefaultEditor, "vim")
	}
	if cfg.DefaultShell != "bash" {
		t.Errorf("DefaultShell: got %q, want %q", cfg.DefaultShell, "bash")
	}
	if cfg.MountSystemGitconfig {
		t.Error("MountSystemGitconfig: expected false")
	}
	if !cfg.MountGhConfig {
		t.Error("MountGhConfig: expected true")
	}
	if cfg.GithubToken != "tok123" {
		t.Errorf("GithubToken: got %q, want %q", cfg.GithubToken, "tok123")
	}
	if cfg.PreferredAgent != "copilot" {
		t.Errorf("PreferredAgent: got %q, want %q", cfg.PreferredAgent, "copilot")
	}
	if cfg.AgentFrameworks.OpenCode.Enabled {
		t.Error("OpenCode.Enabled: expected false")
	}
	if !cfg.AgentFrameworks.Copilot.Enabled {
		t.Error("Copilot.Enabled: expected true")
	}
	if len(cfg.AgentFrameworks.Copilot.Plugins) != 1 || cfg.AgentFrameworks.Copilot.Plugins[0] != "my-plugin" {
		t.Errorf("Copilot.Plugins: got %v", cfg.AgentFrameworks.Copilot.Plugins)
	}
	if cfg.ContainerEnvVars["FOO"] != "bar" {
		t.Errorf("ContainerEnvVars[FOO]: got %q", cfg.ContainerEnvVars["FOO"])
	}
}

func TestLoadGlobalConfigFromPath_Missing(t *testing.T) {
	_, err := loadGlobalConfigFromPath("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoadGlobalConfigFromPath_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(":::invalid: yaml: [\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := loadGlobalConfigFromPath(path)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestLoadGlobalConfigFromPath_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	yamlContent := `
default_editor: nano
default_shell: bash
mount_system_gitconfig: false
mount_gh_config: true
github_token: mytoken
preferred_agent: opencode
agent_frameworks:
  opencode:
    enabled: true
    plugins: [p1]
  copilot:
    enabled: false
container_env_vars:
  K: V
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := loadGlobalConfigFromPath(cfgPath)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if got.DefaultEditor != "nano" {
		t.Errorf("DefaultEditor: got %q, want %q", got.DefaultEditor, "nano")
	}
	if got.GithubToken != "mytoken" {
		t.Errorf("GithubToken: got %q, want %q", got.GithubToken, "mytoken")
	}
	if !got.AgentFrameworks.OpenCode.Enabled {
		t.Error("OpenCode.Enabled: expected true")
	}
	if got.AgentFrameworks.Copilot.Enabled {
		t.Error("Copilot.Enabled: expected false")
	}
	if got.ContainerEnvVars["K"] != "V" {
		t.Errorf("ContainerEnvVars[K]: got %q", got.ContainerEnvVars["K"])
	}
}

func TestPrintCleanConfig_ContainsExpectedKeys(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	printCleanConfig()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	output := buf.String()

	requiredKeys := []string{
		"default_editor",
		"default_shell",
		"mount_system_gitconfig",
		"mount_gh_config",
		"github_token",
		"preferred_agent",
		"agent_frameworks",
		"container_env_vars",
	}
	for _, key := range requiredKeys {
		if !bytes.Contains([]byte(output), []byte(key)) {
			t.Errorf("printCleanConfig output missing key %q", key)
		}
	}
}
