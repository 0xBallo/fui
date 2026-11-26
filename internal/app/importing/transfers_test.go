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

func TestAcceptTransfers(t *testing.T) {
	recs := append(twoAccounts(),
		txRec("a", "out1", "2026-10-01", -5000, "to B", kind(fuiproto.TransactionKindTransfer)),
		txRec("b", "in1", "2026-10-01", 5000, "from A", kind(fuiproto.TransactionKindTransfer)),
		txRec("a", "out2", "2026-10-10", -700, "to B", kind(fuiproto.TransactionKindTransfer)),
		txRec("b", "in2", "2026-10-10", 700, "from A", kind(fuiproto.TransactionKindTransfer)),
	)
	linked := func(e env, ext string) bool { return groupOf(t, e, ext).TransferGroupID != 0 }
	cases := []struct {
		name       string
		apply      func(p *importing.ImportPlan) importing.ApplyOptions
		wantLinked [2]bool
	}{
		{"none", func(*importing.ImportPlan) importing.ApplyOptions { return importing.ApplyOptions{} }, [2]bool{false, false}},
		{"all", func(*importing.ImportPlan) importing.ApplyOptions {
			return importing.ApplyOptions{AcceptTransfers: true}
		}, [2]bool{true, true}},
		{"some", func(p *importing.ImportPlan) importing.ApplyOptions {
			must(0, p.SetAcceptTransfer(1, true))
			return importing.ApplyOptions{}
		}, [2]bool{false, true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(nil)
			plan := e.plan(t, stream("bank", "", recs...), createAccounts)
			if len(plan.Transfers) != 2 {
				t.Fatalf("proposals = %+v", plan.Transfers)
			}
			// Proposals follow the date order of the rows: 0 is the first pair.
			opts := tc.apply(&plan)
			must(e.svc.ApplyImportWith(context.Background(), plan, opts))
			got := [2]bool{linked(e, "out1"), linked(e, "out2")}
			if got != tc.wantLinked {
				t.Fatalf("linked = %v, want %v", got, tc.wantLinked)
			}
		})
	}
	t.Run("out of range", func(t *testing.T) {
		var p importing.ImportPlan
		if err := p.SetAcceptTransfer(0, true); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("row with an archived movement", func(t *testing.T) {
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
		res := must(e.svc.ApplyImportWith(context.Background(), plan, importing.ApplyOptions{AcceptTransfers: true}))
		if res.Transfers != 1 || groupOf(t, e, "bank-in").TransferGroupID == 0 {
			t.Fatalf("result = %+v", res)
		}
		if plan.Transfers[0].Accept {
			t.Fatal("AcceptTransfers mutated the caller's plan")
		}
	})
}

func TestReimportIsIdempotent(t *testing.T) {
	recs := append(twoAccounts(),
		txRec("a", "out", "2026-10-01", -5000, "to B", kind(fuiproto.TransactionKindTransfer)),
		txRec("b", "in", "2026-10-01", 5000, "from A", kind(fuiproto.TransactionKindTransfer)),
		txRec("a", "coffee", "2026-10-02", -300, "coffee"),
	)
	e := newEnv(nil)
	opts := importing.ApplyOptions{AcceptTransfers: true}
	first := must(e.svc.ApplyImportWith(context.Background(), e.plan(t, stream("bank", "sha", recs...), createAccounts), opts))
	if first.Transactions != 3 || first.Transfers != 1 {
		t.Fatalf("first = %+v", first)
	}
	before := snapshot(e.store)
	second := must(e.svc.ApplyImportWith(context.Background(), e.plan(t, stream("bank", "sha", recs...), createAccounts), opts))
	if second.Transactions != 0 || second.Transfers != 0 || second.SecurityOps != 0 {
		t.Fatalf("second = %+v, want nothing", second)
	}
	if after := snapshot(e.store); after != before {
		t.Fatalf("second import changed the archive\nbefore %s\nafter  %s", before, after)
	}
}
