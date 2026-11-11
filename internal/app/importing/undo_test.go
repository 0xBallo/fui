package importing_test

import (
	"context"
	"errors"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/pkg/fuiproto"
)

func TestUndoRestoresPreviousState(t *testing.T) {
	e := newEnv(nil)
	e.seedAccount(t, "Account A", "bank", "a")
	e.seedAccount(t, "Account B", "bank", "b")
	before := snapshot(e.store)
	recs := append(twoAccounts(),
		txRec("a", "out", "2026-10-01", -1100, "to B", kind(fuiproto.TransactionKindTransfer), label("t1")),
		txRec("b", "in", "2026-10-01", 1000, "from A", kind(fuiproto.TransactionKindTransfer), label("t1")),
		txRec("a", "x", "2026-10-03", -300, "coffee"),
	)
	res := e.importAll(t, stream("bank", "sha", recs...))
	if res.Transactions != 3 || snapshot(e.store) == before {
		t.Fatalf("import did not write: %+v", res)
	}
	plan := must(e.svc.PlanUndo(context.Background(), res.Batch.ID))
	if len(plan.Transactions) != 3 || !plan.Bulk || len(plan.Changes) != 2 {
		t.Fatalf("undo plan = %+v", plan)
	}
	if err := e.svc.ApplyUndo(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	if after := snapshot(e.store); after != before {
		t.Fatalf("undo did not restore the state:\nbefore %s\nafter  %s", before, after)
	}
	if got := e.backup.reasons; len(got) != 2 || got[1] != app.BackupReasonBulk {
		t.Fatalf("backups = %v", got)
	}
}

func TestUndoRefusesReconciledBatch(t *testing.T) {
	e := newEnv(nil)
	res := e.importAll(t, stream("bank", "", account("a", "Account A", "EUR"), txRec("a", "x", "2026-10-03", -300, "coffee")))
	tx := e.txs(t)[0]
	tx.Status = domain.StatusReconciled
	err := e.store.WithTx(context.Background(), func(r app.Repos) error {
		return r.Transactions().Update(context.Background(), tx)
	})
	if err != nil {
		t.Fatal(err)
	}
	before := e.store.Snapshot()
	if _, err := e.svc.PlanUndo(context.Background(), res.Batch.ID); !errors.Is(err, domain.ErrReconciled) {
		t.Fatalf("PlanUndo err = %v, want reconciled", err)
	}
	if err := e.svc.ApplyUndo(context.Background(), importing.UndoPlan{Batch: res.Batch}); err == nil {
		t.Fatal("ApplyUndo accepted a reconciled batch")
	}
	if e.store.Snapshot() != before {
		t.Fatal("refused undo changed the archive")
	}
}

func TestUndoUnknownBatch(t *testing.T) {
	e := newEnv(nil)
	if _, err := e.svc.PlanUndo(context.Background(), 42); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want not found", err)
	}
}

func TestUndoUnlinksArchivedTransferLeg(t *testing.T) {
	e := newEnv(nil)
	b := e.seedAccount(t, "Account B", "bank", "b")
	e.seedTx(t, domain.Transaction{
		AccountID: b.ID, Date: date("2026-10-02"), Amount: money(5000), Description: "from A",
		Status: domain.StatusBooked, Kind: domain.KindTransfer, ExternalID: "bank-in",
	})
	only := []fuiproto.Record{
		account("a", "Account A", "EUR"),
		txRec("a", "out", "2026-10-01", -5000, "to B", kind(fuiproto.TransactionKindTransfer)),
	}
	plan := e.plan(t, stream("bank", "", only...), createAccounts)
	plan.Transfers[0].Accept = true
	res := must(e.svc.ApplyImport(context.Background(), plan))
	if groupOf(t, e, "bank-in").TransferGroupID == 0 {
		t.Fatal("archived partner not linked")
	}
	if err := e.svc.ApplyUndo(context.Background(), must(e.svc.PlanUndo(context.Background(), res.Batch.ID))); err != nil {
		t.Fatal(err)
	}
	if got := groupOf(t, e, "bank-in"); got.TransferGroupID != 0 {
		t.Fatalf("archived leg still linked: %+v", got)
	}
}
