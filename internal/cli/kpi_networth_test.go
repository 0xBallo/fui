package cli_test

import (
	"strings"
	"testing"

	"github.com/0xballo/fui/internal/module"
)

// TestKPINetWorthJSON covers the UC-04 "net worth" KPI in the single-currency case: the total
// equals the sum of account balances and each line carries its own and converted amount.
func TestKPINetWorthJSON(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "1000")
	h.account("Savings", "savings", "EUR", "500")

	out, _ := h.mustRun("kpi", "networth", "--json")
	if !strings.Contains(out, `"currency":"EUR"`) || !strings.Contains(out, `"total":150000`) {
		t.Fatalf("stdout = %q, want currency EUR and total 150000", out)
	}
	if !strings.Contains(out, `"name":"Checking"`) || !strings.Contains(out, `"name":"Savings"`) {
		t.Fatalf("stdout = %q, want both account lines", out)
	}
}

// TestKPINetWorthMultiCurrency covers a cross-currency account converted at the fx rate in
// force, summed into the base currency total.
func TestKPINetWorthMultiCurrency(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "1000")
	h.account("Revolut", "checking", "USD", "1000")
	h.mustRun("fx", "set", "USD", "2", "--date", "2026-01-01")

	out, _ := h.mustRun("kpi", "networth", "--json")
	if !strings.Contains(out, `"total":300000`) {
		t.Fatalf("stdout = %q, want total 300000 (1000 EUR + 1000 USD converted at rate 2 = 2000 EUR)", out)
	}
}

// TestKPINetWorthRateMissing covers the "rate_missing" error (exit 1) when a foreign-currency
// account has no fx rate recorded by the requested date.
func TestKPINetWorthRateMissing(t *testing.T) {
	h := newHarness(t)
	h.account("Revolut", "checking", "USD", "1000")

	_, stderr, code := h.run("kpi", "networth", "--json")
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "rate_missing") {
		t.Fatalf("stderr = %q, want rate_missing", stderr)
	}
}

// TestKPINetWorthTableAndCSV covers the table and --csv renderers; --csv must stay in machine
// format (dot decimal) even under the it-IT locale.
func TestKPINetWorthTableAndCSV(t *testing.T) {
	h := newHarness(t)
	h.useItalianLocale()
	h.account("Checking", "checking", "EUR", "1000")

	table, _ := h.mustRun("kpi", "networth")
	if !strings.Contains(table, "Checking") || !strings.Contains(table, "1.000,00") {
		t.Fatalf("table = %q, want it-IT formatted balance 1.000,00", table)
	}

	csv, _ := h.mustRun("kpi", "networth", "--csv")
	if !strings.Contains(csv, "1000.00") || strings.Contains(csv, "1.000,00") {
		t.Fatalf("csv = %q, want machine-format balance 1000.00, not locale-formatted", csv)
	}
}

// TestKPINetWorthAt covers the --at flag.
func TestKPINetWorthAt(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "1000")

	out, _ := h.mustRun("kpi", "networth", "--at", "2026-11-10", "--json")
	if !strings.Contains(out, `"as_of":"2026-11-10"`) {
		t.Fatalf("stdout = %q, want as_of 2026-11-10", out)
	}
}

func TestKPINetWorthIncludesLoanDebtWhenLoansEnabled(t *testing.T) {
	h := newHarness(t)
	enableModules(t, h, module.Loans)
	h.account("Checking", "checking", "EUR", "1000")
	h.account("Car Loan", "loan", "EUR", "-300")

	out, _ := h.mustRun("kpi", "networth", "--json")
	if !strings.Contains(out, `"total":70000`) {
		t.Fatalf("stdout = %q, want total 70000 with loan debt included", out)
	}
}
