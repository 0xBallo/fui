package importing_test

import (
	"context"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/lending"
	"github.com/0xballo/fui/internal/clock"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/pkg/fuiproto"
)

var (
	recurOnly = must(module.Validate([]module.ID{module.Core, module.Recurring}))
	bothOn    = must(module.Validate([]module.ID{module.Core, module.Recurring, module.Loans}))
)

// seedPendingLoan creates a 12-month loan debited from acct and the pending installments due through
// 2026-03-01, which it returns ordered by date.
func (e env) seedPendingLoan(t tb, acct domain.Account) []domain.Transaction {
	t.Helper()
	ctx := context.Background()
	svc := lending.New(e.store, clock.NewFixed(date("2026-01-15").In(time.UTC)), e.backup, lending.Config{})
	p := must(svc.PlanAdd(ctx, bothOn, lending.AddRequest{
		Name: "Mortgage", Kind: "mortgage", Principal: "12000", Disbursed: "2026-01-01", TermMonths: 12,
		RateType: "fixed", Rate: "3.5", Debit: acct.Name,
	}))
	must(svc.ApplyAdd(ctx, p))
	n := must(svc.ApplyInstallments(ctx, must(svc.PlanInstallments(ctx, bothOn, "", "2026-03-01"))))
	if n < 2 {
		t.Fatalf("pending installments = %d, want >= 2", n)
	}
	var out []domain.Transaction
	for _, tx := range e.txs(t) {
		if tx.Status == domain.StatusPending {
			out = append(out, tx)
		}
	}
	if len(out) != n {
		t.Fatalf("pending txs = %d, want %d", len(out), n)
	}
	if out[0].Date.After(out[1].Date) {
		out[0], out[1] = out[1], out[0]
	}
	return out
}

func debit(ref, id string, d domain.Date, amount int64) fuiproto.Record {
	return txRec(ref, id, d.String(), amount, "Mortgage installment")
}

func TestMatchInstallmentConfirmsPending(t *testing.T) {
	e := newEnv(nil)
	acct := e.seedAccount(t, "Main", "bank", "main")
	pend := e.seedPendingLoan(t, acct)
	first := pend[0]

	src := stream("bank", "sha", account("main", "Main", "EUR"),
		debit("main", "a", first.Date, first.Amount.Minor()))
	plan := e.plan(t, src, withModules(bothOn))
	row := plan.Rows[0]
	if row.ConfirmsPending != first.ID || !row.ConfirmsInstallment || !row.Include || row.Class != domain.DedupNew {
		t.Fatalf("row = %+v, want installment match of %d", row, first.ID)
	}
	var found bool
	for _, c := range plan.Changes {
		found = found || c.Summary == "confirm 1 pending loan installments"
	}
	if !found {
		t.Fatalf("changes %+v lack the installment confirmation summary", plan.Changes)
	}

	before := len(e.txs(t))
	res, err := e.svc.ApplyImport(context.Background(), plan)
	if err != nil {
		t.Fatalf("ApplyImport: %v", err)
	}
	if res.InstallmentsConfirmed != 1 || res.RecurrencesConfirmed != 1 || res.Transactions != 1 || len(res.Warnings) != 0 {
		t.Fatalf("result = %+v, want one installment confirmed, no warnings", res)
	}
	txs := e.txs(t)
	// The booked debit becomes the principal transfer out leg; the in leg and the interest expense are new.
	if len(txs) != before+2 {
		t.Fatalf("len(txs) = %d, want %d (principal in leg + interest expense)", len(txs), before+2)
	}
	var debits, ins, interest int
	for _, tx := range txs {
		switch {
		case tx.ID == first.ID:
			if tx.Status != domain.StatusBooked || tx.Kind != domain.KindTransfer || tx.TransferGroupID == 0 {
				t.Fatalf("confirmed tx = %+v, want booked transfer out leg", tx)
			}
			debits++
		case tx.Kind == domain.KindTransfer && tx.Amount.Sign() > 0:
			ins++
		case tx.Kind == domain.KindInterest:
			interest++
		}
	}
	if debits != 1 || ins != 1 || interest != 1 {
		t.Fatalf("debits=%d in-legs=%d interest=%d, want 1 each", debits, ins, interest)
	}
}

func TestMatchInstallmentOutsideWindow(t *testing.T) {
	e := newEnv(nil)
	acct := e.seedAccount(t, "Main", "bank", "main")
	first := e.seedPendingLoan(t, acct)[0]
	late, err := first.Date.AddDays(10)
	if err != nil {
		t.Fatal(err)
	}
	src := stream("bank", "sha", account("main", "Main", "EUR"), debit("main", "a", late, first.Amount.Minor()))
	row := e.plan(t, src, withModules(bothOn)).Rows[0]
	if row.ConfirmsPending != 0 || row.ConfirmsInstallment {
		t.Fatalf("row = %+v, want no match outside the window", row)
	}
}

func TestMatchInstallmentGatedByModule(t *testing.T) {
	e := newEnv(nil)
	acct := e.seedAccount(t, "Main", "bank", "main")
	first := e.seedPendingLoan(t, acct)[0]
	src := stream("bank", "sha", account("main", "Main", "EUR"), debit("main", "a", first.Date, first.Amount.Minor()))
	for name, mods := range map[string]module.Set{"none": {}, "recurring only": recurOnly} {
		row := e.plan(t, src, withModules(mods)).Rows[0]
		if row.ConfirmsPending != 0 || row.ConfirmsInstallment {
			t.Fatalf("%s: row = %+v, want no match with the loans module off", name, row)
		}
	}
}

func TestMatchInstallmentAndRecurrenceCandidates(t *testing.T) {
	e := newEnv(nil)
	acct := e.seedAccount(t, "Main", "bank", "main")
	inst := e.seedPendingLoan(t, acct)[0]
	rec := e.seedPendingRecurrenceTx(t, acct, date("2026-02-20"), -500, "Rent")

	src := stream("bank", "sha", account("main", "Main", "EUR"),
		debit("main", "a", inst.Date, inst.Amount.Minor()),
		txRec("main", "b", "2026-02-20", -500, "Rent"),
		// Same debit again: its only candidate was already used by the first row.
		debit("main", "c", inst.Date, inst.Amount.Minor()),
	)
	plan := e.plan(t, src, withModules(bothOn))
	if r := plan.Rows[0]; r.ConfirmsPending != inst.ID || !r.ConfirmsInstallment {
		t.Fatalf("row 0 = %+v, want installment %d", r, inst.ID)
	}
	if r := plan.Rows[1]; r.ConfirmsPending != rec.ID || r.ConfirmsInstallment {
		t.Fatalf("row 1 = %+v, want recurrence %d", r, rec.ID)
	}
	if r := plan.Rows[2]; r.ConfirmsPending != 0 {
		t.Fatalf("row 2 = %+v, want pending tx not reused", r)
	}

	// With only recurring on, the installment is left alone.
	plan = e.plan(t, src, withModules(recurOnly))
	if plan.Rows[0].ConfirmsPending != 0 || plan.Rows[1].ConfirmsPending != rec.ID {
		t.Fatalf("recurring only: rows = %+v", plan.Rows)
	}
}

func TestApplyInstallmentAmountMismatchWarns(t *testing.T) {
	e := newEnv(nil)
	acct := e.seedAccount(t, "Main", "bank", "main")
	first := e.seedPendingLoan(t, acct)[0]

	// The bank debited more than the schedule: the pending tx is adjusted so it matches the row.
	other := first
	other.Amount = money(first.Amount.Minor() - 1000)
	if err := e.store.WithTx(context.Background(), func(r app.Repos) error {
		return r.Transactions().Update(context.Background(), other)
	}); err != nil {
		t.Fatal(err)
	}
	src := stream("bank", "sha", account("main", "Main", "EUR"), debit("main", "a", first.Date, other.Amount.Minor()))
	res, err := e.svc.ApplyImport(context.Background(), e.plan(t, src, withModules(bothOn)))
	if err != nil {
		t.Fatalf("ApplyImport: %v", err)
	}
	if res.InstallmentsConfirmed != 1 || len(res.Warnings) != 1 || res.Warnings[0].Line == 0 {
		t.Fatalf("result = %+v, want one confirmed installment with a line warning", res)
	}
}
