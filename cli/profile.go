package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const githubAPIBase = "https://api.github.com"

// ProfileFiles holds all content fetched from a Claude profile directory.
type ProfileFiles struct {
	SystemPrompt string            // content of CLAUDE.md; empty if absent
	Rules        map[string]string // filename → content for rules/*.md
	Agents       map[string]string // filename → content for agents/*.md
}

// githubDirEntry is one item returned by the GitHub contents API for a directory.
type githubDirEntry struct {
	Name string `json:"name"`
	Type string `json:"type"` // "file" or "dir"
	URL  string `json:"url"`  // API URL for this item
}

// githubFileEntry is returned by the GitHub contents API for a single file.
type githubFileEntry struct {
	Content  string `json:"content"`  // base64-encoded, may contain newlines
	Encoding string `json:"encoding"` // "base64"
}

// fetchProfile fetches profile files from GitHub (when p.Repo is set) or local disk.
func fetchProfile(p *ClaudeProfile, token string) (*ProfileFiles, error) {
	return fetchProfileWithBaseURL(p, token, githubAPIBase)
}

func fetchProfileWithBaseURL(p *ClaudeProfile, token, baseURL string) (*ProfileFiles, error) {
	if p.Repo != "" {
		if baseURL == "" {
			baseURL = githubAPIBase
		}
		return fetchGitHubProfile(p.Repo, p.Path, token, baseURL)
	}
	return fetchLocalProfile(p.Path)
}

func fetchGitHubProfile(repo, path, token, baseURL string) (*ProfileFiles, error) {
	result := &ProfileFiles{
		Rules:  make(map[string]string),
		Agents: make(map[string]string),
	}

	rootEntries, err := ghListDir(baseURL+"/repos/"+repo+"/contents/"+path, token)
	if err != nil {
		return nil, fmt.Errorf("listing profile directory %s/%s: %w", repo, path, err)
	}

	for _, entry := range rootEntries {
		switch {
		case entry.Type == "file" && entry.Name == "CLAUDE.md":
			content, err := ghFileContent(entry.URL, token)
			if err != nil {
				return nil, fmt.Errorf("fetching CLAUDE.md: %w", err)
			}
			result.SystemPrompt = content

		case entry.Type == "dir" && entry.Name == "rules":
			if err := ghFetchMDDir(entry.URL, token, result.Rules); err != nil {
				return nil, fmt.Errorf("fetching rules/: %w", err)
			}

		case entry.Type == "dir" && entry.Name == "agents":
			if err := ghFetchMDDir(entry.URL, token, result.Agents); err != nil {
				return nil, fmt.Errorf("fetching agents/: %w", err)
			}
		}
	}

	return result, nil
}

// ghFetchMDDir lists a directory via the GitHub API and reads all .md files into dst.
func ghFetchMDDir(dirURL, token string, dst map[string]string) error {
	entries, err := ghListDir(dirURL, token)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Type != "file" || !strings.HasSuffix(e.Name, ".md") {
			continue
		}
		content, err := ghFileContent(e.URL, token)
		if err != nil {
			return fmt.Errorf("fetching %s: %w", e.Name, err)
		}
		dst[e.Name] = content
	}
	return nil
}

func ghListDir(apiURL, token string) ([]githubDirEntry, error) {
	body, err := ghGet(apiURL, token)
	if err != nil {
		return nil, err
	}
	var entries []githubDirEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("parsing directory listing from %s: %w", apiURL, err)
	}
	return entries, nil
}

func ghFileContent(apiURL, token string) (string, error) {
	body, err := ghGet(apiURL, token)
	if err != nil {
		return "", err
	}
	var entry githubFileEntry
	if err := json.Unmarshal(body, &entry); err != nil {
		return "", fmt.Errorf("parsing file entry from %s: %w", apiURL, err)
	}
	if entry.Encoding != "base64" {
		return "", fmt.Errorf("unexpected encoding %q from %s", entry.Encoding, apiURL)
	}
	// GitHub wraps base64 output with newlines; strip before decoding.
	cleaned := strings.ReplaceAll(entry.Content, "\n", "")
	decoded, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return "", fmt.Errorf("base64 decode from %s: %w", apiURL, err)
	}
	return string(decoded), nil
}

func ghGet(apiURL, token string) ([]byte, error) {
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", apiURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: HTTP %d", apiURL, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func fetchLocalProfile(path string) (*ProfileFiles, error) {
	// Expand ~/... paths.
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolving home directory: %w", err)
		}
		path = filepath.Join(home, path[2:])
	}

	result := &ProfileFiles{
		Rules:  make(map[string]string),
		Agents: make(map[string]string),
	}

	if data, err := os.ReadFile(filepath.Join(path, "CLAUDE.md")); err == nil {
		result.SystemPrompt = string(data)
	}

	for _, sub := range []struct {
		dir string
		dst map[string]string
	}{
		{"rules", result.Rules},
		{"agents", result.Agents},
	} {
		entries, err := os.ReadDir(filepath.Join(path, sub.dir))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("reading %s/: %w", sub.dir, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(path, sub.dir, e.Name()))
			if err != nil {
				return nil, fmt.Errorf("reading %s/%s: %w", sub.dir, e.Name(), err)
			}
			sub.dst[e.Name()] = string(data)
		}
	}

	return result, nil
}
