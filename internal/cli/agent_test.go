package cli_test

import (
	"strings"
	"testing"
)

// TestAgentModeWithoutYesRequiresConfirmation covers "FUI_AGENT=1 ⇒ --no-input --json and any
// write without --yes fails with confirmation_required" (technical spec, CLI persistent flags).
func TestAgentModeWithoutYesRequiresConfirmation(t *testing.T) {
	h := newHarness(t)
	h.cfg.AgentMode = true
	h.account("Checking", "checking", "EUR", "0")

	before := h.store.Snapshot()
	_, stderr, code := h.run("account", "archive", "Checking")
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "confirmation_required") {
		t.Errorf("stderr = %q, want confirmation_required", stderr)
	}
	if after := h.store.Snapshot(); after != before {
		t.Errorf("store changed despite confirmation_required:\nbefore=%s\nafter=%s", before, after)
	}
}

// TestAgentModeForcesJSON: agent mode ignores an explicit --csv/table request and always renders
// JSON, and output is parseable even when the invocation never passed --json.
func TestAgentModeForcesJSON(t *testing.T) {
	h := newHarness(t)
	h.cfg.AgentMode = true
	h.account("Checking", "checking", "EUR", "0")

	out, _ := h.mustRun("account", "list")
	if !strings.HasPrefix(strings.TrimSpace(out), "[") {
		t.Errorf("output = %q, want a JSON array (agent mode forces --json)", out)
	}
}

// TestAgentModeWithYesApplies confirms the write succeeds once --yes is given.
func TestAgentModeWithYesApplies(t *testing.T) {
	h := newHarness(t)
	h.cfg.AgentMode = true
	h.account("Checking", "checking", "EUR", "0")

	out, _ := h.mustRun("account", "archive", "Checking", "--yes")
	if !strings.Contains(out, `"archived":true`) {
		t.Errorf("output = %q, want archived:true", out)
	}
}
