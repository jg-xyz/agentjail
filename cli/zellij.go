package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

// shellEscape wraps s in single quotes suitable for POSIX sh assignment.
// Any single quotes within s are safely escaped.
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// sanitizeKDLString removes characters that would break a KDL quoted string.
func sanitizeKDLString(s string) string {
	return strings.NewReplacer(`"`, ``, "\n", ``, "\r", ``).Replace(s)
}

// writeExecutable writes content to path with executable permissions.
func writeExecutable(path, content string) error {
	return os.WriteFile(path, []byte(content), 0755)
}

// buildZellijEntrypoint returns the shell command used to launch zellij inside
// the container. sessionName is used as the zellij session name so it appears
// in the tab bar; pass an empty string to let zellij pick a random name.
//
// --new-session-with-layout (-n) is used instead of --layout because --layout
// combined with --session means "add layout to an existing session", which
// fails with "no active session" when no session exists yet. -n always creates
// a fresh session regardless of context.
func buildZellijEntrypoint(sessionName string) string {
	base := "mise trust --yes /project; mise install; " +
		"ZELLIJ_CONFIG_DIR=/root/.agentjail/zellij mise x -- zellij" +
		" --new-session-with-layout /root/.agentjail/zellij/layout.kdl"
	if sessionName != "" {
		return base + " --session " + shellEscape(sessionName)
	}
	return base
}

// zellijKeybind represents a single bind line parsed from a rendered config.kdl.
type zellijKeybind struct {
	Key    string // e.g. "Ctrl t", "Alt ["
	Action string // e.g. "NewTab", "GoToPreviousTab"
}

// reKeybind matches `bind "KEY" { ACTION; }` lines in KDL.
var reKeybind = regexp.MustCompile(`bind\s+"([^"]+)"\s*\{\s*(\w+)\s*;`)

// parseZellijKeybinds extracts bind entries from a rendered config.kdl string.
func parseZellijKeybinds(config string) []zellijKeybind {
	var result []zellijKeybind
	for _, m := range reKeybind.FindAllStringSubmatch(config, -1) {
		result = append(result, zellijKeybind{Key: m[1], Action: m[2]})
	}
	return result
}

// formatKey converts a zellij key string to a compact display form.
// "Ctrl t" → "Ctrl+T", "Alt w" → "Alt+W", "Alt [" → "Alt+[".
func formatKey(key string) string {
	parts := strings.Fields(key)
	if len(parts) == 0 {
		return key
	}
	last := parts[len(parts)-1]
	if len(last) == 1 && last[0] >= 'a' && last[0] <= 'z' {
		last = strings.ToUpper(last)
	}
	parts[len(parts)-1] = last
	return strings.Join(parts, "+")
}

// buildHintsLine produces the printf content for hints.sh from a set of
// keybinds. It recognises the common tab-management actions and formats them
// as bold key names followed by short labels. GoToPreviousTab and GoToNextTab
// are combined into a single "KEY / KEY cycle tabs" hint when both are present.
// Unrecognised actions (e.g. NewPane, Detach) are intentionally omitted — the
// hints bar is curated, not exhaustive.
//
// The returned string contains literal \033 escape sequences suitable for
// embedding inside a single-quoted printf argument in a POSIX shell script.
// Any single quotes in key names are stripped to prevent breaking that quoting.
func buildHintsLine(binds []zellijKeybind) string {
	byAction := map[string]string{}
	for _, b := range binds {
		if _, exists := byAction[b.Action]; !exists {
			byAction[b.Action] = formatKey(b.Key)
		}
	}

	bold := func(k string) string { return "\\033[1m" + k + "\\033[0m" }

	var parts []string
	if k, ok := byAction["NewTab"]; ok {
		parts = append(parts, bold(k)+" new tab")
	}
	if k, ok := byAction["CloseTab"]; ok {
		parts = append(parts, bold(k)+" close tab")
	}
	prev, hasPrev := byAction["GoToPreviousTab"]
	next, hasNext := byAction["GoToNextTab"]
	switch {
	case hasPrev && hasNext:
		parts = append(parts, bold(prev)+" / "+bold(next)+" cycle tabs")
	case hasPrev:
		parts = append(parts, bold(prev)+" prev tab")
	case hasNext:
		parts = append(parts, bold(next)+" next tab")
	}
	if k, ok := byAction["Quit"]; ok {
		parts = append(parts, bold(k)+" quit")
	}

	if len(parts) == 0 {
		return ""
	}
	line := "  " + strings.Join(parts, "   ")
	// Strip single quotes so the result is safe inside printf '...' in hints.sh.
	return strings.ReplaceAll(line, "'", "")
}

// pluginNameFromURL extracts the filename portion from a plugin URL.
// For example "https://example.com/releases/my-plugin.wasm" → "my-plugin.wasm".
func pluginNameFromURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid plugin URL %q: %w", rawURL, err)
	}
	name := path.Base(u.Path)
	if name == "" || name == "." || name == "/" {
		return "", fmt.Errorf("cannot derive filename from URL %q", rawURL)
	}
	return name, nil
}

// downloadPlugin fetches rawURL and writes it to dst. The caller is responsible
// for ensuring the destination directory exists.
func downloadPlugin(dst, rawURL string) error {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(rawURL) //nolint:gosec // URL is user-supplied config, not attacker-controlled
	if err != nil {
		return fmt.Errorf("GET %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: unexpected status %s", rawURL, resp.Status)
	}
	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	_, copyErr := io.Copy(f, resp.Body)
	f.Close()
	if copyErr != nil {
		_ = os.Remove(dst) // clean up partial file
		return fmt.Errorf("write %s: %w", dst, copyErr)
	}
	return nil
}

// copyPlugins installs .wasm plugin files into pluginsDir and returns their
// in-container paths (/root/.agentjail/zellij/plugins/<name>).
//
// Each plugin is specified by either a local Path or a URL:
//   - Path: the file is copied from the host every launch.
//   - URL:  the file is downloaded on first use and cached; subsequent launches
//     skip the download if the file already exists in pluginsDir.
//
// Missing or unreachable plugins are logged and skipped rather than failing.
func copyPlugins(pluginsDir string, plugins []ZellijPlugin) ([]string, error) {
	if len(plugins) == 0 {
		return nil, nil
	}
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create plugins dir: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to determine home dir: %w", err)
	}

	var containerPaths []string
	for _, p := range plugins {
		var name string

		switch {
		case p.URL != "":
			n, err := pluginNameFromURL(p.URL)
			if err != nil {
				log.Printf("zellij plugin skipped: %v", err)
				continue
			}
			name = n
			dst := filepath.Join(pluginsDir, name)
			if _, err := os.Stat(dst); err == nil {
				// Already cached — skip download.
			} else {
				log.Printf("zellij plugin downloading: %s", p.URL)
				if err := downloadPlugin(dst, p.URL); err != nil {
					log.Printf("zellij plugin download failed, skipping: %v", err)
					continue
				}
			}

		case p.Path != "":
			src := p.Path
			if strings.HasPrefix(src, "~/") {
				src = filepath.Join(homeDir, src[2:])
			}
			name = filepath.Base(src)
			dst := filepath.Join(pluginsDir, name)

			in, err := os.Open(src)
			if err != nil {
				log.Printf("zellij plugin not found, skipping: %s (%v)", src, err)
				continue
			}
			out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
			if err != nil {
				in.Close()
				return nil, fmt.Errorf("failed to create plugin file %s: %w", dst, err)
			}
			_, copyErr := io.Copy(out, in)
			in.Close()
			out.Close()
			if copyErr != nil {
				return nil, fmt.Errorf("failed to copy plugin %s: %w", src, copyErr)
			}

		default:
			log.Printf("zellij plugin entry has neither path nor url, skipping")
			continue
		}

		containerPaths = append(containerPaths, "/root/.agentjail/zellij/plugins/"+name)
	}
	return containerPaths, nil
}

// buildBottomBar returns the KDL fragment for the bottom status bar pane.
// With no plugins it returns the current single-pane form (hints only).
// With plugins it returns a vertical split: hints on the left, each plugin on the right.
func buildBottomBar(containerPluginPaths []string) string {
	hintsCmd := "/root/.agentjail/zellij/tabs/hints.sh"
	if len(containerPluginPaths) == 0 {
		return fmt.Sprintf("        pane size=1 borderless=true command=%q", hintsCmd)
	}
	var sb strings.Builder
	sb.WriteString("        pane size=1 borderless=true split_direction=\"vertical\" {\n")
	sb.WriteString(fmt.Sprintf("            pane command=%q\n", hintsCmd))
	for _, p := range containerPluginPaths {
		sb.WriteString(fmt.Sprintf("            pane { plugin location=%q }\n", "file:"+p))
	}
	sb.WriteString("        }")
	return sb.String()
}

// writeZellijFiles generates the zellij layout, config, and per-tab wrapper
// scripts inside agentJailDir/zellij/. The directory is bind-mounted into the
// container at /root/.agentjail/zellij/.
//
// theme is the zellij color theme (e.g. "tokyo-night-storm").
// agentName labels tab 1 (e.g. "copilot"). Empty → "shell", no auto-launch.
// agentCmd is the command run in the agent tab; empty = plain shell.
// filesCmd is the command run in the files tab (e.g. "rovr").
// shell is the shell binary used for the plain terminal tab (e.g. "zsh", "bash").
//
// The layout and config are rendered from embedded KDL templates. Keybinds in
// config.kdl are parsed to auto-generate hints.sh, so the hints bar always
// reflects whatever is in the template.
func writeZellijFiles(agentJailDir, theme, agentName, agentCmd, filesCmd, shell string, plugins []ZellijPlugin) error {
	zellijDir := filepath.Join(agentJailDir, "zellij")
	tabsDir := filepath.Join(zellijDir, "tabs")
	pluginsDir := filepath.Join(zellijDir, "plugins")

	if err := os.MkdirAll(tabsDir, 0755); err != nil {
		return fmt.Errorf("failed to create zellij tabs dir: %w", err)
	}

	containerPluginPaths, err := copyPlugins(pluginsDir, plugins)
	if err != nil {
		return fmt.Errorf("failed to copy zellij plugins: %w", err)
	}

	agentTabName := sanitizeKDLString(agentName)
	if agentTabName == "" {
		agentTabName = "shell"
	}

	// agent.sh — sets AGENTJAIL_TAB_CMD so the shell rc hook auto-launches the agent.
	var agentScript string
	if agentCmd != "" {
		agentScript = fmt.Sprintf("#!/bin/sh\nexport AGENTJAIL_TAB_CMD=%s\nexec %s\n",
			shellEscape(agentCmd), shellEscape(shell))
	} else {
		agentScript = fmt.Sprintf("#!/bin/sh\nexec %s\n", shellEscape(shell))
	}
	if err := writeExecutable(filepath.Join(tabsDir, "agent.sh"), agentScript); err != nil {
		return fmt.Errorf("failed to write agent.sh: %w", err)
	}

	// files.sh — sets AGENTJAIL_TAB_CMD so the shell rc hook auto-launches the file manager.
	filesScript := fmt.Sprintf("#!/bin/sh\nexport AGENTJAIL_TAB_CMD=%s\nexec %s\n",
		shellEscape(filesCmd), shellEscape(shell))
	if err := writeExecutable(filepath.Join(tabsDir, "files.sh"), filesScript); err != nil {
		return fmt.Errorf("failed to write files.sh: %w", err)
	}

	// config.kdl — rendered from the embedded template. The theme is the only
	// substitution; keybinds are defined statically in the template file.
	themeSafe := sanitizeKDLString(theme)
	if themeSafe == "" {
		themeSafe = "tokyo-night-storm"
	}
	configRendered, err := renderZellijTemplate("templates/configs/zellij/config.kdl",
		struct{ Theme string }{themeSafe})
	if err != nil {
		return fmt.Errorf("failed to render config.kdl: %w", err)
	}
	if err := os.WriteFile(filepath.Join(zellijDir, "config.kdl"), []byte(configRendered), 0644); err != nil {
		return fmt.Errorf("failed to write config.kdl: %w", err)
	}

	// hints.sh — generated by parsing the keybinds out of the rendered config
	// so the hints bar always reflects what is actually in the template.
	// Neither status-bar nor compact-bar show user-defined keybinds in locked
	// mode, so a static pane is the only reliable approach.
	hintsLine := buildHintsLine(parseZellijKeybinds(configRendered))
	hintsScript := "#!/bin/sh\n" +
		"printf '\\033[?25l'\n"
	if hintsLine != "" {
		// No \n — in a size=1 pane a trailing newline scrolls the text off
		// the only visible row, leaving the pane blank.
		hintsScript += "printf '" + hintsLine + "'\n"
	}
	hintsScript += "exec tail -f /dev/null\n"
	if err := writeExecutable(filepath.Join(tabsDir, "hints.sh"), hintsScript); err != nil {
		return fmt.Errorf("failed to write hints.sh: %w", err)
	}

	// layout.kdl — rendered from the embedded template.
	// Shell is sanitized here (removes quotes/newlines) so it is safe to embed
	// in a KDL quoted string; agentTabName was already sanitized above.
	layoutRendered, err := renderZellijTemplate("templates/configs/zellij/layout.kdl",
		struct {
			AgentTabName string
			Shell        string
			BottomBar    string
		}{agentTabName, sanitizeKDLString(shell), buildBottomBar(containerPluginPaths)})
	if err != nil {
		return fmt.Errorf("failed to render layout.kdl: %w", err)
	}
	if err := os.WriteFile(filepath.Join(zellijDir, "layout.kdl"), []byte(layoutRendered), 0644); err != nil {
		return fmt.Errorf("failed to write layout.kdl: %w", err)
	}

	return nil
}

// renderZellijTemplate reads an embedded template by path, executes it with
// data, and returns the rendered string.
func renderZellijTemplate(path string, data any) (string, error) {
	raw, err := templatesFS.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read template %s: %w", path, err)
	}
	tmpl, err := template.New(path).Parse(string(raw))
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", path, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", path, err)
	}
	return buf.String(), nil
}
