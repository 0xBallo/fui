package recurring_test

import (
	"context"
	"errors"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/recurring"
	"github.com/0xballo/fui/internal/domain"
)

func TestPlanConfirmBooksPendingTx(t *testing.T) {
	e := newEnv(t)
	e.addRecurrence(t, nil)
	ctx := context.Background()

	runPlan, err := e.svc.PlanRun(ctx, e.all, recurring.RunRequest{Until: "2026-01-01"})
	if err != nil || len(runPlan.Created) != 1 {
		t.Fatalf("PlanRun: created=%d err=%v", len(runPlan.Created), err)
	}
	if _, err := e.svc.ApplyRun(ctx, runPlan); err != nil {
		t.Fatalf("ApplyRun: %v", err)
	}
	pending := e.listTx(t)
	if len(pending) != 1 {
		t.Fatalf("len(pending) = %d, want 1", len(pending))
	}

	p, err := e.svc.PlanConfirm(ctx, e.all, pending[0].ID)
	if err != nil {
		t.Fatalf("PlanConfirm: %v", err)
	}
	if err := e.svc.ApplyConfirm(ctx, p); err != nil {
		t.Fatalf("ApplyConfirm: %v", err)
	}
	got := e.listTx(t)
	if len(got) != 1 || got[0].Status != domain.StatusBooked {
		t.Fatalf("got %+v, want 1 booked transaction", got)
	}
}

func TestPlanConfirmRejectsNonPending(t *testing.T) {
	e := newEnv(t)
	var txID domain.TransactionID
	ctx := context.Background()
	err := e.store.WithTx(ctx, func(r app.Repos) error {
		tx, err := r.Transactions().Insert(ctx, domain.Transaction{
			AccountID: e.acct.ID, Date: must(domain.ParseDate("2026-01-01")), Amount: money(500),
			Status: domain.StatusBooked, Kind: domain.KindPayment,
		})
		txID = tx.ID
		return err
	})
	if err != nil {
		t.Fatalf("seed transaction: %v", err)
	}

	_, err = e.svc.PlanConfirm(ctx, e.all, txID)
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.CodeInvalidValue {
		t.Fatalf("err = %v, want domain.CodeInvalidValue (not linked to a recurrence)", err)
	}
}

func TestPlanConfirmModuleDisabled(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.PlanConfirm(context.Background(), e.none, 1)
	if !errors.Is(err, app.ErrModuleDisabled) {
		t.Fatalf("err = %v, want app.ErrModuleDisabled", err)
	}
}
