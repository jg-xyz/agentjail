package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// imageExists returns true if the Docker image with the given name exists locally.
func imageExists(imageName string) bool {
	cmd := exec.Command("docker", "image", "inspect", imageName)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// getContainerForDirectory finds a running container that has the given directory mounted.
func getContainerForDirectory(dir string) (string, error) {
	cmd := exec.Command("docker", "ps", "--format", "{{.Names}}\t{{.Mounts}}")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list containers: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}

		containerName := parts[0]
		mounts := parts[1]

		if strings.Contains(mounts, dir) {
			return containerName, nil
		}
	}

	return "", nil
}

// createTempDockerfile creates a temporary Dockerfile from the template and returns its path.
func createTempDockerfile() (string, error) {
	content, err := templatesFS.ReadFile("templates/Dockerfile")
	if err != nil {
		return "", fmt.Errorf("failed to read Dockerfile template: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "Dockerfile-agentjail-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp Dockerfile: %w", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write(content); err != nil {
		return "", fmt.Errorf("failed to write to temp Dockerfile: %w", err)
	}

	return tmpFile.Name(), nil
}
