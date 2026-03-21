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

// agentCommand returns the shell command string to launch the given agent.
func agentCommand(name string) string {
	switch name {
	case "opencode":
		return "opencode"
	case "copilot":
		return "copilot"
	case "claude":
		return "claude"
	default:
		return name
	}
}

// chooseEnabledAgent shows an interactive prompt and returns the chosen agent name.
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
