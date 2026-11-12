package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func TestSavedReportRepoCRUD(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	var first app.SavedReport
	if err := store.WithTx(ctx, func(r app.Repos) error {
		var err error
		first, err = r.SavedReports().Insert(ctx, app.SavedReport{
			Name:       "Monthly cashflow",
			Kind:       "cashflow",
			ParamsJSON: `{"period":{"kind":"last_months","value":3}}`,
		})
		if err != nil {
			return err
		}
		_, err = r.SavedReports().Insert(ctx, app.SavedReport{
			Name:       "monthly CASHFLOW",
			Kind:       "cashflow",
			ParamsJSON: `{}`,
		})
		if !errors.Is(err, app.ErrAlreadyExists) {
			t.Fatalf("insert duplicate report err = %v, want already_exists", err)
		}
		first.ParamsJSON = `{"period":{"kind":"current_year"}}`
		if err := r.SavedReports().Update(ctx, first); err != nil {
			return err
		}
		list, err := r.SavedReports().List(ctx)
		if err != nil || len(list) != 1 || list[0].ID != first.ID {
			t.Fatalf("List = %+v, %v", list, err)
		}
		return nil
	}); err != nil {
		t.Fatalf("saved reports flow: %v", err)
	}

	if err := store.Read(ctx, func(r app.Repos) error {
		got, err := r.SavedReports().GetByName(ctx, "monthly cashflow")
		if err != nil || got.ParamsJSON != first.ParamsJSON {
			t.Fatalf("GetByName = %+v, %v", got, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := store.WithTx(ctx, func(r app.Repos) error {
		if err := r.SavedReports().Delete(ctx, first.ID); err != nil {
			return err
		}
		if err := r.SavedReports().Delete(ctx, first.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("delete missing err = %v, want not_found", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSavedReportAndReconciliationValidation(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	eur := mustCurrency(t, "EUR")
	balance := mustMoney(t, 100, eur)
	date := mustDate(t, "2026-01-31")

	if err := store.WithTx(ctx, func(r app.Repos) error {
		_, err := r.SavedReports().Insert(ctx, app.SavedReport{
			Name:       " ",
			Kind:       "cashflow",
			ParamsJSON: `{}`,
		})
		if !errors.Is(err, domain.ErrMissingField) {
			t.Fatalf("insert empty name err = %v, want missing_field", err)
		}

		err = r.SavedReports().Update(ctx, app.SavedReport{
			ID:         999,
			Name:       "Missing",
			Kind:       "cashflow",
			ParamsJSON: `{}`,
		})
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("update missing report err = %v, want not_found", err)
		}

		if _, err := r.SavedReports().GetByName(ctx, "missing"); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("get by missing name err = %v, want not_found", err)
		}

		_, err = r.Reconciliations().Open(ctx, app.Reconciliation{
			AccountID:        0,
			Date:             date,
			StatementBalance: balance,
		})
		if !errors.Is(err, domain.ErrMissingField) {
			t.Fatalf("open reconciliation missing account err = %v, want missing_field", err)
		}

		_, err = r.Reconciliations().Open(ctx, app.Reconciliation{
			AccountID:        999,
			Date:             date,
			StatementBalance: balance,
		})
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("open reconciliation missing account row err = %v, want not_found", err)
		}

		err = r.Reconciliations().Close(ctx, 1, date, time.Time{})
		if !errors.Is(err, domain.ErrMissingField) {
			t.Fatalf("close reconciliation missing closed_at err = %v, want missing_field", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestReconciliationRepoFlow(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	eur := mustCurrency(t, "EUR")
	acct := insertAccountForTest(t, store, "Checking")
	statementBalance := mustMoney(t, 123_400, eur)
	d1 := mustDate(t, "2026-01-31")
	d2 := mustDate(t, "2026-02-28")
	closedAt1 := time.Date(2026, 2, 1, 11, 12, 13, 0, time.UTC)
	closedAt2 := time.Date(2026, 3, 1, 11, 12, 13, 0, time.UTC)

	if err := store.WithTx(ctx, func(r app.Repos) error {
		first, err := r.Reconciliations().Open(ctx, app.Reconciliation{
			AccountID:        acct.ID,
			Date:             d1,
			StatementBalance: statementBalance,
		})
		if err != nil {
			return err
		}
		if first.ID == 0 || !first.ClosedAt.IsZero() {
			t.Fatalf("Open = %+v", first)
		}
		if _, err := r.Reconciliations().Open(ctx, app.Reconciliation{
			AccountID:        acct.ID,
			Date:             d1,
			StatementBalance: statementBalance,
		}); !errors.Is(err, app.ErrAlreadyExists) {
			t.Fatalf("open duplicate err = %v, want already_exists", err)
		}
		if err := r.Reconciliations().Close(ctx, acct.ID, d1, closedAt1); err != nil {
			return err
		}
		if _, err := r.Reconciliations().Open(ctx, app.Reconciliation{
			AccountID:        acct.ID,
			Date:             d2,
			StatementBalance: statementBalance,
		}); err != nil {
			return err
		}
		if err := r.Reconciliations().Close(ctx, acct.ID, d2, closedAt2); err != nil {
			return err
		}
		got, err := r.Reconciliations().Get(ctx, acct.ID, d1)
		if err != nil || got.ClosedAt.IsZero() {
			t.Fatalf("Get = %+v, %v", got, err)
		}
		last, err := r.Reconciliations().LastClosed(ctx, acct.ID)
		if err != nil || last.Date != d2 {
			t.Fatalf("LastClosed = %+v, %v; want %v", last, err, d2)
		}
		return nil
	}); err != nil {
		t.Fatalf("reconciliations flow: %v", err)
	}

	if err := store.WithTx(ctx, func(r app.Repos) error {
		if err := r.Reconciliations().Close(ctx, acct.ID, mustDate(t, "2026-04-30"), closedAt1); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("close missing err = %v, want not_found", err)
		}
		if _, err := r.Reconciliations().LastClosed(ctx, 999); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("last closed missing account err = %v, want not_found", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
