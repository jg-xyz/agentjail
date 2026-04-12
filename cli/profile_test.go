package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// mockGitHubServer returns a test server serving a minimal two-file profile tree.
func mockGitHubServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)

	serveDir := func(entries []githubDirEntry) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(entries)
		}
	}
	serveFile := func(content string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			entry := githubFileEntry{
				Content:  base64.StdEncoding.EncodeToString([]byte(content)) + "\n",
				Encoding: "base64",
			}
			json.NewEncoder(w).Encode(entry)
		}
	}

	base := "/repos/owner/repo/contents/profiles/test"
	mux.HandleFunc(base, serveDir([]githubDirEntry{
		{Name: "CLAUDE.md", Type: "file", URL: srv.URL + base + "/CLAUDE.md"},
		{Name: "rules", Type: "dir", URL: srv.URL + base + "/rules"},
		{Name: "agents", Type: "dir", URL: srv.URL + base + "/agents"},
	}))
	mux.HandleFunc(base+"/CLAUDE.md", serveFile("# Be concise"))
	mux.HandleFunc(base+"/rules", serveDir([]githubDirEntry{
		{Name: "workflow.md", Type: "file", URL: srv.URL + base + "/rules/workflow.md"},
	}))
	mux.HandleFunc(base+"/rules/workflow.md", serveFile("## Workflow\nRead first."))
	mux.HandleFunc(base+"/agents", serveDir([]githubDirEntry{
		{Name: "builder.md", Type: "file", URL: srv.URL + base + "/agents/builder.md"},
	}))
	mux.HandleFunc(base+"/agents/builder.md", serveFile("# Builder\nWrite code."))

	return srv
}

func TestFetchGitHubProfile(t *testing.T) {
	srv := mockGitHubServer(t)
	defer srv.Close()

	profile := &ClaudeProfile{Repo: "owner/repo", Path: "profiles/test"}
	files, err := fetchProfileWithBaseURL(profile, "", srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if files.SystemPrompt != "# Be concise" {
		t.Errorf("SystemPrompt: got %q, want %q", files.SystemPrompt, "# Be concise")
	}
	if files.Rules["workflow.md"] != "## Workflow\nRead first." {
		t.Errorf("Rules[workflow.md]: got %q", files.Rules["workflow.md"])
	}
	if files.Agents["builder.md"] != "# Builder\nWrite code." {
		t.Errorf("Agents[builder.md]: got %q", files.Agents["builder.md"])
	}
}

func TestFetchLocalProfile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Local prompt"), 0644)
	os.MkdirAll(filepath.Join(dir, "rules"), 0755)
	os.WriteFile(filepath.Join(dir, "rules", "style.md"), []byte("## Style"), 0644)
	os.MkdirAll(filepath.Join(dir, "agents"), 0755)
	os.WriteFile(filepath.Join(dir, "agents", "coder.md"), []byte("# Coder"), 0644)

	profile := &ClaudeProfile{Path: dir}
	files, err := fetchProfileWithBaseURL(profile, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if files.SystemPrompt != "# Local prompt" {
		t.Errorf("SystemPrompt: got %q, want %q", files.SystemPrompt, "# Local prompt")
	}
	if files.Rules["style.md"] != "## Style" {
		t.Errorf("Rules[style.md]: got %q", files.Rules["style.md"])
	}
	if files.Agents["coder.md"] != "# Coder" {
		t.Errorf("Agents[coder.md]: got %q", files.Agents["coder.md"])
	}
}

func TestFetchProfile_MissingCLAUDEmd_StillReturnsRules(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "rules"), 0755)
	os.WriteFile(filepath.Join(dir, "rules", "only.md"), []byte("only rule"), 0644)

	profile := &ClaudeProfile{Path: dir}
	files, err := fetchProfileWithBaseURL(profile, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if files.SystemPrompt != "" {
		t.Errorf("expected empty SystemPrompt when CLAUDE.md absent, got: %q", files.SystemPrompt)
	}
	if files.Rules["only.md"] != "only rule" {
		t.Errorf("Rules[only.md]: got %q", files.Rules["only.md"])
	}
}
