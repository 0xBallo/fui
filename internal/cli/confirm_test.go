package cli_test

import (
	"strings"
	"testing"
)

// TestConfirmBulkAccept drives the huh accessible confirmation prompt with "y" on a bulk plan
// (category merge with transactions to move) on a simulated terminal.
func TestConfirmBulkAccept(t *testing.T) {
	h := newHarness(t)
	h.term = true
	h.category("Dinning", "expense")
	h.category("Dining", "expense")
	h.account("Checking", "checking", "EUR", "0")
	h.mustRun("add", "-10", "Pizza", "-c", "Dinning", "-a", "Checking")
	h.stdin.WriteString("y\n")

	out, stderr := h.mustRun("category", "merge", "Dinning", "Dining")
	if !strings.Contains(out, "Reassigned 1 transaction") {
		t.Errorf("output = %q, stderr=%q, want the reassigned-count note", out, stderr)
	}
}

// TestConfirmBulkDecline drives the prompt with "n": the plan must not be applied and the
// command still exits 0 ("Cancelled: nothing was changed.").
func TestConfirmBulkDecline(t *testing.T) {
	h := newHarness(t)
	h.term = true
	h.category("Dinning", "expense")
	h.category("Dining", "expense")
	h.account("Checking", "checking", "EUR", "0")
	h.mustRun("add", "-10", "Pizza", "-c", "Dinning", "-a", "Checking")
	before := h.store.Snapshot()
	h.stdin.WriteString("n\n")

	out, _, code := h.run("category", "merge", "Dinning", "Dining")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "Cancelled") {
		t.Errorf("output = %q, want a cancellation note", out)
	}
	if after := h.store.Snapshot(); after != before {
		t.Errorf("store changed despite declining confirmation:\nbefore=%s\nafter=%s", before, after)
	}
}

// TestBulkWithoutTerminalFailsConfirmationRequired: a bulk write with no terminal and no --yes
// fails confirmation_required instead of hanging or silently applying.
func TestBulkWithoutTerminalFailsConfirmationRequired(t *testing.T) {
	h := newHarness(t)
	h.category("Dinning", "expense")
	h.category("Dining", "expense")
	h.account("Checking", "checking", "EUR", "0")
	h.mustRun("add", "-10", "Pizza", "-c", "Dinning", "-a", "Checking")
	before := h.store.Snapshot()

	_, stderr, code := h.run("category", "merge", "Dinning", "Dining", "--json")
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
