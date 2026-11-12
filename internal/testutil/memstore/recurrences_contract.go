package memstore

import (
	"context"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func contractRecurrences(t *testing.T, f *fixture) {
	ctx := context.Background()
	acct, cat := recurrenceRefs(ctx, t, f)
	due, later := insertRecurrences(ctx, t, f, acct, cat)
	if due.ID == 0 || later.ID == 0 || due.Description != "rent" {
		t.Fatalf("inserted recurrences = %+v and %+v", due, later)
	}
	assertRecurrenceErrors(ctx, t, f)
	assertRecurrenceLists(ctx, t, f, due, later)

	due.NextDate = mustDate(t, "2026-04-15")
	if err := f.store.WithTx(ctx, func(r app.Repos) error {
		return r.Recurrences().Update(ctx, due)
	}); err != nil {
		t.Fatalf("Update recurrence: %v", err)
	}
	f.do(t, func(r app.Repos) error {
		rows, err := r.Recurrences().Due(ctx, mustDate(t, "2026-03-10"))
		if err != nil || len(rows) != 1 || rows[0].ID != later.ID {
			t.Errorf("Due after update = %+v, %v; want only later", rows, err)
		}
		if err := r.Recurrences().Delete(ctx, due.ID); err != nil {
			t.Errorf("Delete recurrence: %v", err)
		}
		if rows, _ := r.Recurrences().List(ctx); len(rows) != 1 || rows[0].ID != later.ID {
			t.Errorf("List after delete = %+v; want only later", rows)
		}
		return nil
	})
}

func recurrenceRefs(ctx context.Context, t *testing.T, f *fixture) (domain.Account, domain.Category) {
	t.Helper()
	acct := f.account(t, "A")
	var cat domain.Category
	f.do(t, func(r app.Repos) (err error) {
		cat, err = r.Categories().Insert(ctx, domain.Category{Name: "Rent", Kind: domain.CategoryExpense})
		return err
	})
	return acct, cat
}

func insertRecurrences(ctx context.Context, t *testing.T, f *fixture, acct domain.Account, cat domain.Category) (app.Recurrence, app.Recurrence) {
	t.Helper()
	base := app.Recurrence{
		AccountID:   acct.ID,
		CategoryID:  cat.ID,
		Description: " rent ",
		Amount:      mustMoney(t, -900),
		Rule:        mustRecurrenceRule(t, 1, domain.RecurrenceMonths, 0),
		Start:       mustDate(t, "2026-01-15"),
		NextDate:    mustDate(t, "2026-02-15"),
	}
	var due, later app.Recurrence
	f.do(t, func(r app.Repos) error {
		var err error
		due, err = r.Recurrences().Insert(ctx, base)
		if err != nil {
			return err
		}
		later, err = r.Recurrences().Insert(ctx, app.Recurrence{
			AccountID: acct.ID, Amount: mustMoney(t, 2500), CategoryID: 0,
			Rule:  mustRecurrenceRule(t, 2, domain.RecurrenceWeeks, 0),
			Start: mustDate(t, "2026-01-01"), NextDate: mustDate(t, "2026-03-01"),
		})
		return err
	})
	return due, later
}

func assertRecurrenceErrors(ctx context.Context, t *testing.T, f *fixture) {
	t.Helper()
	f.do(t, func(r app.Repos) error {
		_, err := r.Recurrences().Insert(ctx, app.Recurrence{
			AccountID: 99, Amount: mustMoney(t, -10),
			Rule:  mustRecurrenceRule(t, 1, domain.RecurrenceDays, 0),
			Start: mustDate(t, "2026-01-01"), NextDate: mustDate(t, "2026-01-01"),
		})
		wantErr(t, err, domain.ErrNotFound, "Insert recurrence with unknown account")
		_, err = r.Recurrences().Get(ctx, 99)
		wantErr(t, err, domain.ErrNotFound, "Get missing recurrence")
		wantErr(t, r.Recurrences().Update(ctx, app.Recurrence{ID: 99}), domain.ErrNotFound, "Update missing recurrence")
		wantErr(t, r.Recurrences().Delete(ctx, 99), domain.ErrNotFound, "Delete missing recurrence")
		return nil
	})
}

func assertRecurrenceLists(ctx context.Context, t *testing.T, f *fixture, due, later app.Recurrence) {
	t.Helper()
	f.do(t, func(r app.Repos) error {
		rows, err := r.Recurrences().List(ctx)
		if err != nil || len(rows) != 2 || rows[0].ID != due.ID || rows[1].ID != later.ID {
			t.Errorf("List = %+v, %v", rows, err)
		}
		dueRows, err := r.Recurrences().Due(ctx, mustDate(t, "2026-02-20"))
		if err != nil || len(dueRows) != 1 || dueRows[0].ID != due.ID {
			t.Errorf("Due = %+v, %v; want only first", dueRows, err)
		}
		first, err := r.Transactions().Insert(ctx, domain.Transaction{
			AccountID: due.AccountID, Date: mustDate(t, "2026-02-14"), DueDate: mustDate(t, "2026-02-16"),
			RecurrenceID: domain.RecurrenceID(due.ID), Amount: mustMoney(t, -900),
			Status: domain.StatusPending, Kind: domain.KindPayment, Description: "pending A",
		})
		if err != nil {
			return err
		}
		second, err := r.Transactions().Insert(ctx, domain.Transaction{
			AccountID: due.AccountID, Date: mustDate(t, "2026-02-10"), DueDate: mustDate(t, "2026-02-15"),
			RecurrenceID: domain.RecurrenceID(due.ID), Amount: mustMoney(t, -900),
			Status: domain.StatusPending, Kind: domain.KindPayment, Description: "pending B",
		})
		if err != nil {
			return err
		}
		if _, err := r.Transactions().Insert(ctx, domain.Transaction{
			AccountID: due.AccountID, Date: mustDate(t, "2026-02-09"), DueDate: mustDate(t, "2026-02-13"),
			RecurrenceID: domain.RecurrenceID(due.ID), Amount: mustMoney(t, -900),
			Status: domain.StatusBooked, Kind: domain.KindPayment, Description: "booked",
		}); err != nil {
			return err
		}
		pending, err := r.Recurrences().PendingOf(ctx, due.ID)
		if err != nil || len(pending) != 2 || pending[0].ID != second.ID || pending[1].ID != first.ID {
			t.Errorf("PendingOf = %+v, %v; want [%d %d]", pending, err, second.ID, first.ID)
		}
		return nil
	})
}

func mustRecurrenceRule(t *testing.T, every int, unit domain.RecurrenceUnit, dayOfMonth int) domain.RecurrenceRule {
	t.Helper()
	rule, err := domain.NewRecurrenceRule(every, unit, dayOfMonth)
	if err != nil {
		t.Fatal(err)
	}
	return rule
}
