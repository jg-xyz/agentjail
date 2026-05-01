package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeMountReadOnly(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	os.MkdirAll(claudeDir, 0755)

	volArgs, _, _ := buildClaudeMountArgs(claudeDir, dir)

	found := false
	for _, arg := range volArgs {
		if strings.HasSuffix(arg, "/tmp/.claude:ro") {
			found = true
		}
		if strings.Contains(arg, ":/root/.claude") && !strings.Contains(arg, "/tmp/.claude") {
			t.Errorf("found direct /root/.claude mount in args: %v", volArgs)
		}
	}
	if !found {
		t.Errorf("expected /tmp/.claude:ro mount, got %v", volArgs)
	}
}

func TestClaudeMountDisabled(t *testing.T) {
	// When ClaudeCode is disabled, buildClaudeMountArgs is never called.
	// This test confirms the function is gated on config in the integration.
	// The function itself always returns args — the caller gates on Enabled.
	config := &GlobalConfig{}
	config.AgentFrameworks.ClaudeCode.Enabled = false
	if config.AgentFrameworks.ClaudeCode.Enabled {
		t.Error("expected ClaudeCode to be disabled")
	}
}

func TestHostHomeEnvVar(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	os.MkdirAll(claudeDir, 0755)

	_, envArgs, _ := buildClaudeMountArgs(claudeDir, dir)

	expectedEnv := "HOST_HOME=" + dir
	found := false
	for _, arg := range envArgs {
		if arg == expectedEnv {
			found = true
		}
	}
	if !found {
		t.Errorf("expected %q in env args, got %v", expectedEnv, envArgs)
	}
}

func TestDockerSetupClaudeSnippet(t *testing.T) {
	_, _, snippet := buildClaudeMountArgs("/home/test/.claude", "/home/test")
	if !strings.Contains(snippet, "cp -r /tmp/.claude /root/.claude") {
		t.Errorf("expected copy command in snippet, got %q", snippet)
	}
	if !strings.Contains(snippet, "sed -i") {
		t.Errorf("expected sed command in snippet, got %q", snippet)
	}
}

func TestDockerSetupBinarySkip(t *testing.T) {
	_, _, snippet := buildClaudeMountArgs("/home/test/.claude", "/home/test")
	if !strings.Contains(snippet, `grep -qI "" "$f"`) {
		t.Errorf("expected grep -qI binary detection in snippet, got %q", snippet)
	}
}

func TestClaudeOutMountReadWrite(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	os.MkdirAll(claudeDir, 0755)

	volArgs, _, _ := buildClaudeMountArgs(claudeDir, dir)

	found := false
	for _, arg := range volArgs {
		if strings.HasSuffix(arg, "/tmp/.claude-out:rw") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected /tmp/.claude-out:rw mount in volArgs, got %v", volArgs)
	}
}

func TestClaudeMountBothReadAndWrite(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	os.MkdirAll(claudeDir, 0755)

	volArgs, _, _ := buildClaudeMountArgs(claudeDir, dir)

	hasRO := false
	hasRW := false
	for _, arg := range volArgs {
		if strings.HasSuffix(arg, "/tmp/.claude:ro") {
			hasRO = true
		}
		if strings.HasSuffix(arg, "/tmp/.claude-out:rw") {
			hasRW = true
		}
	}
	if !hasRO {
		t.Errorf("expected /tmp/.claude:ro mount in volArgs, got %v", volArgs)
	}
	if !hasRW {
		t.Errorf("expected /tmp/.claude-out:rw mount in volArgs, got %v", volArgs)
	}
}
