package importing_test

import (
	"context"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/pkg/fuiproto"
)

func twoAccounts(extra ...func(*fuiproto.AccountRecord)) []fuiproto.Record {
	return []fuiproto.Record{
		account("a", "Account A", "EUR", extra...),
		account("b", "Account B", "EUR"),
	}
}

func groupOf(t *testing.T, e env, ext string) domain.Transaction {
	t.Helper()
	for _, tx := range e.txs(t) {
		if tx.ExternalID == ext {
			return tx
		}
	}
	t.Fatalf("no transaction with external id %q", ext)
	return domain.Transaction{}
}

func TestLabelledLegsBecomeATransfer(t *testing.T) {
	e := newEnv(nil)
	recs := append(twoAccounts(),
		txRec("a", "out", "2026-10-01", -1100, "to B", kind(fuiproto.TransactionKindTransfer), label("t1")),
		txRec("b", "in", "2026-10-01", 1000, "from A", kind(fuiproto.TransactionKindTransfer), label("t1")),
		txRec("a", "lone", "2026-10-02", -500, "half", kind(fuiproto.TransactionKindTransfer), label("t2")),
	)
	res := e.importAll(t, stream("bank", "", recs...))
	if res.Transfers != 1 {
		t.Fatalf("transfers = %d, want 1", res.Transfers)
	}
	out, in, lone := groupOf(t, e, "out"), groupOf(t, e, "in"), groupOf(t, e, "lone")
	if out.TransferGroupID == 0 || out.TransferGroupID != in.TransferGroupID || lone.TransferGroupID != 0 {
		t.Fatalf("groups out=%d in=%d lone=%d", out.TransferGroupID, in.TransferGroupID, lone.TransferGroupID)
	}
	err := e.store.Read(context.Background(), func(r app.Repos) error {
		g, err := r.Transfers().Get(context.Background(), out.TransferGroupID)
		if err != nil {
			return err
		}
		if g.Fee.Minor() != 100 {
			t.Errorf("inferred fee = %d, want 100", g.Fee.Minor())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLabelledLegsThatCannotLinkAreReported(t *testing.T) {
	e := newEnv(nil)
	recs := append(twoAccounts(),
		txRec("a", "x", "2026-10-01", -100, "out", label("t1")),
		txRec("b", "y", "2026-10-01", 900, "in", label("t1")), // legs sum to +800: not a transfer
	)
	plan := e.plan(t, stream("bank", "", recs...), createAccounts)
	if len(plan.Errors) != 1 {
		t.Fatalf("errors = %+v", plan.Errors)
	}
	if res := must(e.svc.ApplyImport(context.Background(), plan)); res.Transfers != 0 || res.Transactions != 2 {
		t.Fatalf("result = %+v", res)
	}
}

func TestTransferProposals(t *testing.T) {
	recs := append(twoAccounts(),
		txRec("a", "out", "2026-10-01", -5000, "to B", kind(fuiproto.TransactionKindTransfer)),
		txRec("b", "in", "2026-10-02", 5000, "from A", kind(fuiproto.TransactionKindTransfer)),
		txRec("a", "coffee", "2026-10-02", -300, "coffee"),
	)
	t.Run("between two rows", func(t *testing.T) {
		e := newEnv(nil)
		plan := e.plan(t, stream("bank", "", recs...), createAccounts)
		if len(plan.Transfers) != 1 || plan.Transfers[0].Reason != domain.TransferMatchByAmountAndDate || plan.Transfers[0].Other < 0 {
			t.Fatalf("proposals = %+v", plan.Transfers)
		}
		// A proposal is not applied unless accepted.
		must(e.svc.ApplyImport(context.Background(), plan))
		if groupOf(t, e, "out").TransferGroupID != 0 {
			t.Fatal("unaccepted proposal was applied")
		}
	})
	t.Run("accepted", func(t *testing.T) {
		e := newEnv(nil)
		plan := e.plan(t, stream("bank", "", recs...), createAccounts)
		plan.Transfers[0].Accept = true
		res := must(e.svc.ApplyImport(context.Background(), plan))
		out, in := groupOf(t, e, "out"), groupOf(t, e, "in")
		if res.Transfers != 1 || out.TransferGroupID == 0 || out.TransferGroupID != in.TransferGroupID {
			t.Fatalf("result = %+v out=%+v in=%+v", res, out, in)
		}
	})
	t.Run("by counterparty IBAN", func(t *testing.T) {
		e := newEnv(nil)
		const ibanB = "IT60X0542811101000000123456"
		withB := []fuiproto.Record{
			account("a", "Account A", "EUR"),
			account("b", "Account B", "EUR", withIBAN(ibanB)),
			txRec("a", "out", "2026-10-01", -700, "to B", kind(fuiproto.TransactionKindTransfer), counterparty(ibanB)),
			txRec("b", "in", "2026-10-01", 700, "from A", kind(fuiproto.TransactionKindTransfer)),
		}
		plan := e.plan(t, stream("bank", "", withB...), createAccounts)
		if len(plan.Transfers) != 1 || plan.Transfers[0].Reason != domain.TransferMatchByCounterpartyIBAN {
			t.Fatalf("proposals = %+v", plan.Transfers)
		}
	})
	t.Run("with a movement already in the archive", func(t *testing.T) {
		e := newEnv(nil)
		b := e.seedAccount(t, "Account B", "bank", "b")
		partner := e.seedTx(t, domain.Transaction{
			AccountID: b.ID, Date: date("2026-10-02"), Amount: money(5000), Description: "from A",
			Status: domain.StatusBooked, Kind: domain.KindTransfer, ExternalID: "bank-in",
		})
		only := []fuiproto.Record{
			account("a", "Account A", "EUR"),
			txRec("a", "out", "2026-10-01", -5000, "to B", kind(fuiproto.TransactionKindTransfer)),
		}
		plan := e.plan(t, stream("bank", "", only...), createAccounts)
		if len(plan.Transfers) != 1 || plan.Transfers[0].Other != -1 || plan.Transfers[0].Existing != partner.ID {
			t.Fatalf("proposals = %+v", plan.Transfers)
		}
		plan.Transfers[0].Accept = true
		must(e.svc.ApplyImport(context.Background(), plan))
		linked := groupOf(t, e, "bank-in")
		if linked.TransferGroupID == 0 || linked.TransferGroupID != groupOf(t, e, "out").TransferGroupID {
			t.Fatalf("archived partner not linked: %+v", linked)
		}
	})
	t.Run("by an existing account's persisted IBAN", func(t *testing.T) {
		e := newEnv(nil)
		const ibanB = "IT60X0542811101000000123456"
		b := e.seedAccount(t, "Account B", "bank", "b")
		b.IBAN = ibanB
		if err := e.store.WithTx(context.Background(), func(r app.Repos) error {
			return r.Accounts().Update(context.Background(), b)
		}); err != nil {
			t.Fatalf("set iban on existing account: %v", err)
		}
		recs := []fuiproto.Record{
			account("a", "Account A", "EUR"),
			account("b", "Account B", "EUR"), // no IBAN on the record: it comes from the archive
			txRec("a", "out", "2026-10-01", -700, "to B", kind(fuiproto.TransactionKindTransfer), counterparty(ibanB)),
			txRec("b", "in", "2026-10-01", 700, "from A", kind(fuiproto.TransactionKindTransfer)),
		}
		plan := e.plan(t, stream("bank", "", recs...), createAccounts)
		if len(plan.Transfers) != 1 || plan.Transfers[0].Reason != domain.TransferMatchByCounterpartyIBAN {
			t.Fatalf("proposals = %+v", plan.Transfers)
		}
	})
}
