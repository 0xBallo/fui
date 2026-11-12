package budgeting_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/budgeting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/testutil/memstore"
)

// tb is the subset of testing.TB shared by *testing.T and *rapid.T.
type tb interface {
	Helper()
	Fatal(args ...any)
	Fatalf(format string, args ...any)
}

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC) }

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

var eur = must(domain.ParseCurrency("EUR"))

func money(m int64) domain.Money { return must(domain.NewMoney(m, eur)) }
func day(s string) domain.Date   { return must(domain.ParseDate(s)) }

type env struct {
	store      *memstore.Store
	acct       domain.Account
	food, home domain.Category
	trip       domain.Tag
}

func newEnv(t tb) *env {
	t.Helper()
	e := &env{store: memstore.New()}
	ctx := context.Background()
	err := e.store.WithTx(ctx, func(r app.Repos) (err error) {
		if e.acct, err = r.Accounts().Insert(ctx, domain.Account{
			Name: "Main", Kind: domain.AccountChecking, Currency: eur,
			OpeningBalance: money(100000), OpenedOn: day("2026-01-01"),
		}); err != nil {
			return err
		}
		if e.food, err = r.Categories().Insert(ctx, must(domain.NewCategory("Food", domain.CategoryExpense))); err != nil {
			return err
		}
		if e.home, err = r.Categories().Insert(ctx, must(domain.NewCategory("Home", domain.CategoryExpense))); err != nil {
			return err
		}
		e.trip, err = r.Tags().Insert(ctx, must(domain.NewTag("trip")))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *env) spend(t tb, date string, minor int64, cat domain.Category, st domain.TxStatus) domain.Transaction {
	t.Helper()
	ctx := context.Background()
	var tx domain.Transaction
	err := e.store.WithTx(ctx, func(r app.Repos) (err error) {
		tx, err = r.Transactions().Insert(ctx, domain.Transaction{
			AccountID: e.acct.ID, Date: day(date), Amount: money(-minor),
			CategoryID: cat.ID, Status: st, Kind: domain.KindPayment,
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return tx
}

func svc(e *env, mode domain.BudgetMode, mods ...module.ID) *budgeting.Service {
	if mods == nil {
		mods = []module.ID{module.Core, module.Budget}
	}
	return budgeting.New(e.store, fixedClock{}, budgeting.Config{
		BaseCurrency: eur, Mode: mode, Modules: must(module.Validate(mods)), Location: time.UTC,
	})
}

func set(t tb, s *budgeting.Service, req budgeting.SetRequest) domain.Budget {
	t.Helper()
	ctx := context.Background()
	p, err := s.PlanSet(ctx, req)
	if err != nil {
		t.Fatalf("PlanSet(%+v): %v", req, err)
	}
	b, err := s.ApplySet(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestModuleGate(t *testing.T) {
	e := newEnv(t)
	s := svc(e, domain.BudgetCap, module.Core)
	ctx := context.Background()
	_, e1 := s.PlanSet(ctx, budgeting.SetRequest{Name: "x"})
	_, e2 := s.Status(ctx, "")
	_, e3 := s.List(ctx)
	_, e4 := s.PlanCopyPrevious(ctx, "")
	_, e5 := s.PlanMove(ctx, budgeting.MoveRequest{})
	for i, err := range []error{e1, e2, e3, e4, e5} {
		if !errors.Is(err, app.ErrModuleDisabled) {
			t.Errorf("use case %d: err = %v, want module_disabled", i, err)
		}
	}
}

func TestSetErrors(t *testing.T) {
	e := newEnv(t)
	s := svc(e, domain.BudgetCap)
	set(t, s, budgeting.SetRequest{Name: "Food", Categories: []string{"Food"}, Amount: "100"})
	tests := []struct {
		name string
		req  budgeting.SetRequest
		want error
	}{
		{"override of unknown budget", budgeting.SetRequest{Name: "Nope", Month: "2026-03", Amount: "5"}, domain.ErrNotFound},
		{"override without amount", budgeting.SetRequest{Name: "Food", Month: "2026-03"}, domain.ErrMissingField},
		{"no targets", budgeting.SetRequest{Name: "New", Amount: "5"}, domain.ErrMissingField},
		{"category already covered", budgeting.SetRequest{Name: "Food2", Categories: []string{"Food"}, Amount: "5"}, domain.ErrInvalidValue},
		{"unknown category", budgeting.SetRequest{Name: "Z", Categories: []string{"Zzz"}, Amount: "5"}, domain.ErrNotFound},
		{"bad month", budgeting.SetRequest{Name: "Food", Month: "x", Amount: "5"}, domain.ErrInvalidValue},
		{"bad amount", budgeting.SetRequest{Name: "Food", Amount: "abc"}, domain.ErrInvalidValue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := s.PlanSet(context.Background(), tt.req); err == nil {
				t.Fatal("want error")
			}
		})
	}
}

func TestCapStatusSplitPendingAndCarry(t *testing.T) {
	e := newEnv(t)
	s := svc(e, domain.BudgetCap)
	carry := true
	set(t, s, budgeting.SetRequest{Name: "Food", Categories: []string{"Food"}, Amount: "100", Carry: &carry})
	set(t, s, budgeting.SetRequest{Name: "Food", Month: "2026-03", Amount: "200"})
	e.spend(t, "2026-02-10", 6000, e.food, domain.StatusBooked) // Feb: 40 left, carried
	e.spend(t, "2026-03-05", 5000, e.food, domain.StatusBooked)
	e.spend(t, "2026-03-06", 3000, e.food, domain.StatusPending)
	e.spend(t, "2026-03-07", 700, e.home, domain.StatusBooked) // uncovered
	// a 100.00 payment split 60/40 between food and home: only 60 counts for Food.
	tx := e.spend(t, "2026-03-08", 10000, e.food, domain.StatusBooked)
	ctx := context.Background()
	if err := e.store.WithTx(ctx, func(r app.Repos) error {
		return r.Splits().ReplaceSplits(ctx, tx.ID, []domain.Split{
			{Category: e.food.ID, Amount: money(-6000)}, {Category: e.home.ID, Amount: money(-4000)},
		})
	}); err != nil {
		t.Fatal(err)
	}
	st, err := s.Status(ctx, "2026-03")
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Caps) != 1 {
		t.Fatalf("caps = %d", len(st.Caps))
	}
	c := st.Caps[0]
	if c.Amount.Minor() != 20000 || c.CarryIn.Minor() != 4000 || c.Spent.Minor() != 11000 ||
		c.Planned.Minor() != 3000 || c.Remaining.Minor() != 13000 {
		t.Errorf("cap = %+v", c.CapStatus)
	}
	if st.Uncovered.Minor() != 4700 {
		t.Errorf("uncovered = %s, want 47.00", st.Uncovered)
	}
}

func TestEnvelopeAssignMoveCopy(t *testing.T) {
	e := newEnv(t)
	s := svc(e, domain.BudgetEnvelope)
	ctx := context.Background()
	set(t, s, budgeting.SetRequest{Name: "Food", Categories: []string{"Food"}, Amount: "0"})
	set(t, s, budgeting.SetRequest{Name: "Home", Categories: []string{"Home"}, Amount: "0"})
	set(t, s, budgeting.SetRequest{Name: "Trip", Tags: []string{"trip"}, Amount: "0"})
	apply := func(req budgeting.MoveRequest) {
		t.Helper()
		p, err := s.PlanMove(ctx, req)
		if err != nil {
			t.Fatalf("PlanMove(%+v): %v", req, err)
		}
		if err := s.ApplyMove(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	apply(budgeting.MoveRequest{To: "Food", Amount: "300", Date: "2026-02-01"})
	apply(budgeting.MoveRequest{To: "Trip", Amount: "50", Date: "2026-02-01"})
	apply(budgeting.MoveRequest{From: "Food", To: "Home", Amount: "100", Date: "2026-03-02"})
	e.spend(t, "2026-03-03", 5000, e.food, domain.StatusBooked)

	st, err := s.Status(ctx, "2026-03")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int64{}
	for _, l := range st.Envelopes {
		got[l.Budget.Name] = l.Balance.Minor()
	}
	if got["Food"] != 15000 || got["Home"] != 10000 || got["Trip"] != 5000 {
		t.Errorf("balances = %v", got)
	}
	// liquidity 1000.00 - 50.00 spent; envelopes except tag-only: 150 + 100.
	if st.ToAssign.Liquidity.Minor() != 95000 || st.ToAssign.ToAssign.Minor() != 70000 {
		t.Errorf("to assign = %+v", st.ToAssign)
	}

	cp, err := s.PlanCopyPrevious(ctx, "2026-03")
	if err != nil || len(cp.Moves) != 2 || !cp.Bulk {
		t.Fatalf("copy plan = %+v, %v", cp, err)
	}
	if err := s.ApplyCopyPrevious(ctx, cp); err != nil {
		t.Fatal(err)
	}
	st, _ = s.Status(ctx, "2026-03")
	if st.ToAssign.ToAssign.Minor() != 70000-30000 {
		t.Errorf("after copy to assign = %s", st.ToAssign.ToAssign)
	}

	if _, err := svc(e, domain.BudgetCap).PlanMove(ctx, budgeting.MoveRequest{To: "Food", Amount: "1"}); err == nil {
		t.Error("PlanMove in cap mode: want error")
	}
	for _, req := range []budgeting.MoveRequest{
		{To: "Nope", Amount: "1"}, {To: "Food", Amount: "0"}, {From: "Food", To: "Food", Amount: "1"}, {To: "Food", Amount: "x"},
	} {
		if _, err := s.PlanMove(ctx, req); err == nil {
			t.Errorf("PlanMove(%+v): want error", req)
		}
	}
}

func TestCopyPreviousCaps(t *testing.T) {
	e := newEnv(t)
	s := svc(e, domain.BudgetCap)
	ctx := context.Background()
	set(t, s, budgeting.SetRequest{Name: "Food", Categories: []string{"Food"}, Amount: "100"})
	set(t, s, budgeting.SetRequest{Name: "Food", Month: "2026-02", Amount: "250"})
	cp, err := s.PlanCopyPrevious(ctx, "2026-03")
	if err != nil || len(cp.Updates) != 1 || cp.Bulk {
		t.Fatalf("copy plan = %+v, %v", cp, err)
	}
	if err := s.ApplyCopyPrevious(ctx, cp); err != nil {
		t.Fatal(err)
	}
	st, _ := s.Status(ctx, "2026-03")
	if st.Caps[0].Amount.Minor() != 25000 {
		t.Errorf("amount = %s, want 250.00", st.Caps[0].Amount)
	}
	if cp, _ = s.PlanCopyPrevious(ctx, "2026-03"); len(cp.Changes) != 0 {
		t.Errorf("second copy should be a no-op: %+v", cp.Changes)
	}
	if _, err := s.List(ctx); err != nil {
		t.Fatal(err)
	}
}

// TestEnvelopeInvariantsProperty: through the store, envelopes + "Da assegnare" equal the
// liquidity, and each envelope's balance is its net assignments minus its own spending.
func TestEnvelopeInvariantsProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		e := newEnv(t)
		s := svc(e, domain.BudgetEnvelope)
		ctx := context.Background()
		set(t, s, budgeting.SetRequest{Name: "Food", Categories: []string{"Food"}, Amount: "0"})
		set(t, s, budgeting.SetRequest{Name: "Home", Categories: []string{"Home"}, Amount: "0"})
		names := []string{"", "Food", "Home"}
		var wantFood int64
		for range rapid.IntRange(0, 8).Draw(t, "n") {
			month := rapid.IntRange(1, 3).Draw(t, "month")
			date := time.Date(2026, time.Month(month), rapid.IntRange(1, 28).Draw(t, "day"), 0, 0, 0, 0, time.UTC).Format("2006-01-02")
			minor := rapid.Int64Range(1, 50000).Draw(t, "minor")
			if rapid.Bool().Draw(t, "spend") {
				e.spend(t, date, minor, e.food, domain.StatusBooked)
				if month <= 3 {
					wantFood -= minor
				}
				continue
			}
			from, to := rapid.SampledFrom(names).Draw(t, "from"), rapid.SampledFrom(names).Draw(t, "to")
			p, err := s.PlanMove(ctx, budgeting.MoveRequest{From: from, To: to, Date: date, Amount: money(minor).String()[:len(money(minor).String())-4]})
			if err != nil {
				continue // same side: rejected by the engine
			}
			if err := s.ApplyMove(ctx, p); err != nil {
				t.Fatal(err)
			}
			if to == "Food" {
				wantFood += minor
			}
			if from == "Food" {
				wantFood -= minor
			}
		}
		st, err := s.Status(ctx, "2026-03")
		if err != nil {
			t.Fatal(err)
		}
		var sum, food int64
		for _, l := range st.Envelopes {
			sum += l.Balance.Minor()
			if l.Budget.Name == "Food" {
				food = l.Balance.Minor()
			}
		}
		if sum+st.ToAssign.ToAssign.Minor() != st.ToAssign.Liquidity.Minor() {
			t.Fatalf("envelopes %d + to assign %d != liquidity %d", sum, st.ToAssign.ToAssign.Minor(), st.ToAssign.Liquidity.Minor())
		}
		if food != wantFood {
			t.Fatalf("Food balance = %d, want %d", food, wantFood)
		}
	})
}

func TestForeignCurrencyAndTagBudget(t *testing.T) {
	e := newEnv(t)
	s := svc(e, domain.BudgetCap)
	ctx := context.Background()
	usd := must(domain.ParseCurrency("USD"))
	var tx domain.Transaction
	if err := e.store.WithTx(ctx, func(r app.Repos) (err error) {
		acct, err := r.Accounts().Insert(ctx, domain.Account{
			Name: "Usd", Kind: domain.AccountChecking, Currency: usd,
			OpeningBalance: must(domain.NewMoney(0, usd)), OpenedOn: day("2026-01-01"),
		})
		if err != nil {
			return err
		}
		if tx, err = r.Transactions().Insert(ctx, domain.Transaction{
			AccountID: acct.ID, Date: day("2026-03-04"),
			Amount: must(domain.NewMoney(-1000, usd)), CategoryID: e.food.ID, Status: domain.StatusBooked, Kind: domain.KindPayment,
		}); err != nil {
			return err
		}
		return r.Tags().SetTags(ctx, tx.ID, []domain.TagID{e.trip.ID})
	}); err != nil {
		t.Fatal(err)
	}
	set(t, s, budgeting.SetRequest{Name: "Trip", Tags: []string{"trip"}, Amount: "100"})
	if _, err := s.Status(ctx, "2026-03"); !errors.Is(err, app.ErrRateMissing) {
		t.Fatalf("Status without rate: %v, want rate_missing", err)
	}
	if err := e.store.WithTx(ctx, func(r app.Repos) error {
		return r.FXRates().Upsert(ctx, must(domain.NewFXRate(usd, day("2026-03-01"), must(domain.NewRate(500_000_000)))))
	}); err != nil {
		t.Fatal(err)
	}
	st, err := s.Status(ctx, "")
	if err != nil || st.Caps[0].Spent.Minor() != 500 {
		t.Fatalf("Status = %+v, %v; want 5.00 spent", st.Caps, err)
	}
	// replace the targets and the carry flag of an existing budget.
	yes := true
	b := set(t, s, budgeting.SetRequest{Name: "trip", Categories: []string{"Home"}, Carry: &yes})
	if !b.Carry || len(b.Targets) != 1 || b.Targets[0].CategoryID != e.home.ID {
		t.Errorf("updated budget = %+v", b)
	}
}

func TestNoBaseCurrency(t *testing.T) {
	e := newEnv(t)
	s := budgeting.New(e.store, fixedClock{}, budgeting.Config{Modules: must(module.Validate([]module.ID{module.Core, module.Budget}))})
	if _, err := s.Status(context.Background(), ""); err == nil {
		t.Fatal("want error without base currency")
	}
}
