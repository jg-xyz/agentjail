package main

import (
	"encoding/json"
	"testing"
)

func TestGenerateClaudeSettingsJSON_NilWhenEmpty(t *testing.T) {
	cfg := ClaudeFrameworkConfig{Enabled: true}
	data, err := generateClaudeSettingsJSON(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Fatalf("expected nil when no plugins configured, got: %s", data)
	}
}

func TestGenerateClaudeSettingsJSON_NilWhenEmptySlices(t *testing.T) {
	cfg := ClaudeFrameworkConfig{
		Enabled:    true,
		MCPServers: []MCPServer{},
		Hooks:      []ClaudeHook{},
	}
	data, err := generateClaudeSettingsJSON(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Fatalf("expected nil for empty slices, got: %s", data)
	}
}

func TestGenerateClaudeSettingsJSON_MCPServersOnly(t *testing.T) {
	cfg := ClaudeFrameworkConfig{
		Enabled: true,
		MCPServers: []MCPServer{
			{
				Name:    "context7",
				Command: "npx",
				Args:    []string{"-y", "@upstash/context7-mcp"},
				Env:     map[string]string{"TOKEN": "abc"},
			},
		},
	}
	data, err := generateClaudeSettingsJSON(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data == nil {
		t.Fatal("expected non-nil JSON")
	}

	var out claudeSettingsFile
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	srv, ok := out.MCPServers["context7"]
	if !ok {
		t.Fatal("expected 'context7' key in mcpServers")
	}
	if srv.Command != "npx" {
		t.Errorf("command: got %q, want %q", srv.Command, "npx")
	}
	if len(srv.Args) != 2 || srv.Args[0] != "-y" {
		t.Errorf("args: got %v", srv.Args)
	}
	if srv.Env["TOKEN"] != "abc" {
		t.Errorf("env TOKEN: got %q", srv.Env["TOKEN"])
	}
	if out.Hooks != nil {
		t.Errorf("expected no hooks key, got: %v", out.Hooks)
	}
}

func TestGenerateClaudeSettingsJSON_MultipleServers(t *testing.T) {
	cfg := ClaudeFrameworkConfig{
		Enabled: true,
		MCPServers: []MCPServer{
			{Name: "server-a", Command: "cmd-a"},
			{Name: "server-b", Command: "cmd-b"},
		},
	}
	data, err := generateClaudeSettingsJSON(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out claudeSettingsFile
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(out.MCPServers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(out.MCPServers))
	}
	if out.MCPServers["server-a"].Command != "cmd-a" {
		t.Errorf("server-a command wrong")
	}
	if out.MCPServers["server-b"].Command != "cmd-b" {
		t.Errorf("server-b command wrong")
	}
}

func TestGenerateClaudeSettingsJSON_DuplicateServerName(t *testing.T) {
	cfg := ClaudeFrameworkConfig{
		Enabled: true,
		MCPServers: []MCPServer{
			{Name: "dup", Command: "cmd1"},
			{Name: "dup", Command: "cmd2"},
		},
	}
	_, err := generateClaudeSettingsJSON(cfg)
	if err == nil {
		t.Fatal("expected error for duplicate server name")
	}
}

func TestGenerateClaudeSettingsJSON_MissingServerName(t *testing.T) {
	cfg := ClaudeFrameworkConfig{
		Enabled:    true,
		MCPServers: []MCPServer{{Command: "npx"}},
	}
	_, err := generateClaudeSettingsJSON(cfg)
	if err == nil {
		t.Fatal("expected error for missing server name")
	}
}

func TestGenerateClaudeSettingsJSON_HooksOnly(t *testing.T) {
	cfg := ClaudeFrameworkConfig{
		Enabled: true,
		Hooks: []ClaudeHook{
			{Event: "PreToolUse", Matcher: "Bash", Command: "echo hi"},
		},
	}
	data, err := generateClaudeSettingsJSON(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data == nil {
		t.Fatal("expected non-nil JSON")
	}

	var out claudeSettingsFile
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if out.MCPServers != nil {
		t.Errorf("expected no mcpServers key")
	}
	groups, ok := out.Hooks["PreToolUse"]
	if !ok {
		t.Fatal("expected PreToolUse in hooks")
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Matcher != "Bash" {
		t.Errorf("matcher: got %q, want %q", groups[0].Matcher, "Bash")
	}
	if len(groups[0].Hooks) != 1 || groups[0].Hooks[0].Command != "echo hi" {
		t.Errorf("hook command wrong: %v", groups[0].Hooks)
	}
	if groups[0].Hooks[0].Type != "command" {
		t.Errorf("hook type: got %q, want %q", groups[0].Hooks[0].Type, "command")
	}
}

func TestGenerateClaudeSettingsJSON_AllHookEvents(t *testing.T) {
	cfg := ClaudeFrameworkConfig{
		Enabled: true,
		Hooks: []ClaudeHook{
			{Event: "PreToolUse", Command: "pre"},
			{Event: "PostToolUse", Command: "post"},
			{Event: "Notification", Command: "notify"},
			{Event: "Stop", Command: "stop"},
		},
	}
	data, err := generateClaudeSettingsJSON(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out claudeSettingsFile
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, ev := range []string{"PreToolUse", "PostToolUse", "Notification", "Stop"} {
		if _, ok := out.Hooks[ev]; !ok {
			t.Errorf("missing hook event %q", ev)
		}
	}
}

func TestGenerateClaudeSettingsJSON_MCPAndHooks(t *testing.T) {
	cfg := ClaudeFrameworkConfig{
		Enabled: true,
		MCPServers: []MCPServer{
			{Name: "fs", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-filesystem"}},
		},
		Hooks: []ClaudeHook{
			{Event: "PreToolUse", Matcher: "Bash", Command: "logger.sh"},
		},
	}
	data, err := generateClaudeSettingsJSON(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out claudeSettingsFile
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(out.MCPServers) != 1 {
		t.Errorf("expected 1 MCP server, got %d", len(out.MCPServers))
	}
	if len(out.Hooks) != 1 {
		t.Errorf("expected 1 hook event, got %d", len(out.Hooks))
	}
}

func TestGenerateClaudeSettingsJSON_OmitemptyFields(t *testing.T) {
	cfg := ClaudeFrameworkConfig{
		Enabled: true,
		MCPServers: []MCPServer{
			{Name: "minimal", Command: "myserver"},
			// no Args, Env, or Type
		},
	}
	data, err := generateClaudeSettingsJSON(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the JSON does not contain "args", "env", or "type" keys.
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	servers := raw["mcpServers"].(map[string]interface{})
	srv := servers["minimal"].(map[string]interface{})
	for _, key := range []string{"args", "env", "type"} {
		if _, found := srv[key]; found {
			t.Errorf("expected %q to be omitted when empty, but it was present", key)
		}
	}
}

func TestGenerateClaudeSettingsJSON_MissingHookEvent(t *testing.T) {
	cfg := ClaudeFrameworkConfig{
		Enabled: true,
		Hooks:   []ClaudeHook{{Command: "echo"}},
	}
	_, err := generateClaudeSettingsJSON(cfg)
	if err == nil {
		t.Fatal("expected error for missing hook event")
	}
}

func TestGenerateClaudeSettingsJSON_MissingHookCommand(t *testing.T) {
	cfg := ClaudeFrameworkConfig{
		Enabled: true,
		Hooks:   []ClaudeHook{{Event: "PreToolUse"}},
	}
	_, err := generateClaudeSettingsJSON(cfg)
	if err == nil {
		t.Fatal("expected error for missing hook command")
	}
}

func TestGenerateClaudeSettingsJSON_MultipleHooksSameEventAndMatcher(t *testing.T) {
	// Two hooks with the same event+matcher should be merged into one group.
	cfg := ClaudeFrameworkConfig{
		Enabled: true,
		Hooks: []ClaudeHook{
			{Event: "PreToolUse", Matcher: "Bash", Command: "cmd1"},
			{Event: "PreToolUse", Matcher: "Bash", Command: "cmd2"},
		},
	}
	data, err := generateClaudeSettingsJSON(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out claudeSettingsFile
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	groups := out.Hooks["PreToolUse"]
	if len(groups) != 1 {
		t.Fatalf("expected 1 group (merged), got %d", len(groups))
	}
	if len(groups[0].Hooks) != 2 {
		t.Fatalf("expected 2 hook actions in merged group, got %d", len(groups[0].Hooks))
	}
}
