package cli_test

import (
	"strings"
	"testing"
)

// TestTxShowEditRemove exercises the whole lifecycle of one transaction through the CLI.
func TestTxShowEditRemove(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "0")
	h.category("Groceries", "expense")
	h.mustRun("add", "-12.50", "Esselunga", "-c", "Groceries", "-a", "Checking")

	show, _ := h.mustRun("tx", "show", "1", "--json")
	if !strings.Contains(show, `"description":"Esselunga"`) {
		t.Errorf("tx show = %q, want description Esselunga", show)
	}

	edit, _ := h.mustRun("tx", "edit", "1", "--description", "Esselunga weekly", "--json")
	if !strings.Contains(edit, `"description":"Esselunga weekly"`) {
		t.Errorf("tx edit = %q, want updated description", edit)
	}

	rm, _ := h.mustRun("tx", "rm", "1", "--json")
	if !strings.Contains(rm, `"removed":1`) {
		t.Errorf("tx rm = %q, want removed:1", rm)
	}

	list, _ := h.mustRun("tx", "list", "--json")
	if strings.TrimSpace(list) != "[]" {
		t.Errorf("tx list after rm = %q, want an empty array", list)
	}
}

// TestTxShowUnknownIDFails covers the not_found path on a bad id.
func TestTxShowUnknownIDFails(t *testing.T) {
	h := newHarness(t)
	_, stderr, code := h.run("tx", "show", "999", "--json")
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "not_found") {
		t.Errorf("stderr = %q, want not_found", stderr)
	}
}

// TestTransfer covers a same-currency transfer and renders both legs.
func TestTransfer(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "1000")
	h.account("Savings", "savings", "EUR", "0")

	out, _ := h.mustRun("transfer", "--from", "Checking", "--to", "Savings", "--amount", "200", "--json")
	if !strings.Contains(out, `"account":"Checking"`) || !strings.Contains(out, `"account":"Savings"`) {
		t.Errorf("transfer output = %q, want both legs", out)
	}
	if !strings.Contains(out, `"amount":-20000`) || !strings.Contains(out, `"amount":20000`) {
		t.Errorf("transfer output = %q, want -20000 and 20000", out)
	}
}

// TestTransferPositionalAmount covers the functional spec's example "fui transfer 500 --from
// Fineco --to Risparmio": the amount is accepted as the first positional argument, like fui add.
func TestTransferPositionalAmount(t *testing.T) {
	h := newHarness(t)
	h.account("Fineco", "checking", "EUR", "1000")
	h.account("Risparmio", "savings", "EUR", "0")

	out, _ := h.mustRun("transfer", "500", "--from", "Fineco", "--to", "Risparmio", "--json")
	if !strings.Contains(out, `"amount":-50000`) || !strings.Contains(out, `"amount":50000`) {
		t.Errorf("transfer output = %q, want -50000 and 50000", out)
	}
}

// TestTransferPositionalAndFlagAmountMustMatch covers the "error if both differ" rule: a
// positional amount and --amount may both be given only if they agree.
func TestTransferPositionalAndFlagAmountMustMatch(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "1000")
	h.account("Savings", "savings", "EUR", "0")

	out, _ := h.mustRun("transfer", "200", "--from", "Checking", "--to", "Savings", "--amount", "200", "--json")
	if !strings.Contains(out, `"amount":-20000`) {
		t.Errorf("transfer output = %q, want -20000 when positional and --amount agree", out)
	}

	_, stderr, code := h.run("transfer", "200", "--from", "Checking", "--to", "Savings", "--amount", "300", "--json")
	if code != 1 {
		t.Errorf("transfer with conflicting amounts = exit %d, want 1", code)
	}
	if !strings.Contains(stderr, "200") || !strings.Contains(stderr, "300") {
		t.Errorf("stderr = %q, want it to mention both conflicting amounts", stderr)
	}
}

// TestTransferDoubleDashAndHelp covers the "fui transfer -- AMOUNT ..." escape hatch (consistent
// with fui add) and that --help still works despite DisableFlagParsing.
func TestTransferDoubleDashAndHelp(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "1000")
	h.account("Savings", "savings", "EUR", "0")

	out, _ := h.mustRun("transfer", "--", "200", "--from", "Checking", "--to", "Savings", "--json")
	if !strings.Contains(out, `"amount":-20000`) {
		t.Errorf("transfer -- 200 output = %q, want -20000", out)
	}

	help, _ := h.mustRun("transfer", "--help")
	if !strings.Contains(help, "Transfer money between two accounts") {
		t.Errorf("transfer --help = %q, want the command's short description", help)
	}
}

// TestTransferUnexpectedArgument covers the usage error for a stray positional argument beyond
// the optional leading amount.
func TestTransferUnexpectedArgument(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "1000")
	h.account("Savings", "savings", "EUR", "0")

	_, stderr, code := h.run("transfer", "200", "extra", "--from", "Checking", "--to", "Savings")
	if code != 1 {
		t.Errorf("transfer with a stray argument = exit %d, want 1", code)
	}
	if !strings.Contains(stderr, "extra") {
		t.Errorf("stderr = %q, want it to mention the unexpected argument", stderr)
	}
}

// TestTransferUnknownAccountFails covers the not_found path when the source account does not
// exist, propagated from app.Service.PlanTransfer.
func TestTransferUnknownAccountFails(t *testing.T) {
	h := newHarness(t)
	h.account("Savings", "savings", "EUR", "0")

	_, stderr, code := h.run("transfer", "200", "--from", "Unknown", "--to", "Savings", "--json")
	if code != 1 {
		t.Errorf("transfer from an unknown account = exit %d, want 1", code)
	}
	if !strings.Contains(stderr, "not_found") {
		t.Errorf("stderr = %q, want not_found", stderr)
	}
}

// TestAccountShowEdit covers account show and edit.
func TestAccountShowEdit(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "1000")

	show, _ := h.mustRun("account", "show", "Checking", "--json")
	if !strings.Contains(show, `"balance":100000`) {
		t.Errorf("account show = %q, want balance 100000", show)
	}

	edit, _ := h.mustRun("account", "edit", "Checking", "--name", "Main Checking", "--json")
	if !strings.Contains(edit, `"name":"Main Checking"`) {
		t.Errorf("account edit = %q, want renamed account", edit)
	}
}

// TestCategoryRenameAndRemoveWithReassign covers category rename and rm --reassign-to.
func TestCategoryRenameAndRemoveWithReassign(t *testing.T) {
	h := newHarness(t)
	h.category("Dinning", "expense")
	h.category("Dining", "expense")
	h.account("Checking", "checking", "EUR", "0")
	h.mustRun("add", "-10", "Pizza", "-c", "Dinning", "-a", "Checking")

	renamed, _ := h.mustRun("category", "rename", "Dinning", "Takeout", "--json")
	if !strings.Contains(renamed, `"name":"Takeout"`) {
		t.Errorf("category rename = %q, want name Takeout", renamed)
	}

	out, _ := h.mustRun("category", "rm", "Takeout", "--reassign-to", "Dining", "--yes", "--json")
	if !strings.Contains(out, `"reassigned":1`) {
		t.Errorf("category rm = %q, want reassigned:1", out)
	}
}

// TestFXSetAndList covers setting and listing a manual exchange rate.
func TestFXSetAndList(t *testing.T) {
	h := newHarness(t)
	out, _ := h.mustRun("fx", "set", "USD", "1.0842", "--date", "2026-01-01", "--json")
	if !strings.Contains(out, `"currency":"USD"`) {
		t.Errorf("fx set = %q, want currency USD", out)
	}

	list, _ := h.mustRun("fx", "list", "--json")
	if !strings.Contains(list, `"currency":"USD"`) {
		t.Errorf("fx list = %q, want the USD rate", list)
	}
}

// TestBackupAndRestore covers the full backup/restore round trip against the fake Backuper.
func TestBackupAndRestore(t *testing.T) {
	h := newHarness(t)
	out, _ := h.mustRun("backup", "--json")
	if !strings.Contains(out, "/backups/manual.db") {
		t.Errorf("backup output = %q, want the manual backup path", out)
	}

	restored, _ := h.mustRun("restore", "/backups/manual.db", "--yes", "--json")
	if !strings.Contains(restored, "/backups/manual.db") {
		t.Errorf("restore output = %q, want the restored path", restored)
	}
	if len(h.backup.restored) != 1 || h.backup.restored[0] != "/backups/manual.db" {
		t.Errorf("backup.restored = %v, want one call with /backups/manual.db", h.backup.restored)
	}
}

// TestConfigGetSet covers reading and writing a configuration key.
func TestConfigGetSet(t *testing.T) {
	h := newHarness(t)
	got, _ := h.mustRun("config", "get", "currency")
	if strings.TrimSpace(got) != "EUR" {
		t.Errorf("config get currency = %q, want EUR", got)
	}

	h.mustRun("config", "set", "currency", "USD")
	got2, _ := h.mustRun("config", "get", "currency", "--json")
	if !strings.Contains(got2, `"value":"USD"`) {
		t.Errorf("config get currency after set = %q, want USD", got2)
	}
}

// TestConfigSetDryRunDoesNotWrite covers --dry-run on a non-app write command.
func TestConfigSetDryRunDoesNotWrite(t *testing.T) {
	h := newHarness(t)
	h.mustRun("config", "set", "currency", "USD", "--dry-run")
	got, _ := h.mustRun("config", "get", "currency")
	if strings.TrimSpace(got) != "EUR" {
		t.Errorf("config get currency after dry-run set = %q, want unchanged EUR", got)
	}
}

// TestInit creates the default categories; running it twice is harmless.
func TestInit(t *testing.T) {
	h := newHarness(t)
	out, _ := h.mustRun("init", "--currency", "USD", "--json")
	if !strings.Contains(out, `"created":`) {
		t.Errorf("init output = %q, want a created count", out)
	}
	cur, _ := h.mustRun("config", "get", "currency")
	if strings.TrimSpace(cur) != "USD" {
		t.Errorf("currency after init --currency USD = %q, want USD", cur)
	}
}
