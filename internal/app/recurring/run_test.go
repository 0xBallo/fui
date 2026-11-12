package recurring_test

import (
	"context"
	"errors"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/recurring"
	"github.com/0xballo/fui/internal/domain"
)

func TestRunCreatesOccurrencesAndAdvances(t *testing.T) {
	e := newEnv(t)
	rec := e.addRecurrence(t, nil) // monthly, starts 2026-01-01
	ctx := context.Background()

	p, err := e.svc.PlanRun(ctx, e.all, recurring.RunRequest{Until: "2026-03-01"})
	if err != nil {
		t.Fatalf("PlanRun: %v", err)
	}
	if len(p.Created) != 3 {
		t.Fatalf("len(Created) = %d, want 3 (Jan, Feb, Mar)", len(p.Created))
	}
	n, err := e.svc.ApplyRun(ctx, p)
	if err != nil {
		t.Fatalf("ApplyRun: %v", err)
	}
	if n != 3 {
		t.Fatalf("n = %d, want 3", n)
	}
	if e.backup.n != 1 {
		t.Errorf("backup calls = %d, want 1", e.backup.n)
	}
	got := e.mustGet(t, rec.ID)
	if got.NextDate != must(domain.ParseDate("2026-04-01")) {
		t.Errorf("next_date = %s, want 2026-04-01", got.NextDate)
	}
	pending := e.listTx(t)
	if len(pending) != 3 {
		t.Fatalf("len(pending) = %d, want 3", len(pending))
	}
	for _, tx := range pending {
		if tx.Status != domain.StatusPending || tx.RecurrenceID != domain.RecurrenceID(rec.ID) {
			t.Errorf("tx = %+v, want pending linked to recurrence %d", tx, rec.ID)
		}
	}
}

func TestRunTwiceCreatesNoDuplicates(t *testing.T) {
	e := newEnv(t)
	e.addRecurrence(t, nil)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		p, err := e.svc.PlanRun(ctx, e.all, recurring.RunRequest{Until: "2026-03-01"})
		if err != nil {
			t.Fatalf("PlanRun #%d: %v", i, err)
		}
		if _, err := e.svc.ApplyRun(ctx, p); err != nil {
			t.Fatalf("ApplyRun #%d: %v", i, err)
		}
	}
	pending := e.listTx(t)
	if len(pending) != 3 {
		t.Fatalf("len(pending) after running twice = %d, want 3", len(pending))
	}
}

func TestRunRespectsEnd(t *testing.T) {
	e := newEnv(t)
	rec := e.addRecurrence(t, func(r *recurring.AddRequest) { r.End = "2026-02-15" })
	ctx := context.Background()

	p, err := e.svc.PlanRun(ctx, e.all, recurring.RunRequest{Until: "2026-06-01"})
	if err != nil {
		t.Fatalf("PlanRun: %v", err)
	}
	if len(p.Created) != 2 {
		t.Fatalf("len(Created) = %d, want 2 (Jan, Feb, End truncates Mar)", len(p.Created))
	}
	if _, err := e.svc.ApplyRun(ctx, p); err != nil {
		t.Fatalf("ApplyRun: %v", err)
	}
	got := e.mustGet(t, rec.ID)
	if !got.NextDate.IsZero() {
		t.Errorf("next_date = %s, want zero (recurrence exhausted by End)", got.NextDate)
	}

	// Running again after exhaustion creates nothing new: Due() no longer lists it.
	p2, err := e.svc.PlanRun(ctx, e.all, recurring.RunRequest{Until: "2026-12-01"})
	if err != nil {
		t.Fatalf("PlanRun after exhaustion: %v", err)
	}
	if len(p2.Created) != 0 {
		t.Errorf("len(Created) after exhaustion = %d, want 0", len(p2.Created))
	}
}

func TestRunAfterConfirmDoesNotDuplicateExhaustedOccurrence(t *testing.T) {
	e := newEnv(t)
	e.addRecurrence(t, func(r *recurring.AddRequest) { r.End = "2026-01-01" }) // single occurrence
	ctx := context.Background()

	p, err := e.svc.PlanRun(ctx, e.all, recurring.RunRequest{Until: "2026-01-01"})
	if err != nil || len(p.Created) != 1 {
		t.Fatalf("PlanRun: created=%d err=%v", len(p.Created), err)
	}
	if _, err := e.svc.ApplyRun(ctx, p); err != nil {
		t.Fatalf("ApplyRun: %v", err)
	}
	pending := e.listTx(t)
	confirm, err := e.svc.PlanConfirm(ctx, e.all, pending[0].ID)
	if err != nil {
		t.Fatalf("PlanConfirm: %v", err)
	}
	if err := e.svc.ApplyConfirm(ctx, confirm); err != nil {
		t.Fatalf("ApplyConfirm: %v", err)
	}

	// Running again must not recreate the now-booked occurrence: the recurrence is exhausted
	// (zero NextDate), so Due() no longer lists it.
	p2, err := e.svc.PlanRun(ctx, e.all, recurring.RunRequest{Until: "2026-12-01"})
	if err != nil {
		t.Fatalf("PlanRun after confirm: %v", err)
	}
	if len(p2.Created) != 0 {
		t.Fatalf("len(Created) after confirm = %d, want 0", len(p2.Created))
	}
	if len(e.listTx(t)) != 1 {
		t.Fatalf("len(transactions) = %d, want 1", len(e.listTx(t)))
	}
}

func TestRunNothingDueSkipsBackup(t *testing.T) {
	e := newEnv(t)
	e.addRecurrence(t, func(r *recurring.AddRequest) { r.Start = "2026-06-01" })
	ctx := context.Background()

	p, err := e.svc.PlanRun(ctx, e.all, recurring.RunRequest{Until: "2026-01-15"})
	if err != nil {
		t.Fatalf("PlanRun: %v", err)
	}
	if len(p.Created) != 0 {
		t.Fatalf("len(Created) = %d, want 0", len(p.Created))
	}
	if _, err := e.svc.ApplyRun(ctx, p); err != nil {
		t.Fatalf("ApplyRun: %v", err)
	}
	if e.backup.n != 0 {
		t.Errorf("backup calls = %d, want 0", e.backup.n)
	}
}

func TestSkipAdvancesWithoutCreatingTx(t *testing.T) {
	e := newEnv(t)
	rec := e.addRecurrence(t, nil)
	ctx := context.Background()

	p, err := e.svc.PlanSkip(ctx, e.all, rec.ID)
	if err != nil {
		t.Fatalf("PlanSkip: %v", err)
	}
	if err := e.svc.ApplySkip(ctx, p); err != nil {
		t.Fatalf("ApplySkip: %v", err)
	}
	got := e.mustGet(t, rec.ID)
	if got.NextDate != must(domain.ParseDate("2026-02-01")) {
		t.Errorf("next_date = %s, want 2026-02-01", got.NextDate)
	}
	if len(e.listTx(t)) != 0 {
		t.Error("skip must not create a transaction")
	}
}

func TestSkipExhaustsRecurrence(t *testing.T) {
	e := newEnv(t)
	rec := e.addRecurrence(t, func(r *recurring.AddRequest) { r.End = "2026-01-01" }) // single occurrence
	ctx := context.Background()

	p, err := e.svc.PlanSkip(ctx, e.all, rec.ID)
	if err != nil {
		t.Fatalf("PlanSkip: %v", err)
	}
	if !p.Recurrence.NextDate.IsZero() {
		t.Fatalf("NextDate = %s, want zero (exhausted)", p.Recurrence.NextDate)
	}
	if err := e.svc.ApplySkip(ctx, p); err != nil {
		t.Fatalf("ApplySkip: %v", err)
	}
	got := e.mustGet(t, rec.ID)
	if !got.NextDate.IsZero() {
		t.Fatalf("persisted next_date = %s, want zero", got.NextDate)
	}

	// Skipping an already-exhausted recurrence has nothing left to skip to.
	_, err = e.svc.PlanSkip(ctx, e.all, rec.ID)
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.CodeInvalidValue {
		t.Fatalf("err = %v, want domain.CodeInvalidValue", err)
	}
}

func TestSkipUnknownIDErrors(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.PlanSkip(context.Background(), e.all, 999)
	if err == nil {
		t.Fatal("expected an error for an unknown recurrence")
	}
}

func TestSkipModuleDisabled(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.PlanSkip(context.Background(), e.none, 1)
	if !errors.Is(err, app.ErrModuleDisabled) {
		t.Fatalf("err = %v, want app.ErrModuleDisabled", err)
	}
}

func TestRunModuleDisabled(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.PlanRun(context.Background(), e.none, recurring.RunRequest{})
	if !errors.Is(err, app.ErrModuleDisabled) {
		t.Fatalf("err = %v, want app.ErrModuleDisabled", err)
	}
}

func (e env) listTx(t *testing.T) []domain.Transaction {
	t.Helper()
	var out []domain.Transaction
	err := e.store.Read(context.Background(), func(r app.Repos) (err error) {
		f, err := domain.NewTxFilter(domain.TxFilterSpec{})
		if err != nil {
			return err
		}
		out, err = r.Transactions().List(context.Background(), f, nil, 0)
		return err
	})
	if err != nil {
		t.Fatalf("list transactions: %v", err)
	}
	return out
}
