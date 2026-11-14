package app_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// state dumps every transaction with its tags and splits, to compare the archive before and after.
func (e env) state(t tb) string {
	t.Helper()
	ctx := context.Background()
	page, err := e.svc.ListTx(ctx, app.ListTxRequest{Limit: 10000})
	if err != nil {
		t.Fatalf("ListTx: %v", err)
	}
	var b strings.Builder
	for _, v := range page.Items {
		sp, err := e.svc.ListSplits(ctx, v.Transaction.ID)
		if err != nil {
			t.Fatalf("ListSplits: %v", err)
		}
		_ = e.store.Read(ctx, func(r app.Repos) error {
			tags, err := r.Tags().TagsOf(ctx, v.Transaction.ID)
			fmt.Fprintf(&b, "%+v %+v %+v %v\n", v.Transaction, sp, tags, err)
			return nil
		})
	}
	return b.String()
}

func (e env) undo(t tb) {
	t.Helper()
	ctx := context.Background()
	p, err := e.svc.PlanUndo(ctx)
	if err != nil {
		t.Fatalf("PlanUndo: %v", err)
	}
	if err := e.svc.ApplyUndo(ctx, p); err != nil {
		t.Fatalf("ApplyUndo: %v", err)
	}
}

func (e env) edit(t tb, id domain.TransactionID, req app.EditTxRequest) {
	t.Helper()
	ctx := context.Background()
	req.ID = id
	p, err := e.svc.PlanEditTx(ctx, req)
	if err != nil {
		t.Fatalf("PlanEditTx: %v", err)
	}
	if _, err := e.svc.ApplyEditTx(ctx, p); err != nil {
		t.Fatalf("ApplyEditTx: %v", err)
	}
}

func (e env) remove(t tb, id domain.TransactionID) {
	t.Helper()
	ctx := context.Background()
	p, err := e.svc.PlanRemoveTx(ctx, app.RemoveTxRequest{ID: id})
	if err != nil {
		t.Fatalf("PlanRemoveTx: %v", err)
	}
	if err := e.svc.ApplyRemoveTx(ctx, p); err != nil {
		t.Fatalf("ApplyRemoveTx: %v", err)
	}
}

func (e env) transfer(t tb, req app.TransferRequest) app.TransferResult {
	t.Helper()
	ctx := context.Background()
	p, err := e.svc.PlanTransfer(ctx, req)
	if err != nil {
		t.Fatalf("PlanTransfer: %v", err)
	}
	res, err := e.svc.ApplyTransfer(ctx, p)
	if err != nil {
		t.Fatalf("ApplyTransfer: %v", err)
	}
	return res
}

func TestUndoAddRemovesAndWalksBack(t *testing.T) {
	e := newEnv(t)
	e.account(t, "A", "checking", "EUR", "0")
	empty := e.state(t)
	e.tx(t, app.AddTxRequest{Account: "A", Amount: "-5", Date: "2026-10-01"})
	one := e.state(t)
	e.tx(t, app.AddTxRequest{Account: "A", Amount: "-7", Date: "2026-10-02"})

	e.undo(t)
	if got := e.state(t); got != one {
		t.Fatalf("after 1st undo:\n%s\nwant\n%s", got, one)
	}
	e.undo(t)
	if got := e.state(t); got != empty {
		t.Fatalf("after 2nd undo:\n%s\nwant empty", got)
	}
	_, err := e.svc.PlanUndo(t.Context())
	wantCode(t, err, domain.ErrNotFound)
}

func TestUndoRemoveRestoresIDTagsSplits(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	e.account(t, "A", "checking", "EUR", "0")
	e.category(t, "Groceries", "expense")
	e.category(t, "Household", "expense")
	tag := e.tag(t, "trip")
	tx := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-30", Category: "Groceries", Date: "2026-10-01", Notes: "n"})
	tp, err := e.svc.PlanTagTransaction(ctx, app.TagTransactionRequest{ID: tx.ID, Add: []string{tag.Name}})
	wantNoErr(t, err)
	wantNoErr(t, e.svc.ApplyTagTransaction(ctx, tp))
	sp, err := e.svc.PlanSplitTx(ctx, app.SplitTxRequest{ID: tx.ID, Rows: []app.SplitRowRequest{
		{Category: "Groceries", Amount: "-20", Note: "a"}, {Category: "Household", Amount: "-10"},
	}})
	wantNoErr(t, err)
	wantNoErr(t, e.svc.ApplySplitTx(ctx, sp))
	before := e.state(t)

	e.remove(t, tx.ID)
	if e.state(t) != "" {
		t.Fatal("remove left rows")
	}
	e.undo(t)
	if got := e.state(t); got != before {
		t.Fatalf("restored:\n%s\nwant\n%s", got, before)
	}
}

func TestUndoEditAndSplit(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	e.account(t, "A", "checking", "EUR", "0")
	e.category(t, "Groceries", "expense")
	e.category(t, "Household", "expense")
	tx := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-30", Category: "Groceries", Date: "2026-10-01"})
	orig := e.state(t)

	e.edit(t, tx.ID, app.EditTxRequest{Amount: ptr("-40"), Description: ptr("changed"), Category: ptr("Household")})
	edited := e.state(t)
	sp, err := e.svc.PlanSplitTx(ctx, app.SplitTxRequest{ID: tx.ID, Rows: []app.SplitRowRequest{
		{Category: "Groceries", Amount: "-25"}, {Category: "Household", Amount: "-15"},
	}})
	wantNoErr(t, err)
	wantNoErr(t, e.svc.ApplySplitTx(ctx, sp))

	e.undo(t)
	if got := e.state(t); got != edited {
		t.Fatalf("after undo split:\n%s\nwant\n%s", got, edited)
	}
	e.undo(t)
	if got := e.state(t); got != orig {
		t.Fatalf("after undo edit:\n%s\nwant\n%s", got, orig)
	}
}

func TestUndoTransfer(t *testing.T) {
	e := newEnv(t)
	e.account(t, "A", "checking", "EUR", "100")
	e.account(t, "B", "savings", "EUR", "0")
	empty := e.state(t)
	e.transfer(t, app.TransferRequest{From: "A", To: "B", Amount: "10", Fee: "1", Date: "2026-10-01"})

	e.undo(t)
	if got := e.state(t); got != empty {
		t.Fatalf("after undo transfer:\n%s", got)
	}
}

func TestUndoRemoveTransferRestoresGroup(t *testing.T) {
	e := newEnv(t)
	e.account(t, "A", "checking", "EUR", "100")
	e.account(t, "B", "savings", "EUR", "0")
	res := e.transfer(t, app.TransferRequest{From: "A", To: "B", Amount: "10", Fee: "1", Date: "2026-10-01"})
	before := e.state(t)
	e.remove(t, res.Out.ID)
	e.undo(t)
	if got := e.state(t); got != before {
		t.Fatalf("restored transfer:\n%s\nwant\n%s", got, before)
	}
	e.undo(t) // undo the remove's predecessor: the transfer itself
	if e.state(t) != "" {
		t.Fatal("transfer not removed")
	}
}

func TestUndoRefusals(t *testing.T) {
	ctx := t.Context()
	t.Run("stale", func(t *testing.T) {
		e := newEnv(t)
		e.account(t, "A", "checking", "EUR", "0")
		tx := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-5", Date: "2026-10-01"})
		tg := e.tag(t, "x")
		tp, err := e.svc.PlanTagTransaction(ctx, app.TagTransactionRequest{ID: tx.ID, Add: []string{tg.Name}})
		wantNoErr(t, err)
		wantNoErr(t, e.svc.ApplyTagTransaction(ctx, tp))
		before := e.store.Snapshot()
		_, err = e.svc.PlanUndo(ctx)
		wantCode(t, err, domain.ErrInvalidValue)
		if e.store.Snapshot() != before {
			t.Fatal("refusal wrote")
		}
	})
	t.Run("reconciled", func(t *testing.T) {
		e := newEnv(t)
		e.account(t, "A", "checking", "EUR", "0")
		tx := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-5", Date: "2026-10-01", Status: "reconciled"})
		_, err := e.svc.PlanUndo(ctx)
		wantCode(t, err, domain.ErrReconciled)
		_ = tx
	})
	t.Run("plan is dry-run and apply detects journal change", func(t *testing.T) {
		e := newEnv(t)
		e.account(t, "A", "checking", "EUR", "0")
		e.tx(t, app.AddTxRequest{Account: "A", Amount: "-5", Date: "2026-10-01"})
		before := e.store.Snapshot()
		p, err := e.svc.PlanUndo(ctx)
		wantNoErr(t, err)
		if len(p.Changes) != 1 || p.Changes[0].Op != app.OpDelete || e.store.Snapshot() != before {
			t.Fatalf("plan = %+v", p)
		}
		e.tx(t, app.AddTxRequest{Account: "A", Amount: "-6", Date: "2026-10-02"})
		wantCode(t, e.svc.ApplyUndo(ctx, p), domain.ErrInvalidValue)
	})
}

func TestUndoRandomSequences(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		ctx := context.Background()
		e := newEnv(rt)
		e.account(rt, "A", "checking", "EUR", "0")
		e.account(rt, "B", "savings", "EUR", "0")
		e.category(rt, "C1", "expense")
		e.category(rt, "C2", "expense")
		states := []string{e.state(rt)}
		var ids []domain.TransactionID
		for range rapid.IntRange(1, 12).Draw(rt, "ops") {
			half := rapid.IntRange(1, 250).Draw(rt, "half")
			amt := fmt.Sprintf("-%d", 2*half)
			switch k := rapid.IntRange(0, 4).Draw(rt, "op"); {
			case k == 0 || len(ids) == 0:
				ids = append(ids, e.tx(rt, app.AddTxRequest{Account: "A", Amount: amt, Date: "2026-10-01", Category: "C1"}).ID)
			case k == 1:
				e.edit(rt, rapid.SampledFrom(ids).Draw(rt, "id"), app.EditTxRequest{Description: &amt, Category: ptr("C2")}) // not the amount: edit leaves split rows stale, which undo cannot restore
			case k == 2:
				e.transfer(rt, app.TransferRequest{From: "A", To: "B", Amount: amt[1:], Date: "2026-10-02"})
			case k == 3:
				i := rapid.IntRange(0, len(ids)-1).Draw(rt, "rm")
				e.remove(rt, ids[i])
				ids = append(ids[:i], ids[i+1:]...)
			default:
				id := rapid.SampledFrom(ids).Draw(rt, "split")
				v, err := e.svc.ShowTx(ctx, id)
				if err != nil {
					rt.Fatalf("ShowTx: %v", err)
				}
				h := fmt.Sprintf("%d", -v.Transaction.Amount.Minor()/200) // minor/2 in major units
				sp, err := e.svc.PlanSplitTx(ctx, app.SplitTxRequest{ID: id, Rows: []app.SplitRowRequest{
					{Category: "C1", Amount: "-" + h}, {Category: "C2", Amount: "-" + h},
				}})
				if err != nil {
					rt.Fatalf("PlanSplitTx: %v", err)
				}
				if err := e.svc.ApplySplitTx(ctx, sp); err != nil {
					rt.Fatalf("ApplySplitTx: %v", err)
				}
			}
			states = append(states, e.state(rt))
		}
		for i := len(states) - 2; i >= 0; i-- {
			e.undo(rt)
			if got := e.state(rt); got != states[i] {
				rt.Fatalf("after undo to step %d:\n%s\nwant\n%s", i, got, states[i])
			}
		}
		_, err := e.svc.PlanUndo(ctx)
		if !errors.Is(err, domain.ErrNotFound) {
			rt.Fatalf("final PlanUndo err = %v, want not_found", err)
		}
	})
}
