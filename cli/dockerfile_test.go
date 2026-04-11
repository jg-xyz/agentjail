package main

import (
	"strings"
	"testing"
)

// dockerfile reads the embedded Dockerfile template and returns it as a string.
func dockerfile(t *testing.T) string {
	t.Helper()
	data, err := templatesFS.ReadFile("templates/Dockerfile")
	if err != nil {
		t.Fatalf("could not read embedded Dockerfile: %v", err)
	}
	return string(data)
}

func TestDockerfileTemplate_NeovimInstallBlock(t *testing.T) {
	df := dockerfile(t)
	checks := []string{
		`"${EDITOR}" = "nvim"`,
		`apt-get install -y neovim`,
	}
	for _, s := range checks {
		if !strings.Contains(df, s) {
			t.Errorf("Dockerfile missing neovim install block; expected to contain %q", s)
		}
	}
}

func TestDockerfileTemplate_HelixInstallBlock(t *testing.T) {
	df := dockerfile(t)
	checks := []string{
		`"${EDITOR}" = "hx"`,
		`helix-editor/helix`,
		`/usr/local/bin/hx`,
		`HELIX_RUNTIME`,
		`/usr/local/lib/helix/runtime`,
	}
	for _, s := range checks {
		if !strings.Contains(df, s) {
			t.Errorf("Dockerfile missing helix install block; expected to contain %q", s)
		}
	}
}

func TestDockerfileTemplate_FreshInstallBlock(t *testing.T) {
	df := dockerfile(t)
	checks := []string{
		`"${EDITOR}" = "fresh"`,
		`@fresh-editor/fresh-editor`,
		`/usr/local/bin/fresh`,
	}
	for _, s := range checks {
		if !strings.Contains(df, s) {
			t.Errorf("Dockerfile missing fresh install block; expected to contain %q", s)
		}
	}
}

func TestDockerfileTemplate_HelixRuntimeExportedInBothShells(t *testing.T) {
	df := dockerfile(t)
	// HELIX_RUNTIME must be exported in both .zshrc and .bashrc so it's
	// available regardless of the configured shell.
	if strings.Count(df, "HELIX_RUNTIME=/usr/local/lib/helix/runtime") < 2 {
		t.Error("HELIX_RUNTIME export should appear for both zshrc and bashrc")
	}
}

func TestDockerfileTemplate_EditorMOTDShowsNeovim(t *testing.T) {
	df := dockerfile(t)
	if !strings.Contains(df, `"${EDITOR}" = "nvim" ] && AVAILABLE_EDITORS`) {
		t.Error("MOTD block does not conditionally append neovim to AVAILABLE_EDITORS")
	}
}

func TestDockerfileTemplate_EditorMOTDShowsHelix(t *testing.T) {
	df := dockerfile(t)
	if !strings.Contains(df, `"${EDITOR}" = "hx" ] && AVAILABLE_EDITORS`) {
		t.Error("MOTD block does not conditionally append helix to AVAILABLE_EDITORS")
	}
}

func TestDockerfileTemplate_EditorMOTDShowsFresh(t *testing.T) {
	df := dockerfile(t)
	if !strings.Contains(df, `"${EDITOR}" = "fresh" ] && AVAILABLE_EDITORS`) {
		t.Error("MOTD block does not conditionally append fresh to AVAILABLE_EDITORS")
	}
}

func TestDockerfileTemplate_SudoInstalled(t *testing.T) {
	df := dockerfile(t)
	found := false
	for _, line := range strings.Split(df, "\n") {
		if strings.Contains(line, "apt-get install") && strings.Contains(line, "sudo") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Dockerfile apt-get install block should include sudo")
	}
}

func TestDockerfileTemplate_UbuntuUserConfigured(t *testing.T) {
	df := dockerfile(t)
	// usermod must set home to /root so all existing dotfiles are inherited
	if !strings.Contains(df, "usermod") || !strings.Contains(df, "-d /root") {
		t.Error("Dockerfile should configure ubuntu user with home /root via usermod -d /root")
	}
	if !strings.Contains(df, "chown -R ubuntu:ubuntu /root") {
		t.Error("Dockerfile should transfer /root ownership to ubuntu")
	}
	if !strings.Contains(df, "sudoers.d/ubuntu") {
		t.Error("Dockerfile should create a sudoers drop-in for ubuntu")
	}
}

func TestDockerfileTemplate_UserDirective(t *testing.T) {
	df := dockerfile(t)
	if !strings.Contains(df, "\nUSER ubuntu\n") {
		t.Error("Dockerfile should switch to USER ubuntu before the final WORKDIR/CMD")
	}
}

func TestDockerfileTemplate_ExitHooksUseSudoChown(t *testing.T) {
	// The container runs as ubuntu (non-root), so ownership-fix hooks must use
	// sudo chown or they silently fail when restoring host UID/GID.
	df := dockerfile(t)
	if strings.Contains(df, "chown -R \"${HOST_UID}") && !strings.Contains(df, "sudo chown -R \"${HOST_UID}") {
		t.Error("exit hook chown commands must use sudo chown (container runs as non-root ubuntu)")
	}
}

func TestDockerfileTemplate_EditorInstallsAreConditional(t *testing.T) {
	// Each editor install block must have a matching "Not installing" fallback
	// so the build doesn't fail when a different editor is selected.
	df := dockerfile(t)
	for _, editor := range []string{"neovim", "helix", "fresh"} {
		needle := "Not installing " + editor
		if !strings.Contains(df, needle) {
			t.Errorf("Dockerfile missing fallback %q for conditional %s install", needle, editor)
		}
	}
}
