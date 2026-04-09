package main

import (
	"reflect"
	"testing"
)

func TestNonInteractiveExecArgs_Basic(t *testing.T) {
	got := nonInteractiveExecArgs("agentjail.myproj", nil)
	want := []string{"exec", "-i", "agentjail.myproj", "claude"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNonInteractiveExecArgs_WithArgs(t *testing.T) {
	got := nonInteractiveExecArgs("agentjail.myproj", []string{"--debug", "--model", "claude-opus-4-5"})
	want := []string{"exec", "-i", "agentjail.myproj", "claude", "--debug", "--model", "claude-opus-4-5"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAdaptRunArgsForNonInteractive_RemovesTty(t *testing.T) {
	in := []string{"run", "-it", "--rm", "agentjail"}
	got := adaptRunArgsForNonInteractive(in, "")
	for _, arg := range got {
		if arg == "-it" {
			t.Errorf("expected -it to be removed, got %v", got)
		}
	}
	found := false
	for _, arg := range got {
		if arg == "-i" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected -i to be present, got %v", got)
	}
}

// TestAdaptRunArgsForNonInteractive_RemovesName verifies the legacy behaviour
// (empty niContainerName) still drops --name entirely.
func TestAdaptRunArgsForNonInteractive_RemovesName(t *testing.T) {
	in := []string{"run", "-it", "--rm", "--name", "agentjail.myproj", "--hostname", "agentjail", "agentjail"}
	got := adaptRunArgsForNonInteractive(in, "")
	for i, arg := range got {
		if arg == "--name" {
			t.Errorf("expected --name to be removed, found at index %d in %v", i, got)
		}
		if arg == "agentjail.myproj" {
			t.Errorf("expected container name value to be removed, found at index %d in %v", i, got)
		}
	}
}

// TestAdaptRunArgsForNonInteractive_ReplacesName verifies that when a
// niContainerName is provided the --name flag is kept and its value replaced.
func TestAdaptRunArgsForNonInteractive_ReplacesName(t *testing.T) {
	in := []string{"run", "-it", "--rm", "--name", "agentjail.myproj", "--hostname", "agentjail", "agentjail"}
	got := adaptRunArgsForNonInteractive(in, "agentjail-ni.mypro")
	want := []string{"run", "-i", "--rm", "--name", "agentjail-ni.mypro", "--hostname", "agentjail", "agentjail"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAdaptRunArgsForNonInteractive_DoesNotMutateInput(t *testing.T) {
	in := []string{"run", "-it", "--rm", "--name", "agentjail.myproj", "agentjail"}
	original := make([]string, len(in))
	copy(original, in)

	adaptRunArgsForNonInteractive(in, "")

	if !reflect.DeepEqual(in, original) {
		t.Errorf("input slice was mutated: got %v, want %v", in, original)
	}
}

func TestAdaptRunArgsForNonInteractive_PreservesOtherArgs(t *testing.T) {
	in := []string{"run", "-it", "--rm", "--name", "agentjail.myproj", "--hostname", "agentjail", "-v", "/host:/project", "agentjail"}
	got := adaptRunArgsForNonInteractive(in, "")
	want := []string{"run", "-i", "--rm", "--hostname", "agentjail", "-v", "/host:/project", "agentjail"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
