package main

import "fmt"

// enabledAgents returns a list of agent names that are enabled in the config.
func enabledAgents(config *GlobalConfig) []string {
	var agents []string
	if config.AgentFrameworks.Copilot.Enabled {
		agents = append(agents, "copilot")
	}
	if config.AgentFrameworks.OpenCode.Enabled {
		agents = append(agents, "opencode")
	}
	if config.AgentFrameworks.ClaudeCode.Enabled {
		agents = append(agents, "claude")
	}
	return agents
}

// claudeSystemPrompt is appended to Claude Code's context on every launch so it
// knows which CLI tools are pre-installed in the container.
const claudeSystemPrompt = "Available CLI tools in this container: " +
	"git, gh (GitHub), curl, wget, jq, yq, mise (version manager), aws (AWS CLI), " +
	"eza, fd, fzf, rg (ripgrep), node, npm, python3, uv, pip, rich"

// resolveClaudeContext merges the config-level system prompt addition with the
// value of the --claude-context flag. Either or both may be empty. When both
// are non-empty they are joined with a blank line so they remain readable as
// distinct sections inside Claude's system prompt.
func resolveClaudeContext(configVal, flagVal string) string {
	switch {
	case configVal != "" && flagVal != "":
		return configVal + "\n\n" + flagVal
	case configVal != "":
		return configVal
	default:
		return flagVal
	}
}

// agentCommand returns the shell command string to launch the given agent.
// extraContext is appended to Claude Code's system prompt when non-empty.
func agentCommand(name, extraContext string) string {
	switch name {
	case "opencode":
		return "opencode"
	case "copilot":
		return "copilot"
	case "claude", "claude_code":
		prompt := claudeSystemPrompt
		if extraContext != "" {
			prompt += "\n\n" + extraContext
		}
		return "claude --append-system-prompt " + shellEscape(prompt)
	default:
		return name
	}
}

// chooseEnabledAgent shows an interactive prompt and returns the chosen agent name.
// fmt is used intentionally here for the interactive UI — these writes must go
// directly to stdout without logrus level prefixes.
func chooseEnabledAgent(config *GlobalConfig) string {
	agents := enabledAgents(config)
	if len(agents) == 0 {
		return ""
	}
	if len(agents) == 1 {
		return agents[0]
	}
	fmt.Println("Choose an agent to start:")
	for i, a := range agents {
		fmt.Printf("  %d. %s\n", i+1, a)
	}
	fmt.Print("Enter number: ")
	var choice int
	if _, err := fmt.Scan(&choice); err == nil && choice >= 1 && choice <= len(agents) {
		return agents[choice-1]
	}
	return agents[0]
}
