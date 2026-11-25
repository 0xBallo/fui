package lending_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/lending"
	"github.com/0xballo/fui/internal/app/recurring"
	"github.com/0xballo/fui/internal/clock"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// withRecurring rebuilds e.svc so it can also confirm recurrence transactions.
func (e *env) withRecurring() {
	clk := clock.NewFixed(date("2026-01-15").In(time.UTC))
	rec := recurring.New(e.store, clk, e.backup, recurring.Config{})
	e.svc = lending.New(e.store, clk, e.backup, lending.Config{Recurring: rec})
}

func (e env) insert(t tb, tx domain.Transaction) domain.Transaction {
	t.Helper()
	var out domain.Transaction
	err := e.store.WithTx(context.Background(), func(r app.Repos) (err error) {
		out, err = r.Transactions().Insert(context.Background(), must(domain.NewTransaction(tx)))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func (e env) get(t tb, id domain.TransactionID) domain.Transaction {
	t.Helper()
	var out domain.Transaction
	err := e.store.Read(context.Background(), func(r app.Repos) (err error) {
		out, err = r.Transactions().Get(context.Background(), id)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func (e env) confirmWithin(id domain.TransactionID) (lending.ConfirmPlan, error) {
	var p lending.ConfirmPlan
	err := e.store.WithTx(context.Background(), func(r app.Repos) (err error) {
		p, err = lending.ConfirmWithin(context.Background(), r, id)
		return err
	})
	return p, err
}

func TestConfirmWithin(t *testing.T) {
	for _, booked := range []bool{false, true} {
		name := "pending"
		if booked {
			name = "booked"
		}
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			e.add(t, addReq())
			e.generate(t, "2026-02-01")
			tx := e.pending(t)[0]
			if booked {
				tx.Status = domain.StatusBooked
				if err := e.store.WithTx(context.Background(), func(r app.Repos) error {
					return r.Transactions().Update(context.Background(), tx)
				}); err != nil {
					t.Fatal(err)
				}
			}
			p, err := e.confirmWithin(tx.ID)
			if err != nil {
				t.Fatalf("ConfirmWithin: %v", err)
			}
			got := e.get(t, tx.ID)
			if got.Status != domain.StatusBooked || got.Kind != domain.KindTransfer {
				t.Errorf("tx = %+v, want booked transfer", got)
			}
			if p.In.Amount.Minor() <= 0 || p.Expense.Amount.Minor() >= 0 {
				t.Errorf("split: in=%v expense=%v", p.In.Amount, p.Expense.Amount)
			}
			if sum := -p.Principal.Amount.Minor() - p.Expense.Amount.Minor(); sum != -tx.Amount.Minor() {
				t.Errorf("legs sum to %d, debited %d", sum, -tx.Amount.Minor())
			}
			if p.In.Amount.Minor() != -p.Principal.Amount.Minor() {
				t.Errorf("transfer legs differ: %v vs %v", p.In.Amount, p.Principal.Amount)
			}
		})
	}

	t.Run("unlinked", func(t *testing.T) {
		e := newEnv(t)
		e.add(t, addReq())
		plain := e.insert(t, domain.Transaction{
			AccountID: e.main.ID, Date: date("2026-02-01"), Amount: money(-100), Status: domain.StatusBooked,
			Kind: domain.KindPayment, Description: "Other",
		})
		if _, err := e.confirmWithin(plain.ID); !errors.Is(err, &domain.Error{Code: domain.CodeInvalidValue}) {
			t.Fatalf("err = %v, want invalid value", err)
		}
		if got := e.get(t, plain.ID); got.Kind != domain.KindPayment {
			t.Errorf("tx changed: %+v", got)
		}
	})
}

// The split legs always add up to what was debited, interest-only and underpaid cases included.
func TestConfirmWithinSplitProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		e := newEnv(t)
		req := addReq()
		req.Principal = rapid.StringMatching(`[1-9][0-9]{3,5}`).Draw(t, "principal")
		req.TermMonths = rapid.IntRange(1, 60).Draw(t, "term")
		req.Rate = rapid.SampledFrom([]string{"0,5", "2,75", "9,99"}).Draw(t, "rate")
		e.add(t, req)
		e.generate(t, "2026-02-01")
		tx := e.pending(t)[0]
		debited := rapid.Int64Range(1, 2*-tx.Amount.Minor()).Draw(t, "debited")
		tx.Amount = money(-debited)
		if err := e.store.WithTx(context.Background(), func(r app.Repos) error {
			return r.Transactions().Update(context.Background(), tx)
		}); err != nil {
			t.Fatal(err)
		}
		p, err := e.confirmWithin(tx.ID)
		if err != nil {
			t.Fatal(err)
		}
		// Outflow of the debit account: the original (turned into the transfer out or the
		// interest expense) plus the optional separate interest expense.
		if sum := -p.Principal.Amount.Minor() - p.Expense.Amount.Minor(); sum != debited {
			t.Fatalf("legs sum to %d, debited %d", sum, debited)
		}
	})
}

func TestConfirmPending(t *testing.T) {
	ctx := context.Background()
	setup := func(t *testing.T) (env, domain.Transaction, domain.Transaction, domain.Transaction) {
		e := newEnv(t)
		e.withRecurring()
		e.add(t, addReq())
		e.generate(t, "2026-02-01")
		inst := e.pending(t)[0]
		rec := e.insert(t, domain.Transaction{
			AccountID: e.main.ID, Date: date("2026-02-02"), Amount: money(-900), Status: domain.StatusPending,
			Kind: domain.KindPayment, Description: "Rent", RecurrenceID: 1,
		})
		plain := e.insert(t, domain.Transaction{
			AccountID: e.main.ID, Date: date("2026-02-03"), Amount: money(-50), Status: domain.StatusPending,
			Kind: domain.KindPayment, Description: "Other",
		})
		return e, inst, rec, plain
	}
	noLoans := must(module.Validate([]module.ID{module.Core, module.Recurring}))
	noRecurring := module.Set{}

	t.Run("installment", func(t *testing.T) {
		e, inst, _, _ := setup(t)
		got, err := e.svc.ConfirmPending(ctx, e.on, inst.ID)
		if err != nil || got.Kind != lending.PendingInstallment {
			t.Fatalf("got %+v, %v", got, err)
		}
		if tx := e.get(t, inst.ID); tx.Status != domain.StatusBooked || tx.Kind != domain.KindTransfer {
			t.Errorf("tx = %+v", tx)
		}
	})
	t.Run("recurrence", func(t *testing.T) {
		e, _, rec, _ := setup(t)
		got, err := e.svc.ConfirmPending(ctx, e.on, rec.ID)
		if err != nil || got.Kind != lending.PendingRecurrence {
			t.Fatalf("got %+v, %v", got, err)
		}
		if tx := e.get(t, rec.ID); tx.Status != domain.StatusBooked {
			t.Errorf("tx = %+v", tx)
		}
	})
	t.Run("neither", func(t *testing.T) {
		e, _, _, plain := setup(t)
		if _, err := e.svc.ConfirmPending(ctx, e.on, plain.ID); !errors.Is(err, &domain.Error{Code: domain.CodeInvalidValue}) {
			t.Fatalf("err = %v, want invalid value", err)
		}
		if tx := e.get(t, plain.ID); tx.Status != domain.StatusPending {
			t.Errorf("tx changed: %+v", tx)
		}
	})
	t.Run("missing", func(t *testing.T) {
		e, _, _, _ := setup(t)
		if _, err := e.svc.ConfirmPending(ctx, e.on, 9999); err == nil {
			t.Fatal("confirmed a missing transaction")
		}
	})
	t.Run("module disabled", func(t *testing.T) {
		e, inst, rec, _ := setup(t)
		if _, err := e.svc.ConfirmPending(ctx, noLoans, inst.ID); err == nil {
			t.Error("installment confirmed with loans disabled")
		}
		if _, err := e.svc.ConfirmPending(ctx, noRecurring, rec.ID); err == nil {
			t.Error("recurrence confirmed with recurring disabled")
		}
		if e.get(t, inst.ID).Status != domain.StatusPending || e.get(t, rec.ID).Status != domain.StatusPending {
			t.Error("a transaction was written despite the error")
		}
	})
}
