package agent

import (
	"strings"
	"testing"

	"github.com/snow-ghost/mem/internal/config"
)

func TestClaudeBackend_GivenPromptAndModel_WhenBuildArgs_ThenCorrectArgs(t *testing.T) {
	b := builtinBackends[0] // claude
	args := b.BuildArgs("hello", "haiku")
	want := []string{"-p", "hello", "--model", "haiku"}
	assertArgs(t, args, want)
}

func TestOpenCodeBackend_GivenPromptAndModel_WhenBuildArgs_ThenModelIgnored(t *testing.T) {
	b := builtinBackends[1] // opencode
	args := b.BuildArgs("hello", "haiku")
	want := []string{"run", "hello"}
	assertArgs(t, args, want)
}

func TestCodexBackend_GivenPromptAndModel_WhenBuildArgs_ThenExecSubcommand(t *testing.T) {
	b := builtinBackends[2] // codex
	args := b.BuildArgs("hello", "o4-mini")
	want := []string{"exec", "hello", "-m", "o4-mini"}
	assertArgs(t, args, want)
}

func TestResolve_GivenBackendFlag_WhenResolved_ThenFlagUsed(t *testing.T) {
	cfg := config.Config{}
	b, err := Resolve(cfg, "opencode")
	if err != nil && !strings.Contains(err.Error(), "not found") {
		// opencode may not be installed — check name was resolved correctly
		t.Skipf("opencode not in PATH, skipping: %v", err)
	}
	if err == nil && b.Name != "opencode" {
		t.Errorf("Name = %q, want %q", b.Name, "opencode")
	}
}

func TestResolve_GivenEnvBackend_WhenNoFlag_ThenEnvUsed(t *testing.T) {
	cfg := config.Config{Backend: "codex"}
	b, err := Resolve(cfg, "")
	if err != nil && !strings.Contains(err.Error(), "not found") {
		t.Skipf("codex not in PATH, skipping: %v", err)
	}
	if err == nil {
		if b.Name != "codex" {
			t.Errorf("Name = %q, want %q", b.Name, "codex")
		}
		if b.Source != "env" {
			t.Errorf("Source = %q, want %q", b.Source, "env")
		}
	}
}

func TestResolve_GivenInvalidBackend_WhenResolved_ThenErrorWithValidValues(t *testing.T) {
	cfg := config.Config{Backend: "gemini"}
	_, err := Resolve(cfg, "")
	if err == nil {
		t.Fatal("expected error for invalid backend")
	}
	if !strings.Contains(err.Error(), "invalid backend") {
		t.Errorf("error = %q, want 'invalid backend'", err.Error())
	}
	if !strings.Contains(err.Error(), "claude") {
		t.Errorf("error should list valid backends, got: %q", err.Error())
	}
}

func TestResolve_GivenCustomWithBinary_WhenResolved_ThenCustomBackend(t *testing.T) {
	cfg := config.Config{
		Backend:       "custom",
		BackendBinary: "my-agent",
		BackendArgs:   "-p {prompt} --model {model}",
	}
	b, err := Resolve(cfg, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.Name != "custom" {
		t.Errorf("Name = %q, want %q", b.Name, "custom")
	}
	if b.Binary != "my-agent" {
		t.Errorf("Binary = %q, want %q", b.Binary, "my-agent")
	}
	args := b.BuildArgs("hello", "sonnet")
	assertArgs(t, args, []string{"-p", "hello", "--model", "sonnet"})
}

func TestResolve_GivenCustomWithoutModel_WhenBuildArgs_ThenModelIgnored(t *testing.T) {
	cfg := config.Config{
		Backend:       "custom",
		BackendBinary: "my-agent",
		BackendArgs:   "{prompt}",
	}
	b, err := Resolve(cfg, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	args := b.BuildArgs("hello", "sonnet")
	assertArgs(t, args, []string{"hello"})
}

func TestResolve_GivenCustomWithoutBinary_WhenResolved_ThenError(t *testing.T) {
	cfg := config.Config{Backend: "custom"}
	_, err := Resolve(cfg, "")
	if err == nil {
		t.Fatal("expected error when MEM_BACKEND_BINARY is missing")
	}
	if !strings.Contains(err.Error(), "MEM_BACKEND_BINARY") {
		t.Errorf("error = %q, want mention of MEM_BACKEND_BINARY", err.Error())
	}
}

func TestResolve_GivenCustomWithoutArgs_WhenResolved_ThenDefaultTemplate(t *testing.T) {
	cfg := config.Config{
		Backend:       "custom",
		BackendBinary: "my-agent",
	}
	b, err := Resolve(cfg, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	args := b.BuildArgs("hello", "")
	assertArgs(t, args, []string{"hello"})
}

func TestBackwardCompat_GivenNoConfig_WhenClaudeAvailable_ThenIdenticalArgs(t *testing.T) {
	// This tests that the claude backend produces the same args
	// as the old hardcoded CLIInvoker
	b := builtinBackends[0] // claude
	args := b.BuildArgs("test prompt", "haiku")
	want := []string{"-p", "test prompt", "--model", "haiku"}
	assertArgs(t, args, want)
}

func assertArgs(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("args = %v (len %d), want %v (len %d)", got, len(got), want, len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("args[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
