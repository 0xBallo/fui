package cli_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app/lending"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

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

func TestForecastJSONAndTableChart(t *testing.T) {
	h := newHarness(t)
	seedSpendingHistory(h)

	jsonOut, _ := h.mustRun("forecast", "--horizon", "10", "--json")
	if !strings.Contains(jsonOut, `"days":10`) || !strings.Contains(jsonOut, `"total"`) {
		t.Fatalf("forecast --json = %q, want days and total", jsonOut)
	}

	tableOut, _ := h.mustRun("forecast", "--horizon", "5", "--chart")
	if !strings.Contains(tableOut, "DATE") || !strings.Contains(tableOut, "CHART") {
		t.Fatalf("forecast --chart = %q, want DATE/CHART columns", tableOut)
	}

	defaultOut, _ := h.mustRun("forecast", "--json")
	if !strings.Contains(defaultOut, `"days":90`) {
		t.Fatalf("forecast default horizon = %q, want days=90", defaultOut)
	}
}

func TestReportKindsJSON(t *testing.T) {
	h := newHarness(t)
	seedSpendingHistory(h)

	balance, _ := h.mustRun("report", "balance", "--account", "Checking", "--period", "2026-11", "--json")
	if !strings.Contains(balance, `"points"`) {
		t.Fatalf("report balance --json = %q, want points", balance)
	}

	networth, _ := h.mustRun("report", "networth", "--period", "2026-11", "--json")
	if !strings.Contains(networth, `"points"`) {
		t.Fatalf("report networth --json = %q, want points", networth)
	}

	budget, _ := h.mustRun("report", "budget-vs-actual", "--period", "2026-11", "--json")
	if !strings.Contains(budget, `"lines"`) {
		t.Fatalf("report budget-vs-actual --json = %q, want lines", budget)
	}

	payees, _ := h.mustRun("report", "top-payees", "--period", "2026-11", "--json")
	if !strings.Contains(payees, `"rows"`) {
		t.Fatalf("report top-payees --json = %q, want rows", payees)
	}
}

func TestReportSavedLifecycle(t *testing.T) {
	h := newHarness(t)
	seedSpendingHistory(h)

	saved, _ := h.mustRun("report", "save", "monthly-networth", "networth", "--period", "last-1-months", "--json")
	if !strings.Contains(saved, `"name":"monthly-networth"`) {
		t.Fatalf("report save = %q, want saved report name", saved)
	}

	list, _ := h.mustRun("report", "list", "--json")
	if !strings.Contains(list, `"monthly-networth"`) {
		t.Fatalf("report list = %q, want saved report", list)
	}

	run, _ := h.mustRun("report", "run", "monthly-networth", "--json")
	if !strings.Contains(run, `"kind":"net-worth"`) || !strings.Contains(run, `"data"`) {
		t.Fatalf("report run saved = %q, want kind/data", run)
	}

	removed, _ := h.mustRun("report", "rm", "monthly-networth", "--json")
	if !strings.Contains(removed, `"removed":"monthly-networth"`) {
		t.Fatalf("report rm = %q, want removed payload", removed)
	}

	h.mustRun("report", "save", "monthly-networth", "networth", "--period", "last-1-months")
	textRemoved, _ := h.mustRun("report", "rm", "monthly-networth", "--yes")
	if !strings.Contains(textRemoved, `Removed "monthly-networth".`) {
		t.Fatalf("report rm text = %q, want confirmation", textRemoved)
	}
}

func TestReportLoanKinds(t *testing.T) {
	h := newHarness(t)
	seedSpendingHistory(h)
	enableModules(t, h, module.Loans)
	addLoanFixture(t, h, "Checking")

	debt, _ := h.mustRun("report", "remaining-debt", "--loan", "Car", "--period", "2026-11", "--json")
	if !strings.Contains(debt, `"loan":"Car"`) {
		t.Fatalf("report remaining-debt --json = %q, want loan Car", debt)
	}
	interest, _ := h.mustRun("report", "interest-paid", "--loan", "Car", "--period", "2026-11", "--json")
	if !strings.Contains(interest, `"rows"`) {
		t.Fatalf("report interest-paid --json = %q, want rows", interest)
	}
}

func TestReportLegacyRunKinds(t *testing.T) {
	h := newHarness(t)
	seedSpendingHistory(h)

	categories, _ := h.mustRun("report", "run", "categories", "--month", "2026-11", "--json")
	if !strings.Contains(categories, `"rows"`) {
		t.Fatalf("report run categories --json = %q, want rows", categories)
	}
	filtered, _ := h.mustRun("report", "run", "categories", "--from", "2026-11-01", "--to", "2026-11-30", "--account", "Checking", "--json")
	if !strings.Contains(filtered, `"rows"`) {
		t.Fatalf("report run categories filtered --json = %q, want rows", filtered)
	}

	cashflow, _ := h.mustRun("report", "run", "cashflow", "--month", "2026-11", "--json")
	if !strings.Contains(cashflow, `"months"`) {
		t.Fatalf("report run cashflow --json = %q, want months", cashflow)
	}
}

func TestReportKindValidationErrors(t *testing.T) {
	h := newHarness(t)
	seedSpendingHistory(h)

	_, stderr, code := h.run("report", "balance", "--period", "bogus", "--json")
	if code != 1 || !strings.Contains(stderr, "invalid_value") {
		t.Fatalf("report balance invalid period: code=%d stderr=%q", code, stderr)
	}

	_, stderr, code = h.run("report", "balance", "--period", "2026-11", "--json")
	if code != 1 || !strings.Contains(stderr, "missing_field") {
		t.Fatalf("report balance missing account: code=%d stderr=%q", code, stderr)
	}

	_, stderr, code = h.run("report", "remaining-debt", "--period", "2026-11", "--json")
	if code != 1 || !strings.Contains(stderr, "missing_field") {
		t.Fatalf("report remaining-debt missing loan: code=%d stderr=%q", code, stderr)
	}

	_, stderr, code = h.run("report", "run", "categories", "--account", "Nope", "--json")
	if code != 1 || !strings.Contains(stderr, "not_found") {
		t.Fatalf("report run categories unknown account: code=%d stderr=%q", code, stderr)
	}
}

func enableModules(t *testing.T, h *harness, extra ...module.ID) {
	t.Helper()
	base := append([]module.ID{}, module.Default().IDs()...)
	base = append(base, extra...)
	set, err := module.Validate(base)
	if err != nil {
		t.Fatalf("module.Validate: %v", err)
	}
	h.cfg.Modules.Enabled = set.IDs()
	h.reporting.Analytics = reporting.NewAnalytics(
		h.store,
		fixedClock{time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)},
		reportingSource{store: h.store},
		reporting.Config{
			Locale:        domain.Locale(h.cfg.Locale),
			BaseCurrency:  currency(t, h.cfg.Currency),
			MonthStartDay: h.cfg.MonthStartDay,
			HorizonDays:   config.DefaultForecastDays,
			Modules:       set,
		},
	)
}

func addLoanFixture(t *testing.T, h *harness, debitAccount string) {
	t.Helper()
	enabled, err := h.cfg.ModulesEnabled()
	if err != nil {
		t.Fatalf("cfg.ModulesEnabled: %v", err)
	}
	loanSvc := lending.New(
		h.store,
		fixedClock{time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)},
		h.backup,
		lending.Config{Locale: domain.Locale(h.cfg.Locale), Location: time.UTC},
	)
	plan, err := loanSvc.PlanAdd(context.Background(), enabled, lending.AddRequest{
		Name:       "Car",
		Kind:       "loan",
		Principal:  "1200",
		Disbursed:  "2026-03-01",
		TermMonths: 12,
		RateType:   "fixed",
		Rate:       "2",
		Debit:      debitAccount,
	})
	if err != nil {
		t.Fatalf("PlanAdd loan: %v", err)
	}
	if _, err := loanSvc.ApplyAdd(context.Background(), plan); err != nil {
		t.Fatalf("ApplyAdd loan: %v", err)
	}
}
