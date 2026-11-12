package cli_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// update is set by "go test ./internal/cli/... -run TestGolden -update" to refresh the golden
// files below instead of comparing against them, per the technical spec's "Golden file come
// contratto": every refresh must show up in the PR diff and be explained in the commit message.
var update = flag.Bool("update", false, "update golden files instead of comparing against them")

// checkGolden compares got against testdata/golden/name, or writes it there with -update.
func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden file %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file %s: %v (run with -update to create it)", path, err)
	}
	if string(want) != string(got) {
		t.Errorf("output does not match golden file %s (run with -update to refresh it)\ngot:\n%s\nwant:\n%s",
			path, got, want)
	}
}

// TestGoldenAccountList covers the --json and --csv renderers of a list command.
func TestGoldenAccountList(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "1000")
	h.account("Savings", "savings", "EUR", "5000")

	outJSON, _ := h.mustRun("account", "list", "--json")
	checkGolden(t, "account_list.json", []byte(outJSON))

	outCSV, _ := h.mustRun("account", "list", "--csv")
	checkGolden(t, "account_list.csv", []byte(outCSV))
}

// TestGoldenTxList covers the --json renderer of a list command with money and dates.
func TestGoldenTxList(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "0")
	h.category("Groceries", "expense")
	h.mustRun("add", "-12.50", "Esselunga", "-c", "Groceries", "-a", "Checking", "--date", "2026-01-05")

	out, _ := h.mustRun("tx", "list", "--json")
	checkGolden(t, "tx_list.json", []byte(out))
}

// TestGoldenHelpJSON freezes the shape of "fui help --json": the full command tree with flags,
// examples and output schema names, used by scripts and agents to discover the CLI.
func TestGoldenHelpJSON(t *testing.T) {
	h := newHarness(t)
	out, _ := h.mustRun("help", "--json")
	checkGolden(t, "help.json", []byte(out))
}

// TestGoldenModuleListJSON freezes the stable machine-readable module list.
func TestGoldenModuleListJSON(t *testing.T) {
	h := newHarness(t)
	out, _ := h.mustRun("module", "list", "--json")
	checkGolden(t, "module_list.json", []byte(out))
}

// TestGoldenReportRunCategories covers the --json renderer of report run categories.
func TestGoldenReportRunCategories(t *testing.T) {
	h := newHarness(t)
	seedSpendingHistory(h)
	out, _ := h.mustRun("report", "run", "categories", "--month", "2026-11", "--json")
	checkGolden(t, "report_run_categories.json", []byte(out))
}

// TestGoldenKPIRunway covers the --json renderer of kpi runway.
func TestGoldenKPIRunway(t *testing.T) {
	h := newHarness(t)
	seedSpendingHistory(h)
	out, _ := h.mustRun("kpi", "runway", "--at", "2026-11-10", "--json")
	checkGolden(t, "kpi_runway.json", []byte(out))
}
