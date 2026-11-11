package cli_test

import (
	"strings"
	"testing"
)

// seedSpendingHistory adds one Checking account and three months of grocery spending plus one
// income transaction, so report run and kpi runway have enough data: spending reports need any
// transaction in the requested period, and runway needs at least three complete months of
// outflows before "now" (fixed at 2026-11-11 by the harness clock).
func seedSpendingHistory(h *harness) {
	h.account("Checking", "checking", "EUR", "1000")
	h.category("Groceries", "expense")
	h.category("Salary", "income")
	h.mustRun("add", "-50", "Esselunga", "-c", "Groceries", "-a", "Checking", "--date", "2026-08-05")
	h.mustRun("add", "-60", "Esselunga", "-c", "Groceries", "-a", "Checking", "--date", "2026-09-05")
	h.mustRun("add", "-70", "Esselunga", "-c", "Groceries", "-a", "Checking", "--date", "2026-10-05")
	h.mustRun("add", "-80", "Esselunga", "-c", "Groceries", "-a", "Checking", "--date", "2026-11-05")
	h.mustRun("add", "2000", "Paycheck", "-c", "Salary", "-a", "Checking", "--date", "2026-11-01")
}

func TestReportRunCategoriesJSON(t *testing.T) {
	h := newHarness(t)
	seedSpendingHistory(h)
	stdout, _ := h.mustRun("report", "run", "categories", "--month", "2026-11", "--json")
	if !strings.Contains(stdout, `"category_name":"Groceries"`) {
		t.Fatalf("stdout missing Groceries row: %s", stdout)
	}
	if !strings.Contains(stdout, `"total":8000`) {
		t.Fatalf("stdout missing total 8000: %s", stdout)
	}
}

func TestReportRunCategoriesTable(t *testing.T) {
	h := newHarness(t)
	h.term = true
	seedSpendingHistory(h)
	stdout, _ := h.mustRun("report", "run", "categories", "--month", "2026-11")
	if !strings.Contains(stdout, "Groceries") || !strings.Contains(stdout, "80.00") {
		t.Fatalf("stdout missing expected table content: %s", stdout)
	}
}

func TestReportRunCategoriesCSV(t *testing.T) {
	h := newHarness(t)
	seedSpendingHistory(h)
	stdout, _ := h.mustRun("report", "run", "categories", "--month", "2026-11", "--csv")
	if !strings.Contains(stdout, "CATEGORY,TOTAL") || !strings.Contains(stdout, "Groceries,80.00") {
		t.Fatalf("stdout missing expected csv content: %s", stdout)
	}
}

func TestReportRunCashFlow(t *testing.T) {
	h := newHarness(t)
	seedSpendingHistory(h)
	stdout, _ := h.mustRun("report", "run", "cashflow", "--month", "2026-11", "--json")
	if !strings.Contains(stdout, `"total_inflow":200000`) {
		t.Fatalf("stdout missing total_inflow 200000: %s", stdout)
	}
	if !strings.Contains(stdout, `"total_outflow":8000`) {
		t.Fatalf("stdout missing total_outflow 8000: %s", stdout)
	}
}

func TestReportRunUnknownName(t *testing.T) {
	h := newHarness(t)
	seedSpendingHistory(h)
	_, stderr, code := h.run("report", "run", "bogus", "--json")
	if code == 0 {
		t.Fatalf("expected non-zero exit, got stdout code 0, stderr %q", stderr)
	}
	if !strings.Contains(stderr, "invalid_value") {
		t.Fatalf("stderr missing invalid_value code: %s", stderr)
	}
}

func TestReportRunAccountFilter(t *testing.T) {
	h := newHarness(t)
	seedSpendingHistory(h)
	h.account("Savings", "savings", "EUR", "0")
	h.mustRun("add", "-5", "Interest fee", "-c", "Groceries", "-a", "Savings", "--date", "2026-11-06")
	stdout, _ := h.mustRun("report", "run", "categories", "--month", "2026-11", "--account", "Checking", "--json")
	if !strings.Contains(stdout, `"total":8000`) {
		t.Fatalf("expected account filter to exclude Savings leg: %s", stdout)
	}
}

func TestReportRunCashFlowTableAndCSV(t *testing.T) {
	h := newHarness(t)
	h.term = true
	seedSpendingHistory(h)
	table, _ := h.mustRun("report", "run", "cashflow", "--month", "2026-11")
	if !strings.Contains(table, "MONTH") || !strings.Contains(table, "2,000.00") {
		t.Fatalf("cashflow table missing expected content: %s", table)
	}
	csv, _ := h.mustRun("report", "run", "cashflow", "--month", "2026-11", "--csv")
	if !strings.Contains(csv, "MONTH,INFLOW,OUTFLOW,NET") {
		t.Fatalf("cashflow csv missing header: %s", csv)
	}
}

func TestReportRunFromTo(t *testing.T) {
	h := newHarness(t)
	seedSpendingHistory(h)
	stdout, _ := h.mustRun("report", "run", "categories", "--from", "2026-11-01", "--to", "2026-11-30", "--json")
	if !strings.Contains(stdout, `"total":8000`) {
		t.Fatalf("stdout missing total 8000: %s", stdout)
	}
}

func TestReportRunUnknownAccount(t *testing.T) {
	h := newHarness(t)
	seedSpendingHistory(h)
	_, stderr, code := h.run("report", "run", "categories", "--account", "Nope", "--json")
	if code == 0 {
		t.Fatalf("expected non-zero exit for unknown account, stderr %q", stderr)
	}
}

func TestKPIRunway(t *testing.T) {
	h := newHarness(t)
	seedSpendingHistory(h)
	stdout, _ := h.mustRun("kpi", "runway", "--json")
	if !strings.Contains(stdout, `"currency":"EUR"`) {
		t.Fatalf("stdout missing currency: %s", stdout)
	}
	if !strings.Contains(stdout, `"months"`) || !strings.Contains(stdout, `"zero_date"`) {
		t.Fatalf("stdout missing expected runway fields: %s", stdout)
	}
}

func TestKPIRunwayNotEnoughData(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "1000")
	_, stderr, code := h.run("kpi", "runway", "--json")
	if code == 0 {
		t.Fatalf("expected non-zero exit for not-enough-data, stderr %q", stderr)
	}
	if !strings.Contains(stderr, "not_enough_data") {
		t.Fatalf("stderr missing not_enough_data code: %s", stderr)
	}
}

func TestKPIRunwayTable(t *testing.T) {
	h := newHarness(t)
	h.term = true
	seedSpendingHistory(h)
	stdout, _ := h.mustRun("kpi", "runway")
	if !strings.Contains(stdout, "AS OF") || !strings.Contains(stdout, "ZERO DATE") {
		t.Fatalf("stdout missing expected table headers: %s", stdout)
	}
}

func TestKPIRunwayCSVAndAccountFilter(t *testing.T) {
	h := newHarness(t)
	seedSpendingHistory(h)
	csv, _ := h.mustRun("kpi", "runway", "--csv", "--account", "Checking")
	if !strings.Contains(csv, "AS OF,LIQUIDITY,MONTHLY SPENDING,MONTHS,DAYS,ZERO DATE") {
		t.Fatalf("csv missing expected header: %s", csv)
	}
}

func TestKPIRunwayAt(t *testing.T) {
	h := newHarness(t)
	seedSpendingHistory(h)
	stdout, _ := h.mustRun("kpi", "runway", "--at", "2026-11-10", "--json")
	if !strings.Contains(stdout, `"as_of":"2026-11-10"`) {
		t.Fatalf("stdout missing as_of 2026-11-10: %s", stdout)
	}
}
