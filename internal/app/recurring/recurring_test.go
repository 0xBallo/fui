package recurring_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/recurring"
	"github.com/0xballo/fui/internal/domain"
)

func TestPlanAdd(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	p, err := e.svc.PlanAdd(ctx, e.all, recurring.AddRequest{
		Account: "Main", Category: "Food", Description: "Rent", Amount: "100",
		Every: 1, Unit: "months", Start: "2026-01-01",
	})
	if err != nil {
		t.Fatalf("PlanAdd: %v", err)
	}
	if len(p.Changes) != 1 || p.Changes[0].Op != app.OpCreate {
		t.Fatalf("unexpected plan changes: %+v", p.Changes)
	}
	rec, err := e.svc.ApplyAdd(ctx, p)
	if err != nil {
		t.Fatalf("ApplyAdd: %v", err)
	}
	if rec.ID == 0 {
		t.Fatal("expected a non-zero ID")
	}
	if diff := cmp.Diff(money(10000).Minor(), rec.Amount.Minor()); diff != "" {
		t.Errorf("amount mismatch (-want +got):\n%s", diff)
	}
	if rec.NextDate != must(domain.ParseDate("2026-01-01")) {
		t.Errorf("next_date = %s, want 2026-01-01", rec.NextDate)
	}
}

func TestPlanAddModuleDisabled(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.PlanAdd(context.Background(), e.none, recurring.AddRequest{
		Account: "Main", Amount: "10", Every: 1, Unit: "months",
	})
	if !errors.Is(err, app.ErrModuleDisabled) {
		t.Fatalf("err = %v, want app.ErrModuleDisabled", err)
	}
}

func TestPlanAddUnknownAccountErrors(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.PlanAdd(context.Background(), e.all, recurring.AddRequest{
		Account: "Nope", Amount: "10", Every: 1, Unit: "months",
	})
	if err == nil {
		t.Fatal("expected an error for an unknown account")
	}
}

func TestPlanAddInvalidAmountErrors(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.PlanAdd(context.Background(), e.all, recurring.AddRequest{
		Account: "Main", Amount: "not a number", Every: 1, Unit: "months",
	})
	if err == nil {
		t.Fatal("expected an error for an invalid amount")
	}
}

func TestPlanAddInvalidUnitErrors(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.PlanAdd(context.Background(), e.all, recurring.AddRequest{
		Account: "Main", Amount: "10", Every: 1, Unit: "centuries",
	})
	if err == nil {
		t.Fatal("expected an error for an invalid unit")
	}
}

func (e env) addRecurrence(t *testing.T, mod func(*recurring.AddRequest)) app.Recurrence {
	t.Helper()
	req := recurring.AddRequest{
		Account: "Main", Category: "Food", Description: "Rent", Amount: "100",
		Every: 1, Unit: "months", Start: "2026-01-01",
	}
	if mod != nil {
		mod(&req)
	}
	ctx := context.Background()
	p, err := e.svc.PlanAdd(ctx, e.all, req)
	if err != nil {
		t.Fatalf("PlanAdd: %v", err)
	}
	rec, err := e.svc.ApplyAdd(ctx, p)
	if err != nil {
		t.Fatalf("ApplyAdd: %v", err)
	}
	return rec
}

func TestPlanEditUpdatesFields(t *testing.T) {
	e := newEnv(t)
	rec := e.addRecurrence(t, nil)
	ctx := context.Background()

	desc := "New rent"
	every := 2
	p, err := e.svc.PlanEdit(ctx, e.all, recurring.EditRequest{ID: rec.ID, Description: &desc, Every: &every})
	if err != nil {
		t.Fatalf("PlanEdit: %v", err)
	}
	if err := e.svc.ApplyEdit(ctx, p); err != nil {
		t.Fatalf("ApplyEdit: %v", err)
	}
	got := e.mustGet(t, rec.ID)
	if got.Description != "New rent" || got.Rule.Every != 2 {
		t.Errorf("got %+v, want Description=New rent Every=2", got)
	}
}

func TestPlanEditNoChangesErrors(t *testing.T) {
	e := newEnv(t)
	rec := e.addRecurrence(t, nil)
	_, err := e.svc.PlanEdit(context.Background(), e.all, recurring.EditRequest{ID: rec.ID})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.CodeMissingField {
		t.Fatalf("err = %v, want domain.CodeMissingField", err)
	}
}

func TestPlanEditClearsCategoryAndEnd(t *testing.T) {
	e := newEnv(t)
	rec := e.addRecurrence(t, func(r *recurring.AddRequest) { r.End = "2026-06-01" })
	ctx := context.Background()

	empty := ""
	p, err := e.svc.PlanEdit(ctx, e.all, recurring.EditRequest{ID: rec.ID, Category: &empty, End: &empty})
	if err != nil {
		t.Fatalf("PlanEdit: %v", err)
	}
	if err := e.svc.ApplyEdit(ctx, p); err != nil {
		t.Fatalf("ApplyEdit: %v", err)
	}
	got := e.mustGet(t, rec.ID)
	if got.CategoryID != 0 || !got.End.IsZero() {
		t.Errorf("got %+v, want CategoryID=0 End=zero", got)
	}
}

func TestPlanRemove(t *testing.T) {
	e := newEnv(t)
	rec := e.addRecurrence(t, nil)
	ctx := context.Background()

	p, err := e.svc.PlanRemove(ctx, e.all, rec.ID)
	if err != nil {
		t.Fatalf("PlanRemove: %v", err)
	}
	if err := e.svc.ApplyRemove(ctx, p); err != nil {
		t.Fatalf("ApplyRemove: %v", err)
	}
	list, err := e.svc.List(ctx, e.all)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("list = %+v, want empty", list)
	}
}

func TestList(t *testing.T) {
	e := newEnv(t)
	e.addRecurrence(t, nil)
	e.addRecurrence(t, func(r *recurring.AddRequest) { r.Description = "Second" })

	list, err := e.svc.List(context.Background(), e.all)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len(list) = %d, want 2", len(list))
	}
}

func TestListModuleDisabled(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.List(context.Background(), e.none)
	if !errors.Is(err, app.ErrModuleDisabled) {
		t.Fatalf("err = %v, want app.ErrModuleDisabled", err)
	}
}

func TestPlanEditChangesUnitAndAmount(t *testing.T) {
	e := newEnv(t)
	rec := e.addRecurrence(t, nil)
	ctx := context.Background()

	unit := "weeks"
	amount := "50"
	p, err := e.svc.PlanEdit(ctx, e.all, recurring.EditRequest{ID: rec.ID, Unit: &unit, Amount: &amount})
	if err != nil {
		t.Fatalf("PlanEdit: %v", err)
	}
	if err := e.svc.ApplyEdit(ctx, p); err != nil {
		t.Fatalf("ApplyEdit: %v", err)
	}
	got := e.mustGet(t, rec.ID)
	if got.Rule.Unit != domain.RecurrenceWeeks || got.Amount.Minor() != 5000 {
		t.Errorf("got %+v, want unit=weeks amount=5000", got)
	}
}

func TestPlanEditInvalidUnitErrors(t *testing.T) {
	e := newEnv(t)
	rec := e.addRecurrence(t, nil)
	unit := "centuries"
	_, err := e.svc.PlanEdit(context.Background(), e.all, recurring.EditRequest{ID: rec.ID, Unit: &unit})
	if err == nil {
		t.Fatal("expected an error for an invalid unit")
	}
}

func TestPlanEditUnknownIDErrors(t *testing.T) {
	e := newEnv(t)
	desc := "x"
	_, err := e.svc.PlanEdit(context.Background(), e.all, recurring.EditRequest{ID: 999, Description: &desc})
	if err == nil {
		t.Fatal("expected an error for an unknown recurrence")
	}
}

func TestPlanRemoveUnknownIDErrors(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.PlanRemove(context.Background(), e.all, 999)
	if err == nil {
		t.Fatal("expected an error for an unknown recurrence")
	}
}

func TestPlanRemoveModuleDisabled(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.PlanRemove(context.Background(), e.none, 1)
	if !errors.Is(err, app.ErrModuleDisabled) {
		t.Fatalf("err = %v, want app.ErrModuleDisabled", err)
	}
}

func (e env) mustGet(t *testing.T, id app.RecurrenceID) app.Recurrence {
	t.Helper()
	var out app.Recurrence
	err := e.store.Read(context.Background(), func(r app.Repos) (err error) {
		out, err = r.Recurrences().Get(context.Background(), id)
		return err
	})
	if err != nil {
		t.Fatalf("get recurrence %d: %v", id, err)
	}
	return out
}
