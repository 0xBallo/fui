package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/testutil/memstore"
)

// tb is the subset of testing.TB shared by *testing.T and *rapid.T.
type tb interface {
	Helper()
	Fatalf(format string, args ...any)
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type spyBackup struct {
	reasons  []string
	restored []string
}

func (b *spyBackup) Backup(_ context.Context, reason string) (string, error) {
	b.reasons = append(b.reasons, reason)
	return "/backups/" + reason + ".db", nil
}

func (b *spyBackup) Restore(_ context.Context, path string) error {
	b.restored = append(b.restored, path)
	return nil
}

type env struct {
	svc    *app.Service
	store  *memstore.Store
	backup *spyBackup
}

// newEnv builds a Service over an empty memstore. "Now" is 2026-11-11 10:00 UTC, the zone is
// UTC, the locale en-US and the base currency EUR unless cfg says otherwise.
func newEnv(t tb, mutate ...func(*app.Config)) env {
	t.Helper()
	cfg := app.Config{Locale: domain.LocaleENUS, Location: time.UTC, BaseCurrency: currency(t, "EUR")}
	for _, m := range mutate {
		m(&cfg)
	}
	e := env{store: memstore.New(), backup: &spyBackup{}}
	e.svc = app.New(e.store, fixedClock{time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)}, e.backup, cfg)
	return e
}

func currency(t tb, code string) domain.Currency {
	t.Helper()
	c, err := domain.ParseCurrency(code)
	if err != nil {
		t.Fatalf("ParseCurrency(%q): %v", code, err)
	}
	return c
}

func (e env) account(t tb, name, kind, cur, opening string) domain.Account {
	t.Helper()
	ctx := context.Background()
	p, err := e.svc.PlanAddAccount(ctx, app.AddAccountRequest{
		Name: name, Kind: kind, Currency: cur, OpeningBalance: opening, OpenedOn: "2026-01-01",
	})
	if err != nil {
		t.Fatalf("PlanAddAccount(%s): %v", name, err)
	}
	a, err := e.svc.ApplyAddAccount(ctx, p)
	if err != nil {
		t.Fatalf("ApplyAddAccount(%s): %v", name, err)
	}
	return a
}

func (e env) category(t tb, name, kind string) domain.Category {
	t.Helper()
	ctx := context.Background()
	p, err := e.svc.PlanAddCategory(ctx, app.AddCategoryRequest{Name: name, Kind: kind})
	if err != nil {
		t.Fatalf("PlanAddCategory(%s): %v", name, err)
	}
	c, err := e.svc.ApplyAddCategory(ctx, p)
	if err != nil {
		t.Fatalf("ApplyAddCategory(%s): %v", name, err)
	}
	return c
}

func (e env) tag(t tb, name string) domain.Tag {
	t.Helper()
	ctx := context.Background()
	p, err := e.svc.PlanAddTag(ctx, app.AddTagRequest{Name: name})
	if err != nil {
		t.Fatalf("PlanAddTag(%s): %v", name, err)
	}
	tg, err := e.svc.ApplyAddTag(ctx, p)
	if err != nil {
		t.Fatalf("ApplyAddTag(%s): %v", name, err)
	}
	return tg
}

func (e env) payee(t tb, name string, aliases ...string) domain.Payee {
	t.Helper()
	ctx := context.Background()
	p, err := e.svc.PlanAddPayee(ctx, app.AddPayeeRequest{Name: name, Aliases: aliases})
	if err != nil {
		t.Fatalf("PlanAddPayee(%s): %v", name, err)
	}
	py, err := e.svc.ApplyAddPayee(ctx, p)
	if err != nil {
		t.Fatalf("ApplyAddPayee(%s): %v", name, err)
	}
	return py
}

func (e env) tx(t tb, req app.AddTxRequest) domain.Transaction {
	t.Helper()
	ctx := context.Background()
	p, err := e.svc.PlanAddTx(ctx, req)
	if err != nil {
		t.Fatalf("PlanAddTx(%+v): %v", req, err)
	}
	tx, err := e.svc.ApplyAddTx(ctx, p)
	if err != nil {
		t.Fatalf("ApplyAddTx: %v", err)
	}
	return tx
}

// wantCode fails unless err carries the code of target.
func wantCode(t *testing.T, err error, target *domain.Error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("error = %v, want code %q", err, target.Code)
	}
}

func wantNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func ptr(s string) *string { return &s }
