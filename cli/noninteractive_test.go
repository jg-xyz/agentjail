package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNonInteractiveExecArgs_Basic(t *testing.T) {
	got := nonInteractiveExecArgs("agentjail.myproj", nil)
	want := []string{"exec", "-i", "agentjail.myproj", "claude"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNonInteractiveExecArgs_WithArgs(t *testing.T) {
	got := nonInteractiveExecArgs("agentjail.myproj", []string{"--debug", "--model", "claude-opus-4-5"})
	want := []string{"exec", "-i", "agentjail.myproj", "claude", "--debug", "--model", "claude-opus-4-5"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAdaptRunArgsForNonInteractive_RemovesTty(t *testing.T) {
	in := []string{"run", "-it", "--rm", "agentjail"}
	got := adaptRunArgsForNonInteractive(in, "")
	for _, arg := range got {
		if arg == "-it" {
			t.Errorf("expected -it to be removed, got %v", got)
		}
	}
	found := false
	for _, arg := range got {
		if arg == "-i" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected -i to be present, got %v", got)
	}
}

// TestAdaptRunArgsForNonInteractive_RemovesName verifies the legacy behaviour
// (empty niContainerName) still drops --name entirely.
func TestAdaptRunArgsForNonInteractive_RemovesName(t *testing.T) {
	in := []string{"run", "-it", "--rm", "--name", "agentjail.myproj", "--hostname", "agentjail", "agentjail"}
	got := adaptRunArgsForNonInteractive(in, "")
	for i, arg := range got {
		if arg == "--name" {
			t.Errorf("expected --name to be removed, found at index %d in %v", i, got)
		}
		if arg == "agentjail.myproj" {
			t.Errorf("expected container name value to be removed, found at index %d in %v", i, got)
		}
	}
}

// TestAdaptRunArgsForNonInteractive_ReplacesName verifies that when a
// niContainerName is provided the --name flag is kept and its value replaced.
func TestAdaptRunArgsForNonInteractive_ReplacesName(t *testing.T) {
	in := []string{"run", "-it", "--rm", "--name", "agentjail.myproj", "--hostname", "agentjail", "agentjail"}
	got := adaptRunArgsForNonInteractive(in, "agentjail-ni.mypro")
	want := []string{"run", "-i", "--rm", "--name", "agentjail-ni.mypro", "--hostname", "agentjail", "agentjail"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAdaptRunArgsForNonInteractive_DoesNotMutateInput(t *testing.T) {
	in := []string{"run", "-it", "--rm", "--name", "agentjail.myproj", "agentjail"}
	original := make([]string, len(in))
	copy(original, in)

	adaptRunArgsForNonInteractive(in, "")

	if !reflect.DeepEqual(in, original) {
		t.Errorf("input slice was mutated: got %v, want %v", in, original)
	}
}

func TestAdaptRunArgsForNonInteractive_PreservesOtherArgs(t *testing.T) {
	in := []string{"run", "-it", "--rm", "--name", "agentjail.myproj", "--hostname", "agentjail", "-v", "/host:/project", "agentjail"}
	got := adaptRunArgsForNonInteractive(in, "")
	want := []string{"run", "-i", "--rm", "--hostname", "agentjail", "-v", "/host:/project", "agentjail"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestTryNILock_Acquisition(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.lock")

	f, ok := tryNILock(path)
	if !ok {
		t.Fatal("expected to acquire lock, got false")
	}
	if f == nil {
		t.Fatal("expected non-nil file on successful acquisition")
	}
	releaseNILock(f)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected lock file to be removed after release")
	}
}

func TestTryNILock_Contention(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.lock")

	f, ok := tryNILock(path)
	if !ok {
		t.Fatal("expected first acquisition to succeed")
	}
	defer releaseNILock(f)

	_, ok2 := tryNILock(path)
	if ok2 {
		t.Error("expected second acquisition to fail while lock is held")
	}
}

func TestTryNILock_StaleLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.lock")

	// Create a stale lock file with a very old modification time.
	staleFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		t.Fatalf("failed to create stale lock: %v", err)
	}
	staleFile.Close()
	oldTime := time.Now().Add(-120 * time.Second)
	if err := os.Chtimes(path, oldTime, oldTime); err != nil {
		t.Fatalf("failed to backdate lock file: %v", err)
	}

	f, ok := tryNILock(path)
	if !ok {
		t.Fatal("expected stale lock to be removed and new lock acquired")
	}
	releaseNILock(f)
}

func TestBuildNIClaudeArgs_NoDockerSetup(t *testing.T) {
	got := buildNIClaudeArgs("", "myprompt", nil)
	want := []string{"claude", "--append-system-prompt", "myprompt"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestBuildNIClaudeArgs_NoDockerSetup_WithExtraArgs(t *testing.T) {
	got := buildNIClaudeArgs("", "myprompt", []string{"--debug"})
	want := []string{"claude", "--append-system-prompt", "myprompt", "--debug"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestBuildNIClaudeArgs_WithDockerSetup(t *testing.T) {
	setup := "cp -r /tmp/.claude /root/.claude; "
	got := buildNIClaudeArgs(setup, "myprompt", nil)
	if len(got) < 5 {
		t.Fatalf("expected at least 5 args, got %v", got)
	}
	if got[0] != "-e" {
		t.Errorf("first arg should be -e, got %q", got[0])
	}
	if got[1] != "_AGENTJAIL_SYS=myprompt" {
		t.Errorf("second arg should be _AGENTJAIL_SYS=myprompt, got %q", got[1])
	}
	if got[2] != "sh" || got[3] != "-c" {
		t.Errorf("expected sh -c, got %v", got[2:4])
	}
	script := got[4]
	if !strings.Contains(script, setup) {
		t.Errorf("script missing dockerSetup, got %q", script)
	}
	if !strings.Contains(script, `exec claude --append-system-prompt "$_AGENTJAIL_SYS"`) {
		t.Errorf("script missing exec claude invocation, got %q", script)
	}
}

func TestBuildNIClaudeArgs_WithDockerSetup_WithExtraArgs(t *testing.T) {
	setup := "cp -r /tmp/.claude /root/.claude; "
	got := buildNIClaudeArgs(setup, "myprompt", []string{"--model", "opus"})
	script := got[len(got)-1]
	if !strings.Contains(script, "--model opus") {
		t.Errorf("script missing extra args, got %q", script)
	}
}
