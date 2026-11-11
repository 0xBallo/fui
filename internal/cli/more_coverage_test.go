package cli_test

import (
	"strings"
	"testing"
)

// TestCategoryAddListFormats covers category add (default kind, JSON output) and category list
// in table, JSON and CSV formats.
func TestCategoryAddListFormats(t *testing.T) {
	h := newHarness(t)
	out, _ := h.mustRun("category", "add", "Freelance", "--kind", "income", "--json")
	if !strings.Contains(out, `"kind":"income"`) {
		t.Errorf("category add output = %q, want kind income", out)
	}
	h.mustRun("category", "add", "Groceries")

	table, _ := h.mustRun("category", "list")
	if !strings.Contains(table, "Freelance") || !strings.Contains(table, "Groceries") {
		t.Errorf("category list table = %q, want both categories", table)
	}

	js, _ := h.mustRun("category", "list", "--json")
	if !strings.Contains(js, `"name":"Freelance"`) {
		t.Errorf("category list json = %q, want Freelance", js)
	}

	csv, _ := h.mustRun("category", "list", "--csv")
	if !strings.Contains(csv, "Freelance") {
		t.Errorf("category list csv = %q, want Freelance", csv)
	}
}

// TestFXListFormats covers fx list in table and CSV formats (JSON already covered elsewhere).
func TestFXListFormats(t *testing.T) {
	h := newHarness(t)
	h.mustRun("fx", "set", "USD", "1.0842", "--date", "2026-01-01")

	table, _ := h.mustRun("fx", "list")
	if !strings.Contains(table, "USD") {
		t.Errorf("fx list table = %q, want USD", table)
	}
	csv, _ := h.mustRun("fx", "list", "--csv")
	if !strings.Contains(csv, "USD") {
		t.Errorf("fx list csv = %q, want USD", csv)
	}
}

// TestAccountListFormats covers account list in table and CSV formats.
func TestAccountListFormats(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "1000")

	table, _ := h.mustRun("account", "list")
	if !strings.Contains(table, "Checking") {
		t.Errorf("account list table = %q, want Checking", table)
	}
	csv, _ := h.mustRun("account", "list", "--csv")
	if !strings.Contains(csv, "Checking") {
		t.Errorf("account list csv = %q, want Checking", csv)
	}
}

// TestAccountAddEditIBAN covers account add --iban, account edit --iban (set and clear), and the
// invalid_value error for a malformed IBAN.
func TestAccountAddEditIBAN(t *testing.T) {
	h := newHarness(t)
	out, _ := h.mustRun("account", "add", "Revolut", "--kind", "checking", "--currency", "EUR",
		"--iban", "de89 3704 0044 0532 0130 00", "--json")
	if !strings.Contains(out, `"iban":"DE89370400440532013000"`) {
		t.Errorf("account add --iban output = %q, want normalized iban", out)
	}

	out, _ = h.mustRun("account", "edit", "Revolut", "--iban", "GB29NWBK60161331926819", "--json")
	if !strings.Contains(out, `"iban":"GB29NWBK60161331926819"`) {
		t.Errorf("account edit --iban output = %q, want new iban", out)
	}

	out, _ = h.mustRun("account", "edit", "Revolut", "--iban", "", "--json")
	if strings.Contains(out, `"iban"`) {
		t.Errorf("account edit --iban \"\" output = %q, want iban cleared", out)
	}

	_, stderr, code := h.run("account", "add", "Checking2", "--kind", "checking", "--currency", "EUR", "--iban", "not-an-iban")
	if code != 1 || !strings.Contains(stderr, "invalid iban") {
		t.Errorf("account add with bad iban = exit %d, stderr %q, want exit 1 and invalid iban", code, stderr)
	}
}

// TestTxListFormats covers tx list in table and CSV formats.
func TestTxListFormats(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "0")
	h.category("Groceries", "expense")
	h.mustRun("add", "-10", "Esselunga", "-c", "Groceries", "-a", "Checking")

	table, _ := h.mustRun("tx", "list")
	if !strings.Contains(table, "Esselunga") {
		t.Errorf("tx list table = %q, want Esselunga", table)
	}
	csv, _ := h.mustRun("tx", "list", "--csv")
	if !strings.Contains(csv, "Esselunga") {
		t.Errorf("tx list csv = %q, want Esselunga", csv)
	}
}

// TestPromptStringOnTerminal covers the interactive prompt path: a missing required flag on a
// terminal (outside agent/no-input mode) asks for it via the accessible huh form.
func TestPromptStringOnTerminal(t *testing.T) {
	h := newHarness(t)
	h.term = true
	h.account("Checking", "checking", "EUR", "1000")
	h.account("Savings", "savings", "EUR", "0")
	h.stdin.WriteString("Checking\n")

	out, stderr := h.mustRun("transfer", "--to", "Savings", "--amount", "100", "--json")
	if !strings.Contains(out, `"account":"Checking"`) {
		t.Errorf("output = %q, stderr=%q, want the prompted-for source account", out, stderr)
	}
}

// TestPromptStringMissingOnNonTerminalFails covers the same missing flag outside a terminal.
func TestPromptStringMissingOnNonTerminalFails(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "1000")
	h.account("Savings", "savings", "EUR", "0")

	_, stderr, code := h.run("transfer", "--to", "Savings", "--amount", "100", "--json")
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "missing_field") {
		t.Errorf("stderr = %q, want missing_field", stderr)
	}
}

// TestRestoreWithoutYesRequiresConfirmation covers restore without --yes outside a terminal.
func TestRestoreWithoutYesRequiresConfirmation(t *testing.T) {
	h := newHarness(t)
	_, stderr, code := h.run("restore", "/backups/manual.db", "--json")
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "confirmation_required") {
		t.Errorf("stderr = %q, want confirmation_required", stderr)
	}
	if len(h.backup.restored) != 0 {
		t.Errorf("backup.restored = %v, want no calls", h.backup.restored)
	}
}

// TestDryRunJSONRendersPlanDTO covers --dry-run --json, which has its own JSON shape
// (render.PlanDTO) distinct from the final applied result.
func TestDryRunJSONRendersPlanDTO(t *testing.T) {
	h := newHarness(t)
	out, _ := h.mustRun("account", "add", "Savings", "--kind", "savings", "--currency", "EUR", "--dry-run", "--json")
	if !strings.Contains(out, `"changes":[`) || !strings.Contains(out, `"bulk":false`) {
		t.Errorf("output = %q, want a PlanDTO with changes and bulk", out)
	}
}

// TestRestoreDryRun covers --dry-run on restore: it must not call Restore.
func TestRestoreDryRun(t *testing.T) {
	h := newHarness(t)
	out, _ := h.mustRun("restore", "/backups/manual.db", "--dry-run")
	if !strings.Contains(out, "/backups/manual.db") {
		t.Errorf("output = %q, want the plan to mention the path", out)
	}
	if len(h.backup.restored) != 0 {
		t.Errorf("backup.restored = %v, want no calls under --dry-run", h.backup.restored)
	}
}

// TestConfigSetInvalidKeyFails covers config set with an unknown key.
func TestConfigSetInvalidKeyFails(t *testing.T) {
	h := newHarness(t)
	_, stderr, code := h.run("config", "set", "bogus.key", "value", "--json")
	if code == 0 {
		t.Fatalf("exit = 0, want a failure for an unknown config key; stderr=%s", stderr)
	}
}
