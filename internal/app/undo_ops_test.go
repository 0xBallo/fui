package app_test

import (
	"context"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func (e env) categorize(t tb, category string, ids ...domain.TransactionID) {
	t.Helper()
	ctx := context.Background()
	p, err := e.svc.PlanCategorizeTx(ctx, app.CategorizeTxRequest{IDs: ids, Category: category})
	if err != nil {
		t.Fatalf("PlanCategorizeTx: %v", err)
	}
	if err := e.svc.ApplyCategorizeTx(ctx, p); err != nil {
		t.Fatalf("ApplyCategorizeTx: %v", err)
	}
}

func (e env) tagOp(t tb, id domain.TransactionID, name string, untag bool) {
	t.Helper()
	ctx := context.Background()
	if untag {
		p, err := e.svc.PlanUntagTransaction(ctx, app.UntagTransactionRequest{ID: id, Remove: []string{name}})
		if err == nil {
			err = e.svc.ApplyUntagTransaction(ctx, p)
		}
		if err != nil {
			t.Fatalf("untag: %v", err)
		}
		return
	}
	p, err := e.svc.PlanTagTransaction(ctx, app.TagTransactionRequest{ID: id, Add: []string{name}})
	if err == nil {
		err = e.svc.ApplyTagTransaction(ctx, p)
	}
	if err != nil {
		t.Fatalf("tag: %v", err)
	}
}

func TestUndoCategorizeTagUntag(t *testing.T) {
	ctx := t.Context()
	setup := func(t *testing.T) (env, []domain.TransactionID) {
		e := newEnv(t)
		e.account(t, "A", "checking", "EUR", "0")
		e.category(t, "C1", "expense")
		e.tag(t, "x")
		e.tag(t, "y")
		a := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-5", Date: "2026-10-01", Category: "C1"})
		b := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-6", Date: "2026-10-02"})
		return e, []domain.TransactionID{a.ID, b.ID}
	}
	t.Run("categorize is one entry for the batch", func(t *testing.T) {
		e, ids := setup(t)
		e.category(t, "C2", "expense")
		before := e.state(t)
		e.categorize(t, "C2", ids...)
		e.undo(t)
		if got := e.state(t); got != before {
			t.Fatalf("after undo:\n%s\nwant:\n%s", got, before)
		}
	})
	t.Run("tag and untag", func(t *testing.T) {
		e, ids := setup(t)
		s0 := e.state(t)
		e.tagOp(t, ids[0], "x", false)
		s1 := e.state(t)
		e.tagOp(t, ids[0], "x", true)
		e.undo(t)
		if got := e.state(t); got != s1 {
			t.Fatalf("after undoing untag:\n%s\nwant:\n%s", got, s1)
		}
		e.undo(t)
		if got := e.state(t); got != s0 {
			t.Fatalf("after undoing tag:\n%s\nwant:\n%s", got, s0)
		}
	})
	t.Run("no-op tag writes no entry", func(t *testing.T) {
		e, ids := setup(t)
		e.tagOp(t, ids[0], "x", false)
		e.tagOp(t, ids[0], "x", false)
		e.undo(t) // undoes the real tag, not a phantom entry
		v, err := e.svc.ShowTx(ctx, ids[0])
		wantNoErr(t, err)
		if len(v.Tags) != 0 {
			t.Fatalf("tags = %v", v.Tags)
		}
	})
	t.Run("tag after categorize keeps categorize undoable", func(t *testing.T) {
		e, ids := setup(t)
		e.category(t, "C2", "expense")
		e.categorize(t, "C2", ids[0])
		e.tagOp(t, ids[0], "x", false)
		e.undo(t) // the tag
		e.undo(t) // the categorize
		v, err := e.svc.ShowTx(ctx, ids[0])
		wantNoErr(t, err)
		if v.Category != "C1" {
			t.Fatalf("category = %q", v.Category)
		}
	})
	t.Run("reconciled refuses", func(t *testing.T) {
		e, ids := setup(t)
		e.categorize(t, "C1", ids[1])
		e.with(t, func(r app.Repos) error {
			cur, err := r.Transactions().Get(ctx, ids[1])
			if err != nil {
				return err
			}
			cur.Status = domain.StatusReconciled
			return r.Transactions().Update(ctx, cur)
		})
		_, err := e.svc.PlanUndo(ctx)
		wantCode(t, err, domain.ErrReconciled)
	})
}
