package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAndLoadMetadata_RoundTrip(t *testing.T) {
	dir := t.TempDir()

	original := &AgentJailMetadata{
		ContainerName:    "agentjail.myproj",
		Network:          "bridge",
		Volumes:          []string{"/host:/container", "/a:/b"},
		EnvironmentVars:  map[string]string{"EDITOR": "micro", "SHELL": "zsh"},
		ImageVersion:     "agentjail",
		CreatedAt:        time.Now().Truncate(time.Second),
		AgentJailVersion: "1.0.0",
	}

	if err := saveMetadata(dir, original); err != nil {
		t.Fatalf("saveMetadata failed: %v", err)
	}

	got, err := loadMetadata(dir)
	if err != nil {
		t.Fatalf("loadMetadata failed: %v", err)
	}
	if got == nil {
		t.Fatal("loadMetadata returned nil")
	}
	if got.ContainerName != original.ContainerName {
		t.Errorf("ContainerName: got %q, want %q", got.ContainerName, original.ContainerName)
	}
	if got.Network != original.Network {
		t.Errorf("Network: got %q, want %q", got.Network, original.Network)
	}
	if len(got.Volumes) != len(original.Volumes) {
		t.Errorf("Volumes length: got %d, want %d", len(got.Volumes), len(original.Volumes))
	}
	if got.EnvironmentVars["EDITOR"] != "micro" {
		t.Errorf("EnvironmentVars[EDITOR]: got %q", got.EnvironmentVars["EDITOR"])
	}
	if got.AgentJailVersion != original.AgentJailVersion {
		t.Errorf("AgentJailVersion: got %q, want %q", got.AgentJailVersion, original.AgentJailVersion)
	}
}

func TestLoadMetadata_Missing(t *testing.T) {
	dir := t.TempDir()
	got, err := loadMetadata(dir)
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil metadata for missing file, got: %+v", got)
	}
}

func TestLoadMetadata_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "metadata.json")
	if err := os.WriteFile(metaPath, []byte("{invalid json}"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := loadMetadata(dir)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestSaveMetadata_UpdatesLastUsed(t *testing.T) {
	dir := t.TempDir()
	before := time.Now().Add(-time.Second)

	meta := &AgentJailMetadata{
		ContainerName:    "test",
		AgentJailVersion: "1.0.0",
		CreatedAt:        before,
		LastUsed:         before,
	}

	if err := saveMetadata(dir, meta); err != nil {
		t.Fatal(err)
	}

	got, err := loadMetadata(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !got.LastUsed.After(before) {
		t.Errorf("LastUsed should have been updated, got %v (before was %v)", got.LastUsed, before)
	}
}

func TestCheckVersionUpdate_SameVersion(t *testing.T) {
	dir := t.TempDir()
	meta := &AgentJailMetadata{
		ContainerName:    "test",
		AgentJailVersion: "1.0.0",
		CreatedAt:        time.Now(),
	}
	if err := saveMetadata(dir, meta); err != nil {
		t.Fatal(err)
	}

	if err := checkVersionUpdate(dir, "1.0.0"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Version should be unchanged
	got, _ := loadMetadata(dir)
	if got.AgentJailVersion != "1.0.0" {
		t.Errorf("version should remain 1.0.0, got %q", got.AgentJailVersion)
	}
}

func TestCheckVersionUpdate_NewVersion(t *testing.T) {
	dir := t.TempDir()
	meta := &AgentJailMetadata{
		ContainerName:    "test",
		AgentJailVersion: "1.0.0",
		CreatedAt:        time.Now(),
	}
	if err := saveMetadata(dir, meta); err != nil {
		t.Fatal(err)
	}

	if err := checkVersionUpdate(dir, "2.0.0"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	got, _ := loadMetadata(dir)
	if got.AgentJailVersion != "2.0.0" {
		t.Errorf("version should be updated to 2.0.0, got %q", got.AgentJailVersion)
	}
}

func TestCheckVersionUpdate_NoExistingMetadata(t *testing.T) {
	dir := t.TempDir()
	// No metadata file — should be a no-op
	if err := checkVersionUpdate(dir, "1.0.0"); err != nil {
		t.Errorf("unexpected error with no existing metadata: %v", err)
	}
}
