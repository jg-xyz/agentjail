package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
// the container.
func buildZellijEntrypoint() string {
	return "mise trust --yes /project; mise install; " +
		"ZELLIJ_CONFIG_DIR=/root/.agentjail/zellij exec mise x -- zellij" +
		" --layout /root/.agentjail/zellij/layout.kdl"
}

// writeZellijFiles generates the zellij layout, config, and per-tab wrapper
// scripts inside agentJailDir/zellij/. The directory is bind-mounted into the
// container at /root/.agentjail/zellij/.
//
// sessionName is the zellij session name (e.g. the project directory name).
// theme is the zellij color theme (e.g. "tokyo-night-storm").
// agentName labels tab 1 (e.g. "copilot"). Empty → "shell", no auto-launch.
// agentCmd is the command run in the agent tab; empty = plain shell.
// filesCmd is the command run in the files tab (e.g. "rovr").
// shell is the shell binary used for the plain terminal tab (e.g. "zsh", "bash").
//
// The agent and files tabs set AGENTJAIL_TAB_CMD before exec'ing the shell,
// which the .zshrc lazy-launcher hook picks up to run the program on first prompt.
func writeZellijFiles(agentJailDir, sessionName, theme, agentName, agentCmd, filesCmd, shell string) error {
	zellijDir := filepath.Join(agentJailDir, "zellij")
	tabsDir := filepath.Join(zellijDir, "tabs")

	if err := os.MkdirAll(tabsDir, 0755); err != nil {
		return fmt.Errorf("failed to create zellij tabs dir: %w", err)
	}

	agentTabName := sanitizeKDLString(agentName)
	if agentTabName == "" {
		agentTabName = "shell"
	}

	// agent.sh — sets AGENTJAIL_TAB_CMD so the .zshrc hook auto-launches the agent.
	var agentScript string
	if agentCmd != "" {
		agentScript = fmt.Sprintf("#!/bin/sh\nexport AGENTJAIL_TAB_CMD=%s\nexec zsh\n",
			shellEscape(agentCmd))
	} else {
		agentScript = "#!/bin/sh\nexec zsh\n"
	}
	if err := writeExecutable(filepath.Join(tabsDir, "agent.sh"), agentScript); err != nil {
		return fmt.Errorf("failed to write agent.sh: %w", err)
	}

	// files.sh — sets AGENTJAIL_TAB_CMD so the .zshrc hook auto-launches the file manager.
	filesScript := fmt.Sprintf("#!/bin/sh\nexport AGENTJAIL_TAB_CMD=%s\nexec zsh\n",
		shellEscape(filesCmd))
	if err := writeExecutable(filepath.Join(tabsDir, "files.sh"), filesScript); err != nil {
		return fmt.Errorf("failed to write files.sh: %w", err)
	}

	// layout.kdl — three tabs; agent is focused on start.
	// The terminal tab uses the configured shell; agent/files tabs always use
	// zsh because the lazy-launcher hook lives in .zshrc.
	// default_tab_template includes tab-bar and a compact status-bar (size=1)
	// showing only the configured keybinding hints.
	sessionNameSafe := sanitizeKDLString(sessionName)
	if sessionNameSafe == "" {
		sessionNameSafe = "agentjail"
	}
	layout := fmt.Sprintf(`layout {
    default_tab_template {
        pane size=1 borderless=true {
            plugin location="zellij:tab-bar"
        }
        children
        pane size=1 borderless=true {
            plugin location="zellij:status-bar"
        }
    }
    tab name="%s" focus=true {
        pane command="/root/.agentjail/zellij/tabs/agent.sh" {
            cwd "/project"
        }
    }
    tab name="terminal" {
        pane command="%s" {
            cwd "/project"
        }
    }
    tab name="files" {
        pane command="/root/.agentjail/zellij/tabs/files.sh" {
            cwd "/project"
        }
    }
}
`, agentTabName, sanitizeKDLString(shell))

	if err := os.WriteFile(filepath.Join(zellijDir, "layout.kdl"), []byte(layout), 0644); err != nil {
		return fmt.Errorf("failed to write layout.kdl: %w", err)
	}

	// config.kdl — regenerated on every run so theme and keybinds stay in sync
	// with the agentjail config. Users should not edit this file directly.
	themeSafe := sanitizeKDLString(theme)
	if themeSafe == "" {
		themeSafe = "tokyo-night-storm"
	}
	config := fmt.Sprintf(`// zellij configuration for agentjail — auto-generated, do not edit
// https://zellij.dev/documentation/configuration

pane_frames false
theme "%s"

keybinds {
    normal {
        bind "Ctrl t" { NewTab; }
    }
}
`, themeSafe)

	if err := os.WriteFile(filepath.Join(zellijDir, "config.kdl"), []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write config.kdl: %w", err)
	}

	return nil
}
