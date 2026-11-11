// Package screentest builds a fixture Service over an in-memory store, shared by the tests of
// every screen package so each one exercises the real use cases instead of hand-rolled mocks.
package screentest

import (
	"context"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/testutil/memstore"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type stubBackup struct{ n int }

func (b *stubBackup) Backup(context.Context, string) (string, error) {
	b.n++
	return "/backups/test.db", nil
}

func (b *stubBackup) Restore(context.Context, string) error { return nil }

// Fixture is a Service wired over a memstore, ready for a screen's constructor. Now is frozen at
// 2026-06-15 09:00 UTC, the locale en-US, and the base currency EUR unless Mutate overrides them.
type Fixture struct {
	Service *app.Service
	Store   *memstore.Store
}

// New builds an empty fixture, applying mutate (if any) to the default Config before Service
// construction.
func New(t *testing.T, mutate ...func(*app.Config)) Fixture {
	t.Helper()
	cur, err := domain.ParseCurrency("EUR")
	if err != nil {
		t.Fatalf("ParseCurrency: %v", err)
	}
	cfg := app.Config{Locale: domain.LocaleENUS, Location: time.UTC, BaseCurrency: cur}
	for _, m := range mutate {
		m(&cfg)
	}
	store := memstore.New()
	clock := fixedClock{now: time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)}
	svc := app.New(store, clock, &stubBackup{}, cfg)
	return Fixture{Service: svc, Store: store}
}

// Account creates an account and fails the test on error.
func (f Fixture) Account(t *testing.T, req app.AddAccountRequest) domain.Account {
	t.Helper()
	ctx := context.Background()
	plan, err := f.Service.PlanAddAccount(ctx, req)
	if err != nil {
		t.Fatalf("PlanAddAccount(%+v): %v", req, err)
	}
	a, err := f.Service.ApplyAddAccount(ctx, plan)
	if err != nil {
		t.Fatalf("ApplyAddAccount: %v", err)
	}
	return a
}

// Category creates a category and fails the test on error.
func (f Fixture) Category(t *testing.T, req app.AddCategoryRequest) domain.Category {
	t.Helper()
	ctx := context.Background()
	plan, err := f.Service.PlanAddCategory(ctx, req)
	if err != nil {
		t.Fatalf("PlanAddCategory(%+v): %v", req, err)
	}
	c, err := f.Service.ApplyAddCategory(ctx, plan)
	if err != nil {
		t.Fatalf("ApplyAddCategory: %v", err)
	}
	return c
}

// Tx creates a transaction and fails the test on error.
func (f Fixture) Tx(t *testing.T, req app.AddTxRequest) domain.Transaction {
	t.Helper()
	ctx := context.Background()
	plan, err := f.Service.PlanAddTx(ctx, req)
	if err != nil {
		t.Fatalf("PlanAddTx(%+v): %v", req, err)
	}
	tx, err := f.Service.ApplyAddTx(ctx, plan)
	if err != nil {
		t.Fatalf("ApplyAddTx: %v", err)
	}
	return tx
}
