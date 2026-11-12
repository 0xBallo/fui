package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExportToStdout covers the default path: no --output, the exported stream goes straight to
// stdout with no extra confirmation mixed in.
func TestExportToStdout(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "0")
	h.category("Food", "expense")
	h.mustRun("add", "-10", "Pizza", "-c", "Food", "-a", "Checking")

	out, _ := h.mustRun("export", "--format", "csv")
	if !strings.Contains(out, "Pizza") {
		t.Errorf("export output = %q, want it to contain the transaction", out)
	}
}

// TestExportToFileWritesConfirmation covers --output FILE: the stream is written to the file and
// a short confirmation is printed instead of the stream itself.
func TestExportToFileWritesConfirmation(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "0")
	h.category("Food", "expense")
	h.mustRun("add", "-10", "Pizza", "-c", "Food", "-a", "Checking")

	path := filepath.Join(t.TempDir(), "out.csv")
	out, _ := h.mustRun("export", "--format", "csv", "--output", path, "--json")
	if !strings.Contains(out, `"path"`) || !strings.Contains(out, path) {
		t.Errorf("export confirmation = %q, want it to mention %q", out, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read exported file: %v", err)
	}
	if !strings.Contains(string(data), "Pizza") {
		t.Errorf("exported file = %q, want it to contain the transaction", string(data))
	}
}

// TestExportAllDumpsEveryEntity covers --all --output DIR: one file per entity is created.
func TestExportAllDumpsEveryEntity(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "0")
	h.category("Food", "expense")
	h.mustRun("add", "-10", "Pizza", "-c", "Food", "-a", "Checking")

	dir := filepath.Join(t.TempDir(), "dump")
	h.mustRun("export", "--all", "--format", "json", "--output", dir)

	for _, name := range []string{"accounts.json", "categories.json", "transactions.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("stat %s: %v", name, err)
		}
	}
}

// TestExportAllRejectsFUIFormat covers "--all does not support --format fui" since Dump only
// accepts json/csv.
func TestExportAllRejectsFUIFormat(t *testing.T) {
	h := newHarness(t)
	_, stderr, code := h.run("export", "--all", "--format", "fui", "--output", t.TempDir())
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "fui") {
		t.Errorf("stderr = %q, want it to mention the rejected fui format", stderr)
	}
}

// TestExportAllRequiresOutput covers --all without --output failing with missing_field, since a
// directory of files cannot go to stdout.
func TestExportAllRequiresOutput(t *testing.T) {
	h := newHarness(t)
	_, stderr, code := h.run("export", "--all", "--json")
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, `"code":"missing_field"`) {
		t.Errorf("stderr = %q, want code missing_field", stderr)
	}
}
