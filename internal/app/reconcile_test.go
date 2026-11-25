package app_test

import (
	"errors"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

func TestPlanReconcileShowsDifferenceAndUnreconciled(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	e.account(t, "A", "checking", "EUR", "100")
	tx1 := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-10", Date: "2026-10-01", Status: "booked"})
	e.tx(t, app.AddTxRequest{Account: "A", Amount: "-5", Date: "2026-10-02", Status: "reconciled"})
	e.tx(t, app.AddTxRequest{Account: "A", Amount: "-1", Date: "2026-10-03", Status: "pending"})
	e.tx(t, app.AddTxRequest{Account: "A", Amount: "-2", Date: "2026-10-11", Status: "booked"})

	p, err := e.svc.PlanReconcile(ctx, module.Default(), app.ReconcileRequest{
		Account: "A", StatementBalance: "84", Date: "2026-10-10",
	})
	wantNoErr(t, err)
	if p.Difference.Minor() != -100 {
		t.Fatalf("difference = %d, want -100", p.Difference.Minor())
	}
	if len(p.Unreconciled) != 1 || p.Unreconciled[0].ID != tx1.ID {
		t.Fatalf("unreconciled = %+v, want [%d]", p.Unreconciled, tx1.ID)
	}
	if len(p.ReconciledIDs) != 0 || len(p.Changes) != 0 || p.Bulk {
		t.Fatalf("plan should not close: %+v", p)
	}
}

func TestApplyReconcileClosesAtZeroDifference(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	acct := e.account(t, "A", "checking", "EUR", "100")
	tx1 := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-10", Date: "2026-10-01", Status: "booked"})
	tx2 := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-5", Date: "2026-10-02", Status: "reconciled"})
	tx3 := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-1", Date: "2026-10-03", Status: "pending"})
	tx4 := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-2", Date: "2026-10-11", Status: "booked"})

	p, err := e.svc.PlanReconcile(ctx, module.Default(), app.ReconcileRequest{
		Account: "A", StatementBalance: "85", Date: "2026-10-10",
	})
	wantNoErr(t, err)
	if p.Difference.Minor() != 0 || len(p.ReconciledIDs) != 1 || p.ReconciledIDs[0] != tx1.ID || !p.Bulk {
		t.Fatalf("zero-difference plan = %+v", p)
	}

	wantNoErr(t, e.svc.ApplyReconcile(ctx, module.Default(), p))
	v1, err := e.svc.ShowTx(ctx, tx1.ID)
	wantNoErr(t, err)
	if v1.Transaction.Status != domain.StatusReconciled {
		t.Fatalf("tx1 status = %s, want reconciled", v1.Transaction.Status)
	}
	v2, err := e.svc.ShowTx(ctx, tx2.ID)
	wantNoErr(t, err)
	if v2.Transaction.Status != domain.StatusReconciled {
		t.Fatalf("tx2 status = %s, want reconciled", v2.Transaction.Status)
	}
	v3, err := e.svc.ShowTx(ctx, tx3.ID)
	wantNoErr(t, err)
	if v3.Transaction.Status != domain.StatusPending {
		t.Fatalf("tx3 status = %s, want pending", v3.Transaction.Status)
	}
	v4, err := e.svc.ShowTx(ctx, tx4.ID)
	wantNoErr(t, err)
	if v4.Transaction.Status != domain.StatusBooked {
		t.Fatalf("tx4 status = %s, want booked", v4.Transaction.Status)
	}

	var rec app.Reconciliation
	err = e.store.Read(ctx, func(r app.Repos) (err error) {
		rec, err = r.Reconciliations().Get(ctx, acct.ID, p.Date)
		return err
	})
	wantNoErr(t, err)
	if rec.ClosedAt.IsZero() {
		t.Fatal("reconciliation is not closed")
	}
	if rec.StatementBalance.Minor() != 8500 {
		t.Fatalf("statement balance = %d, want 8500", rec.StatementBalance.Minor())
	}
	if len(e.backup.reasons) != 1 || e.backup.reasons[0] != app.BackupReasonBulk {
		t.Fatalf("backup reasons = %v", e.backup.reasons)
	}
}

func TestApplyReconcileRefusesNonZeroDifference(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	e.account(t, "A", "checking", "EUR", "100")
	e.tx(t, app.AddTxRequest{Account: "A", Amount: "-10", Date: "2026-10-01", Status: "booked"})

	p, err := e.svc.PlanReconcile(ctx, module.Default(), app.ReconcileRequest{
		Account: "A", StatementBalance: "80", Date: "2026-10-10",
	})
	wantNoErr(t, err)
	before := e.store.Snapshot()
	err = e.svc.ApplyReconcile(ctx, module.Default(), p)
	wantCode(t, err, domain.ErrInvalidValue)
	if after := e.store.Snapshot(); after != before {
		t.Fatal("ApplyReconcile wrote despite non-zero difference")
	}
}

func TestReconcileRequiresCoreModule(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	_, err := e.svc.PlanReconcile(ctx, module.Set{}, app.ReconcileRequest{
		Account: "A", StatementBalance: "0", Date: "2026-10-10",
	})
	if !errors.Is(err, app.ErrModuleDisabled) {
		t.Fatalf("PlanReconcile err = %v, want module_disabled", err)
	}
	if err := e.svc.ApplyReconcile(ctx, module.Set{}, app.ReconcilePlan{}); !errors.Is(err, app.ErrModuleDisabled) {
		t.Fatalf("ApplyReconcile err = %v, want module_disabled", err)
	}
}

func ticks(t *testing.T, e env, id domain.AccountID) []domain.TransactionID {
	t.Helper()
	var out []domain.TransactionID
	wantNoErr(t, e.store.Read(t.Context(), func(r app.Repos) (err error) {
		out, err = r.Reconciliations().Ticks(t.Context(), id)
		return err
	}))
	return out
}

func TestReconcileTicks(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	acct := e.account(t, "A", "checking", "EUR", "100")
	e.account(t, "B", "checking", "EUR", "0")
	tx1 := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-10", Date: "2026-10-01", Status: "booked"})
	tx2 := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-5", Date: "2026-10-02", Status: "booked"})
	foreign := e.tx(t, app.AddTxRequest{Account: "B", Amount: "-1", Date: "2026-10-02", Status: "booked"})
	md := module.Default()
	req := func(tick, untick []domain.TransactionID) app.ReconcileRequest {
		return app.ReconcileRequest{Account: "A", StatementBalance: "85", Date: "2026-10-10", Tick: tick, Untick: untick}
	}

	// Partial ticks: difference != 0, persisted, not closed.
	p, err := e.svc.PlanReconcile(ctx, md, req([]domain.TransactionID{tx1.ID}, nil))
	wantNoErr(t, err)
	if p.Difference.Minor() != -500 || p.Closing || len(p.Unreconciled) != 1 || p.Unreconciled[0].ID != tx2.ID {
		t.Fatalf("partial plan = %+v", p)
	}
	wantNoErr(t, e.svc.ApplyReconcile(ctx, md, p))
	if got := ticks(t, e, acct.ID); len(got) != 1 || got[0] != tx1.ID {
		t.Fatalf("ticks = %v", got)
	}
	if v, err := e.svc.ShowTx(ctx, tx1.ID); err != nil || v.Transaction.Status != domain.StatusBooked {
		t.Fatalf("tx1 = %+v, %v; want still booked", v.Transaction, err)
	}

	// Persisted ticks drive the next plan; untick drops them.
	p, err = e.svc.PlanReconcile(ctx, md, req(nil, []domain.TransactionID{tx1.ID}))
	wantNoErr(t, err)
	if p.Difference.Minor() != -1500 || len(p.Ticked) != 0 {
		t.Fatalf("untick plan = %+v", p)
	}
	wantNoErr(t, e.svc.ApplyReconcile(ctx, md, p))
	if got := ticks(t, e, acct.ID); len(got) != 0 {
		t.Fatalf("ticks after untick = %v", got)
	}

	// Foreign transaction is rejected.
	_, err = e.svc.PlanReconcile(ctx, md, req([]domain.TransactionID{foreign.ID}, nil))
	wantCode(t, err, domain.ErrInvalidValue)

	// Full ticks close and clear the ticks.
	p, err = e.svc.PlanReconcile(ctx, md, app.ReconcileRequest{
		Account: "A", StatementBalance: "85", Date: "2026-10-10", Tick: []domain.TransactionID{tx1.ID, tx2.ID},
	})
	wantNoErr(t, err)
	if !p.Closing || !p.Difference.IsZero() || len(p.ReconciledIDs) != 2 {
		t.Fatalf("full plan = %+v", p)
	}
	wantNoErr(t, e.svc.ApplyReconcile(ctx, md, p))
	for _, id := range []domain.TransactionID{tx1.ID, tx2.ID} {
		v, err := e.svc.ShowTx(ctx, id)
		wantNoErr(t, err)
		if v.Transaction.Status != domain.StatusReconciled {
			t.Fatalf("tx %d status = %s", id, v.Transaction.Status)
		}
	}
	if got := ticks(t, e, acct.ID); len(got) != 0 {
		t.Fatalf("ticks after close = %v", got)
	}
}

func TestApplyReconcileRefusesClosingWithDifference(t *testing.T) {
	e := newEnv(t)
	acct := e.account(t, "A", "checking", "EUR", "100")
	m, err := domain.NewMoney(5, acct.Currency)
	wantNoErr(t, err)
	err = e.svc.ApplyReconcile(t.Context(), module.Default(), app.ReconcilePlan{
		Account: acct, Ticking: true, Closing: true, Difference: m,
	})
	wantCode(t, err, domain.ErrInvalidValue)
}
