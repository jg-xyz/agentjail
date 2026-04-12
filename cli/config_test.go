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
mount_gh_config_dir: true
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
	if !cfg.MountGhConfigDir {
		t.Error("MountGhConfigDir: expected true")
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
mount_gh_config_dir: true
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
		"mount_gh_config_dir",
		"github_token",
		"inject_gh_auth_token",
		"preferred_agent",
		"zellij_theme",
		"agent_frameworks",
		"container_env_vars",
		"port_mappings",
	}
	for _, key := range requiredKeys {
		if !bytes.Contains([]byte(output), []byte(key)) {
			t.Errorf("printCleanConfig output missing key %q", key)
		}
	}
}

func TestLoadGlobalConfigFromPath_PortMappings(t *testing.T) {
dir := t.TempDir()
path := filepath.Join(dir, "config.yaml")
content := `
default_editor: micro
default_shell: zsh
port_mappings:
  - "8080:8080"
  - "127.0.0.1:3000:3000"
  - "5432:5432/tcp"
`
if err := os.WriteFile(path, []byte(content), 0644); err != nil {
t.Fatal(err)
}

cfg, err := loadGlobalConfigFromPath(path)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if len(cfg.PortMappings) != 3 {
t.Fatalf("expected 3 port mappings, got %d: %v", len(cfg.PortMappings), cfg.PortMappings)
}
if cfg.PortMappings[0] != "8080:8080" {
t.Errorf("PortMappings[0]: got %q, want %q", cfg.PortMappings[0], "8080:8080")
}
if cfg.PortMappings[1] != "127.0.0.1:3000:3000" {
		t.Errorf("PortMappings[1]: got %q, want %q", cfg.PortMappings[1], "127.0.0.1:3000:3000")
	}
	if cfg.PortMappings[2] != "5432:5432/tcp" {
		t.Errorf("PortMappings[2]: got %q, want %q", cfg.PortMappings[2], "5432:5432/tcp")
	}
}

func TestLoadGlobalConfigFromPath_PortMappingsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
default_editor: micro
default_shell: zsh
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadGlobalConfigFromPath(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.PortMappings) != 0 {
		t.Errorf("expected empty port mappings, got %v", cfg.PortMappings)
	}
}

func TestLoadGlobalConfigFromPath_InjectGhAuthToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `
default_editor: micro
default_shell: zsh
inject_gh_auth_token: true
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadGlobalConfigFromPath(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.InjectGhAuthToken {
		t.Error("InjectGhAuthToken: expected true")
	}
}

func TestZellijEnabled(t *testing.T) {
	trueVal := true
	falseVal := false
	cases := []struct {
		name      string
		useZellij *bool
		want      bool
	}{
		{"nil defaults to true", nil, true},
		{"explicit true", &trueVal, true},
		{"explicit false", &falseVal, false},
	}
	for _, c := range cases {
		cfg := &GlobalConfig{UseZellij: c.useZellij}
		if got := cfg.ZellijEnabled(); got != c.want {
			t.Errorf("%s: ZellijEnabled() = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestZellijThemeOrDefault(t *testing.T) {
	cases := []struct {
		name  string
		theme string
		want  string
	}{
		{"empty defaults to tokyo-night-storm", "", "tokyo-night-storm"},
		{"custom theme returned as-is", "catppuccin-mocha", "catppuccin-mocha"},
		{"another custom theme", "gruvbox-dark", "gruvbox-dark"},
	}
	for _, c := range cases {
		cfg := &GlobalConfig{ZellijTheme: c.theme}
		if got := cfg.ZellijThemeOrDefault(); got != c.want {
			t.Errorf("%s: ZellijThemeOrDefault() = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestFileBrowserCmd(t *testing.T) {
	cases := []struct {
		name        string
		fileBrowser string
		want        string
	}{
		{"empty defaults to rovr", "", "rovr"},
		{"custom value returned as-is", "yazi", "yazi"},
		{"another custom value", "ranger", "ranger"},
	}
	for _, c := range cases {
		cfg := &GlobalConfig{FileBrowser: c.fileBrowser}
		if got := cfg.FileBrowserCmd(); got != c.want {
			t.Errorf("%s: FileBrowserCmd() = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestLoadGlobalConfigFromPath_OptionalEditors(t *testing.T) {
	cases := []struct {
		editor string
	}{
		{"nvim"},
		{"hx"},
		{"fresh"},
	}
	for _, c := range cases {
		t.Run(c.editor, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			content := "default_editor: " + c.editor + "\ndefault_shell: zsh\n"
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
			cfg, err := loadGlobalConfigFromPath(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.DefaultEditor != c.editor {
				t.Errorf("DefaultEditor: got %q, want %q", cfg.DefaultEditor, c.editor)
			}
		})
	}
}

func TestPrintCleanConfig_DocumentsOptionalEditors(t *testing.T) {
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

	for _, editor := range []string{"nvim", "hx", "fresh"} {
		if !bytes.Contains([]byte(output), []byte(editor)) {
			t.Errorf("printCleanConfig output does not mention optional editor %q", editor)
		}
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	cases := []struct {
		name        string
		envShell    string
		envEditor   string
		envFileMgr  string
		wantShell   string
		wantEditor  string
		wantBrowser string
	}{
		{
			name:        "no env vars — config values unchanged",
			wantShell:   "zsh",
			wantEditor:  "micro",
			wantBrowser: "rovr",
		},
		{
			name:        "AGENTJAIL_SHELL overrides shell",
			envShell:    "bash",
			wantShell:   "bash",
			wantEditor:  "micro",
			wantBrowser: "rovr",
		},
		{
			name:        "AGENTJAIL_EDITOR overrides editor",
			envEditor:   "nvim",
			wantShell:   "zsh",
			wantEditor:  "nvim",
			wantBrowser: "rovr",
		},
		{
			name:        "AGENTJAIL_FILE_BROWSER overrides file browser",
			envFileMgr:  "yazi",
			wantShell:   "zsh",
			wantEditor:  "micro",
			wantBrowser: "yazi",
		},
		{
			name:        "all three env vars set",
			envShell:    "bash",
			envEditor:   "hx",
			envFileMgr:  "ranger",
			wantShell:   "bash",
			wantEditor:  "hx",
			wantBrowser: "ranger",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setOrUnset := func(key, val string) {
				if val != "" {
					t.Setenv(key, val)
				} else {
					t.Setenv(key, "")
					os.Unsetenv(key)
				}
			}
			setOrUnset("AGENTJAIL_SHELL", c.envShell)
			setOrUnset("AGENTJAIL_EDITOR", c.envEditor)
			setOrUnset("AGENTJAIL_FILE_BROWSER", c.envFileMgr)

			cfg := &GlobalConfig{
				DefaultShell:  "zsh",
				DefaultEditor: "micro",
				FileBrowser:   "rovr",
			}
			cfg.applyEnvOverrides()

			if cfg.DefaultShell != c.wantShell {
				t.Errorf("DefaultShell: got %q, want %q", cfg.DefaultShell, c.wantShell)
			}
			if cfg.DefaultEditor != c.wantEditor {
				t.Errorf("DefaultEditor: got %q, want %q", cfg.DefaultEditor, c.wantEditor)
			}
			if cfg.FileBrowser != c.wantBrowser {
				t.Errorf("FileBrowser: got %q, want %q", cfg.FileBrowser, c.wantBrowser)
			}
		})
	}
}

func TestLoadGlobalConfigFromPath_InjectGhAuthTokenDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `
default_editor: micro
default_shell: zsh
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadGlobalConfigFromPath(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.InjectGhAuthToken {
		t.Error("InjectGhAuthToken: expected false (default)")
	}
}

func TestLoadGlobalConfigFromPath_ClaudeAppendSystemPrompt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
default_editor: micro
default_shell: zsh
claude_append_system_prompt: "Always write tests before code."
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadGlobalConfigFromPath(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ClaudeAppendSystemPrompt != "Always write tests before code." {
		t.Errorf("ClaudeAppendSystemPrompt: got %q, want %q",
			cfg.ClaudeAppendSystemPrompt, "Always write tests before code.")
	}
}

func TestPrintCleanConfig_DocumentsClaudeAppendSystemPrompt(t *testing.T) {
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
	if !bytes.Contains(buf.Bytes(), []byte("claude_append_system_prompt")) {
		t.Error("printCleanConfig output missing claude_append_system_prompt key")
	}
}

func TestLoadGlobalConfigFromPath_ClaudeMCPServers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
agent_frameworks:
  claude:
    enabled: true
    mcp_servers:
      - name: context7
        command: npx
        args: ["-y", "@upstash/context7-mcp"]
        env:
          DEFAULT_MINIMUM_TOKENS: "10000"
      - name: filesystem
        command: npx
        args: ["-y", "@modelcontextprotocol/server-filesystem", "/project"]
        type: stdio
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadGlobalConfigFromPath(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.AgentFrameworks.ClaudeCode.Enabled {
		t.Error("ClaudeCode.Enabled: expected true")
	}
	servers := cfg.AgentFrameworks.ClaudeCode.MCPServers
	if len(servers) != 2 {
		t.Fatalf("expected 2 MCP servers, got %d", len(servers))
	}
	if servers[0].Name != "context7" {
		t.Errorf("servers[0].Name: got %q, want %q", servers[0].Name, "context7")
	}
	if servers[0].Command != "npx" {
		t.Errorf("servers[0].Command: got %q", servers[0].Command)
	}
	if servers[0].Env["DEFAULT_MINIMUM_TOKENS"] != "10000" {
		t.Errorf("servers[0].Env: got %v", servers[0].Env)
	}
	if servers[1].Name != "filesystem" {
		t.Errorf("servers[1].Name: got %q", servers[1].Name)
	}
	if servers[1].Type != "stdio" {
		t.Errorf("servers[1].Type: got %q, want %q", servers[1].Type, "stdio")
	}
}

func TestLoadGlobalConfigFromPath_ClaudeHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
agent_frameworks:
  claude:
    enabled: true
    hooks:
      - event: PreToolUse
        matcher: "Bash"
        command: "echo pre"
      - event: PostToolUse
        command: "echo post"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadGlobalConfigFromPath(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hooks := cfg.AgentFrameworks.ClaudeCode.Hooks
	if len(hooks) != 2 {
		t.Fatalf("expected 2 hooks, got %d", len(hooks))
	}
	if hooks[0].Event != "PreToolUse" {
		t.Errorf("hooks[0].Event: got %q", hooks[0].Event)
	}
	if hooks[0].Matcher != "Bash" {
		t.Errorf("hooks[0].Matcher: got %q", hooks[0].Matcher)
	}
	if hooks[1].Event != "PostToolUse" {
		t.Errorf("hooks[1].Event: got %q", hooks[1].Event)
	}
	if hooks[1].Matcher != "" {
		t.Errorf("hooks[1].Matcher: expected empty, got %q", hooks[1].Matcher)
	}
}

func TestLoadGlobalConfigFromPath_ClaudeOldPluginsFieldIgnored(t *testing.T) {
	// Old configs with plugins: [...] under claude: should load without error,
	// and MCPServers/Hooks should be empty (the old field is silently ignored).
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
agent_frameworks:
  claude:
    enabled: true
    plugins:
      - some-old-plugin
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadGlobalConfigFromPath(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.AgentFrameworks.ClaudeCode.Enabled {
		t.Error("ClaudeCode.Enabled: expected true")
	}
	if len(cfg.AgentFrameworks.ClaudeCode.MCPServers) != 0 {
		t.Errorf("expected no MCPServers, got %v", cfg.AgentFrameworks.ClaudeCode.MCPServers)
	}
	if len(cfg.AgentFrameworks.ClaudeCode.Hooks) != 0 {
		t.Errorf("expected no Hooks, got %v", cfg.AgentFrameworks.ClaudeCode.Hooks)
	}
}

func TestPrintCleanConfig_DocumentsClaudePlugins(t *testing.T) {
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
	for _, key := range []string{"mcp_servers", "hooks"} {
		if !bytes.Contains([]byte(output), []byte(key)) {
			t.Errorf("printCleanConfig output missing %q (Claude plugin docs)", key)
		}
	}
}
