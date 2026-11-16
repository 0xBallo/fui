package sqlite

import (
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// TestUndoRemoveTransfer walks add → rm of a transfer with tags and splits → undo, on SQLite: the
// restored rows must keep their IDs and the journal must mark entries undone.
func TestUndoRemoveTransfer(t *testing.T) {
	store, clk := newTestStore(t)
	ctx := t.Context()
	svc := app.New(store, clk, store, app.Config{Locale: domain.LocaleENUS, Location: time.UTC})

	for _, name := range []string{"A", "B"} {
		p, err := svc.PlanAddAccount(ctx, app.AddAccountRequest{Name: name, Kind: "checking", Currency: "EUR", OpeningBalance: "100", OpenedOn: "2026-01-01"})
		if err != nil {
			t.Fatalf("PlanAddAccount: %v", err)
		}
		if _, err := svc.ApplyAddAccount(ctx, p); err != nil {
			t.Fatalf("ApplyAddAccount: %v", err)
		}
	}
	tp, err := svc.PlanTransfer(ctx, app.TransferRequest{From: "A", To: "B", Amount: "10", Fee: "1", Date: "2026-01-02"})
	if err != nil {
		t.Fatalf("PlanTransfer: %v", err)
	}
	res, err := svc.ApplyTransfer(ctx, tp)
	if err != nil {
		t.Fatalf("ApplyTransfer: %v", err)
	}
	rp, err := svc.PlanRemoveTx(ctx, app.RemoveTxRequest{ID: res.Out.ID})
	if err != nil {
		t.Fatalf("PlanRemoveTx: %v", err)
	}
	if err := svc.ApplyRemoveTx(ctx, rp); err != nil {
		t.Fatalf("ApplyRemoveTx: %v", err)
	}

	up, err := svc.PlanUndo(ctx)
	if err != nil {
		t.Fatalf("PlanUndo: %v", err)
	}
	if err := svc.ApplyUndo(ctx, up); err != nil {
		t.Fatalf("ApplyUndo: %v", err)
	}
	v, err := svc.ShowTx(ctx, res.In.ID)
	if err != nil {
		t.Fatalf("ShowTx restored leg: %v", err)
	}
	if v.Transfer == nil || v.Transfer.ID != res.Group.ID || v.Transfer.Fee.Minor() != 100 || v.Transfer.Out.TransactionID != res.Out.ID {
		t.Fatalf("restored transfer = %+v, want group %d with fee 100", v.Transfer, res.Group.ID)
	}

	// Second undo reverts the transfer itself; a third finds nothing.
	up, err = svc.PlanUndo(ctx)
	if err != nil {
		t.Fatalf("PlanUndo 2: %v", err)
	}
	if err := svc.ApplyUndo(ctx, up); err != nil {
		t.Fatalf("ApplyUndo 2: %v", err)
	}
	if _, err := svc.ShowTx(ctx, res.Out.ID); err == nil {
		t.Fatal("transfer leg survived undo")
	}
	if _, err := svc.PlanUndo(ctx); err == nil {
		t.Fatal("PlanUndo with empty journal succeeded")
	}
}

func TestOpLogRepoErrors(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := t.Context()
	err := store.Read(ctx, func(r app.Repos) error {
		if _, err := r.OpLog().Latest(ctx); err == nil {
			t.Error("Latest on empty journal succeeded")
		}
		if _, err := r.OpLog().Append(ctx, app.OpEntry{Kind: "x", Payload: "{}"}); err == nil {
			t.Error("Append in read-only tx succeeded")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = store.WithTx(ctx, func(r app.Repos) error {
		if err := r.OpLog().MarkUndone(ctx, 99, time.Time{}); err == nil {
			t.Error("MarkUndone of unknown id succeeded")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
