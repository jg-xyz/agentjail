package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeMountReadWrite(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	os.MkdirAll(claudeDir, 0755)

	volArgs, _, _ := buildClaudeMountArgs(claudeDir, dir)

	found := false
	for _, arg := range volArgs {
		if strings.HasSuffix(arg, "/tmp/.claude:rw") {
			found = true
		}
		if strings.Contains(arg, ":/root/.claude") {
			t.Errorf("found direct /root/.claude mount in args: %v", volArgs)
		}
	}
	if !found {
		t.Errorf("expected /tmp/.claude:rw mount, got %v", volArgs)
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

func TestClaudeSingleMount(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	os.MkdirAll(claudeDir, 0755)

	volArgs, _, _ := buildClaudeMountArgs(claudeDir, dir)

	// Expect exactly one -v flag and one mount spec (two elements total).
	if len(volArgs) != 2 {
		t.Errorf("expected exactly one mount (-v <spec>), got %v", volArgs)
	}
}

func TestClaudeMountContainsHostPath(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	os.MkdirAll(claudeDir, 0755)

	volArgs, _, _ := buildClaudeMountArgs(claudeDir, dir)

	found := false
	for _, arg := range volArgs {
		if strings.HasPrefix(arg, claudeDir+":") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected host claude path %q in mount spec, got %v", claudeDir, volArgs)
	}
}
