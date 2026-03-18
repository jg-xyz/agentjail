package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ensureFileFromTemplate checks if targetPath exists. If not, it writes the template content to it.
func ensureFileFromTemplate(targetPath string, templateName string) error {
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		content, err := templatesFS.ReadFile("templates/" + templateName)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", templateName, err)
		}

		if err := os.WriteFile(targetPath, content, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", targetPath, err)
		}
		fmt.Printf("Created %s from template.\n", targetPath)
	}
	return nil
}

// createAgentJailFolder creates the .agentjail folder and ensures it's set up properly
func createAgentJailFolder(baseDir string) (string, error) {
	agentJailDir := filepath.Join(baseDir, ".agentjail")

	if err := os.MkdirAll(agentJailDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create .agentjail directory: %w", err)
	}

	historyFile := filepath.Join(agentJailDir, "bash_history")
	if _, err := os.Stat(historyFile); os.IsNotExist(err) {
		if err := os.WriteFile(historyFile, []byte{}, 0644); err != nil {
			return "", fmt.Errorf("failed to create history file: %w", err)
		}
	}

	return agentJailDir, nil
}

// updateGitignore updates .gitignore to ignore .agentjail folder
func updateGitignore(baseDir string) error {
	gitignoreFile := filepath.Join(baseDir, ".gitignore")

	gitignoreContent := ""
	if data, err := os.ReadFile(gitignoreFile); err == nil {
		gitignoreContent = string(data)
	}

	if strings.Contains(gitignoreContent, ".agentjail") {
		return nil
	}

	newContent := gitignoreContent
	if newContent != "" && !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	newContent += ".agentjail/\n"

	if err := os.WriteFile(gitignoreFile, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to update .gitignore: %w", err)
	}

	fmt.Println("Added .agentjail/ to .gitignore")
	return nil
}

// copyTemplateConfigs copies tool-specific configs from templates to .agentjail
func copyTemplateConfigs(agentJailDir string, config *GlobalConfig) error {
	// Always copy rovr
	rovrDir := filepath.Join(agentJailDir, "rovr")
	if err := os.MkdirAll(rovrDir, 0755); err != nil {
		return err
	}

	for _, file := range []string{"config.toml", "pins.json"} {
		content, err := templatesFS.ReadFile("templates/configs/rovr/" + file)
		if err != nil {
			continue // Might not exist
		}
		if err := os.WriteFile(filepath.Join(rovrDir, file), content, 0644); err != nil {
			return err
		}
	}

	// Copy opencode if enabled
	if config.AgentFrameworks.OpenCode.Enabled {
		opencodeDir := filepath.Join(agentJailDir, "opencode")
		if err := os.MkdirAll(opencodeDir, 0755); err != nil {
			return err
		}
		content, err := templatesFS.ReadFile("templates/configs/opencode/opencode.json")
		if err == nil {
			if err := os.WriteFile(filepath.Join(opencodeDir, "opencode.json"), content, 0644); err != nil {
				return err
			}
		}
	}

	// Copy copilot if enabled
	if config.AgentFrameworks.Copilot.Enabled {
		copilotDir := filepath.Join(agentJailDir, "copilot")
		if err := os.MkdirAll(copilotDir, 0755); err != nil {
			return err
		}
		for _, file := range []string{"config.json", "mcp.json"} {
			content, err := templatesFS.ReadFile("templates/configs/copilot/" + file)
			if err != nil {
				continue
			}
			if err := os.WriteFile(filepath.Join(copilotDir, file), content, 0644); err != nil {
				return err
			}
		}
	}

	return nil
}
