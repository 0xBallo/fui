package recurring_test

import (
	"context"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/recurring"
	"github.com/0xballo/fui/internal/clock"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/testutil/memstore"
)

type spyBackup struct{ n int }

func (b *spyBackup) Backup(context.Context, string) (string, error) { b.n++; return "b", nil }
func (b *spyBackup) Restore(context.Context, string) error          { return nil }

type env struct {
	svc    *recurring.Service
	store  *memstore.Store
	backup *spyBackup
	clock  *clock.Fixed
	acct   domain.Account
	food   domain.Category
	all    module.Set
	none   module.Set
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func money(minor int64) domain.Money {
	return must(domain.NewMoney(minor, must(domain.ParseCurrency("EUR"))))
}

func newEnv(t *testing.T) env {
	t.Helper()
	e := env{
		store:  memstore.New(),
		backup: &spyBackup{},
		clock:  clock.NewFixed(must(domain.ParseDate("2026-01-15")).In(time.UTC)),
		all:    module.Default(),
		none:   module.Set{},
	}
	e.svc = recurring.New(e.store, e.clock, e.backup, recurring.Config{})
	ctx := context.Background()
	err := e.store.WithTx(ctx, func(r app.Repos) (err error) {
		if e.acct, err = r.Accounts().Insert(ctx, domain.Account{
			Name: "Main", Kind: domain.AccountChecking, Currency: money(0).Currency(), OpeningBalance: money(0),
			OpenedOn: must(domain.ParseDate("2026-01-01")),
		}); err != nil {
			return err
		}
		e.food, err = r.Categories().Insert(ctx, must(domain.NewCategory("Food", domain.CategoryExpense)))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return e
}
