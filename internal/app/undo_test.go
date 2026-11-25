package app_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

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
		e.with(t, func(r app.Repos) error { return r.Tags().SetTags(ctx, tx.ID, nil) })
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
				id := rapid.SampledFrom(ids).Draw(rt, "id")
				p, err := e.svc.PlanEditTx(ctx, app.EditTxRequest{ID: id, Amount: &amt, Category: ptr("C2")})
				if errors.Is(err, domain.ErrInvalidValue) { // split transaction: amount is locked
					continue
				}
				if err != nil {
					rt.Fatalf("PlanEditTx: %v", err)
				}
				if _, err := e.svc.ApplyEditTx(ctx, p); err != nil {
					rt.Fatalf("ApplyEditTx: %v", err)
				}
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

func TestEditAmountOfSplitTxRejected(t *testing.T) {
	e, tx := splitEnv(t)
	ctx := t.Context()
	sp, err := e.svc.PlanSplitTx(ctx, app.SplitTxRequest{ID: tx.ID, Rows: []app.SplitRowRequest{
		{Category: "Groceries", Amount: "-20"}, {Category: "Household", Amount: "-10"},
	}})
	wantNoErr(t, err)
	wantNoErr(t, e.svc.ApplySplitTx(ctx, sp))

	_, err = e.svc.PlanEditTx(ctx, app.EditTxRequest{ID: tx.ID, Amount: ptr("-40")})
	wantCode(t, err, domain.ErrInvalidValue)
	// Same amount and other fields stay editable, and undoing that edit works on a split tx.
	e.edit(t, tx.ID, app.EditTxRequest{Amount: ptr("-30"), Notes: ptr("x")})
	before := e.state(t)
	e.edit(t, tx.ID, app.EditTxRequest{Description: ptr("d")})
	e.undo(t)
	if got := e.state(t); got != before {
		t.Fatalf("undo edit of split tx:\n%s\nwant\n%s", got, before)
	}
}

func (e env) with(t tb, fn func(r app.Repos) error) {
	t.Helper()
	if err := e.store.WithTx(context.Background(), fn); err != nil {
		t.Fatalf("WithTx: %v", err)
	}
}

// tagJournaled sets the tags of id and journals it as a masked tag entry.
func (e env) tagJournaled(t tb, id domain.TransactionID, tg domain.Tag) {
	t.Helper()
	e.with(t, func(r app.Repos) error {
		j, err := app.BeginJournal(context.Background(), r, id)
		if err != nil {
			return err
		}
		if err := r.Tags().SetTags(context.Background(), id, []domain.TagID{tg.ID}); err != nil {
			return err
		}
		return j.Commit(context.Background(), r, fixedClock{time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)},
			app.OpKindTag, app.UndoFieldTags)
	})
}

func TestUndoMask(t *testing.T) {
	ctx := t.Context()
	setup := func(t *testing.T) (env, domain.Transaction, domain.Tag) {
		e := newEnv(t)
		e.account(t, "A", "checking", "EUR", "0")
		tx := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-5", Date: "2026-10-01", Description: "old"})
		tg := e.tag(t, "x")
		e.tagJournaled(t, tx.ID, tg)
		return e, tx, tg
	}
	t.Run("description edit is not stale and survives", func(t *testing.T) {
		e, tx, _ := setup(t)
		e.with(t, func(r app.Repos) error {
			cur, err := r.Transactions().Get(ctx, tx.ID)
			if err != nil {
				return err
			}
			cur.Description = "new"
			return r.Transactions().Update(ctx, cur)
		})
		e.undo(t)
		v, err := e.svc.ShowTx(ctx, tx.ID)
		wantNoErr(t, err)
		if v.Transaction.Description != "new" || len(v.Tags) != 0 {
			t.Fatalf("after undo: %q tags=%v", v.Transaction.Description, v.Tags)
		}
	})
	t.Run("changed tags are stale", func(t *testing.T) {
		e, tx, _ := setup(t)
		e.with(t, func(r app.Repos) error { return r.Tags().SetTags(ctx, tx.ID, nil) })
		_, err := e.svc.PlanUndo(ctx)
		wantCode(t, err, domain.ErrInvalidValue)
	})
	t.Run("reconciled refuses", func(t *testing.T) {
		e, tx, _ := setup(t)
		e.with(t, func(r app.Repos) error {
			cur, err := r.Transactions().Get(ctx, tx.ID)
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

func TestUndoTrackedRowsAndGroups(t *testing.T) {
	ctx := t.Context()
	e := newEnv(t)
	e.account(t, "A", "checking", "EUR", "0")
	e.with(t, func(r app.Repos) error {
		j, err := app.BeginJournal(ctx, r, 999) // missing ids are ignored
		if err != nil {
			return err
		}
		cur, err := domain.ParseCurrency("EUR")
		if err != nil {
			return err
		}
		m, err := domain.NewMoney(-100, cur)
		if err != nil {
			return err
		}
		d, err := domain.ParseDate("2026-10-01")
		if err != nil {
			return err
		}
		tx, err := domain.NewTransaction(domain.Transaction{AccountID: 1, Date: d, Amount: m, Status: domain.StatusBooked, Kind: domain.KindPayment})
		if err != nil {
			return err
		}
		tx, err = r.Transactions().Insert(ctx, tx)
		if err != nil {
			return err
		}
		j.Track(tx.ID, tx.ID)
		return j.Commit(ctx, r, fixedClock{time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)}, app.OpKindRecurConfirm)
	})
	e.undo(t)
	if got := e.state(t); got != "" {
		t.Fatalf("state after undo = %q", got)
	}
}

func TestUndoDanglingRefs(t *testing.T) {
	ctx := t.Context()
	e := newEnv(t)
	e.account(t, "A", "checking", "EUR", "0")
	py := e.payee(t, "Shop")
	var batch domain.ImportBatchID
	var rec app.RecurrenceID
	var id domain.TransactionID
	e.with(t, func(r app.Repos) error {
		b, err := r.Imports().InsertBatch(ctx, app.ImportBatch{Connector: "x", Source: "s", SourceSHA256: "ab"})
		if err != nil {
			return err
		}
		batch = b.ID
		cur, _ := domain.ParseCurrency("EUR")
		m, _ := domain.NewMoney(-100, cur)
		d, _ := domain.ParseDate("2026-10-01")
		rule, err := domain.NewRecurrenceRule(1, domain.RecurrenceMonths, 1)
		if err != nil {
			return err
		}
		rc, err := r.Recurrences().Insert(ctx, app.Recurrence{AccountID: 1, Amount: m, Rule: rule, Start: d, NextDate: d})
		if err != nil {
			return err
		}
		rec = rc.ID
		tx, err := domain.NewTransaction(domain.Transaction{
			AccountID: 1, Date: d, Amount: m, Status: domain.StatusBooked, Kind: domain.KindPayment,
			PayeeID: py.ID, ImportBatchID: batch, RecurrenceID: domain.RecurrenceID(rec),
		})
		if err != nil {
			return err
		}
		tx, err = r.Transactions().Insert(ctx, tx)
		id = tx.ID
		return err
	})
	e.remove(t, id)
	e.with(t, func(r app.Repos) error {
		if err := r.Payees().Delete(ctx, py.ID); err != nil {
			return err
		}
		if err := r.Imports().DeleteBatch(ctx, batch); err != nil {
			return err
		}
		return r.Recurrences().Delete(ctx, rec)
	})
	p, err := e.svc.PlanUndo(ctx)
	wantNoErr(t, err)
	if len(p.Warnings) != 3 {
		t.Fatalf("warnings = %v", p.Warnings)
	}
	wantNoErr(t, e.svc.ApplyUndo(ctx, p))
	v, err := e.svc.ShowTx(ctx, id)
	wantNoErr(t, err)
	if tr := v.Transaction; tr.PayeeID != 0 || tr.ImportBatchID != 0 || tr.RecurrenceID != 0 {
		t.Fatalf("refs not cleared: %+v", tr)
	}
}

func TestUndoImplicitPayee(t *testing.T) {
	ctx := t.Context()
	payees := func(e env) int {
		var n int
		e.with(t, func(r app.Repos) error {
			l, err := r.Payees().List(ctx)
			n = len(l)
			return err
		})
		return n
	}
	t.Run("removed on undo", func(t *testing.T) {
		e := newEnv(t)
		e.account(t, "A", "checking", "EUR", "0")
		e.tx(t, app.AddTxRequest{Account: "A", Amount: "-5", Date: "2026-10-01", Payee: "Brand New"})
		if payees(e) != 1 {
			t.Fatal("payee not created")
		}
		e.undo(t)
		if payees(e) != 0 {
			t.Fatal("payee kept")
		}
	})
	t.Run("removed with its last user", func(t *testing.T) {
		e := newEnv(t)
		e.account(t, "A", "checking", "EUR", "0")
		e.tx(t, app.AddTxRequest{Account: "A", Amount: "-5", Date: "2026-10-01", Payee: "Brand New"})
		e.tx(t, app.AddTxRequest{Account: "A", Amount: "-6", Date: "2026-10-02", Payee: "Brand New"})
		e.undo(t)
		e.undo(t)
		if payees(e) != 0 {
			t.Fatal("payee should go with its last user")
		}
	})
	t.Run("kept when another row uses it", func(t *testing.T) {
		e := newEnv(t)
		e.account(t, "A", "checking", "EUR", "0")
		tx := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-5", Date: "2026-10-01", Payee: "Brand New"})
		e.with(t, func(r app.Repos) error {
			l, _ := r.Payees().List(ctx)
			other := tx
			other.ID = 0
			other.PayeeID = l[0].ID
			_, err := r.Transactions().Insert(ctx, other)
			return err
		})
		e.undo(t)
		if payees(e) != 1 {
			t.Fatal("payee in use was deleted")
		}
	})
}
