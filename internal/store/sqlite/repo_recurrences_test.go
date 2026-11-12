package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func TestRecurrencePendingOfAndDue(t *testing.T) {
	s, _ := newTestStore(t)
	seedSearchRefs(t, s)
	ctx := context.Background()
	eur := mustCurrency(t, "EUR")
	rule, err := domain.NewRecurrenceRule(1, domain.RecurrenceMonths, 0)
	if err != nil {
		t.Fatal(err)
	}

	var rec app.Recurrence
	if err := s.WithTx(ctx, func(r app.Repos) (err error) {
		rec, err = r.Recurrences().Insert(ctx, app.Recurrence{
			AccountID:   1,
			CategoryID:  1,
			Description: "Rent",
			Amount:      mustMoney(t, -50000, eur),
			Rule:        rule,
			Start:       mustDate(t, "2026-01-01"),
			NextDate:    mustDate(t, "2026-02-01"),
		})
		return err
	}); err != nil {
		t.Fatalf("insert recurrence: %v", err)
	}

	var txA, txB, txC domain.Transaction
	if err := s.WithTx(ctx, func(r app.Repos) error {
		var err error
		txA, err = r.Transactions().Insert(ctx, domain.Transaction{
			AccountID: 1, Date: mustDate(t, "2026-02-01"), Amount: mustMoney(t, -50000, eur),
			Status: domain.StatusPending, Kind: domain.KindPayment, Description: "Rent A",
		})
		if err != nil {
			return err
		}
		txB, err = r.Transactions().Insert(ctx, domain.Transaction{
			AccountID: 1, Date: mustDate(t, "2026-01-31"), Amount: mustMoney(t, -50000, eur),
			Status: domain.StatusPending, Kind: domain.KindPayment, Description: "Rent B",
		})
		if err != nil {
			return err
		}
		txC, err = r.Transactions().Insert(ctx, domain.Transaction{
			AccountID: 1, Date: mustDate(t, "2026-02-01"), Amount: mustMoney(t, -50000, eur),
			Status: domain.StatusBooked, Kind: domain.KindPayment, Description: "Rent C",
		})
		return err
	}); err != nil {
		t.Fatalf("insert txs: %v", err)
	}

	_, err = s.writeDB.ExecContext(ctx, `UPDATE transactions
		SET recurrence_id = ?, due_date = CASE id
			WHEN ? THEN '2026-02-02'
			WHEN ? THEN '2026-01-30'
			WHEN ? THEN '2026-01-29'
		END
		WHERE id IN (?, ?, ?)`, rec.ID, txA.ID, txB.ID, txC.ID, txA.ID, txB.ID, txC.ID)
	if err != nil {
		t.Fatalf("link txs to recurrence: %v", err)
	}

	if err := s.Read(ctx, func(r app.Repos) error {
		rows, err := r.Recurrences().PendingOf(ctx, rec.ID)
		if err != nil {
			return err
		}
		if len(rows) != 2 || rows[0].ID != txB.ID || rows[1].ID != txA.ID {
			t.Fatalf("PendingOf = %+v, want [%d %d]", rows, txB.ID, txA.ID)
		}
		dueRows, err := r.Recurrences().Due(ctx, mustDate(t, "2026-02-01"))
		if err != nil {
			return err
		}
		if len(dueRows) != 1 || dueRows[0].ID != rec.ID {
			t.Fatalf("Due = %+v, want recurrence %d", dueRows, rec.ID)
		}
		_, err = r.Recurrences().PendingOf(ctx, app.RecurrenceID(999))
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("PendingOf missing recurrence error = %v, want ErrNotFound", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRecurrenceCRUDAndValidation(t *testing.T) {
	s, _ := newTestStore(t)
	seedSearchRefs(t, s)
	ctx := context.Background()
	eur := mustCurrency(t, "EUR")

	if err := s.Read(ctx, func(r app.Repos) error {
		_, err := r.Recurrences().Due(ctx, domain.Date{})
		if !errors.Is(err, domain.ErrMissingField) {
			t.Fatalf("Due zero date error = %v, want ErrMissingField", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	rule, err := domain.NewRecurrenceRule(1, domain.RecurrenceMonths, 31)
	if err != nil {
		t.Fatal(err)
	}
	var rec app.Recurrence
	if err := s.WithTx(ctx, func(r app.Repos) error {
		_, err := r.Recurrences().Insert(ctx, app.Recurrence{
			AccountID: 1, CategoryID: 99,
			Amount: mustMoney(t, -100, eur), Rule: rule,
			Start: mustDate(t, "2026-01-01"), NextDate: mustDate(t, "2026-01-31"),
		})
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("insert with missing category error = %v, want ErrNotFound", err)
		}
		rec, err = r.Recurrences().Insert(ctx, app.Recurrence{
			AccountID: 1, CategoryID: 1, Description: " salary ",
			Amount: mustMoney(t, 120000, eur), Rule: rule,
			Start: mustDate(t, "2026-01-01"), End: mustDate(t, "2026-12-31"), NextDate: mustDate(t, "2026-01-31"),
		})
		return err
	}); err != nil {
		t.Fatal(err)
	}

	if rec.ID == 0 || rec.Description != "salary" {
		t.Fatalf("inserted recurrence = %+v", rec)
	}
	if err := s.Read(ctx, func(r app.Repos) error {
		got, err := r.Recurrences().Get(ctx, rec.ID)
		if err != nil {
			return err
		}
		if got.ID != rec.ID || got.End.IsZero() {
			t.Fatalf("Get = %+v, want %+v", got, rec)
		}
		all, err := r.Recurrences().List(ctx)
		if err != nil {
			return err
		}
		if len(all) != 1 || all[0].ID != rec.ID {
			t.Fatalf("List = %+v, want only recurrence %d", all, rec.ID)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	rec.Description = "salary updated"
	if err := s.WithTx(ctx, func(r app.Repos) error {
		if err := r.Recurrences().Update(ctx, rec); err != nil {
			return err
		}
		bad := rec
		bad.Rule = domain.RecurrenceRule{Every: 0, Unit: domain.RecurrenceMonths}
		err := r.Recurrences().Update(ctx, bad)
		if !errors.Is(err, domain.ErrInvalidValue) {
			t.Fatalf("invalid recurrence update error = %v, want ErrInvalidValue", err)
		}
		err = r.Recurrences().Update(ctx, app.Recurrence{ID: 999})
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("update missing recurrence error = %v, want ErrNotFound", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.WithTx(ctx, func(r app.Repos) error {
		if err := r.Recurrences().Delete(ctx, rec.ID); err != nil {
			return err
		}
		err := r.Recurrences().Delete(ctx, rec.ID)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("delete missing recurrence error = %v, want ErrNotFound", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
