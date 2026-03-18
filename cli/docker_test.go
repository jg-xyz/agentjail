package main

import (
	"os"
	"strings"
	"testing"
)

func TestCreateTempDockerfile_CreatesFile(t *testing.T) {
	path, err := createTempDockerfile()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read temp Dockerfile: %v", err)
	}
	if len(data) == 0 {
		t.Error("temp Dockerfile is empty")
	}
	// Dockerfile should contain FROM instruction
	if !strings.Contains(string(data), "FROM") {
		t.Errorf("Dockerfile content does not contain FROM: %q", string(data))
	}
}

func TestCreateTempDockerfile_UniquePerCall(t *testing.T) {
	path1, err := createTempDockerfile()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path1)

	path2, err := createTempDockerfile()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path2)

	if path1 == path2 {
		t.Error("expected unique temp file paths per call")
	}
}

func TestGetContainerForDirectory_EmptyOutput(t *testing.T) {
	// When docker returns no containers, should return empty string without error.
	// We can't mock exec.Command easily, but we can verify the parsing logic
	// handles empty/malformed lines gracefully by testing the directory-not-found path
	// with a directory that won't appear in any real container mounts.
	container, err := getContainerForDirectory("/nonexistent/unique/test/path/xyz123")
	// We expect either an error (docker not available) or an empty container name.
	if err == nil && container != "" {
		t.Errorf("expected empty container name for nonexistent directory, got %q", container)
	}
}

func TestImageExists_UnknownImage(t *testing.T) {
	// An image with a clearly nonexistent name should return false (or fail gracefully).
	result := imageExists("agentjail-test-image-that-does-not-exist-xyz123")
	if result {
		t.Error("expected false for nonexistent image")
	}
}
