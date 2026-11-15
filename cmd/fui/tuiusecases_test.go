package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app/investing"
	"github.com/0xballo/fui/internal/app/recurring"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/app/simulating"
	"github.com/0xballo/fui/internal/cli"
	"github.com/0xballo/fui/internal/clock"
	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/store/sqlite"
)

// emptySource is a reporting.Source over an archive with no data.
type emptySource struct{}

func (emptySource) ListAccounts(context.Context, reporting.AccountFilter) ([]domain.Account, error) {
	return nil, nil
}

func (emptySource) ListCategories(context.Context, []domain.CategoryID) ([]domain.Category, error) {
	return nil, nil
}

func (emptySource) ListTransactions(context.Context, reporting.TransactionFilter) ([]domain.Transaction, error) {
	return nil, nil
}

func (emptySource) BalanceAt(context.Context, domain.AccountID, domain.Date) (domain.Money, error) {
	return domain.Money{}, nil
}

func (emptySource) FXRateOnOrBefore(context.Context, domain.Currency, domain.Currency, domain.Date) (domain.Rate, error) {
	var r domain.Rate
	return r, nil
}

// TestTUIUseCasesForwardReports checks the dashboard adapter reaches the real reporting use cases
// (an empty archive yields no spending rows).
func TestTUIUseCasesForwardReports(t *testing.T) {
	clk := clock.NewFixed(time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC))
	cfg := config.Defaults()
	eur, err := domain.ParseCurrency("EUR")
	if err != nil {
		t.Fatal(err)
	}
	rep, err := newReporting(nil, emptySource{}, clk, &cfg, eur)
	if err != nil {
		t.Fatalf("newReporting: %v", err)
	}
	uc := newTUIUseCases(nil, rep)
	ctx := context.Background()
	asOf, err := domain.NewDate(2026, 1, 15)
	if err != nil {
		t.Fatal(err)
	}
	// With no accounts the real runway use case rejects the request: proof the call is forwarded.
	if _, err := uc.Runway(ctx, reporting.RunwayParams{AsOf: asOf, MonthStartDay: 1, BaseCurrency: eur}); err == nil {
		t.Fatal("Runway on an empty archive: want the use case's error")
	}
	from, err := domain.NewDate(2026, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	period, err := domain.NewPeriod(from, asOf)
	if err != nil {
		t.Fatal(err)
	}
	res, err := uc.Spending(ctx, reporting.SpendingParams{Period: period, MonthStartDay: 1, BaseCurrency: eur})
	if err != nil {
		t.Fatalf("Spending: %v", err)
	}
	if len(res.Rows) != 0 {
		t.Fatalf("Spending rows = %d, want 0", len(res.Rows))
	}
}

// TestTUIConfigWiresV02Screens checks the composition root hands the TUI every v0.2 use case, and
// that the module set comes from the configuration.
func TestTUIConfigWiresV02Screens(t *testing.T) {
	cfg := config.Defaults()
	eur, err := domain.ParseCurrency("EUR")
	if err != nil {
		t.Fatal(err)
	}
	tc := tuiConfig(&cfg, &cli.Deps{}, nil, clock.NewFixed(time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)), eur)
	if tc.Budget == nil || tc.Report == nil || tc.Importer == nil || tc.Settings == nil ||
		tc.ModuleConfig == nil || tc.ThemeConfig == nil {
		t.Fatalf("tuiConfig left a v0.2 port unset: %+v", tc)
	}
	if !tc.Modules.Has(module.Budget) {
		t.Fatalf("default modules should include budget: %v", tc.Modules.IDs())
	}
}

// TestTUIConfigWiresV03Features drives every v0.3 adapter over a real, empty archive: each call
// must reach its service (module gate open under the full preset) and succeed.
func TestTUIConfigWiresV03Features(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewFixed(time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC))
	store, err := sqlite.Open(ctx, sqlite.Options{Path: filepath.Join(t.TempDir(), "fui.db"), BackupDir: t.TempDir(), BackupKeep: 3, Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	cfg := config.Defaults()
	full, err := module.Preset("full")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Modules.Enabled = full.IDs()
	eur, err := domain.ParseCurrency("EUR")
	if err != nil {
		t.Fatal(err)
	}
	deps := &cli.Deps{}
	deps.Simulating = simulating.New(store, clk, simulating.Config{BaseCurrency: eur})
	deps.Recurring = recurring.New(store, clk, store, recurring.Config{})
	tc := tuiConfig(&cfg, deps, store, clk, eur)
	if tc.Positions == nil || tc.NetWorth == nil || tc.Insights == nil || tc.Compare == nil || tc.Simulations == nil {
		t.Fatalf("tuiConfig left a v0.3 port unset: %+v", tc)
	}
	if _, err := tc.Positions.ListPositions(ctx, investing.PositionsRequest{}); err != nil {
		t.Errorf("ListPositions: %v", err)
	}
	if _, err := tc.NetWorth.NetWorthAt(ctx, domain.Date{}); err != nil {
		t.Errorf("NetWorthAt: %v", err)
	}
	if _, err := tc.Insights.Run(ctx); err != nil {
		t.Errorf("Insights.Run: %v", err)
	}
	if _, err := tc.Compare.ComparePeriods(ctx, reporting.CompareRequest{A: "last-month", B: "this-month"}); err != nil {
		t.Errorf("ComparePeriods: %v", err)
	}
	if _, err := tc.Simulations.List(ctx); err != nil {
		t.Errorf("Simulations.List: %v", err)
	}
	if tc.Simulations.BaseCurrency() != eur {
		t.Errorf("BaseCurrency = %v", tc.Simulations.BaseCurrency())
	}
}
