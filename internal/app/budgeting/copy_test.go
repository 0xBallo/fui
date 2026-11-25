package budgeting_test

import (
	"context"
	"errors"
	"testing"

	"github.com/0xballo/fui/internal/app/budgeting"
	"github.com/0xballo/fui/internal/domain"
)

func TestCopyFrom(t *testing.T) {
	e := newEnv(t)
	s := svc(e, domain.BudgetCap)
	ctx := context.Background()
	set(t, s, budgeting.SetRequest{Name: "Food", Categories: []string{"Food"}, Amount: "100"})
	set(t, s, budgeting.SetRequest{Name: "Food", Month: "2026-01", Amount: "300"})
	set(t, s, budgeting.SetRequest{Name: "Food", Month: "2026-02", Amount: "200"})
	set(t, s, budgeting.SetRequest{Name: "Home", Categories: []string{"Home"}, Amount: "50", Period: "year"})

	cp, err := s.PlanCopy(ctx, budgeting.CopyRequest{Month: "2026-03"})
	if err != nil || len(cp.Updates) != 1 || cp.Updates[0].Overrides[domain.MonthKey{Year: 2026, Month: 3}].Minor() != 20000 {
		t.Fatalf("default previous: %+v, %v", cp, err)
	}
	cp, err = s.PlanCopy(ctx, budgeting.CopyRequest{Month: "2026-03", From: "2026-01"})
	if err != nil || len(cp.Updates) != 1 || cp.Updates[0].Overrides[domain.MonthKey{Year: 2026, Month: 3}].Minor() != 30000 {
		t.Fatalf("explicit from: %+v, %v", cp, err)
	}
	if _, err := s.PlanCopy(ctx, budgeting.CopyRequest{Month: "2026-03", From: "2026-03"}); !isInvalid(err) {
		t.Errorf("from == month: %v", err)
	}
	if _, err := s.PlanCopy(ctx, budgeting.CopyRequest{Month: "2026-03", From: "zz"}); err == nil {
		t.Error("bad from: want error")
	}
}

func isInvalid(err error) bool { return errors.Is(err, domain.ErrInvalidValue) }

func TestCopyFromEnvelope(t *testing.T) {
	e := newEnv(t)
	s := svc(e, domain.BudgetEnvelope)
	ctx := context.Background()
	set(t, s, budgeting.SetRequest{Name: "Food", Categories: []string{"Food"}, Amount: "0"})
	for _, req := range []budgeting.MoveRequest{
		{To: "Food", Amount: "300", Date: "2026-01-05"},
		{From: "Food", Amount: "50", Date: "2026-01-20"},
		{To: "Food", Amount: "999", Date: "2026-02-05"},
	} {
		p, err := s.PlanMove(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.ApplyMove(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	cp, err := s.PlanCopy(ctx, budgeting.CopyRequest{Month: "2026-03", From: "2026-01"})
	if err != nil || len(cp.Moves) != 1 || cp.Moves[0].Amount.Minor() != 25000 || cp.Moves[0].Date.String() != "2026-03-01" {
		t.Fatalf("copy plan = %+v, %v", cp, err)
	}
}
