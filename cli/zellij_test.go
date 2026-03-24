package main

import (
	"net/http"
	"net/http/httptest"
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
		if err := writeZellijFiles(dir, "tokyo-night-storm", "copilot", "copilot", "rovr", "zsh", nil); err != nil {
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

		// agent.sh should set AGENTJAIL_TAB_CMD and exec the configured shell
		agentSh := readFile(t, filepath.Join(dir, "zellij", "tabs", "agent.sh"))
		if !strings.Contains(agentSh, "AGENTJAIL_TAB_CMD") {
			t.Error("agent.sh missing AGENTJAIL_TAB_CMD")
		}
		if !strings.Contains(agentSh, "'copilot'") {
			t.Error("agent.sh should contain shell-escaped agent command")
		}
		if !strings.Contains(agentSh, "exec 'zsh'") {
			t.Errorf("agent.sh should exec the configured shell (zsh); got:\n%s", agentSh)
		}

		// files.sh should reference the file browser command and exec the configured shell
		filesSh := readFile(t, filepath.Join(dir, "zellij", "tabs", "files.sh"))
		if !strings.Contains(filesSh, "'rovr'") {
			t.Error("files.sh should contain shell-escaped files command")
		}
		if !strings.Contains(filesSh, "exec 'zsh'") {
			t.Errorf("files.sh should exec the configured shell (zsh); got:\n%s", filesSh)
		}

		// scripts must be executable
		assertExecutable(t, filepath.Join(dir, "zellij", "tabs", "agent.sh"))
		assertExecutable(t, filepath.Join(dir, "zellij", "tabs", "files.sh"))
	})

	t.Run("no agent configured", func(t *testing.T) {
		dir2 := t.TempDir()
		if err := writeZellijFiles(dir2, "gruvbox-dark", "", "", "rovr", "bash", nil); err != nil {
			t.Fatalf("writeZellijFiles: %v", err)
		}
		layout := readFile(t, filepath.Join(dir2, "zellij", "layout.kdl"))
		if !strings.Contains(layout, `tab name="shell"`) {
			t.Errorf("expected fallback tab name 'shell'; got:\n%s", layout)
		}
		// agent.sh should just exec the configured shell, no AGENTJAIL_TAB_CMD
		agentSh := readFile(t, filepath.Join(dir2, "zellij", "tabs", "agent.sh"))
		if strings.Contains(agentSh, "AGENTJAIL_TAB_CMD") {
			t.Error("agent.sh should not set AGENTJAIL_TAB_CMD when no agent is configured")
		}
		if !strings.Contains(agentSh, "exec 'bash'") {
			t.Errorf("agent.sh should exec the configured shell (bash); got:\n%s", agentSh)
		}
		// files.sh should also use the configured shell
		filesSh2 := readFile(t, filepath.Join(dir2, "zellij", "tabs", "files.sh"))
		if !strings.Contains(filesSh2, "exec 'bash'") {
			t.Errorf("files.sh should exec the configured shell (bash); got:\n%s", filesSh2)
		}
		// terminal tab should use bash
		if !strings.Contains(layout, `command="bash"`) {
			t.Errorf("terminal tab should use configured shell 'bash'; got:\n%s", layout)
		}
	})

	t.Run("agent name with special chars is sanitized", func(t *testing.T) {
		dir3 := t.TempDir()
		if err := writeZellijFiles(dir3, "tokyo-night-storm", `bad"name`, "cmd", "rovr", "zsh", nil); err != nil {
			t.Fatalf("writeZellijFiles: %v", err)
		}
		layout := readFile(t, filepath.Join(dir3, "zellij", "layout.kdl"))
		if strings.Contains(layout, `"bad"name"`) {
			t.Error("layout should not contain unescaped quotes from agent name")
		}
	})
}

func TestParseZellijKeybinds(t *testing.T) {
	config := `
keybinds clear-defaults=true {
    locked {
        bind "Ctrl t" { NewTab; }
        bind "Alt w" { CloseTab; }
        bind "Alt [" { GoToPreviousTab; }
        bind "Alt ]" { GoToNextTab; }
        bind "Alt q" { Quit; }
    }
}`
	binds := parseZellijKeybinds(config)
	byAction := map[string]string{}
	for _, b := range binds {
		byAction[b.Action] = b.Key
	}
	cases := map[string]string{
		"NewTab":          "Ctrl t",
		"CloseTab":        "Alt w",
		"GoToPreviousTab": "Alt [",
		"GoToNextTab":     "Alt ]",
		"Quit":            "Alt q",
	}
	for action, wantKey := range cases {
		if got := byAction[action]; got != wantKey {
			t.Errorf("action %s: got key %q, want %q", action, got, wantKey)
		}
	}
}

func TestFormatKey(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Ctrl t", "Ctrl+T"},
		{"Alt w", "Alt+W"},
		{"Alt [", "Alt+["},
		{"Alt ]", "Alt+]"},
		{"Alt q", "Alt+Q"},
		{"Ctrl Shift q", "Ctrl+Shift+Q"},
	}
	for _, c := range cases {
		if got := formatKey(c.in); got != c.want {
			t.Errorf("formatKey(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBuildHintsLine(t *testing.T) {
	binds := []zellijKeybind{
		{"Ctrl t", "NewTab"},
		{"Alt w", "CloseTab"},
		{"Alt [", "GoToPreviousTab"},
		{"Alt ]", "GoToNextTab"},
		{"Alt q", "Quit"},
	}
	line := buildHintsLine(binds)
	for _, want := range []string{"Ctrl+T", "new tab", "Alt+W", "close tab", "Alt+[", "Alt+]", "cycle tabs", "Alt+Q", "quit"} {
		if !strings.Contains(line, want) {
			t.Errorf("hints line missing %q; got:\n%s", want, line)
		}
	}
	// prev/next should be combined, not listed separately
	if strings.Contains(line, "prev tab") || strings.Contains(line, "next tab") {
		t.Errorf("prev/next tab should be combined as 'cycle tabs'; got:\n%s", line)
	}
}

func TestBuildHintsLine_OnlyPrev(t *testing.T) {
	binds := []zellijKeybind{{"Alt [", "GoToPreviousTab"}}
	line := buildHintsLine(binds)
	if !strings.Contains(line, "prev tab") {
		t.Errorf("expected 'prev tab' when only GoToPreviousTab present; got: %s", line)
	}
}

func TestBuildHintsLine_Empty(t *testing.T) {
	if got := buildHintsLine(nil); got != "" {
		t.Errorf("expected empty string for no binds; got %q", got)
	}
}

func TestHintsReflectConfig(t *testing.T) {
	// hints.sh must contain the same shortcuts that are in config.kdl
	dir := t.TempDir()
	if err := writeZellijFiles(dir, "tokyo-night-storm", "copilot", "copilot", "rovr", "zsh", nil); err != nil {
		t.Fatalf("writeZellijFiles: %v", err)
	}
	hints := readFile(t, filepath.Join(dir, "zellij", "tabs", "hints.sh"))
	for _, want := range []string{"Alt+T", "Alt+W", "Alt+[", "Alt+]", "Alt+Q"} {
		if !strings.Contains(hints, want) {
			t.Errorf("hints.sh missing key %q; got:\n%s", want, hints)
		}
	}
}

func TestWriteZellijFiles_EmptyFilesCmd(t *testing.T) {
	dir := t.TempDir()
	if err := writeZellijFiles(dir, "tokyo-night-storm", "copilot", "copilot", "", "zsh", nil); err != nil {
		t.Fatalf("writeZellijFiles with empty filesCmd: %v", err)
	}

	// files.sh should still be created even when filesCmd is empty
	filesSh := readFile(t, filepath.Join(dir, "zellij", "tabs", "files.sh"))
	if !strings.Contains(filesSh, "AGENTJAIL_TAB_CMD=''") {
		t.Errorf("files.sh should set AGENTJAIL_TAB_CMD to empty string; got:\n%s", filesSh)
	}
	assertExecutable(t, filepath.Join(dir, "zellij", "tabs", "files.sh"))
}

func TestConfigLockedModeAndKeybinds(t *testing.T) {
	dir := t.TempDir()
	if err := writeZellijFiles(dir, "tokyo-night-storm", "copilot", "copilot", "rovr", "zsh", nil); err != nil {
		t.Fatalf("writeZellijFiles: %v", err)
	}
	config := readFile(t, filepath.Join(dir, "zellij", "config.kdl"))
	if !strings.Contains(config, `default_mode "locked"`) {
		t.Errorf("config.kdl missing default_mode locked; got:\n%s", config)
	}
	if !strings.Contains(config, `"Alt q"`) {
		t.Errorf("config.kdl missing quit keybind; got:\n%s", config)
	}
	if !strings.Contains(config, `"Alt w"`) {
		t.Errorf("config.kdl missing close-tab keybind; got:\n%s", config)
	}
	if !strings.Contains(config, `"Alt t"`) {
		t.Errorf("config.kdl missing new tab keybind; got:\n%s", config)
	}
	if !strings.Contains(config, `"Alt ["`) || !strings.Contains(config, `"Alt ]"`) {
		t.Errorf("config.kdl missing tab navigation keybinds; got:\n%s", config)
	}
}

func TestLayoutHintsPane(t *testing.T) {
	dir := t.TempDir()
	if err := writeZellijFiles(dir, "tokyo-night-storm", "copilot", "copilot", "rovr", "zsh", nil); err != nil {
		t.Fatalf("writeZellijFiles: %v", err)
	}
	layout := readFile(t, filepath.Join(dir, "zellij", "layout.kdl"))
	if strings.Contains(layout, `zellij:compact-bar`) || strings.Contains(layout, `zellij:status-bar`) {
		t.Errorf("layout.kdl should not use compact-bar or status-bar plugins; got:\n%s", layout)
	}
	if !strings.Contains(layout, `hints.sh`) {
		t.Errorf("layout.kdl should use static hints pane; got:\n%s", layout)
	}

	hints := readFile(t, filepath.Join(dir, "zellij", "tabs", "hints.sh"))
	if !strings.Contains(hints, "tail -f /dev/null") {
		t.Errorf("hints.sh should sleep forever with tail; got:\n%s", hints)
	}
	assertExecutable(t, filepath.Join(dir, "zellij", "tabs", "hints.sh"))
}

func TestRenderZellijTemplate_BadPath(t *testing.T) {
	_, err := renderZellijTemplate("templates/configs/zellij/nonexistent.kdl", nil)
	if err == nil {
		t.Error("expected error for nonexistent template path")
	}
}

func TestRenderZellijTemplate_ThemeSubstitution(t *testing.T) {
	got, err := renderZellijTemplate("templates/configs/zellij/config.kdl",
		struct{ Theme string }{"gruvbox-dark"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, `theme "gruvbox-dark"`) {
		t.Errorf("theme not substituted; got:\n%s", got)
	}
	// Template placeholder must not appear in output
	if strings.Contains(got, "{{") {
		t.Errorf("unrendered template directive in output; got:\n%s", got)
	}
}

func TestBuildHintsLine_StripsSingleQuotes(t *testing.T) {
	// If a key name somehow contains a single quote it must be stripped so
	// the result is safe inside printf '...' in hints.sh.
	binds := []zellijKeybind{{"Alt '", "NewTab"}}
	line := buildHintsLine(binds)
	if strings.Contains(line, "'") {
		t.Errorf("buildHintsLine should strip single quotes; got: %s", line)
	}
}

func TestBuildZellijEntrypoint(t *testing.T) {
	t.Run("with session name", func(t *testing.T) {
		got := buildZellijEntrypoint("myproject")
		if !strings.Contains(got, "--new-session-with-layout /root/.agentjail/zellij/layout.kdl") {
			t.Errorf("missing --new-session-with-layout flag; got %q", got)
		}
		if strings.Contains(got, " --layout ") {
			t.Errorf("must use --new-session-with-layout, not --layout; got %q", got)
		}
		if !strings.Contains(got, "--session") {
			t.Errorf("missing --session flag; got %q", got)
		}
		if !strings.Contains(got, "'myproject'") {
			t.Errorf("missing session name; got %q", got)
		}
	})
	t.Run("empty session name omits flag", func(t *testing.T) {
		got := buildZellijEntrypoint("")
		if strings.Contains(got, "--session") {
			t.Errorf("--session should be omitted when name is empty; got %q", got)
		}
		if !strings.Contains(got, "--new-session-with-layout") {
			t.Errorf("missing --new-session-with-layout flag; got %q", got)
		}
	})
	t.Run("does not use exec so post-zellij chown can run", func(t *testing.T) {
		got := buildZellijEntrypoint("myproject")
		// "exec zellij" would replace the shell, making any trailing command dead code.
		// The entrypoint must not use exec so the chown wrapper in main.go fires.
		if strings.Contains(got, "exec mise") || strings.Contains(got, "exec zellij") {
			t.Errorf("entrypoint must not use exec (would prevent post-exit chown); got %q", got)
		}
	})
}

func TestBuildBottomBar_NoPlugins(t *testing.T) {
	got := buildBottomBar(nil)
	if !strings.Contains(got, `command="/root/.agentjail/zellij/tabs/hints.sh"`) {
		t.Errorf("expected hints.sh command pane; got:\n%s", got)
	}
	if strings.Contains(got, "split_direction") {
		t.Errorf("no-plugin bottom bar should be a single pane, not a split; got:\n%s", got)
	}
}

func TestBuildBottomBar_WithPlugin(t *testing.T) {
	paths := []string{"/root/.agentjail/zellij/plugins/my-plugin.wasm"}
	got := buildBottomBar(paths)
	if !strings.Contains(got, `split_direction="vertical"`) {
		t.Errorf("plugin bottom bar should be a vertical split; got:\n%s", got)
	}
	if !strings.Contains(got, `command="/root/.agentjail/zellij/tabs/hints.sh"`) {
		t.Errorf("plugin bottom bar should still contain hints.sh; got:\n%s", got)
	}
	if !strings.Contains(got, `file:/root/.agentjail/zellij/plugins/my-plugin.wasm`) {
		t.Errorf("plugin bottom bar missing plugin location; got:\n%s", got)
	}
}

func TestBuildBottomBar_MultiplePlugins(t *testing.T) {
	paths := []string{
		"/root/.agentjail/zellij/plugins/plugin-a.wasm",
		"/root/.agentjail/zellij/plugins/plugin-b.wasm",
	}
	got := buildBottomBar(paths)
	if !strings.Contains(got, "plugin-a.wasm") {
		t.Errorf("missing plugin-a; got:\n%s", got)
	}
	if !strings.Contains(got, "plugin-b.wasm") {
		t.Errorf("missing plugin-b; got:\n%s", got)
	}
}

func TestCopyPlugins_CopiesFile(t *testing.T) {
	// Write a fake .wasm file in a temp src dir
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "my-plugin.wasm")
	if err := os.WriteFile(srcFile, []byte("fake wasm content"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	dstDir := t.TempDir()
	paths, err := copyPlugins(dstDir, []ZellijPlugin{{Path: srcFile}})
	if err != nil {
		t.Fatalf("copyPlugins: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 container path, got %d", len(paths))
	}
	if paths[0] != "/root/.agentjail/zellij/plugins/my-plugin.wasm" {
		t.Errorf("unexpected container path: %s", paths[0])
	}
	// Verify the file was actually copied
	data, err := os.ReadFile(filepath.Join(dstDir, "my-plugin.wasm"))
	if err != nil {
		t.Fatalf("copied file not found: %v", err)
	}
	if string(data) != "fake wasm content" {
		t.Errorf("copied file content mismatch: %s", data)
	}
}

func TestCopyPlugins_MissingFileSkipped(t *testing.T) {
	dstDir := t.TempDir()
	paths, err := copyPlugins(dstDir, []ZellijPlugin{{Path: "/nonexistent/plugin.wasm"}})
	if err != nil {
		t.Fatalf("copyPlugins should not error on missing file; got: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("missing file should be skipped; got paths: %v", paths)
	}
}

func TestCopyPlugins_Empty(t *testing.T) {
	paths, err := copyPlugins(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected no paths for empty input; got: %v", paths)
	}
}

func TestWriteZellijFiles_WithPlugin(t *testing.T) {
	// Create a fake plugin .wasm
	pluginSrc := filepath.Join(t.TempDir(), "zellij-claude-limits.wasm")
	if err := os.WriteFile(pluginSrc, []byte("wasm"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	dir := t.TempDir()
	plugins := []ZellijPlugin{{Path: pluginSrc}}
	if err := writeZellijFiles(dir, "tokyo-night-storm", "claude", "claude", "rovr", "zsh", plugins); err != nil {
		t.Fatalf("writeZellijFiles: %v", err)
	}

	layout := readFile(t, filepath.Join(dir, "zellij", "layout.kdl"))
	if !strings.Contains(layout, "zellij-claude-limits.wasm") {
		t.Errorf("layout.kdl should reference the plugin wasm; got:\n%s", layout)
	}
	if !strings.Contains(layout, "split_direction") {
		t.Errorf("layout.kdl should use a split bottom bar when plugins are present; got:\n%s", layout)
	}
	if !strings.Contains(layout, "hints.sh") {
		t.Errorf("layout.kdl should still contain hints.sh; got:\n%s", layout)
	}

	// Verify the .wasm was copied into the plugins dir
	if _, err := os.Stat(filepath.Join(dir, "zellij", "plugins", "zellij-claude-limits.wasm")); err != nil {
		t.Errorf("plugin .wasm not copied to plugins dir: %v", err)
	}
}

func TestPluginNameFromURL(t *testing.T) {
	cases := []struct {
		url     string
		want    string
		wantErr bool
	}{
		{"https://github.com/user/repo/releases/download/v1.0/plugin.wasm", "plugin.wasm", false},
		{"https://example.com/dist/zellij-claude-limits.wasm", "zellij-claude-limits.wasm", false},
		{"https://example.com/", "", true},
		{"not a url ://", "", true},
	}
	for _, c := range cases {
		got, err := pluginNameFromURL(c.url)
		if c.wantErr {
			if err == nil {
				t.Errorf("pluginNameFromURL(%q): expected error, got %q", c.url, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("pluginNameFromURL(%q): unexpected error: %v", c.url, err)
			continue
		}
		if got != c.want {
			t.Errorf("pluginNameFromURL(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

func TestCopyPlugins_URLDownload(t *testing.T) {
	const wasmContent = "fake wasm from server"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(wasmContent))
	}))
	defer srv.Close()

	pluginURL := srv.URL + "/dist/server-plugin.wasm"
	dstDir := t.TempDir()
	paths, err := copyPlugins(dstDir, []ZellijPlugin{{URL: pluginURL}})
	if err != nil {
		t.Fatalf("copyPlugins: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 container path, got %d", len(paths))
	}
	if paths[0] != "/root/.agentjail/zellij/plugins/server-plugin.wasm" {
		t.Errorf("unexpected container path: %s", paths[0])
	}
	data, err := os.ReadFile(filepath.Join(dstDir, "server-plugin.wasm"))
	if err != nil {
		t.Fatalf("downloaded file not found: %v", err)
	}
	if string(data) != wasmContent {
		t.Errorf("file content mismatch: got %q, want %q", data, wasmContent)
	}
}

func TestCopyPlugins_URLCached(t *testing.T) {
	// Server should only be hit once; second call uses the cached file.
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Write([]byte("wasm"))
	}))
	defer srv.Close()

	pluginURL := srv.URL + "/dist/cached-plugin.wasm"
	dstDir := t.TempDir()

	// First call: download
	if _, err := copyPlugins(dstDir, []ZellijPlugin{{URL: pluginURL}}); err != nil {
		t.Fatalf("first copyPlugins: %v", err)
	}
	// Second call: should use cache, no new HTTP request
	if _, err := copyPlugins(dstDir, []ZellijPlugin{{URL: pluginURL}}); err != nil {
		t.Fatalf("second copyPlugins: %v", err)
	}
	if hits != 1 {
		t.Errorf("expected exactly 1 HTTP request (cached on second call), got %d", hits)
	}
}

func TestCopyPlugins_URLNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	dstDir := t.TempDir()
	paths, err := copyPlugins(dstDir, []ZellijPlugin{{URL: srv.URL + "/missing.wasm"}})
	if err != nil {
		t.Fatalf("copyPlugins should not hard-fail on 404; got: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("failed download should be skipped; got paths: %v", paths)
	}
}

func TestCopyPlugins_EmptyEntry(t *testing.T) {
	dstDir := t.TempDir()
	paths, err := copyPlugins(dstDir, []ZellijPlugin{{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("empty entry should be skipped; got: %v", paths)
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
