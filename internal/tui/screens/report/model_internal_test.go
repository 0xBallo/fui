package report

import (
	"strings"
	"testing"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/theme"
)

// newTestModel builds a bare Model for the result-rendering unit tests below, which exercise
// refreshResultView directly over synthetic reporting results instead of running the real use
// cases (reporting.RemainingDebt and InterestPaid need the loans module, out of scope for these
// table-formatting checks).
func newTestModel() *Model {
	return New(nil, domain.LocaleENUS, 1, theme.Default())
}

func amt(t *testing.T, minor int64, code string) reporting.Amount {
	t.Helper()
	return reporting.Amount{Minor: minor, Currency: code}
}

func TestRefreshResultViewDebt(t *testing.T) {
	m := newTestModel()
	m.result = reportResult{kind: reporting.KindDebt, data: reporting.DebtResult{
		Loan: "Mortgage",
		SeriesResult: reporting.SeriesResult{
			Period: "2026-06-01..2026-06-30",
			Points: []reporting.Point{{Date: "2026-06-30", Amount: amt(t, -120000, "EUR")}},
		},
	}}
	m.refreshResultView()
	if m.note != "Loan: Mortgage" {
		t.Fatalf("note = %q, want loan name", m.note)
	}
	if got := m.table.View(); !strings.Contains(got, "2026-06-30") {
		t.Fatalf("table missing point, got:\n%s", got)
	}
}

func TestRefreshResultViewInterest(t *testing.T) {
	m := newTestModel()
	m.result = reportResult{kind: reporting.KindInterest, data: reporting.InterestResult{
		Period: "2026",
		Rows: []reporting.InterestRow{
			{Loan: "Mortgage", Year: 2026, Interest: amt(t, 50000, "EUR"), Fees: amt(t, 1000, "EUR")},
		},
	}}
	m.refreshResultView()
	got := m.table.View()
	if !strings.Contains(got, "Mortgage") || !strings.Contains(got, "2026") {
		t.Fatalf("expected loan/year in table, got:\n%s", got)
	}
}

func TestRefreshResultViewUnknownKind(t *testing.T) {
	m := newTestModel()
	m.result = reportResult{kind: "spending", data: struct{}{}}
	m.refreshResultView()
	if !strings.Contains(m.note, "no TUI view yet") {
		t.Fatalf("note = %q, want the no-view fallback", m.note)
	}
}

func TestFmtAmountInvalidCurrency(t *testing.T) {
	m := newTestModel()
	if got := m.fmtAmount(reporting.Amount{Minor: 100}); got != "?" {
		t.Fatalf("fmtAmount(invalid currency) = %q, want \"?\"", got)
	}
}

func TestViewFormAndFormTitle(t *testing.T) {
	m := newTestModel()
	m.openSaveForm()
	m.result.kind = reporting.KindNetWorth // openSaveForm needs a non-empty result kind
	m.openSaveForm()
	if got := m.View(); !strings.Contains(got, "Save report") || !strings.Contains(got, "Name:") {
		t.Fatalf("expected save form view, got:\n%s", got)
	}

	m.openExportForm()
	if got := m.View(); !strings.Contains(got, "Export transactions") {
		t.Fatalf("expected export form view, got:\n%s", got)
	}
}
