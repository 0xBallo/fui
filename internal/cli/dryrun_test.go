package cli_test

import (
	"strings"
	"testing"
)

// TestDryRunLeavesStoreUnchanged covers "--dry-run renders the plan and returns without
// applying" for both a simple write (account add) and a bulk write (category merge).
func TestDryRunLeavesStoreUnchanged(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "0")
	before := h.store.Snapshot()

	out, _, code := h.run("account", "add", "Savings", "--kind", "savings", "--currency", "EUR", "--dry-run")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "Savings") {
		t.Errorf("plan output = %q, want it to mention Savings", out)
	}
	if after := h.store.Snapshot(); after != before {
		t.Errorf("store changed under --dry-run:\nbefore=%s\nafter=%s", before, after)
	}
}

// TestDryRunBulkPlanNeedsNoConfirmation: --dry-run never asks for confirmation even on a bulk
// plan, since it never applies anything.
func TestDryRunBulkPlanNeedsNoConfirmation(t *testing.T) {
	h := newHarness(t)
	h.category("Dinning", "expense")
	h.category("Dining", "expense")
	h.account("Checking", "checking", "EUR", "0")
	h.mustRun("add", "-10", "Pizza", "-c", "Dinning", "-a", "Checking")
	before := h.store.Snapshot()

	_, stderr, code := h.run("category", "merge", "Dinning", "Dining", "--dry-run")
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr)
	}
	if after := h.store.Snapshot(); after != before {
		t.Errorf("store changed under --dry-run:\nbefore=%s\nafter=%s", before, after)
	}
}
