package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectGitBranch_ValidBranch(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get working dir: %v", err)
	}
	branch := detectGitBranch(dir)
	if branch == "" {
		t.Error("expected non-empty branch name for git repo, got empty string")
	}
	if branch == "HEAD" {
		t.Error("expected real branch name, got detached HEAD sentinel")
	}
}

func TestDetectGitBranch_NotGitRepo(t *testing.T) {
	branch := detectGitBranch(os.TempDir())
	if branch != "" {
		t.Errorf("expected empty branch for non-git dir, got %q", branch)
	}
}

func TestSanitizeBranchName_SpecialChars(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"feature/user-auth", "feature-user-auth"},
		{"fix(api):payment", "fix-api--payment"},
		{"main", "main"},
		{"release-v1.2.3", "release-v1-2-3"},
		{"feat_new", "feat_new"},
		{"hello world", "hello-world"},
	}
	for _, tc := range cases {
		got := sanitizeBranchName(tc.input)
		if got != tc.want {
			t.Errorf("sanitizeBranchName(%q): got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestBuildSessionName_WithBranch(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get working dir: %v", err)
	}
	name := buildSessionName(dir)
	base := filepath.Base(dir)
	if !strings.HasPrefix(name, base) {
		t.Errorf("session name %q does not start with folder name %q", name, base)
	}
	if !strings.Contains(name, "_") {
		t.Errorf("session name %q missing _ separator (expected <folder>_<branch>)", name)
	}
}

func TestBuildSessionName_NoGit(t *testing.T) {
	name := buildSessionName(os.TempDir())
	want := filepath.Base(os.TempDir())
	if name != want {
		t.Errorf("buildSessionName for non-git dir: got %q, want %q", name, want)
	}
}
