package report_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	scr "github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/report"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/theme"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

// reportingSource adapts a Fixture's store into reporting.Source, mirroring the translation a
// real composition root performs (internal/tui/screens/screentest keeps its own copy private, so
// each screen that needs the full reporting.Analytics builds its own, same as budget does with
// fixedClock).
type reportingSource struct{ store app.Store }

func (s reportingSource) ListAccounts(ctx context.Context, filter reporting.AccountFilter) ([]domain.Account, error) {
	var out []domain.Account
	err := s.store.Read(ctx, func(r app.Repos) error {
		all, err := r.Accounts().List(ctx)
		if err != nil {
			return err
		}
		ids := map[domain.AccountID]bool{}
		for _, id := range filter.AccountIDs {
			ids[id] = true
		}
		for _, a := range all {
			if !filter.IncludeArchived && a.Archived {
				continue
			}
			if len(ids) > 0 && !ids[a.ID] {
				continue
			}
			out = append(out, a)
		}
		return nil
	})
	return out, err
}

func (s reportingSource) ListCategories(ctx context.Context, categoryIDs []domain.CategoryID) ([]domain.Category, error) {
	var out []domain.Category
	err := s.store.Read(ctx, func(r app.Repos) error {
		all, err := r.Categories().List(ctx)
		if err != nil {
			return err
		}
		if len(categoryIDs) == 0 {
			out = all
			return nil
		}
		ids := map[domain.CategoryID]bool{}
		for _, id := range categoryIDs {
			ids[id] = true
		}
		for _, c := range all {
			if ids[c.ID] {
				out = append(out, c)
			}
		}
		return nil
	})
	return out, err
}

func (s reportingSource) ListTransactions(ctx context.Context, filter reporting.TransactionFilter) ([]domain.Transaction, error) {
	var out []domain.Transaction
	err := s.store.Read(ctx, func(r app.Repos) error {
		f, err := domain.NewTxFilter(domain.TxFilterSpec{
			From: filter.Period.From(), To: filter.Period.To(), AccountIDs: filter.AccountIDs, Desc: true,
		})
		if err != nil {
			return err
		}
		txs, err := r.Transactions().List(ctx, f, nil, 0)
		out = txs
		return err
	})
	return out, err
}

func (s reportingSource) BalanceAt(ctx context.Context, accountID domain.AccountID, date domain.Date) (domain.Money, error) {
	var out domain.Money
	err := s.store.Read(ctx, func(r app.Repos) error {
		a, err := r.Accounts().Get(ctx, accountID)
		if err != nil {
			return err
		}
		sum, err := r.Transactions().SumUpTo(ctx, a.ID, date)
		if err != nil {
			return err
		}
		moves, err := domain.NewMoney(sum, a.Currency)
		if err != nil {
			return err
		}
		out, err = a.OpeningBalance.Add(moves)
		return err
	})
	return out, err
}

func (s reportingSource) FXRateOnOrBefore(ctx context.Context, from, _ domain.Currency, date domain.Date) (domain.Rate, error) {
	var out domain.Rate
	err := s.store.Read(ctx, func(r app.Repos) error {
		fx, err := r.FXRates().OnOrBefore(ctx, from, date)
		out = fx.Rate
		return err
	})
	return out, err
}

// testUseCases wires reporting.Analytics, importing.Service and app.Service's Today/ListAccounts
// into report.UseCases, same role the real composition root (task V32) will play.
type testUseCases struct {
	analytics *reporting.Analytics
	importer  *importing.Service
	core      *app.Service
}

func (u testUseCases) BalanceOverTime(ctx context.Context, req reporting.ReportRequest) (reporting.SeriesResult, error) {
	return u.analytics.BalanceOverTime(ctx, req)
}

func (u testUseCases) NetWorth(ctx context.Context, req reporting.ReportRequest) (reporting.SeriesResult, error) {
	return u.analytics.NetWorth(ctx, req)
}

func (u testUseCases) RemainingDebt(ctx context.Context, req reporting.ReportRequest) (reporting.DebtResult, error) {
	return u.analytics.RemainingDebt(ctx, req)
}

func (u testUseCases) InterestPaid(ctx context.Context, req reporting.ReportRequest) (reporting.InterestResult, error) {
	return u.analytics.InterestPaid(ctx, req)
}

func (u testUseCases) TopPayees(ctx context.Context, req reporting.ReportRequest) (reporting.PayeesResult, error) {
	return u.analytics.TopPayees(ctx, req)
}

func (u testUseCases) BudgetVsActual(ctx context.Context, req reporting.ReportRequest) (reporting.BudgetResult, error) {
	return u.analytics.BudgetVsActual(ctx, req)
}

func (u testUseCases) Forecast(ctx context.Context, req reporting.ForecastRequest) (reporting.ForecastResult, error) {
	return u.analytics.Forecast(ctx, req)
}

func (u testUseCases) List(ctx context.Context) ([]app.SavedReport, error) {
	return u.analytics.List(ctx)
}

func (u testUseCases) Run(ctx context.Context, ref string) (reporting.SavedResult, error) {
	return u.analytics.Run(ctx, ref)
}

func (u testUseCases) PlanSave(ctx context.Context, name, kind string, spec reporting.SavedSpec) (reporting.SavePlan, error) {
	return u.analytics.PlanSave(ctx, name, kind, spec)
}

func (u testUseCases) ApplySave(ctx context.Context, p reporting.SavePlan) (app.SavedReport, error) {
	return u.analytics.ApplySave(ctx, p)
}

func (u testUseCases) PlanRemove(ctx context.Context, ref string) (reporting.RemovePlan, error) {
	return u.analytics.PlanRemove(ctx, ref)
}

func (u testUseCases) ApplyRemove(ctx context.Context, p reporting.RemovePlan) error {
	return u.analytics.ApplyRemove(ctx, p)
}

func (u testUseCases) Today() (domain.Date, error) { return u.core.Today() }

func (u testUseCases) ListAccounts(ctx context.Context, req app.ListAccountsRequest) ([]app.AccountBalance, error) {
	return u.core.ListAccounts(ctx, req)
}

func (u testUseCases) Export(ctx context.Context, req importing.ExportRequest, w io.Writer) (int, error) {
	return u.importer.Export(ctx, req, w)
}

type stubBackup struct{}

func (stubBackup) Backup(context.Context, string) (string, error) { return "", nil }
func (stubBackup) Restore(context.Context, string) error          { return nil }

// harness wires reporting.Analytics and importing.Service over the same store screentest.Fixture
// built, so tests exercise the real use cases, the same convention budget's tests follow for
// budgeting.Service.
type harness struct {
	fx  screentest.Fixture
	uc  testUseCases
	now time.Time
	eur domain.Currency
}

func newHarness(t *testing.T) harness {
	t.Helper()
	fx := screentest.New(t)
	eur, err := domain.ParseCurrency("EUR")
	if err != nil {
		t.Fatalf("ParseCurrency: %v", err)
	}
	now := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	clock := fixedClock{now: now}
	src := reportingSource{store: fx.Store}
	analytics := reporting.NewAnalytics(fx.Store, clock, src, reporting.Config{
		Locale: domain.LocaleENUS, BaseCurrency: eur, MonthStartDay: 1, Location: time.UTC, Modules: module.Default(),
	})
	importer := importing.New(fx.Store, clock, stubBackup{}, nil)
	uc := testUseCases{analytics: analytics, importer: importer, core: fx.Service}
	return harness{fx: fx, uc: uc, now: now, eur: eur}
}

func newFixture(t *testing.T, h harness) scr.Screen {
	t.Helper()
	s := scr.Screen(report.New(h.uc, domain.LocaleENUS, 1, theme.Default()))
	s.SetSize(120, 30)
	return s
}

func run(t *testing.T, s scr.Screen, cmd tea.Cmd) scr.Screen {
	t.Helper()
	for cmd != nil {
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				s = run(t, s, c)
			}
			return s
		}
		s, cmd = s.Update(msg)
	}
	return s
}

func sendKey(s scr.Screen, k tea.KeyType) (scr.Screen, tea.Cmd) {
	return s.Update(tea.KeyMsg{Type: k})
}

func sendRunes(s scr.Screen, str string) (scr.Screen, tea.Cmd) {
	return s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(str)})
}

func TestReportTitleAndKeyMap(t *testing.T) {
	h := newHarness(t)
	s := newFixture(t, h)
	if s.Title() != "Reports" {
		t.Fatalf("Title() = %q", s.Title())
	}
	if len(s.KeyMap()) == 0 {
		t.Fatal("KeyMap() is empty")
	}
	if s.InputActive() {
		t.Fatal("InputActive() = true before any form is open")
	}
}

func TestReportDefaultNetWorth(t *testing.T) {
	h := newHarness(t)
	h.fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})

	s := newFixture(t, h)
	s = run(t, s, s.Init())

	view := s.View()
	if !strings.Contains(view, "Net worth") {
		t.Fatalf("expected default report to be Net worth, got:\n%s", view)
	}
	if !strings.Contains(view, "1,000.00") && !strings.Contains(view, "1000.00") {
		t.Fatalf("expected opening balance in view, got:\n%s", view)
	}
}

func TestReportCycleKinds(t *testing.T) {
	h := newHarness(t)
	h.fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})
	s := newFixture(t, h)
	s = run(t, s, s.Init())

	s, cmd := sendKey(s, tea.KeyRight)
	s = run(t, s, cmd)
	if !strings.Contains(s.View(), "Budget vs actual") {
		t.Fatalf("expected Budget vs actual after cycling right, got:\n%s", s.View())
	}

	s, cmd = sendKey(s, tea.KeyLeft)
	s = run(t, s, cmd)
	if !strings.Contains(s.View(), "Net worth") {
		t.Fatalf("expected Net worth after cycling back, got:\n%s", s.View())
	}
}

func TestReportBalanceOverTimeWithAccountParam(t *testing.T) {
	h := newHarness(t)
	h.fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})
	s := newFixture(t, h)
	s = run(t, s, s.Init())

	// Cycle to "Balance over time" (index 4 from Net worth, moving right 4 times). Its
	// automatic run errors because no account is set yet; the root (not under test here)
	// surfaces that on the status bar, so the report view itself stays on the previous kind
	// until the params form below supplies an account and reruns it.
	var cmd tea.Cmd
	for range 4 {
		s, cmd = sendKey(s, tea.KeyRight)
		s = run(t, s, cmd)
	}

	// Open the params form: leave the period field blank (enter moves to the next field), type
	// the account name, submit.
	s, cmd = sendRunes(s, "p")
	s = run(t, s, cmd)
	if !s.InputActive() {
		t.Fatal("InputActive() = false with the params form open")
	}
	s, cmd = sendKey(s, tea.KeyEnter) // period -> account
	s = run(t, s, cmd)
	s, cmd = sendRunes(s, "Checking")
	s = run(t, s, cmd)
	s, cmd = sendKey(s, tea.KeyEnter) // accept the "Checking" suggestion
	s = run(t, s, cmd)
	s, cmd = sendKey(s, tea.KeyEnter) // submit
	s = run(t, s, cmd)

	if s.InputActive() {
		t.Fatal("InputActive() = true after submitting the params form")
	}
	if !strings.Contains(s.View(), "Balance over time") {
		t.Fatalf("expected Balance over time, got:\n%s", s.View())
	}
	if !strings.Contains(s.View(), "2026-06") {
		t.Fatalf("expected a June 2026 balance point, got:\n%s", s.View())
	}
}

func TestReportTopPayees(t *testing.T) {
	h := newHarness(t)
	h.fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})
	h.fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-50", Date: "2026-06-05", Status: "booked"})

	s := newFixture(t, h)
	s = run(t, s, s.Init())

	var cmd tea.Cmd
	for range 2 {
		s, cmd = sendKey(s, tea.KeyRight)
		s = run(t, s, cmd)
	}
	if !strings.Contains(s.View(), "Top payees") {
		t.Fatalf("expected Top payees, got:\n%s", s.View())
	}
	if !strings.Contains(s.View(), "TOTAL") {
		t.Fatalf("expected a TOTAL row, got:\n%s", s.View())
	}
}

func TestReportForecast(t *testing.T) {
	h := newHarness(t)
	h.fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})

	s := newFixture(t, h)
	s = run(t, s, s.Init())

	var cmd tea.Cmd
	for range 3 {
		s, cmd = sendKey(s, tea.KeyRight)
		s = run(t, s, cmd)
	}
	if !strings.Contains(s.View(), "Forecast") {
		t.Fatalf("expected Forecast, got:\n%s", s.View())
	}
}

func TestReportSaveListRunDelete(t *testing.T) {
	h := newHarness(t)
	h.fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})
	s := newFixture(t, h)
	s = run(t, s, s.Init())

	// Save the default Net worth report.
	s, cmd := sendRunes(s, "s")
	s = run(t, s, cmd)
	if !s.InputActive() {
		t.Fatal("InputActive() = false with the save form open")
	}
	s, cmd = sendRunes(s, "my-networth")
	s = run(t, s, cmd)
	s, cmd = sendKey(s, tea.KeyEnter)
	s = run(t, s, cmd)

	// Open the saved list and check it shows up.
	s, cmd = sendRunes(s, "v")
	s = run(t, s, cmd)
	if !strings.Contains(s.View(), "my-networth") {
		t.Fatalf("expected saved report in list, got:\n%s", s.View())
	}

	// Run it from the list.
	s, cmd = sendKey(s, tea.KeyEnter)
	s = run(t, s, cmd)
	if !strings.Contains(s.View(), "saved: my-networth") {
		t.Fatalf("expected saved-origin in title, got:\n%s", s.View())
	}

	// Back to the saved list, delete it with confirmation.
	s, cmd = sendRunes(s, "v")
	s = run(t, s, cmd)
	s, cmd = sendRunes(s, "d")
	s = run(t, s, cmd)
	if !strings.Contains(s.View(), "Remove saved report") {
		t.Fatalf("expected confirm dialog, got:\n%s", s.View())
	}
	s, cmd = sendRunes(s, "y")
	s = run(t, s, cmd)
	if strings.Contains(s.View(), "my-networth") {
		t.Fatalf("expected saved report removed, got:\n%s", s.View())
	}
}

func TestReportExportTransactions(t *testing.T) {
	h := newHarness(t)
	h.fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})
	h.fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-50", Description: "Groceries run", Date: "2026-06-05", Status: "booked"})

	s := newFixture(t, h)
	s = run(t, s, s.Init())

	dir := t.TempDir()
	path := filepath.Join(dir, "export.csv")

	s, cmd := sendRunes(s, "e")
	s = run(t, s, cmd)
	if !s.InputActive() {
		t.Fatal("InputActive() = false with the export form open")
	}
	s, cmd = sendRunes(s, path)
	s = run(t, s, cmd)
	s, cmd = sendKey(s, tea.KeyEnter) // move to the format field
	s = run(t, s, cmd)
	s, cmd = sendKey(s, tea.KeyEnter) // submit with the default csv format
	run(t, s, cmd)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", path, err)
	}
	if !strings.Contains(string(data), "Groceries run") {
		t.Fatalf("expected exported CSV to contain the description, got:\n%s", data)
	}
}

func TestReportExportMissingPathError(t *testing.T) {
	h := newHarness(t)
	s := newFixture(t, h)
	s = run(t, s, s.Init())

	s, cmd := sendRunes(s, "e")
	s = run(t, s, cmd)
	s, cmd = sendKey(s, tea.KeyEnter) // move to the format field, path still empty
	s = run(t, s, cmd)
	_, cmd = sendKey(s, tea.KeyEnter) // submit
	if cmd == nil {
		t.Fatal("expected a command for the missing-path error")
	}
	msg := cmd()
	if _, ok := msg.(scr.ErrorMsg); !ok {
		t.Fatalf("expected screens.ErrorMsg, got %T", msg)
	}
}
