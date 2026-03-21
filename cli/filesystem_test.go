package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateAgentJailFolder_CreatesDir(t *testing.T) {
	base := t.TempDir()

	dir, err := createAgentJailFolder(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(base, ".agentjail")
	if dir != expected {
		t.Errorf("returned path: got %q, want %q", dir, expected)
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error(".agentjail directory was not created")
	}
}

func TestCreateAgentJailFolder_CreatesBashHistory(t *testing.T) {
	base := t.TempDir()

	dir, err := createAgentJailFolder(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	historyFile := filepath.Join(dir, "bash_history")
	if _, err := os.Stat(historyFile); os.IsNotExist(err) {
		t.Error("bash_history file was not created")
	}
}

func TestCreateAgentJailFolder_Idempotent(t *testing.T) {
	base := t.TempDir()

	// Write something to bash_history first
	agentDir := filepath.Join(base, ".agentjail")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatal(err)
	}
	historyFile := filepath.Join(agentDir, "bash_history")
	if err := os.WriteFile(historyFile, []byte("existing content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Call again — should not overwrite existing history
	if _, err := createAgentJailFolder(base); err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}

	data, err := os.ReadFile(historyFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "existing content" {
		t.Errorf("bash_history was overwritten; got %q", string(data))
	}
}

func TestUpdateGitignore_AddsEntry(t *testing.T) {
	base := t.TempDir()

	if err := updateGitignore(base); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(base, ".gitignore"))
	if err != nil {
		t.Fatalf("could not read .gitignore: %v", err)
	}
	if !strings.Contains(string(data), ".agentjail/") {
		t.Errorf(".agentjail/ not found in .gitignore: %q", string(data))
	}
}

func TestUpdateGitignore_Idempotent(t *testing.T) {
	base := t.TempDir()

	// First call
	if err := updateGitignore(base); err != nil {
		t.Fatal(err)
	}
	// Second call
	if err := updateGitignore(base); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(base, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	count := strings.Count(string(data), ".agentjail/")
	if count != 1 {
		t.Errorf("expected .agentjail/ exactly once, found %d times", count)
	}
}

func TestUpdateGitignore_AppendsToExisting(t *testing.T) {
	base := t.TempDir()
	existing := "node_modules/\ndist/\n"
	if err := os.WriteFile(filepath.Join(base, ".gitignore"), []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	if err := updateGitignore(base); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(base, ".gitignore"))
	content := string(data)
	if !strings.Contains(content, "node_modules/") {
		t.Error("existing .gitignore content was lost")
	}
	if !strings.Contains(content, ".agentjail/") {
		t.Error(".agentjail/ was not appended")
	}
}

func TestUpdateGitignore_AlreadyPresent(t *testing.T) {
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, ".gitignore"), []byte(".agentjail/\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := updateGitignore(base); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(base, ".gitignore"))
	if strings.Count(string(data), ".agentjail/") != 1 {
		t.Error(".agentjail/ should appear exactly once")
	}
}

func TestEnsureFileFromTemplate_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "opencode.json")

	if err := ensureFileFromTemplate(target, "configs/opencode/opencode.json"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if len(data) == 0 {
		t.Error("created file is empty")
	}
}

func TestEnsureFileFromTemplate_DoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "opencode.json")

	original := []byte(`{"existing": true}`)
	if err := os.WriteFile(target, original, 0644); err != nil {
		t.Fatal(err)
	}

	if err := ensureFileFromTemplate(target, "configs/opencode/opencode.json"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(target)
	if string(data) != string(original) {
		t.Errorf("existing file was overwritten: got %q", string(data))
	}
}

func TestEnsureFileFromTemplate_InvalidTemplate(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")

	err := ensureFileFromTemplate(target, "nonexistent/template.txt")
	if err == nil {
		t.Error("expected error for nonexistent template, got nil")
	}
}

func TestCopyTemplateConfigs_AlwaysCopiesRovr(t *testing.T) {
	dir := t.TempDir()
	cfg := &GlobalConfig{} // no agents enabled

	if err := copyTemplateConfigs(dir, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, f := range []string{"config.toml", "pins.json"} {
		path := filepath.Join(dir, "rovr", f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("rovr/%s was not created", f)
		}
	}
}

func TestCopyTemplateConfigs_OpenCode(t *testing.T) {
	dir := t.TempDir()
	cfg := &GlobalConfig{
		AgentFrameworks: AgentFrameworksConfig{
			OpenCode: FrameworkConfig{Enabled: true},
		},
	}

	if err := copyTemplateConfigs(dir, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(dir, "opencode", "opencode.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("opencode/opencode.json was not created")
	}
}

func TestCopyTemplateConfigs_CopilotCreatesDir(t *testing.T) {
	dir := t.TempDir()
	cfg := &GlobalConfig{
		AgentFrameworks: AgentFrameworksConfig{
			Copilot: FrameworkConfig{Enabled: true},
		},
	}

	if err := copyTemplateConfigs(dir, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	copilotDir := filepath.Join(dir, "copilot")
	if _, err := os.Stat(copilotDir); os.IsNotExist(err) {
		t.Error("copilot directory was not created")
	}

	for _, f := range []string{"config.json", "mcp.json"} {
		path := filepath.Join(copilotDir, f)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			t.Errorf("copilot/%s was not created", f)
			continue
		}
		if err != nil {
			t.Errorf("unexpected error stating copilot/%s: %v", f, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("copilot/%s is empty, expected non-empty template", f)
		}
	}
}

func TestCopyTemplateConfigs_ZellijEnabledByDefault(t *testing.T) {
	dir := t.TempDir()
	cfg := &GlobalConfig{} // UseZellij nil → defaults to enabled

	if err := copyTemplateConfigs(dir, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// zellij config and layout are now generated by writeZellijFiles, not
	// copyTemplateConfigs, so we just verify the call succeeds without error.
}

func TestCopyTemplateConfigs_ZellijDisabled(t *testing.T) {
	dir := t.TempDir()
	falseVal := false
	cfg := &GlobalConfig{UseZellij: &falseVal}

	if err := copyTemplateConfigs(dir, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "zellij")); err == nil {
		t.Error("zellij/ directory should not be created when use_zellij is false")
	}
}

func TestUpdateGitignore_ExistingFileWithoutTrailingNewline(t *testing.T) {
	base := t.TempDir()
	// No trailing newline — the function should insert one before the new entry.
	existing := "node_modules/"
	if err := os.WriteFile(filepath.Join(base, ".gitignore"), []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	if err := updateGitignore(base); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(base, ".gitignore"))
	content := string(data)
	if !strings.Contains(content, "node_modules/\n.agentjail/") {
		t.Errorf("expected newline inserted before .agentjail/; got: %q", content)
	}
}

func TestCopyTemplateConfigs_ClaudeCode(t *testing.T) {
	dir := t.TempDir()
	cfg := &GlobalConfig{
		AgentFrameworks: AgentFrameworksConfig{
			ClaudeCode: FrameworkConfig{Enabled: true},
		},
	}

	if err := copyTemplateConfigs(dir, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Rovr configs should always be present.
	for _, f := range []string{"config.toml", "pins.json"} {
		if _, err := os.Stat(filepath.Join(dir, "rovr", f)); os.IsNotExist(err) {
			t.Errorf("rovr/%s missing when ClaudeCode is enabled", f)
		}
	}

	// ClaudeCode has no template configs of its own; no claude/ dir should appear.
	if _, err := os.Stat(filepath.Join(dir, "claude")); err == nil {
		t.Error("unexpected claude/ directory created for ClaudeCode framework")
	}
}

func TestCopyTemplateConfigs_OpenCodeNotEnabled(t *testing.T) {
	dir := t.TempDir()
	cfg := &GlobalConfig{
		AgentFrameworks: AgentFrameworksConfig{
			OpenCode: FrameworkConfig{Enabled: false},
		},
	}

	if err := copyTemplateConfigs(dir, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(dir, "opencode", "opencode.json")
	if _, err := os.Stat(path); err == nil {
		t.Error("opencode/opencode.json should not be created when OpenCode is disabled")
	}
}
