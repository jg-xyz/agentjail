package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// AgentJailMetadata structure for .agentjail/metadata.json
type AgentJailMetadata struct {
	ContainerName    string            `json:"container_name"`
	Network          string            `json:"network,omitempty"`
	Volumes          []string          `json:"volumes"`
	EnvironmentVars  map[string]string `json:"environment_vars"`
	ImageVersion     string            `json:"image_version"`
	CreatedAt        time.Time         `json:"created_at"`
	LastUsed         time.Time         `json:"last_used"`
	AgentJailVersion string            `json:"agentjail_version"`
}

// saveMetadata saves the container metadata to .agentjail/metadata.json
func saveMetadata(agentJailDir string, metadata *AgentJailMetadata) error {
	metadata.LastUsed = time.Now()

	metadataFile := filepath.Join(agentJailDir, "metadata.json")

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metadataFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}

// loadMetadata loads existing metadata from .agentjail/metadata.json
func loadMetadata(agentJailDir string) (*AgentJailMetadata, error) {
	metadataFile := filepath.Join(agentJailDir, "metadata.json")

	data, err := os.ReadFile(metadataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var metadata AgentJailMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &metadata, nil
}

// checkVersionUpdate checks if agentjail version changed and updates metadata
func checkVersionUpdate(agentJailDir string, currentVersion string) error {
	existingMetadata, err := loadMetadata(agentJailDir)
	if err != nil {
		return fmt.Errorf("failed to load existing metadata: %w", err)
	}

	if existingMetadata != nil && existingMetadata.AgentJailVersion != currentVersion {
		fmt.Printf("AgentJail version updated from %s to %s\n", existingMetadata.AgentJailVersion, currentVersion)
		existingMetadata.AgentJailVersion = currentVersion
		existingMetadata.LastUsed = time.Now()

		if err := saveMetadata(agentJailDir, existingMetadata); err != nil {
			return fmt.Errorf("failed to update metadata with new version: %w", err)
		}
	}

	return nil
}
