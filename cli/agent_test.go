package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestEnabledAgents_NoneEnabled(t *testing.T) {
	cfg := &GlobalConfig{}
	agents := enabledAgents(cfg)
	if len(agents) != 0 {
		t.Errorf("expected 0 agents, got %d: %v", len(agents), agents)
	}
}

func TestEnabledAgents_CopilotOnly(t *testing.T) {
	cfg := &GlobalConfig{
		AgentFrameworks: AgentFrameworksConfig{
			Copilot: FrameworkConfig{Enabled: true},
		},
	}
	agents := enabledAgents(cfg)
	if len(agents) != 1 || agents[0] != "copilot" {
		t.Errorf("expected [copilot], got %v", agents)
	}
}

func TestEnabledAgents_OpenCodeOnly(t *testing.T) {
	cfg := &GlobalConfig{
		AgentFrameworks: AgentFrameworksConfig{
			OpenCode: FrameworkConfig{Enabled: true},
		},
	}
	agents := enabledAgents(cfg)
	if len(agents) != 1 || agents[0] != "opencode" {
		t.Errorf("expected [opencode], got %v", agents)
	}
}

func TestEnabledAgents_BothEnabled(t *testing.T) {
	cfg := &GlobalConfig{
		AgentFrameworks: AgentFrameworksConfig{
			Copilot:  FrameworkConfig{Enabled: true},
			OpenCode: FrameworkConfig{Enabled: true},
		},
	}
	agents := enabledAgents(cfg)
	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %d: %v", len(agents), agents)
	}
	// copilot should be listed before opencode (order from enabledAgents)
	if agents[0] != "copilot" || agents[1] != "opencode" {
		t.Errorf("unexpected order: %v", agents)
	}
}

func TestEnabledAgents_ClaudeCodeOnly(t *testing.T) {
	cfg := &GlobalConfig{
		AgentFrameworks: AgentFrameworksConfig{
			ClaudeCode: ClaudeFrameworkConfig{Enabled: true},
		},
	}
	agents := enabledAgents(cfg)
	if len(agents) != 1 || agents[0] != "claude" {
		t.Errorf("expected [claude], got %v", agents)
	}
}

func TestEnabledAgents_AllThreeEnabled(t *testing.T) {
	cfg := &GlobalConfig{
		AgentFrameworks: AgentFrameworksConfig{
			Copilot:    FrameworkConfig{Enabled: true},
			OpenCode:   FrameworkConfig{Enabled: true},
			ClaudeCode: ClaudeFrameworkConfig{Enabled: true},
		},
	}
	agents := enabledAgents(cfg)
	if len(agents) != 3 {
		t.Errorf("expected 3 agents, got %d: %v", len(agents), agents)
	}
	if agents[0] != "copilot" || agents[1] != "opencode" || agents[2] != "claude" {
		t.Errorf("unexpected order: %v", agents)
	}
}

func TestAgentCommand_Known(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"opencode", "opencode"},
		{"copilot", "copilot"},
	}
	for _, tt := range tests {
		if got := agentCommand(tt.input, ""); got != tt.expected {
			t.Errorf("agentCommand(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestAgentCommand_Claude(t *testing.T) {
	got := agentCommand("claude", "")
	if !strings.HasPrefix(got, "claude --append-system-prompt ") {
		t.Errorf("agentCommand(claude) should start with 'claude --append-system-prompt', got %q", got)
	}
	if !strings.Contains(got, "rg") || !strings.Contains(got, "fd") {
		t.Errorf("agentCommand(claude) system prompt missing expected tools, got %q", got)
	}
}

func TestAgentCommand_ClaudeExtraContext(t *testing.T) {
	got := agentCommand("claude", "Always write tests.")
	if !strings.Contains(got, "Always write tests.") {
		t.Errorf("agentCommand(claude, extra) should contain extra context, got %q", got)
	}
	// Both the base tools list and the extra context must appear.
	if !strings.Contains(got, "rg") {
		t.Errorf("agentCommand(claude, extra) should still contain base tools, got %q", got)
	}
	// The two sections must be separated by a blank line.
	if !strings.Contains(got, "\n\n") {
		t.Errorf("agentCommand(claude, extra) base prompt and extra context should be separated by blank line, got %q", got)
	}
}

func TestResolveClaudeContext(t *testing.T) {
	cases := []struct {
		configVal string
		flagVal   string
		want      string
	}{
		{"", "", ""},
		{"from config", "", "from config"},
		{"", "from flag", "from flag"},
		{"from config", "from flag", "from config\n\nfrom flag"},
	}
	for _, tt := range cases {
		got := resolveClaudeContext(tt.configVal, tt.flagVal)
		if got != tt.want {
			t.Errorf("resolveClaudeContext(%q, %q) = %q, want %q", tt.configVal, tt.flagVal, got, tt.want)
		}
	}
}

func TestAgentCommand_Unknown(t *testing.T) {
	// Unknown names pass through unchanged
	if got := agentCommand("mytool", ""); got != "mytool" {
		t.Errorf("agentCommand(unknown) = %q, want %q", got, "mytool")
	}
}

func TestChooseEnabledAgent_NoAgents(t *testing.T) {
	cfg := &GlobalConfig{}
	if got := chooseEnabledAgent(cfg); got != "" {
		t.Errorf("expected empty string for no agents, got %q", got)
	}
}

func TestChooseEnabledAgent_SingleAgent(t *testing.T) {
	cfg := &GlobalConfig{
		AgentFrameworks: AgentFrameworksConfig{
			Copilot: FrameworkConfig{Enabled: true},
		},
	}
	if got := chooseEnabledAgent(cfg); got != "copilot" {
		t.Errorf("expected copilot, got %q", got)
	}
}

func withStdin(t *testing.T, input string, fn func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(w, input)
	w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()
	fn()
}

func multiAgentCfg() *GlobalConfig {
	return &GlobalConfig{
		AgentFrameworks: AgentFrameworksConfig{
			Copilot:  FrameworkConfig{Enabled: true},
			OpenCode: FrameworkConfig{Enabled: true},
		},
	}
}

func TestChooseEnabledAgent_MultipleAgents_ValidChoice(t *testing.T) {
	var got string
	withStdin(t, "2\n", func() {
		got = chooseEnabledAgent(multiAgentCfg())
	})
	if got != "opencode" {
		t.Errorf("expected opencode (choice 2), got %q", got)
	}
}

func TestChooseEnabledAgent_MultipleAgents_OutOfRange(t *testing.T) {
	var got string
	withStdin(t, "99\n", func() {
		got = chooseEnabledAgent(multiAgentCfg())
	})
	if got != "copilot" {
		t.Errorf("expected fallback to copilot for out-of-range input, got %q", got)
	}
}

func TestChooseEnabledAgent_MultipleAgents_NonNumeric(t *testing.T) {
	var got string
	withStdin(t, "abc\n", func() {
		got = chooseEnabledAgent(multiAgentCfg())
	})
	if got != "copilot" {
		t.Errorf("expected fallback to copilot for non-numeric input, got %q", got)
	}
}

func TestChooseEnabledAgent_MultipleAgents_EOF(t *testing.T) {
	var got string
	withStdin(t, "", func() {
		got = chooseEnabledAgent(multiAgentCfg())
	})
	if got != "copilot" {
		t.Errorf("expected fallback to copilot on EOF, got %q", got)
	}
}

func TestAgentCommand_ClaudeCodeAlias(t *testing.T) {
	got := agentCommand("claude_code", "")
	if !strings.HasPrefix(got, "claude --append-system-prompt ") {
		t.Errorf("agentCommand(claude_code) should start with 'claude --append-system-prompt', got %q", got)
	}
}
func TestResolveClaudeContext_AllThreeSources(t *testing.T) {
	// Simulates profile prompt prepended in main.go, then config+flag via resolveClaudeContext.
	profilePrompt := "profile rules"
	configVal := "team conventions"
	flagVal := "session note"

	extra := resolveClaudeContext(configVal, flagVal)
	result := profilePrompt + "\n\n" + extra

	want := "profile rules\n\nteam conventions\n\nsession note"
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}
