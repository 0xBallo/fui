package app_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func TestAddTx(t *testing.T) {
	tests := []struct {
		name    string
		locale  domain.Locale
		req     app.AddTxRequest
		wantErr *domain.Error
		check   func(t *testing.T, tx domain.Transaction)
	}{
		{
			name: "defaults: today, booked, payment, no category",
			req:  app.AddTxRequest{Account: "Conto", Amount: "-12.50", Description: "  Esselunga "},
			check: func(t *testing.T, tx domain.Transaction) {
				if tx.Date.String() != "2026-11-11" || tx.Status != domain.StatusBooked || tx.Kind != domain.KindPayment ||
					tx.Amount.Minor() != -1250 || tx.Description != "Esselunga" || tx.CategoryID != 0 {
					t.Errorf("tx = %+v", tx)
				}
			},
		},
		{
			name:   "italian locale, relative date, category by prefix",
			locale: domain.LocaleITIT,
			req:    app.AddTxRequest{Account: "Contanti", Amount: "-12,50", Date: "-3d", Category: "spe", Status: "pending", Kind: "fee", Notes: "n"},
			check: func(t *testing.T, tx domain.Transaction) {
				if tx.Date.String() != "2026-11-08" || tx.CategoryID == 0 || tx.Status != domain.StatusPending ||
					tx.Kind != domain.KindFee || tx.Notes != "n" || tx.Amount.Minor() != -1250 {
					t.Errorf("tx = %+v", tx)
				}
			},
		},
		{name: "account missing", req: app.AddTxRequest{Amount: "1"}, wantErr: domain.ErrMissingField},
		{name: "account ambiguous", req: app.AddTxRequest{Account: "c", Amount: "1"}, wantErr: domain.ErrAmbiguousName},
		{name: "account unknown", req: app.AddTxRequest{Account: "zzz", Amount: "1"}, wantErr: domain.ErrNotFound},
		{name: "archived account is not addressable", req: app.AddTxRequest{Account: "Vecchio", Amount: "1"}, wantErr: domain.ErrNotFound},
		{name: "category unknown", req: app.AddTxRequest{Account: "Contanti", Amount: "1", Category: "zzz"}, wantErr: domain.ErrNotFound},
		{name: "amount invalid", req: app.AddTxRequest{Account: "Contanti", Amount: "x"}, wantErr: domain.ErrInvalidAmount},
		{name: "date invalid", req: app.AddTxRequest{Account: "Contanti", Amount: "1", Date: "x"}, wantErr: domain.ErrInvalidDate},
		{name: "status invalid", req: app.AddTxRequest{Account: "Contanti", Amount: "1", Status: "x"}, wantErr: domain.ErrInvalidValue},
		{name: "kind invalid", req: app.AddTxRequest{Account: "Contanti", Amount: "1", Kind: "x"}, wantErr: domain.ErrInvalidValue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t, func(c *app.Config) {
				if tt.locale != "" {
					c.Locale = tt.locale
				}
			})
			e.account(t, "Contanti", "cash", "EUR", "0")
			e.account(t, "Conto", "checking", "EUR", "0")
			e.account(t, "Vecchio", "cash", "EUR", "0")
			ap, err := e.svc.PlanArchiveAccount(t.Context(), "Vecchio")
			wantNoErr(t, err)
			_, err = e.svc.ApplyArchiveAccount(t.Context(), ap)
			wantNoErr(t, err)
			e.category(t, "Spesa", "expense")

			p, err := e.svc.PlanAddTx(t.Context(), tt.req)
			if tt.wantErr != nil {
				wantCode(t, err, tt.wantErr)
				return
			}
			wantNoErr(t, err)
			if len(p.Changes) != 1 || p.Changes[0].Op != app.OpCreate || p.Changes[0].Entity != app.EntityTransaction {
				t.Errorf("changes = %+v", p.Changes)
			}
			tx, err := e.svc.ApplyAddTx(t.Context(), p)
			wantNoErr(t, err)
			tt.check(t, tx)
		})
	}
}

func TestAmbiguousAccountListsCandidates(t *testing.T) {
	e := newEnv(t)
	e.account(t, "Fineco", "checking", "EUR", "0")
	e.account(t, "Fineco Risparmio", "savings", "EUR", "0")
	e.account(t, "Cash", "cash", "EUR", "0")
	_, err := e.svc.PlanAddTx(t.Context(), app.AddTxRequest{Account: "fin", Amount: "1"})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.CodeAmbiguousName {
		t.Fatalf("error = %v, want ambiguous_name", err)
	}
	if got := fmt.Sprint(de.Details["candidates"]); got != "[Fineco Fineco Risparmio]" {
		t.Errorf("candidates = %s", got)
	}
}

func TestListTx(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	e.account(t, "A", "checking", "EUR", "0")
	e.account(t, "B", "checking", "EUR", "0")
	e.account(t, "Usd", "checking", "USD", "0")
	e.category(t, "Food", "expense")
	e.category(t, "Fun", "expense")
	for i := range 7 {
		e.tx(t, app.AddTxRequest{
			Account: []string{"A", "B"}[i%2], Amount: fmt.Sprint(-(i + 1) * 10), Description: fmt.Sprintf("Shop %d", i),
			Date: fmt.Sprintf("2026-10-%02d", 1+i/2), Category: []string{"Food", "Fun"}[i%2], Status: []string{"booked", "pending"}[i%2],
		})
	}
	count := func(req app.ListTxRequest) int {
		t.Helper()
		page, err := e.svc.ListTx(ctx, req)
		wantNoErr(t, err)
		return len(page.Items)
	}
	tests := []struct {
		name string
		req  app.ListTxRequest
		want int
	}{
		{"all", app.ListTxRequest{}, 7},
		{"period", app.ListTxRequest{From: "2026-10-02", To: "2026-10-03"}, 4},
		{"account", app.ListTxRequest{Accounts: []string{"a"}}, 4},
		{"category", app.ListTxRequest{Categories: []string{"fun"}}, 3},
		{"status", app.ListTxRequest{Statuses: []string{"Pending"}}, 3},
		{"text", app.ListTxRequest{Text: "shop 3"}, 1},
		{"amount range", app.ListTxRequest{Accounts: []string{"A", "B"}, MinAmount: "20", MaxAmount: "40"}, 3},
		{"amount range over EUR accounts only", app.ListTxRequest{Accounts: []string{"A"}, MinAmount: "30"}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := count(tt.req); got != tt.want {
				t.Errorf("%d rows, want %d", got, tt.want)
			}
		})
	}
	for name, req := range map[string]app.ListTxRequest{
		"amount range over mixed currencies": {MinAmount: "1"},
		"bad status":                         {Statuses: []string{"x"}},
		"unknown account":                    {Accounts: []string{"zzz"}},
		"bad date":                           {From: "x"},
	} {
		if _, err := e.svc.ListTx(ctx, req); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}

	var seen []domain.TransactionID
	req := app.ListTxRequest{Limit: 3}
	for pages := 0; ; pages++ {
		page, err := e.svc.ListTx(ctx, req)
		wantNoErr(t, err)
		for _, it := range page.Items {
			seen = append(seen, it.Transaction.ID)
			if it.Account == "" || it.Category == "" {
				t.Errorf("view without names: %+v", it)
			}
		}
		if page.Next == nil {
			break
		}
		if pages > 3 {
			t.Fatal("pagination does not terminate")
		}
		req.After = page.Next
	}
	if len(seen) != 7 {
		t.Errorf("pagination returned %d rows, want 7", len(seen))
	}
}

// TestListTxFilters covers the T17 additions to ListTxRequest: Month, Year, Uncategorized,
// Kinds, Sign and the Payees not-yet-supported error.
func TestListTxFilters(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	e.account(t, "A", "checking", "EUR", "0")
	e.category(t, "Food", "expense")
	e.tx(t, app.AddTxRequest{Account: "A", Amount: "-10", Description: "Jan payment", Date: "2026-01-10", Category: "Food", Kind: "payment"})
	e.tx(t, app.AddTxRequest{Account: "A", Amount: "-20", Description: "Feb fee", Date: "2026-02-10", Category: "Food", Kind: "fee"})
	e.tx(t, app.AddTxRequest{Account: "A", Amount: "5", Description: "Jan refund, no category", Date: "2026-01-15", Kind: "refund"})

	count := func(req app.ListTxRequest) int {
		t.Helper()
		page, err := e.svc.ListTx(ctx, req)
		wantNoErr(t, err)
		return len(page.Items)
	}
	tests := []struct {
		name string
		req  app.ListTxRequest
		want int
	}{
		{"month", app.ListTxRequest{Month: "2026-01"}, 2},
		{"year", app.ListTxRequest{Year: 2026}, 3},
		{"uncategorized", app.ListTxRequest{Uncategorized: true}, 1},
		{"kind", app.ListTxRequest{Kinds: []string{"fee"}}, 1},
		{"sign income", app.ListTxRequest{Sign: domain.TxSignIncome}, 1},
		{"sign expense", app.ListTxRequest{Sign: domain.TxSignExpense}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := count(tt.req); got != tt.want {
				t.Errorf("%d rows, want %d", got, tt.want)
			}
		})
	}
	for name, req := range map[string]app.ListTxRequest{
		"payee not supported yet": {Payees: []string{"Esselunga"}},
		"bad kind":                {Kinds: []string{"x"}},
		"month and year":          {Month: "2026-01", Year: 2026},
	} {
		if _, err := e.svc.ListTx(ctx, req); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	var de *domain.Error
	if _, err := e.svc.ListTx(ctx, app.ListTxRequest{Payees: []string{"x"}}); !errors.As(err, &de) || de.Code != domain.CodeNotFound {
		t.Errorf("payee filter error = %v, want not_found", err)
	}

	// Sort without Desc is ascending; the default (no Sort) is newest first.
	page, err := e.svc.ListTx(ctx, app.ListTxRequest{Sort: domain.TxSortAmount})
	wantNoErr(t, err)
	if len(page.Items) != 3 || page.Items[0].Transaction.Amount.Minor() > page.Items[1].Transaction.Amount.Minor() {
		t.Errorf("sort amount ascending: %+v", page.Items)
	}
	page, err = e.svc.ListTx(ctx, app.ListTxRequest{})
	wantNoErr(t, err)
	if len(page.Items) != 3 || page.Items[0].Transaction.Date.String() != "2026-02-10" {
		t.Errorf("default newest first: %+v", page.Items)
	}
}

func TestShowEditRemoveTx(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	e.account(t, "A", "checking", "EUR", "0")
	e.category(t, "Food", "expense")
	tx := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-10", Description: "Lunch", Date: "2026-10-01"})

	v, err := e.svc.ShowTx(ctx, tx.ID)
	wantNoErr(t, err)
	if v.Account != "A" || v.Transfer != nil || v.Transaction != tx {
		t.Errorf("ShowTx = %+v", v)
	}
	_, err = e.svc.ShowTx(ctx, 999)
	wantCode(t, err, domain.ErrNotFound)

	p, err := e.svc.PlanEditTx(ctx, app.EditTxRequest{
		ID: tx.ID, Amount: ptr("-11.5"), Date: ptr("2026-10-02"), Description: ptr("Dinner"), Notes: ptr("n"),
		Category: ptr("food"), Status: ptr("pending"), Kind: ptr("other"),
	})
	wantNoErr(t, err)
	got, err := e.svc.ApplyEditTx(ctx, p)
	wantNoErr(t, err)
	if got.Amount.Minor() != -1150 || got.Date.String() != "2026-10-02" || got.Description != "Dinner" || got.CategoryID == 0 ||
		got.Status != domain.StatusPending || got.Kind != domain.KindOther || got.Notes != "n" {
		t.Errorf("edited = %+v", got)
	}
	p, err = e.svc.PlanEditTx(ctx, app.EditTxRequest{ID: tx.ID, Category: ptr("")})
	wantNoErr(t, err)
	if got, _ = e.svc.ApplyEditTx(ctx, p); got.CategoryID != 0 {
		t.Errorf("category not cleared: %+v", got)
	}
	_, err = e.svc.PlanEditTx(ctx, app.EditTxRequest{ID: tx.ID})
	wantCode(t, err, domain.ErrMissingField)
	_, err = e.svc.PlanEditTx(ctx, app.EditTxRequest{ID: 999, Notes: ptr("x")})
	wantCode(t, err, domain.ErrNotFound)

	rp, err := e.svc.PlanRemoveTx(ctx, app.RemoveTxRequest{ID: tx.ID})
	wantNoErr(t, err)
	if len(rp.Changes) != 1 || rp.Changes[0].Op != app.OpDelete {
		t.Errorf("changes = %+v", rp.Changes)
	}
	wantNoErr(t, e.svc.ApplyRemoveTx(ctx, rp))
	_, err = e.svc.ShowTx(ctx, tx.ID)
	wantCode(t, err, domain.ErrNotFound)
}

func TestReconciledNeedsConfirmation(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	e.account(t, "A", "checking", "EUR", "0")
	tx := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-10", Status: "reconciled"})

	_, err := e.svc.PlanEditTx(ctx, app.EditTxRequest{ID: tx.ID, Notes: ptr("x")})
	wantCode(t, err, domain.ErrReconciled)
	_, err = e.svc.PlanRemoveTx(ctx, app.RemoveTxRequest{ID: tx.ID})
	wantCode(t, err, domain.ErrReconciled)

	p, err := e.svc.PlanEditTx(ctx, app.EditTxRequest{ID: tx.ID, Notes: ptr("x"), ConfirmReconciled: true})
	wantNoErr(t, err)
	_, err = e.svc.ApplyEditTx(ctx, p)
	wantNoErr(t, err)
	rp, err := e.svc.PlanRemoveTx(ctx, app.RemoveTxRequest{ID: tx.ID, ConfirmReconciled: true})
	wantNoErr(t, err)
	wantNoErr(t, e.svc.ApplyRemoveTx(ctx, rp))
}

func TestCategorizeTx(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	e.account(t, "A", "checking", "EUR", "0")
	e.category(t, "Food", "expense")
	e.category(t, "Fun", "expense")
	tx1 := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-10", Category: "Food"})
	tx2 := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-20"})

	p, err := e.svc.PlanCategorizeTx(ctx, app.CategorizeTxRequest{IDs: []domain.TransactionID{tx1.ID, tx2.ID}, Category: "Fun"})
	wantNoErr(t, err)
	if !p.Bulk {
		t.Error("plan is not bulk")
	}
	if len(p.Changes) != 2 || len(p.Txs) != 2 {
		t.Fatalf("plan = %+v", p)
	}
	wantNoErr(t, e.svc.ApplyCategorizeTx(ctx, p))
	if len(e.backup.reasons) != 1 || e.backup.reasons[0] != app.BackupReasonBulk {
		t.Errorf("backup reasons = %v, want one %q", e.backup.reasons, app.BackupReasonBulk)
	}

	for _, id := range []domain.TransactionID{tx1.ID, tx2.ID} {
		v, err := e.svc.ShowTx(ctx, id)
		wantNoErr(t, err)
		if v.Category != "Fun" {
			t.Errorf("ShowTx(%d).Category = %q, want %q", id, v.Category, "Fun")
		}
	}

	// Clearing the category on tx1 alone leaves tx2 untouched.
	p, err = e.svc.PlanCategorizeTx(ctx, app.CategorizeTxRequest{IDs: []domain.TransactionID{tx1.ID}, Category: ""})
	wantNoErr(t, err)
	wantNoErr(t, e.svc.ApplyCategorizeTx(ctx, p))
	v, err := e.svc.ShowTx(ctx, tx1.ID)
	wantNoErr(t, err)
	if v.Category != "" {
		t.Errorf("category not cleared: %q", v.Category)
	}
	v, err = e.svc.ShowTx(ctx, tx2.ID)
	wantNoErr(t, err)
	if v.Category != "Fun" {
		t.Errorf("unrelated transaction changed: %q", v.Category)
	}
}

func TestCategorizeTxRejectsEmptyOrUnknownIDs(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	e.account(t, "A", "checking", "EUR", "0")
	before := e.store.Snapshot()

	_, err := e.svc.PlanCategorizeTx(ctx, app.CategorizeTxRequest{})
	wantCode(t, err, domain.ErrMissingField)
	_, err = e.svc.PlanCategorizeTx(ctx, app.CategorizeTxRequest{IDs: []domain.TransactionID{999}})
	wantCode(t, err, domain.ErrNotFound)
	if after := e.store.Snapshot(); after != before {
		t.Error("PlanCategorizeTx wrote to the store")
	}
}

func TestCategorizeTxReconciledNeedsConfirmation(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	e.account(t, "A", "checking", "EUR", "0")
	e.category(t, "Fun", "expense")
	tx := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-10", Status: "reconciled"})

	_, err := e.svc.PlanCategorizeTx(ctx, app.CategorizeTxRequest{IDs: []domain.TransactionID{tx.ID}, Category: "Fun"})
	wantCode(t, err, domain.ErrReconciled)

	p, err := e.svc.PlanCategorizeTx(ctx, app.CategorizeTxRequest{
		IDs: []domain.TransactionID{tx.ID}, Category: "Fun", ConfirmReconciled: true,
	})
	wantNoErr(t, err)
	wantNoErr(t, e.svc.ApplyCategorizeTx(ctx, p))
}

// ApplyCategorizeTx writes every transaction of the plan in one WithTx: if any update fails, none
// of the others are persisted either.
func TestCategorizeTxApplyIsAtomic(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	e.account(t, "A", "checking", "EUR", "0")
	e.category(t, "Fun", "expense")
	tx := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-10"})

	p, err := e.svc.PlanCategorizeTx(ctx, app.CategorizeTxRequest{IDs: []domain.TransactionID{tx.ID}, Category: "Fun"})
	wantNoErr(t, err)
	// Forge a second, nonexistent transaction into the plan to force ApplyCategorizeTx to fail
	// partway through the loop.
	bogus := tx
	bogus.ID = 999
	p.Txs = append(p.Txs, bogus)

	err = e.svc.ApplyCategorizeTx(ctx, p)
	wantCode(t, err, domain.ErrNotFound)

	v, showErr := e.svc.ShowTx(ctx, tx.ID)
	wantNoErr(t, showErr)
	if v.Category != "" {
		t.Errorf("partial write survived a failed ApplyCategorizeTx: category = %q", v.Category)
	}
}
