package cli_test

import (
	"strings"
	"testing"
)

// TestHelpPlainText covers "fui help" without --json: it must fall back to cobra's normal usage
// text instead of the JSON tree.
func TestHelpPlainText(t *testing.T) {
	h := newHarness(t)
	out, _ := h.mustRun("help")
	if !strings.Contains(out, "Usage:") {
		t.Errorf("output = %q, want cobra's usual usage text", out)
	}
}

// TestHelpOnSubcommand covers "fui help account" rendering the account command's own help.
func TestHelpOnSubcommand(t *testing.T) {
	h := newHarness(t)
	out, _ := h.mustRun("help", "account")
	if !strings.Contains(out, "account") {
		t.Errorf("output = %q, want help for the account command", out)
	}
}

// TestVersionFlag covers the auto-generated --version flag (cobra.Command.Version).
func TestVersionFlag(t *testing.T) {
	h := newHarness(t)
	out, _ := h.mustRun("--version")
	if !strings.Contains(out, "test") {
		t.Errorf("output = %q, want the configured version %q", out, "test")
	}
}

// TestCompletionBash covers one of the three required shells; zsh and fish share the same code
// path through cobra's generators.
func TestCompletionBash(t *testing.T) {
	h := newHarness(t)
	out, _ := h.mustRun("completion", "bash")
	if !strings.Contains(out, "bash completion") {
		t.Errorf("output does not look like a bash completion script: %q", out[:min(80, len(out))])
	}
}

// TestNoSubcommandRunsTUI covers "fui" with no args: it must call the injected RunTUI, and fail
// clearly when none was given.
func TestNoSubcommandRunsTUI(t *testing.T) {
	h := newHarness(t)
	_, stderr, code := h.run()
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "no TUI available") {
		t.Errorf("stderr = %q, want a no-TUI-available message", stderr)
	}
}
