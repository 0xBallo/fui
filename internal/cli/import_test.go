package cli_test

import (
	"strconv"
	"strings"
	"testing"
)

// TestImportFUIDryRunShowsSummaryAndLeavesStoreUnchanged covers the ".fui.jsonl defaults to the
// fui connector" rule and "--dry-run renders the plan and returns without applying" for `import`.
func TestImportFUIDryRunShowsSummaryAndLeavesStoreUnchanged(t *testing.T) {
	h := newHarness(t)
	file := writeFUIFile(t, t.TempDir(), "statement.fui.jsonl",
		fuiStream("fui", "statement.fui.jsonl", "sha-1",
			fuiAccount("acc-1", "Checking", "EUR"),
			fuiTx("acc-1", "tx-1", "2026-11-01", -1000, "Pizza"),
		)...)
	before := h.store.Snapshot()

	out, _, code := h.run("import", file, "--create-accounts", "--dry-run")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	for _, want := range []string{"connector", "fui", "new", "1"} {
		if !strings.Contains(out, want) {
			t.Errorf("dry-run output = %q, want it to mention %q", out, want)
		}
	}
	if after := h.store.Snapshot(); after != before {
		t.Errorf("store changed under --dry-run:\nbefore=%s\nafter=%s", before, after)
	}
}

// TestImportAppliesCreatesAccountAndTransaction covers the full apply path: --create-accounts
// plus --yes writes the new account and transaction, and `import list` then shows the batch.
func TestImportAppliesCreatesAccountAndTransaction(t *testing.T) {
	h := newHarness(t)
	file := writeFUIFile(t, t.TempDir(), "statement.fui.jsonl",
		fuiStream("fui", "statement.fui.jsonl", "sha-1",
			fuiAccount("acc-1", "Checking", "EUR"),
			fuiTx("acc-1", "tx-1", "2026-11-01", -1000, "Pizza"),
		)...)

	out, _ := h.mustRun("import", file, "--create-accounts", "--yes", "--json")
	if !strings.Contains(out, `"transactions":1`) || !strings.Contains(out, `"accounts":1`) {
		t.Errorf("import result = %q, want transactions:1 and accounts:1", out)
	}

	listOut, _ := h.mustRun("import", "list", "--json")
	if !strings.Contains(listOut, `"connector":"fui"`) || !strings.Contains(listOut, "statement.fui.jsonl") {
		t.Errorf("import list = %q, want the batch just created", listOut)
	}
}

// TestImportWithoutConnectorFlagFailsForNonFUIFile covers "a file not ending in .fui.jsonl with
// no --connector fails with missing_field", since only .fui.jsonl defaults the connector.
func TestImportWithoutConnectorFlagFailsForNonFUIFile(t *testing.T) {
	h := newHarness(t)
	_, stderr, code := h.run("import", "statement.csv", "--json")
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, `"code":"missing_field"`) {
		t.Errorf("stderr = %q, want code missing_field", stderr)
	}
}

// TestImportUnmappedWithoutTerminalFails covers "otherwise unmapped_accounts": outside a
// terminal, an unmapped account ref cannot be resolved interactively, so applying fails with
// importing.CodeUnmappedAccounts.
func TestImportUnmappedWithoutTerminalFails(t *testing.T) {
	h := newHarness(t)
	file := writeFUIFile(t, t.TempDir(), "statement.fui.jsonl",
		fuiStream("fui", "statement.fui.jsonl", "sha-1",
			fuiAccount("acc-1", "Checking", "EUR"),
			fuiTx("acc-1", "tx-1", "2026-11-01", -1000, "Pizza"),
		)...)
	before := h.store.Snapshot()

	_, stderr, code := h.run("import", file, "--yes", "--json")
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, `"code":"unmapped_accounts"`) {
		t.Errorf("stderr = %q, want code unmapped_accounts", stderr)
	}
	if after := h.store.Snapshot(); after != before {
		t.Errorf("store changed despite unmapped_accounts:\nbefore=%s\nafter=%s", before, after)
	}
}

// TestImportMapBindsExistingAccount covers --map ref=account: the row binds to the named
// account instead of creating a new one.
func TestImportMapBindsExistingAccount(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "0")
	file := writeFUIFile(t, t.TempDir(), "statement.fui.jsonl",
		fuiStream("fui", "statement.fui.jsonl", "sha-1",
			fuiAccount("acc-1", "Current account", "EUR"),
			fuiTx("acc-1", "tx-1", "2026-11-01", -1000, "Pizza"),
		)...)

	out, _ := h.mustRun("import", file, "--map", "acc-1=Checking", "--yes", "--json")
	if !strings.Contains(out, `"accounts":0`) {
		t.Errorf("import result = %q, want accounts:0 (mapped, not created)", out)
	}
}

// TestImportCSVConnectorUsesProfileAndAccountFlags covers the csv connector path: --profile and
// --account build the connector config, resolved by fakeConnectors to a fixed record set.
func TestImportCSVConnectorUsesProfileAndAccountFlags(t *testing.T) {
	h := newHarness(t)
	h.connect.csvFixtures["mybank"] = fuiStream("csv", "statement.csv", "sha-2",
		fuiAccount("Checking", "Checking", "EUR"),
		fuiTx("Checking", "tx-1", "2026-11-01", -1000, "Pizza"),
	)
	out, _ := h.mustRun("import", "statement.csv", "--connector", "csv", "--profile", "mybank",
		"--create-accounts", "--yes", "--json")
	if !strings.Contains(out, `"transactions":1`) {
		t.Errorf("import result = %q, want transactions:1", out)
	}
}

// TestImportAlreadyImportedFlag covers the already-imported flag: importing the same source
// digest twice surfaces already_imported_batch_id on the second dry-run.
func TestImportAlreadyImportedFlag(t *testing.T) {
	h := newHarness(t)
	file := writeFUIFile(t, t.TempDir(), "statement.fui.jsonl",
		fuiStream("fui", "statement.fui.jsonl", "sha-dup",
			fuiAccount("acc-1", "Checking", "EUR"),
			fuiTx("acc-1", "tx-1", "2026-11-01", -1000, "Pizza"),
		)...)
	h.mustRun("import", file, "--create-accounts", "--yes")

	out, _ := h.mustRun("import", file, "--dry-run", "--json")
	if !strings.Contains(out, "already_imported_batch_id") {
		t.Errorf("second plan = %q, want already_imported_batch_id", out)
	}
}

// TestImportUndoRemovesBatchAndTransactions covers `import undo`: it removes the batch's
// transactions and the batch itself.
func TestImportUndoRemovesBatchAndTransactions(t *testing.T) {
	h := newHarness(t)
	file := writeFUIFile(t, t.TempDir(), "statement.fui.jsonl",
		fuiStream("fui", "statement.fui.jsonl", "sha-1",
			fuiAccount("acc-1", "Checking", "EUR"),
			fuiTx("acc-1", "tx-1", "2026-11-01", -1000, "Pizza"),
		)...)
	resOut, _ := h.mustRun("import", file, "--create-accounts", "--yes", "--json")
	id := importBatchID(t, resOut)

	out, _ := h.mustRun("import", "undo", strconv.FormatInt(id, 10), "--yes", "--json")
	if !strings.Contains(out, `"removed_transactions":1`) {
		t.Errorf("undo result = %q, want removed_transactions:1", out)
	}

	listOut, _ := h.mustRun("import", "list", "--json")
	if strings.TrimSpace(listOut) != "[]" && strings.TrimSpace(listOut) != "null" {
		t.Errorf("import list after undo = %q, want empty", listOut)
	}
}

// importBatchID extracts "batch_id":N from a JSON import result for the next command.
func importBatchID(t *testing.T, jsonOut string) int64 {
	t.Helper()
	const key = `"batch_id":`
	i := strings.Index(jsonOut, key)
	if i < 0 {
		t.Fatalf("no batch_id in %q", jsonOut)
	}
	rest := jsonOut[i+len(key):]
	end := strings.IndexAny(rest, ",}")
	if end < 0 {
		t.Fatalf("malformed batch_id in %q", jsonOut)
	}
	id, err := strconv.ParseInt(rest[:end], 10, 64)
	if err != nil {
		t.Fatalf("parse batch_id from %q: %v", jsonOut, err)
	}
	return id
}
