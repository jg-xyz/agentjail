package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// writeConfig is a test helper that writes content to a temp file and returns its path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// readConfig is a test helper that reads and parses the YAML at path.
func readConfig(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestRunConfigUpdateFromPath_AlreadyUpToDate(t *testing.T) {
	path := writeConfig(t, `default_editor: micro
default_shell: zsh
mount_system_gitconfig: true
mount_gh_config: true
use_zellij: true
zellij_theme: tokyo-night-storm
file_browser: rovr
zellij_plugins: []
inject_gh_auth_token: false
preferred_agent: ""
github_token: ""
anthropic_api_key: ""
container_env_vars: {}
port_mappings: []
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
`)
	before, _ := os.ReadFile(path)
	if err := runConfigUpdateFromPath(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	after, _ := os.ReadFile(path)
	// File should be unchanged.
	if string(before) != string(after) {
		t.Errorf("expected no change but file was modified")
	}
	// No backup should be created.
	entries, _ := os.ReadDir(filepath.Dir(path))
	for _, e := range entries {
		if strings.Contains(e.Name(), ".bkup.") {
			t.Errorf("unexpected backup file created: %s", e.Name())
		}
	}
}

func TestRunConfigUpdateFromPath_AddsMissingFields(t *testing.T) {
	// Minimal config — many fields absent.
	path := writeConfig(t, `default_editor: vim
default_shell: bash
`)
	if err := runConfigUpdateFromPath(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := readConfig(t, path)

	// Original values must be preserved.
	if cfg["default_editor"] != "vim" {
		t.Errorf("default_editor: got %v, want vim", cfg["default_editor"])
	}
	if cfg["default_shell"] != "bash" {
		t.Errorf("default_shell: got %v, want bash", cfg["default_shell"])
	}

	// Missing fields must be filled with defaults.
	for _, key := range []string{
		"mount_system_gitconfig", "mount_gh_config", "use_zellij",
		"zellij_theme", "file_browser", "zellij_plugins",
		"inject_gh_auth_token", "preferred_agent", "github_token",
		"anthropic_api_key", "container_env_vars", "port_mappings",
		"agent_frameworks",
	} {
		if _, ok := cfg[key]; !ok {
			t.Errorf("expected key %q to be added but it is missing", key)
		}
	}
	if cfg["zellij_theme"] != "tokyo-night-storm" {
		t.Errorf("zellij_theme: got %v, want tokyo-night-storm", cfg["zellij_theme"])
	}
	if cfg["file_browser"] != "rovr" {
		t.Errorf("file_browser: got %v, want rovr", cfg["file_browser"])
	}
}

func TestRunConfigUpdateFromPath_BackupCreated(t *testing.T) {
	// Missing a field so an update (and therefore a backup) is triggered.
	path := writeConfig(t, `default_editor: micro
default_shell: zsh
`)
	if err := runConfigUpdateFromPath(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	var backups []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "config.yaml.bkup.") {
			backups = append(backups, e.Name())
		}
	}
	if len(backups) != 1 {
		t.Errorf("expected 1 backup file, got %d: %v", len(backups), backups)
	}
}

func TestRunConfigUpdateFromPath_CommentsPreserved(t *testing.T) {
	path := writeConfig(t, `# top-level comment
default_editor: micro  # inline comment
default_shell: zsh
`)
	if err := runConfigUpdateFromPath(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "# top-level comment") {
		t.Error("top-level comment was lost")
	}
	if !strings.Contains(content, "# inline comment") {
		t.Error("inline comment was lost")
	}
}

func TestRunConfigUpdateFromPath_AgentFrameworksMissingSubkeys(t *testing.T) {
	// agent_frameworks present but missing the "claude" sub-key.
	path := writeConfig(t, `default_editor: micro
default_shell: zsh
mount_system_gitconfig: true
mount_gh_config: true
use_zellij: true
zellij_theme: tokyo-night-storm
file_browser: rovr
zellij_plugins: []
inject_gh_auth_token: false
preferred_agent: ""
github_token: ""
anthropic_api_key: ""
container_env_vars: {}
port_mappings: []
agent_frameworks:
  opencode:
    enabled: false
  copilot:
    enabled: true
`)
	if err := runConfigUpdateFromPath(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := readConfig(t, path)
	af, ok := cfg["agent_frameworks"].(map[string]interface{})
	if !ok {
		t.Fatal("agent_frameworks is not a map")
	}
	if _, ok := af["claude"]; !ok {
		t.Error("expected agent_frameworks.claude to be added")
	}
	// Existing sub-keys must still be present.
	if _, ok := af["opencode"]; !ok {
		t.Error("agent_frameworks.opencode was removed")
	}
	if _, ok := af["copilot"]; !ok {
		t.Error("agent_frameworks.copilot was removed")
	}
}

func TestRunConfigUpdateFromPath_CreatesMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	// File does not exist yet.
	if err := runConfigUpdateFromPath(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected config file to be created")
	}
	cfg := readConfig(t, path)
	if cfg["default_editor"] != "micro" {
		t.Errorf("default_editor: got %v, want micro", cfg["default_editor"])
	}
}

func TestRunConfigUpdateFromPath_IdempotentOnRepeatedRuns(t *testing.T) {
	path := writeConfig(t, `default_editor: micro
default_shell: zsh
`)
	// First run adds missing fields.
	if err := runConfigUpdateFromPath(path); err != nil {
		t.Fatalf("first run error: %v", err)
	}
	after1, _ := os.ReadFile(path)

	// Second run should be a no-op.
	if err := runConfigUpdateFromPath(path); err != nil {
		t.Fatalf("second run error: %v", err)
	}
	after2, _ := os.ReadFile(path)

	if string(after1) != string(after2) {
		t.Error("second run modified the file; update is not idempotent")
	}

	// Only one backup from the first run.
	entries, _ := os.ReadDir(filepath.Dir(path))
	var backups int
	for _, e := range entries {
		if strings.Contains(e.Name(), ".bkup.") {
			backups++
		}
	}
	if backups != 1 {
		t.Errorf("expected 1 backup after two runs, got %d", backups)
	}
}

func TestRunConfigUpdateFromPath_ExistingValuesNotOverwritten(t *testing.T) {
	path := writeConfig(t, `default_editor: nano
default_shell: bash
preferred_agent: opencode
zellij_theme: catppuccin-mocha
agent_frameworks:
  claude:
    enabled: true
    plugins: []
`)
	if err := runConfigUpdateFromPath(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := readConfig(t, path)
	if cfg["default_editor"] != "nano" {
		t.Errorf("default_editor overwritten: got %v", cfg["default_editor"])
	}
	if cfg["default_shell"] != "bash" {
		t.Errorf("default_shell overwritten: got %v", cfg["default_shell"])
	}
	if cfg["preferred_agent"] != "opencode" {
		t.Errorf("preferred_agent overwritten: got %v", cfg["preferred_agent"])
	}
	if cfg["zellij_theme"] != "catppuccin-mocha" {
		t.Errorf("zellij_theme overwritten: got %v", cfg["zellij_theme"])
	}
	af := cfg["agent_frameworks"].(map[string]interface{})
	claude := af["claude"].(map[string]interface{})
	if claude["enabled"] != true {
		t.Errorf("agent_frameworks.claude.enabled overwritten: got %v", claude["enabled"])
	}
}
