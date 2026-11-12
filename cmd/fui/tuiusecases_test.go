package main

import (
	"context"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/cli"
	"github.com/0xballo/fui/internal/clock"
	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
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
