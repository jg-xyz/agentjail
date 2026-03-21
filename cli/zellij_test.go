package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellEscape(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"copilot", "'copilot'"},
		{"opencode", "'opencode'"},
		{"claude", "'claude'"},
		// single quote inside input must be safely escaped
		{"it's", "'it'\"'\"'s'"},
		// empty string
		{"", "''"},
		// spaces are preserved inside single quotes
		{"gh copilot-in-the-shell", "'gh copilot-in-the-shell'"},
		// multiple consecutive single quotes
		{"a''b", "'a'\"'\"''\"'\"'b'"},
	}
	for _, c := range cases {
		got := shellEscape(c.input)
		if got != c.want {
			t.Errorf("shellEscape(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestSanitizeKDLString(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"copilot", "copilot"},
		{`tab"name`, "tabname"},
		{"line\nnewline", "linenewline"},
		{"line\r\nnewline", "linenewline"},
		{"normal-tab-name", "normal-tab-name"},
		// tab character is not in the strip list and should be preserved
		{"tab\there", "tab\there"},
		// multiple problematic chars
		{"a\"b\nc\"d", "abcd"},
	}
	for _, c := range cases {
		got := sanitizeKDLString(c.input)
		if got != c.want {
			t.Errorf("sanitizeKDLString(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestWriteZellijFiles(t *testing.T) {
	dir := t.TempDir()

	t.Run("with agent", func(t *testing.T) {
		if err := writeZellijFiles(dir, "myproject", "tokyo-night-storm", "copilot", "copilot", "rovr", "zsh"); err != nil {
			t.Fatalf("writeZellijFiles: %v", err)
		}

		// layout.kdl should exist and name the agent tab correctly
		layout := readFile(t, filepath.Join(dir, "zellij", "layout.kdl"))

		// config.kdl should contain the theme
		config := readFile(t, filepath.Join(dir, "zellij", "config.kdl"))
		if !strings.Contains(config, `theme "tokyo-night-storm"`) {
			t.Errorf("config.kdl missing theme; got:\n%s", config)
		}
		if !strings.Contains(layout, `tab name="copilot"`) {
			t.Errorf("layout missing agent tab name; got:\n%s", layout)
		}
		if !strings.Contains(layout, `tab name="terminal"`) {
			t.Error("layout missing terminal tab")
		}
		if !strings.Contains(layout, `tab name="files"`) {
			t.Error("layout missing files tab")
		}
		if !strings.Contains(layout, `command="zsh"`) {
			t.Error("layout terminal tab should use configured shell (zsh)")
		}

		// agent.sh should set AGENTJAIL_TAB_CMD
		agentSh := readFile(t, filepath.Join(dir, "zellij", "tabs", "agent.sh"))
		if !strings.Contains(agentSh, "AGENTJAIL_TAB_CMD") {
			t.Error("agent.sh missing AGENTJAIL_TAB_CMD")
		}
		if !strings.Contains(agentSh, "'copilot'") {
			t.Error("agent.sh should contain shell-escaped agent command")
		}

		// files.sh should reference the file browser command
		filesSh := readFile(t, filepath.Join(dir, "zellij", "tabs", "files.sh"))
		if !strings.Contains(filesSh, "'rovr'") {
			t.Error("files.sh should contain shell-escaped files command")
		}

		// scripts must be executable
		assertExecutable(t, filepath.Join(dir, "zellij", "tabs", "agent.sh"))
		assertExecutable(t, filepath.Join(dir, "zellij", "tabs", "files.sh"))
	})

	t.Run("no agent configured", func(t *testing.T) {
		dir2 := t.TempDir()
		if err := writeZellijFiles(dir2, "testproj", "gruvbox-dark", "", "", "rovr", "bash"); err != nil {
			t.Fatalf("writeZellijFiles: %v", err)
		}
		layout := readFile(t, filepath.Join(dir2, "zellij", "layout.kdl"))
		if !strings.Contains(layout, `tab name="shell"`) {
			t.Errorf("expected fallback tab name 'shell'; got:\n%s", layout)
		}
		// agent.sh should just exec the shell, no AGENTJAIL_TAB_CMD
		agentSh := readFile(t, filepath.Join(dir2, "zellij", "tabs", "agent.sh"))
		if strings.Contains(agentSh, "AGENTJAIL_TAB_CMD") {
			t.Error("agent.sh should not set AGENTJAIL_TAB_CMD when no agent is configured")
		}
		// terminal tab should use bash
		if !strings.Contains(layout, `command="bash"`) {
			t.Errorf("terminal tab should use configured shell 'bash'; got:\n%s", layout)
		}
	})

	t.Run("agent name with special chars is sanitized", func(t *testing.T) {
		dir3 := t.TempDir()
		if err := writeZellijFiles(dir3, "proj", "tokyo-night-storm", `bad"name`, "cmd", "rovr", "zsh"); err != nil {
			t.Fatalf("writeZellijFiles: %v", err)
		}
		layout := readFile(t, filepath.Join(dir3, "zellij", "layout.kdl"))
		if strings.Contains(layout, `"bad"name"`) {
			t.Error("layout should not contain unescaped quotes from agent name")
		}
	})
}

func TestWriteZellijFiles_EmptyFilesCmd(t *testing.T) {
	dir := t.TempDir()
	if err := writeZellijFiles(dir, "myproject", "tokyo-night-storm", "copilot", "copilot", "", "zsh"); err != nil {
		t.Fatalf("writeZellijFiles with empty filesCmd: %v", err)
	}

	// files.sh should still be created even when filesCmd is empty
	filesSh := readFile(t, filepath.Join(dir, "zellij", "tabs", "files.sh"))
	if !strings.Contains(filesSh, "AGENTJAIL_TAB_CMD=''") {
		t.Errorf("files.sh should set AGENTJAIL_TAB_CMD to empty string; got:\n%s", filesSh)
	}
	assertExecutable(t, filepath.Join(dir, "zellij", "tabs", "files.sh"))
}

func TestLayoutHasNoSessionName(t *testing.T) {
	dir := t.TempDir()
	if err := writeZellijFiles(dir, "myproject", "tokyo-night-storm", "copilot", "copilot", "rovr", "zsh"); err != nil {
		t.Fatalf("writeZellijFiles: %v", err)
	}
	layout := readFile(t, filepath.Join(dir, "zellij", "layout.kdl"))
	if strings.Contains(layout, "session_name") {
		t.Errorf("layout.kdl must not contain session_name (not a valid layout node); got:\n%s", layout)
	}
}

func TestBuildZellijEntrypoint(t *testing.T) {
	got := buildZellijEntrypoint()
	if !strings.Contains(got, "--layout /root/.agentjail/zellij/layout.kdl") {
		t.Errorf("buildZellijEntrypoint: missing --layout flag; got %q", got)
	}
	if strings.Contains(got, "--session") {
		t.Errorf("buildZellijEntrypoint: must not use --session (causes 'no active session' error); got %q", got)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readFile(%s): %v", path, err)
	}
	return string(b)
}

func assertExecutable(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat(%s): %v", path, err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("%s is not executable (mode %s)", path, info.Mode())
	}
}
