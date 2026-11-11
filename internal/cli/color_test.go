package cli_test

import (
	"os"
	"strings"
	"testing"
)

// TestNoColorDisablesAnsiOnTerminal: with IsTerminal true but NO_COLOR set, the table renderer
// must not emit ANSI escapes even for a negative (outflow) amount.
func TestNoColorDisablesAnsiOnTerminal(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	h := newHarness(t)
	h.term = true
	h.account("Checking", "checking", "EUR", "0")
	h.mustRun("add", "-10.00", "Rent", "-a", "Checking")

	out, _ := h.mustRun("tx", "list")
	if strings.Contains(out, "\x1b[") {
		t.Errorf("output contains ANSI escapes with NO_COLOR set: %q", out)
	}
}

// TestColorOnTerminalWithoutNoColor is the control case for TestNoColorDisablesAnsiOnTerminal:
// absent NO_COLOR, a terminal gets ANSI color on the negative amount.
func TestColorOnTerminalWithoutNoColor(t *testing.T) {
	if v, ok := os.LookupEnv("NO_COLOR"); ok && v != "" {
		t.Setenv("NO_COLOR", "")
	}
	h := newHarness(t)
	h.term = true
	h.account("Checking", "checking", "EUR", "0")
	h.mustRun("add", "-10.00", "Rent", "-a", "Checking")

	out, _ := h.mustRun("tx", "list")
	if !strings.Contains(out, "\x1b[31m") {
		t.Errorf("output = %q, want ANSI red on the outflow amount", out)
	}
}
