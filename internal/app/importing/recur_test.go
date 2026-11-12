package importing_test

import (
	"context"
	"testing"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

func withModules(m module.Set) func(*importing.PlanRequest) {
	return func(r *importing.PlanRequest) { r.Modules = m }
}

// seedPendingRecurrenceTx inserts a pending transaction linked to a (fake, non-zero) recurrence.
func (e env) seedPendingRecurrenceTx(t tb, acct domain.Account, d domain.Date, amount int64, desc string) domain.Transaction {
	t.Helper()
	return e.seedTx(t, domain.Transaction{
		AccountID: acct.ID, Date: d, DueDate: d, Amount: money(amount), Description: desc,
		Status: domain.StatusPending, Kind: domain.KindPayment, RecurrenceID: 1,
	})
}

func TestMatchRecurrencesConfirmsPending(t *testing.T) {
	e := newEnv(nil)
	acct := e.seedAccount(t, "Main", "bank", "main")
	pending := e.seedPendingRecurrenceTx(t, acct, date("2026-10-01"), -500, "Rent")

	src := stream("bank", "sha",
		account("main", "Main", "EUR"),
		txRec("main", "a", "2026-10-01", -500, "Rent"),
	)
	plan := e.plan(t, src, withModules(module.Default()))
	row := plan.Rows[0]
	if row.ConfirmsPending != pending.ID {
		t.Fatalf("row.ConfirmsPending = %d, want %d", row.ConfirmsPending, pending.ID)
	}
	if !row.Include || row.Class != domain.DedupNew {
		t.Fatalf("row = %+v, want Include=true Class=new", row)
	}

	res, err := e.svc.ApplyImport(context.Background(), plan)
	if err != nil {
		t.Fatalf("ApplyImport: %v", err)
	}
	if res.RecurrencesConfirmed != 1 {
		t.Fatalf("RecurrencesConfirmed = %d, want 1", res.RecurrencesConfirmed)
	}
	if res.Transactions != 1 {
		t.Fatalf("Transactions = %d, want 1", res.Transactions)
	}
	txs := e.txs(t)
	if len(txs) != 1 {
		t.Fatalf("len(txs) = %d, want 1 (no duplicate inserted)", len(txs))
	}
	if txs[0].ID != pending.ID || txs[0].Status != domain.StatusBooked || txs[0].RecurrenceID != pending.RecurrenceID {
		t.Fatalf("txs[0] = %+v, want the pending tx booked, keeping its recurrence link", txs[0])
	}
}

func TestMatchRecurrencesGatedByModule(t *testing.T) {
	e := newEnv(nil)
	acct := e.seedAccount(t, "Main", "bank", "main")
	e.seedPendingRecurrenceTx(t, acct, date("2026-10-01"), -500, "Rent")

	src := stream("bank", "sha",
		account("main", "Main", "EUR"),
		txRec("main", "a", "2026-10-01", -500, "Rent"),
	)
	// Modules left at its zero value: the recurring module is off, so the row is handled by the
	// regular dedup pipeline instead of being matched against the pending transaction.
	plan := e.plan(t, src)
	if row := plan.Rows[0]; row.ConfirmsPending != 0 {
		t.Fatalf("row.ConfirmsPending = %d, want 0 when the module is off", row.ConfirmsPending)
	}
}

func TestMatchRecurrencesDoesNotDoubleConfirm(t *testing.T) {
	e := newEnv(nil)
	acct := e.seedAccount(t, "Main", "bank", "main")
	pending := e.seedPendingRecurrenceTx(t, acct, date("2026-10-01"), -500, "Rent")

	src := stream("bank", "sha",
		account("main", "Main", "EUR"),
		txRec("main", "a", "2026-10-01", -500, "Rent"),
		txRec("main", "b", "2026-10-02", -500, "Rent"),
	)
	plan := e.plan(t, src, withModules(module.Default()))
	if plan.Rows[0].ConfirmsPending != pending.ID {
		t.Fatalf("row 0 ConfirmsPending = %d, want %d", plan.Rows[0].ConfirmsPending, pending.ID)
	}
	if plan.Rows[1].ConfirmsPending != 0 {
		t.Fatalf("row 1 ConfirmsPending = %d, want 0 (the only pending match was already taken)", plan.Rows[1].ConfirmsPending)
	}

	res, err := e.svc.ApplyImport(context.Background(), plan)
	if err != nil {
		t.Fatalf("ApplyImport: %v", err)
	}
	if res.RecurrencesConfirmed != 1 {
		t.Fatalf("RecurrencesConfirmed = %d, want 1", res.RecurrencesConfirmed)
	}
	if len(e.txs(t)) != 2 {
		t.Fatalf("len(txs) = %d, want 2 (one confirmed, one inserted)", len(e.txs(t)))
	}
}

func TestMatchRecurrencesIgnoresPendingTxWithoutRecurrence(t *testing.T) {
	e := newEnv(nil)
	acct := e.seedAccount(t, "Main", "bank", "main")
	e.seedTx(t, domain.Transaction{ // pending, but not created by a recurrence
		AccountID: acct.ID, Date: date("2026-10-01"), Amount: money(-500), Description: "Rent",
		Status: domain.StatusPending, Kind: domain.KindPayment,
	})

	src := stream("bank", "sha",
		account("main", "Main", "EUR"),
		txRec("main", "a", "2026-10-01", -500, "Rent"),
	)
	plan := e.plan(t, src, withModules(module.Default()))
	if row := plan.Rows[0]; row.ConfirmsPending != 0 {
		t.Fatalf("row.ConfirmsPending = %d, want 0 for a pending tx with no recurrence", row.ConfirmsPending)
	}
}
