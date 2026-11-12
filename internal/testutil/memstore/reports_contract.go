package memstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func contractSavedReports(t *testing.T, f *fixture) {
	ctx := context.Background()
	one := openContractSavedReport(ctx, t, f)
	if one.ID == 0 {
		t.Fatal("inserted report has zero ID")
	}

	checkContractSavedReportReadAndUpdate(ctx, t, f, &one)
	checkContractSavedReportDelete(ctx, t, f, one.ID)
}

func openContractSavedReport(ctx context.Context, t *testing.T, f *fixture) app.SavedReport {
	t.Helper()
	var one app.SavedReport
	f.do(t, func(r app.Repos) error {
		var err error
		one, err = r.SavedReports().Insert(ctx, app.SavedReport{
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
			t.Fatalf("duplicate name err = %v, want already_exists", err)
		}
		return nil
	})
	return one
}

func checkContractSavedReportReadAndUpdate(ctx context.Context, t *testing.T, f *fixture, one *app.SavedReport) {
	t.Helper()
	f.do(t, func(r app.Repos) error {
		got, err := r.SavedReports().Get(ctx, one.ID)
		if err != nil || got.Name != one.Name {
			t.Fatalf("Get = %+v, %v", got, err)
		}
		gotByName, err := r.SavedReports().GetByName(ctx, "monthly cashflow")
		if err != nil || gotByName.ID != one.ID {
			t.Fatalf("GetByName = %+v, %v", gotByName, err)
		}
		one.ParamsJSON = `{"period":{"kind":"current_year"}}`
		if err := r.SavedReports().Update(ctx, *one); err != nil {
			t.Fatalf("Update: %v", err)
		}
		got, err = r.SavedReports().Get(ctx, one.ID)
		if err != nil || got.ParamsJSON != one.ParamsJSON {
			t.Fatalf("Get after update = %+v, %v", got, err)
		}
		list, err := r.SavedReports().List(ctx)
		if err != nil || len(list) != 1 || list[0].ID != one.ID {
			t.Fatalf("List = %+v, %v", list, err)
		}
		return nil
	})
}

func checkContractSavedReportDelete(ctx context.Context, t *testing.T, f *fixture, id app.SavedReportID) {
	t.Helper()
	f.do(t, func(r app.Repos) error {
		if err := r.SavedReports().Delete(ctx, id); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if _, err := r.SavedReports().Get(ctx, id); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("Get missing err = %v, want not_found", err)
		}
		if err := r.SavedReports().Delete(ctx, id); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("Delete missing err = %v, want not_found", err)
		}
		return nil
	})
}

func contractReconciliations(t *testing.T, f *fixture) {
	ctx := context.Background()
	acct := f.account(t, "Recon")
	eur := mustEUR(t)
	balance, err := domain.NewMoney(120_000, eur)
	if err != nil {
		t.Fatal(err)
	}
	d1 := mustDate(t, "2026-01-31")
	d2 := mustDate(t, "2026-02-28")
	closed1 := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
	closed2 := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	other := f.account(t, "No reconciliations yet")

	openAndCheckDuplicateReconciliation(ctx, t, f, acct.ID, d1, balance)
	closeAndCheckLastReconciliation(ctx, t, f, acct.ID, d1, d2, balance, closed1, closed2)
	checkMissingReconciliationRows(ctx, t, f, acct.ID, other.ID, closed1)
}

func openAndCheckDuplicateReconciliation(
	ctx context.Context, t *testing.T, f *fixture, accountID domain.AccountID, date domain.Date, balance domain.Money,
) {
	t.Helper()
	f.do(t, func(r app.Repos) error {
		first, err := r.Reconciliations().Open(ctx, app.Reconciliation{
			AccountID:        accountID,
			Date:             date,
			StatementBalance: balance,
		})
		if err != nil {
			return err
		}
		if first.ID == 0 || !first.ClosedAt.IsZero() {
			t.Fatalf("Open = %+v", first)
		}
		_, err = r.Reconciliations().Open(ctx, app.Reconciliation{
			AccountID:        accountID,
			Date:             date,
			StatementBalance: balance,
		})
		if !errors.Is(err, app.ErrAlreadyExists) {
			t.Fatalf("duplicate open err = %v, want already_exists", err)
		}
		return nil
	})
}

func closeAndCheckLastReconciliation(
	ctx context.Context, t *testing.T, f *fixture, accountID domain.AccountID, d1, d2 domain.Date, balance domain.Money, closed1, closed2 time.Time,
) {
	t.Helper()
	f.do(t, func(r app.Repos) error {
		if err := r.Reconciliations().Close(ctx, accountID, d1, closed1); err != nil {
			t.Fatalf("Close d1: %v", err)
		}
		if _, err := r.Reconciliations().Open(ctx, app.Reconciliation{
			AccountID:        accountID,
			Date:             d2,
			StatementBalance: balance,
		}); err != nil {
			t.Fatalf("Open d2: %v", err)
		}
		if err := r.Reconciliations().Close(ctx, accountID, d2, closed2); err != nil {
			t.Fatalf("Close d2: %v", err)
		}
		got, err := r.Reconciliations().Get(ctx, accountID, d1)
		if err != nil || got.ClosedAt.IsZero() {
			t.Fatalf("Get = %+v, %v", got, err)
		}
		last, err := r.Reconciliations().LastClosed(ctx, accountID)
		if err != nil || last.Date != d2 {
			t.Fatalf("LastClosed = %+v, %v; want date %v", last, err, d2)
		}
		return nil
	})
}

func checkMissingReconciliationRows(
	ctx context.Context, t *testing.T, f *fixture, accountID, otherAccountID domain.AccountID, closedAt time.Time,
) {
	t.Helper()
	f.do(t, func(r app.Repos) error {
		if err := r.Reconciliations().Close(ctx, accountID, mustDate(t, "2026-04-30"), closedAt); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("Close missing err = %v, want not_found", err)
		}
		if _, err := r.Reconciliations().LastClosed(ctx, otherAccountID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("LastClosed missing err = %v, want not_found", err)
		}
		return nil
	})
}
