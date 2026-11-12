package app_test

import (
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func TestPayees(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	amazon := e.payee(t, "Amazon", "AMZN Mktp")
	e.payee(t, "Netflix")

	for name, req := range map[string]app.AddPayeeRequest{
		"duplicate name":  {Name: "AMAZON"},
		"duplicate alias": {Name: "X", Aliases: []string{"amzn mktp"}},
		"empty name":      {Name: " "},
	} {
		if _, err := e.svc.PlanAddPayee(ctx, req); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}

	p, err := e.svc.PlanRenamePayee(ctx, app.RenamePayeeRequest{Payee: "amaz", NewName: "Amazon EU"})
	wantNoErr(t, err)
	py, err := e.svc.ApplyRenamePayee(ctx, p)
	wantNoErr(t, err)
	if py.ID != amazon.ID || py.Name != "Amazon EU" || len(py.Aliases) != 1 {
		t.Errorf("renamed = %+v", py)
	}

	payees, err := e.svc.ListPayees(ctx)
	wantNoErr(t, err)
	if len(payees) != 2 {
		t.Errorf("ListPayees = %+v", payees)
	}

	_, err = e.svc.PlanRemovePayee(ctx, app.RemovePayeeRequest{Payee: "zzz"})
	wantCode(t, err, domain.ErrNotFound)
}

func payeeEnv(t *testing.T) (env, domain.Transaction, domain.Payee, domain.Payee) {
	t.Helper()
	e := newEnv(t)
	e.account(t, "A", "checking", "EUR", "0")
	e.category(t, "Bar", "expense")
	amazon := e.payee(t, "Amazon")
	netflix := e.payee(t, "Netflix")
	tx := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-1", Category: "Bar", Date: "2026-10-01"})
	return e, tx, amazon, netflix
}

func TestRemovePayee(t *testing.T) {
	e, tx, amazon, _ := payeeEnv(t)
	ctx := t.Context()

	p, err := e.svc.PlanRemovePayee(ctx, app.RemovePayeeRequest{Payee: "netflix"})
	wantNoErr(t, err)
	wantNoErr(t, e.svc.ApplyRemovePayee(ctx, p))
	payees, err := e.svc.ListPayees(ctx)
	wantNoErr(t, err)
	if len(payees) != 1 {
		t.Errorf("payees left = %+v, want only amazon", payees)
	}

	// used payee cannot be removed
	tx.PayeeID = amazon.ID
	wantNoErr(t, e.store.WithTx(ctx, func(r app.Repos) error { return r.Transactions().Update(ctx, tx) }))
	p2, err := e.svc.PlanRemovePayee(ctx, app.RemovePayeeRequest{Payee: "amazon"})
	wantNoErr(t, err)
	err = e.svc.ApplyRemovePayee(ctx, p2)
	wantCode(t, err, app.ErrInUse)
}

func TestMergePayee(t *testing.T) {
	e, tx, amazon, netflix := payeeEnv(t)
	ctx := t.Context()
	tx.PayeeID = amazon.ID
	wantNoErr(t, e.store.WithTx(ctx, func(r app.Repos) error { return r.Transactions().Update(ctx, tx) }))

	p, err := e.svc.PlanMergePayee(ctx, app.MergePayeeRequest{Source: "amazon", Target: "netflix"})
	wantNoErr(t, err)
	if !p.Bulk || p.Moved != 1 || len(p.Changes) != 2 {
		t.Fatalf("plan = %+v, want a bulk plan moving 1 transaction then deleting", p)
	}
	wantNoErr(t, e.svc.ApplyMergePayee(ctx, p))

	payees, err := e.svc.ListPayees(ctx)
	wantNoErr(t, err)
	if len(payees) != 1 {
		t.Fatalf("payees left = %+v, want only netflix", payees)
	}
	if payees[0].ID != netflix.ID || len(payees[0].Aliases) != 1 || payees[0].Aliases[0] != "Amazon" {
		t.Errorf("merged payee = %+v, want Amazon kept as an alias", payees[0])
	}
	var got domain.Transaction
	err = e.store.Read(ctx, func(r app.Repos) error {
		var gerr error
		got, gerr = r.Transactions().Get(ctx, tx.ID)
		return gerr
	})
	wantNoErr(t, err)
	if got.PayeeID != netflix.ID {
		t.Errorf("transaction payee = %d, want %d", got.PayeeID, netflix.ID)
	}

	_, err = e.svc.PlanMergePayee(ctx, app.MergePayeeRequest{Source: "netflix", Target: "netflix"})
	wantCode(t, err, domain.ErrInvalidValue)
}
