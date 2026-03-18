package main

import "testing"

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

func TestAgentCommand_Known(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"opencode", "opencode"},
		{"copilot", "copilot"},
	}
	for _, tt := range tests {
		if got := agentCommand(tt.input); got != tt.expected {
			t.Errorf("agentCommand(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestAgentCommand_Unknown(t *testing.T) {
	// Unknown names pass through unchanged
	if got := agentCommand("mytool"); got != "mytool" {
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
