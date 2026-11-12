package cli_test

import (
	"strings"
	"testing"
)

// TestFilterFlagsIncompatibleCombinations covers every pair of mutually exclusive filter flags on
// tx list, tx search and export: --month/--from, --month/--year and --income/--expense all fail
// with invalid_filter (exit 1), whether the CLI validates it (--income/--expense) or domain does
// (the period flags).
func TestFilterFlagsIncompatibleCombinations(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"month and from", []string{"--month", "2026-01", "--from", "2026-01-01"}},
		{"month and year", []string{"--month", "2026-01", "--year", "2026"}},
		{"income and expense", []string{"--income", "--expense"}},
	}
	cmds := [][]string{
		{"tx", "list"},
		{"tx", "search", "text"},
		{"export", "--format", "csv"},
	}
	for _, cmd := range cmds {
		for _, tt := range tests {
			t.Run(strings.Join(cmd, "_")+"/"+tt.name, func(t *testing.T) {
				h := newHarness(t)
				h.account("Checking", "checking", "EUR", "0")
				args := append(append([]string{}, cmd...), tt.args...)
				_, stderr, code := h.run(args...)
				if code != 1 {
					t.Fatalf("exit = %d, want 1; stderr=%s", code, stderr)
				}
				if !strings.Contains(stderr, "invalid_filter") {
					t.Errorf("stderr = %q, want invalid_filter", stderr)
				}
			})
		}
	}
}

// TestFilterFlagsSelectMatchingTransactions covers that every filter flag actually narrows tx
// list, not just that it parses.
func TestFilterFlagsSelectMatchingTransactions(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "0")
	h.category("Food", "expense")
	h.mustRun("add", "-10", "Groceries", "-c", "Food", "-a", "Checking", "--date", "2026-01-10")
	h.mustRun("add", "--", "-20", "Rent", "-a", "Checking", "--date", "2026-02-10", "--kind", "fee")
	h.mustRun("add", "5", "Refund", "-a", "Checking", "--date", "2026-01-15", "--kind", "refund")

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"month", []string{"--month", "2026-01"}, `"count":2`},
		{"year", []string{"--year", "2026"}, `"count":3`},
		{"uncategorized", []string{"--uncategorized"}, `"count":2`},
		{"kind", []string{"--kind", "fee"}, `"count":1`},
		{"income", []string{"--income"}, `"count":1`},
		{"expense", []string{"--expense"}, `"count":2`},
		{"sort amount", []string{"--sort", "amount"}, `"amount":-2000`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, _ := h.mustRun(append([]string{"tx", "list", "--json"}, tt.args...)...)
			if !strings.Contains(out, tt.want) {
				t.Errorf("tx list %v = %q, want it to contain %q", tt.args, out, tt.want)
			}
		})
	}
}

// TestFilterFlagsPayeeNotSupported covers that --payee fails with not_found: the payees archive
// does not exist yet (noted in GAPS).
func TestFilterFlagsPayeeNotSupported(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "0")
	_, stderr, code := h.run("tx", "list", "--payee", "Esselunga", "--json")
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "not_found") {
		t.Errorf("stderr = %q, want not_found", stderr)
	}
}

// TestFilterFlagsInvalidBatch covers --batch with a non-numeric value failing with
// invalid_filter instead of a confusing parse error.
func TestFilterFlagsInvalidBatch(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "0")
	_, stderr, code := h.run("tx", "list", "--batch", "x")
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "invalid_filter") {
		t.Errorf("stderr = %q, want invalid_filter", stderr)
	}
}
