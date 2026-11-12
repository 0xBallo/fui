package cli

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/budgeting"
	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/testutil/memstore"
)

// newBudgetTestDeps builds Deps over a fresh, empty memstore with both Service and Budgeting
// wired over it, for the budget command tests (budget_*_test.go). These commands are not yet
// registered in root.go (see V32), so tests build and run them directly, the same way
// tag_test.go does with newTestDeps.
func newBudgetTestDeps(t *testing.T, mode domain.BudgetMode) *Deps {
	t.Helper()
	store := memstore.New()
	clock := internalFixedClock{time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)}
	eur, err := domain.ParseCurrency("EUR")
	if err != nil {
		t.Fatalf("ParseCurrency(EUR): %v", err)
	}
	svc := app.New(store, clock, internalFakeBackup{}, app.Config{
		Locale: domain.LocaleENUS, Location: time.UTC, BaseCurrency: eur,
	})
	bgt := budgeting.New(store, clock, budgeting.Config{
		Locale: domain.LocaleENUS, Location: time.UTC, BaseCurrency: eur, MonthStartDay: 1,
		Mode: mode, Modules: module.Default(),
	})
	cfg := &config.Config{
		Currency: "EUR", Locale: config.LocaleUS, MonthStartDay: 1,
		Budget:  config.BudgetConfig{Mode: string(mode)},
		Modules: config.ModulesConfig{Enabled: module.Default().IDs()},
	}
	return &Deps{
		Service: svc, Budgeting: bgt, Config: cfg, Stdin: &bytes.Buffer{}, Stdout: io.Discard,
		Stderr: io.Discard, Version: "test",
	}
}
