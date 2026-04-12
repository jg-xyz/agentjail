package main

import (
	"encoding/json"
	"fmt"
)

// claudeSettingsFile mirrors the subset of Claude Code's settings.json that
// AgentJail populates — MCP servers and hooks. It is serialised to
// .agentjail/claude/settings.local.json and bind-mounted into the container
// at /project/.claude/settings.local.json so it merges (at highest priority)
// with global and project settings without shadowing either.
type claudeSettingsFile struct {
	MCPServers map[string]claudeMCPEntry    `json:"mcpServers,omitempty"`
	Hooks      map[string][]claudeHookGroup `json:"hooks,omitempty"`
}

// claudeMCPEntry is one entry under mcpServers in Claude's settings.json.
type claudeMCPEntry struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Type    string            `json:"type,omitempty"`
}

// claudeHookGroup is one element in the per-event hook array.
type claudeHookGroup struct {
	Matcher string             `json:"matcher,omitempty"`
	Hooks   []claudeHookAction `json:"hooks"`
}

// claudeHookAction is the actual command to run inside a hook group.
type claudeHookAction struct {
	Type    string `json:"type"`    // always "command"
	Command string `json:"command"`
}

// generateClaudeSettingsJSON converts a ClaudeFrameworkConfig into the JSON
// bytes for a settings.local.json file.
//
// Returns (nil, nil) when there are no MCP servers or hooks configured — the
// caller should skip writing the file in that case.
//
// Returns an error if any MCP server name is duplicated, since duplicate names
// produce silently-dropped entries in Claude's mcpServers map.
func generateClaudeSettingsJSON(cfg ClaudeFrameworkConfig) ([]byte, error) {
	if len(cfg.MCPServers) == 0 && len(cfg.Hooks) == 0 {
		return nil, nil
	}

	settings := claudeSettingsFile{}

	// Build mcpServers map.
	if len(cfg.MCPServers) > 0 {
		seen := make(map[string]bool, len(cfg.MCPServers))
		settings.MCPServers = make(map[string]claudeMCPEntry, len(cfg.MCPServers))
		for _, srv := range cfg.MCPServers {
			if srv.Name == "" {
				return nil, fmt.Errorf("mcp_servers entry is missing required 'name' field")
			}
			if seen[srv.Name] {
				return nil, fmt.Errorf("duplicate mcp_servers name %q", srv.Name)
			}
			seen[srv.Name] = true
			settings.MCPServers[srv.Name] = claudeMCPEntry{
				Command: srv.Command,
				Args:    srv.Args,
				Env:     srv.Env,
				Type:    srv.Type,
			}
		}
	}

	// Build hooks map grouped by event type.
	if len(cfg.Hooks) > 0 {
		// Group hooks by event, preserving order within each event.
		type hookKey struct{ event, matcher string }
		// Collect hooks grouped by (event, matcher) to merge same-matcher hooks.
		groupMap := make(map[hookKey][]claudeHookAction)
		var eventOrder []string
		seenEvent := make(map[string]bool)
		type groupEntry struct {
			key hookKey
		}
		var groupOrder []groupEntry

		for _, h := range cfg.Hooks {
			if h.Event == "" {
				return nil, fmt.Errorf("hooks entry is missing required 'event' field")
			}
			if h.Command == "" {
				return nil, fmt.Errorf("hooks entry for event %q is missing required 'command' field", h.Event)
			}
			k := hookKey{event: h.Event, matcher: h.Matcher}
			if _, exists := groupMap[k]; !exists {
				groupOrder = append(groupOrder, groupEntry{key: k})
			}
			groupMap[k] = append(groupMap[k], claudeHookAction{
				Type:    "command",
				Command: h.Command,
			})
			if !seenEvent[h.Event] {
				seenEvent[h.Event] = true
				eventOrder = append(eventOrder, h.Event)
			}
		}

		// Build per-event group slices in input order.
		eventGroups := make(map[string][]claudeHookGroup)
		for _, ge := range groupOrder {
			k := ge.key
			eventGroups[k.event] = append(eventGroups[k.event], claudeHookGroup{
				Matcher: k.matcher,
				Hooks:   groupMap[k],
			})
		}

		settings.Hooks = make(map[string][]claudeHookGroup, len(eventOrder))
		for _, ev := range eventOrder {
			settings.Hooks[ev] = eventGroups[ev]
		}
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshalling claude settings: %w", err)
	}
	return data, nil
}
