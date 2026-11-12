package rules_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/rules"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/testutil/memstore"
)

type spyBackup struct{ n int }

func (b *spyBackup) Backup(context.Context, string) (string, error) { b.n++; return "b", nil }
func (b *spyBackup) Restore(context.Context, string) error          { return nil }

type env struct {
	svc         *rules.Service
	store       *memstore.Store
	backup      *spyBackup
	acct        domain.Account
	food, other domain.Category
	shop        domain.Payee
	tag         domain.Tag
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func money(m int64) domain.Money { return must(domain.NewMoney(m, must(domain.ParseCurrency("EUR")))) }

func newEnv(t *testing.T) env {
	t.Helper()
	e := env{store: memstore.New(), backup: &spyBackup{}}
	e.svc = rules.New(e.store, e.backup)
	ctx := context.Background()
	err := e.store.WithTx(ctx, func(r app.Repos) (err error) {
		if e.acct, err = r.Accounts().Insert(ctx, domain.Account{
			Name: "Main", Kind: domain.AccountChecking, Currency: money(0).Currency(), OpeningBalance: money(0),
			OpenedOn: must(domain.ParseDate("2026-01-01")),
		}); err != nil {
			return err
		}
		if e.food, err = r.Categories().Insert(ctx, must(domain.NewCategory("Food", domain.CategoryExpense))); err != nil {
			return err
		}
		if e.other, err = r.Categories().Insert(ctx, must(domain.NewCategory("Other", domain.CategoryExpense))); err != nil {
			return err
		}
		if e.shop, err = r.Payees().Insert(ctx, must(domain.NewPayee("Shop"))); err != nil {
			return err
		}
		e.tag, err = r.Tags().Insert(ctx, must(domain.NewTag("weekly")))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func (e env) tx(t *testing.T, desc string, minor int64, mod func(*domain.Transaction)) domain.Transaction {
	t.Helper()
	tx := domain.Transaction{
		AccountID: e.acct.ID, Date: must(domain.ParseDate("2026-10-01")), Amount: money(minor), Description: desc,
		Status: domain.StatusBooked, Kind: domain.KindPayment,
	}
	if mod != nil {
		mod(&tx)
	}
	var out domain.Transaction
	err := e.store.WithTx(context.Background(), func(r app.Repos) (err error) {
		out, err = r.Transactions().Insert(context.Background(), tx)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func (e env) get(t *testing.T, id domain.TransactionID) (tx domain.Transaction, tags []domain.Tag) {
	t.Helper()
	err := e.store.Read(context.Background(), func(r app.Repos) (err error) {
		if tx, err = r.Transactions().Get(context.Background(), id); err != nil {
			return err
		}
		tags, err = r.Tags().TagsOf(context.Background(), id)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return tx, tags
}

func (e env) add(t *testing.T, req rules.AddRequest) domain.Rule {
	t.Helper()
	p, err := e.svc.PlanAdd(context.Background(), req)
	if err != nil {
		t.Fatalf("PlanAdd: %v", err)
	}
	r, err := e.svc.ApplyAdd(context.Background(), p)
	if err != nil {
		t.Fatalf("ApplyAdd: %v", err)
	}
	return r
}

func TestAddEditRemoveList(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	r := e.add(t, rules.AddRequest{
		Name: "groceries", Priority: 10,
		Conditions: rules.ConditionsSpec{DescriptionContains: " Super ", Sign: "Outflow", Accounts: []string{"main"}},
		Actions:    rules.ActionsSpec{Category: "food", Payee: "shop", Tags: []string{"weekly"}},
	})
	want := domain.Rule{
		Name: "groceries", Priority: 10, Enabled: true,
		Conditions: domain.RuleConditions{
			DescriptionContains: "super", AccountIDs: []domain.AccountID{e.acct.ID},
			Amount: domain.RuleAmountRange{Sign: domain.RuleSignOutflow},
		},
		Actions: domain.RuleActions{CategoryID: e.food.ID, PayeeID: e.shop.ID, TagIDs: []domain.TagID{e.tag.ID}},
	}
	if diff := cmp.Diff(want, r); diff != "" {
		t.Fatalf("rule (-want +got):\n%s", diff)
	}

	if _, err := e.svc.PlanAdd(ctx, rules.AddRequest{Name: "GROCERIES", Actions: rules.ActionsSpec{Category: "food"}}); !errors.Is(err, app.ErrAlreadyExists) {
		t.Fatalf("duplicate name: got %v", err)
	}
	if _, err := e.svc.PlanAdd(ctx, rules.AddRequest{Name: "x", Actions: rules.ActionsSpec{Category: "nope"}}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown category: got %v", err)
	}
	if _, err := e.svc.PlanAdd(ctx, rules.AddRequest{Name: "x"}); !errors.Is(err, domain.ErrMissingField) {
		t.Fatalf("no actions: got %v", err)
	}

	off := false
	prio := 5
	p, err := e.svc.PlanEdit(ctx, rules.EditRequest{
		Name: "Groceries", Priority: &prio, Enabled: &off,
		Actions: &rules.ActionsSpec{Category: "other"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.ApplyEdit(ctx, p); err != nil {
		t.Fatal(err)
	}
	list, err := e.svc.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("List = %v, %v", list, err)
	}
	if g := list[0]; g.Priority != 5 || g.Enabled || g.Actions.CategoryID != e.other.ID || g.Actions.PayeeID != 0 {
		t.Fatalf("edited rule = %+v", g)
	}
	if _, err := e.svc.PlanEdit(ctx, rules.EditRequest{Name: "groceries"}); !errors.Is(err, domain.ErrMissingField) {
		t.Fatalf("empty edit: got %v", err)
	}
	if _, err := e.svc.PlanEdit(ctx, rules.EditRequest{Name: "ghost", Priority: &prio}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("edit unknown: got %v", err)
	}

	rm, err := e.svc.PlanRemove(ctx, "groceries")
	if err != nil || rm.Changes[0].Op != app.OpDelete {
		t.Fatalf("PlanRemove = %+v, %v", rm, err)
	}
	if err := e.svc.ApplyRemove(ctx, rm); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.PlanRemove(ctx, "groceries"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("remove twice: got %v", err)
	}
}

func TestTest(t *testing.T) {
	e := newEnv(t)
	e.add(t, rules.AddRequest{
		Name: "coffee", Priority: 1, Conditions: rules.ConditionsSpec{DescriptionContains: "bar"},
		Actions: rules.ActionsSpec{Category: "food"},
	})
	e.add(t, rules.AddRequest{
		Name: "off", Disabled: true, Priority: 0, Conditions: rules.ConditionsSpec{DescriptionContains: "bar"},
		Actions: rules.ActionsSpec{Category: "other"},
	})
	ctx := context.Background()

	got, err := e.svc.Test(ctx, rules.TestRequest{Text: "Bar Roma", Account: "main", AmountMinor: -250})
	if err != nil || got.CategoryID != e.food.ID || got.CategoryRule != "coffee" {
		t.Fatalf("Test text = %+v, %v", got, err)
	}
	tx := e.tx(t, "no match", -100, nil)
	got, err = e.svc.Test(ctx, rules.TestRequest{TxID: tx.ID})
	if err != nil || got.CategoryRule != "" {
		t.Fatalf("Test tx = %+v, %v", got, err)
	}
	if _, err := e.svc.Test(ctx, rules.TestRequest{TxID: 999}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Test unknown tx: %v", err)
	}
	if _, err := e.svc.Test(ctx, rules.TestRequest{Text: "x", Account: "zzz"}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Test unknown account: %v", err)
	}
}

func filter(t *testing.T, spec domain.TxFilterSpec) domain.TxFilter {
	t.Helper()
	return must(domain.NewTxFilter(spec))
}

func TestApplyFillsUnsetAndIsIdempotent(t *testing.T) {
	e := newEnv(t)
	e.add(t, rules.AddRequest{
		Name: "bar", Priority: 1, Conditions: rules.ConditionsSpec{DescriptionContains: "bar"},
		Actions: rules.ActionsSpec{Category: "food", Payee: "shop", Tags: []string{"weekly"}},
	})
	plain := e.tx(t, "Bar Roma", -300, nil)
	kept := e.tx(t, "Bar Milano", -300, func(x *domain.Transaction) { x.CategoryID = e.other.ID })
	e.tx(t, "Cinema", -300, nil)
	rec := e.tx(t, "Bar Napoli", -300, func(x *domain.Transaction) { x.Status = domain.StatusReconciled })
	ctx := context.Background()

	p, err := e.svc.PlanApply(ctx, rules.ApplyRequest{Filter: filter(t, domain.TxFilterSpec{})})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Effects) != 2 || p.Skipped != 1 || !p.Bulk || len(p.Changes) != 2 {
		t.Fatalf("plan = %d effects, %d skipped, bulk=%v", len(p.Effects), p.Skipped, p.Bulk)
	}
	if err := e.svc.ApplyApply(ctx, p); err != nil {
		t.Fatal(err)
	}
	if e.backup.n != 1 {
		t.Fatalf("backups = %d, want 1", e.backup.n)
	}
	if tx, tags := e.get(t, plain.ID); tx.CategoryID != e.food.ID || tx.PayeeID != e.shop.ID || len(tags) != 1 {
		t.Fatalf("plain = %+v tags=%v", tx, tags)
	}
	if tx, tags := e.get(t, kept.ID); tx.CategoryID != e.other.ID || tx.PayeeID != e.shop.ID || len(tags) != 1 {
		t.Fatalf("kept category = %+v tags=%v", tx, tags)
	}
	if tx, _ := e.get(t, rec.ID); tx.CategoryID != 0 {
		t.Fatalf("reconciled tx changed: %+v", tx)
	}

	again, err := e.svc.PlanApply(ctx, rules.ApplyRequest{Filter: filter(t, domain.TxFilterSpec{})})
	if err != nil || len(again.Effects) != 0 || again.Bulk {
		t.Fatalf("re-apply plan = %+v, %v", again, err)
	}
	if err := e.svc.ApplyApply(ctx, again); err != nil || e.backup.n != 1 {
		t.Fatalf("empty apply: %v, backups=%d", err, e.backup.n)
	}

	forced, err := e.svc.PlanApply(ctx, rules.ApplyRequest{Filter: filter(t, domain.TxFilterSpec{}), ConfirmReconciled: true})
	if err != nil || len(forced.Effects) != 1 || forced.Skipped != 0 {
		t.Fatalf("confirmed plan = %+v, %v", forced, err)
	}
}

func TestPlanApplyDoesNotWrite(t *testing.T) {
	e := newEnv(t)
	e.add(t, rules.AddRequest{Name: "bar", Conditions: rules.ConditionsSpec{DescriptionContains: "bar"}, Actions: rules.ActionsSpec{Category: "food"}})
	e.tx(t, "bar", -1, nil)
	before := e.store.Snapshot()
	if _, err := e.svc.PlanApply(context.Background(), rules.ApplyRequest{Filter: filter(t, domain.TxFilterSpec{})}); err != nil {
		t.Fatal(err)
	}
	if e.store.Snapshot() != before {
		t.Fatal("PlanApply wrote")
	}
}
