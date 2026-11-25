package importing_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/pkg/fuiproto"
)

type fakeModuleWriter struct {
	calls [][]module.ID
	err   error
}

func (f *fakeModuleWriter) SetModulesEnabled(_ context.Context, ids []module.ID) error {
	f.calls = append(f.calls, ids)
	return f.err
}

func secOpRec(cashTx string) fuiproto.Record {
	return fuiproto.Record{Type: fuiproto.RecordTypeSecurityOp, SecurityOp: &fuiproto.SecurityOpRecord{
		Account: "r1", Date: "2026-10-01", Kind: fuiproto.SecurityOpKindBuy, Quantity: 1000000, UnitPrice: 100, CashTx: cashTx,
		Security: fuiproto.SecurityMeta{Name: "World ETF", Ticker: "VWCE", Kind: fuiproto.SecurityKindETF, Currency: "EUR", QuantityScale: 6},
	}}
}

func loanRecs() []fuiproto.Record {
	return []fuiproto.Record{
		account("l1", "Mortgage", "EUR", func(a *fuiproto.AccountRecord) { a.Kind = fuiproto.AccountKindLoan }),
		{Type: fuiproto.RecordTypeSchedule, Schedule: &fuiproto.ScheduleRecord{
			Account: "l1", ValidFrom: "2026-01-01", Rate: 300, RateType: fuiproto.RateTypeFixed,
			DebitAccount: "r1", Kind: fuiproto.LoanKindMortgage, Principal: 100, DisbursedOn: "2026-01-01", TermMonths: 12,
		}},
		{Type: fuiproto.RecordTypeInstallment, Installment: &fuiproto.InstallmentRecord{Account: "l1", DueDate: "2026-11-01", Principal: 100}},
	}
}

func brokerStream() importing.Source {
	return stream("fui", "", broker("r1", "Broker", "EUR"),
		txRec("r1", "cash-1", "2026-10-01", -100, "buy cash leg"), secOpRec("cash-1"))
}

func TestModuleRecordsListed(t *testing.T) {
	e := newEnv(nil)
	recs := append([]fuiproto.Record{broker("r1", "Broker", "EUR"), secOpRec("")}, loanRecs()...)
	plan := e.plan(t, stream("fui", "", recs...), createAccounts)
	want := []importing.ModuleRecords{
		{Module: module.Investments, Count: 1, Kinds: []string{"security_op"}, Lines: []int{3}},
		{Module: module.Loans, Count: 2, Kinds: []string{"schedule", "installment"}, Lines: []int{5, 6}},
	}
	if diff := cmp.Diff(want, plan.ModuleRecords); diff != "" {
		t.Fatalf("module records (-want +got):\n%s", diff)
	}
	loans := must(module.Validate([]module.ID{module.Core, module.Recurring, module.Loans}))
	if p := e.plan(t, stream("fui", "", recs...), createAccounts, withModules(loans)); len(p.ModuleRecords) != 1 || p.ModuleRecords[0].Module != module.Investments {
		t.Fatalf("with loans enabled: %+v", p.ModuleRecords)
	}
}

func TestApplyModules(t *testing.T) {
	t.Run("skipped without enabling", func(t *testing.T) {
		e := newEnv(nil)
		recs := append([]fuiproto.Record{broker("r1", "Broker", "EUR"), txRec("r1", "c", "2026-10-01", -100, "x"), secOpRec("c")}, loanRecs()...)
		res := must(e.svc.ApplyImport(context.Background(), e.plan(t, stream("fui", "", recs...), createAccounts)))
		if res.SecurityOps != 0 || res.Securities != 0 || res.Transactions != 1 {
			t.Fatalf("result = %+v", res)
		}
		if len(res.Warnings) != 3 {
			t.Fatalf("warnings = %+v, want security_op + schedule + installment", res.Warnings)
		}
	})
	t.Run("enabled then imported", func(t *testing.T) {
		e := newEnv(nil)
		w := &fakeModuleWriter{}
		plan := e.plan(t, brokerStream(), createAccounts)
		res := must(e.svc.ApplyImportWith(context.Background(), plan, importing.ApplyOptions{
			EnableModules: []module.ID{module.Investments}, Modules: module.Default(), ModuleWriter: w,
		}))
		if res.SecurityOps != 1 || res.Securities != 1 || len(res.Warnings) != 0 {
			t.Fatalf("result = %+v", res)
		}
		if len(w.calls) != 1 || !module.Default().Has(module.Core) {
			t.Fatalf("writer calls = %v", w.calls)
		}
		if !contains(w.calls[0], module.Investments) {
			t.Fatalf("enabled = %v, want investments", w.calls[0])
		}
	})
	t.Run("restored when the import fails", func(t *testing.T) {
		e := newEnv(nil)
		w := &fakeModuleWriter{}
		plan := e.plan(t, brokerStream(), createAccounts)
		plan.SecurityOps[0].CashTx = "missing"
		_, err := e.svc.ApplyImportWith(context.Background(), plan, importing.ApplyOptions{
			EnableModules: []module.ID{module.Investments}, Modules: module.Default(), ModuleWriter: w,
		})
		if err == nil || len(w.calls) != 2 || contains(w.calls[1], module.Investments) {
			t.Fatalf("err=%v calls=%v", err, w.calls)
		}
		if len(e.txs(t)) != 0 {
			t.Fatal("failed import left transactions")
		}
	})
	t.Run("invalid module and missing writer", func(t *testing.T) {
		e := newEnv(nil)
		plan := e.plan(t, brokerStream(), createAccounts)
		for _, opts := range []importing.ApplyOptions{
			{EnableModules: []module.ID{"nope"}, ModuleWriter: &fakeModuleWriter{}},
			{EnableModules: []module.ID{module.Investments}},
		} {
			if _, err := e.svc.ApplyImportWith(context.Background(), plan, opts); err == nil {
				t.Fatalf("opts %+v: want error", opts)
			}
		}
		w := &fakeModuleWriter{err: errors.New("disk full")}
		_, err := e.svc.ApplyImportWith(context.Background(), plan, importing.ApplyOptions{EnableModules: []module.ID{module.Investments}, ModuleWriter: w})
		if err == nil || len(e.txs(t)) != 0 {
			t.Fatalf("writer failure: err=%v", err)
		}
	})
}

func contains(ids []module.ID, id module.ID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func cryptoOpRec() fuiproto.Record {
	rec := secOpRec("c")
	rec.SecurityOp.Security = fuiproto.SecurityMeta{
		Name: "Bitcoin", Ticker: "BTC", Kind: fuiproto.SecurityKindCrypto, Currency: "EUR", QuantityScale: 8,
	}
	return rec
}

func TestSecurityOpNeedsInvestmentAccount(t *testing.T) {
	inv := must(module.Validate([]module.ID{module.Core, module.Investments}))
	e := newEnv(nil)
	src := stream("fui", "", account("r1", "Checking", "EUR"), txRec("r1", "c", "2026-10-01", -100, "buy"), secOpRec("c"))
	plan := e.plan(t, src, createAccounts, withModules(inv))
	if len(plan.SecurityOps) != 0 || len(plan.Errors) != 1 || !strings.Contains(plan.Errors[0].Message, "cannot be held in a checking account") {
		t.Fatalf("ops=%d errors=%+v, want no op and one account-kind error", len(plan.SecurityOps), plan.Errors)
	}
}

func TestCryptoOpNeedsCryptoModule(t *testing.T) {
	src := stream("fui", "", broker("r1", "Broker", "EUR"), txRec("r1", "c", "2026-10-01", -100, "buy"), cryptoOpRec())
	inv := must(module.Validate([]module.ID{module.Core, module.Investments}))
	both := must(module.Validate([]module.ID{module.Core, module.Investments, module.Crypto}))

	t.Run("crypto off is tagged and skipped", func(t *testing.T) {
		e := newEnv(nil)
		plan := e.plan(t, src, createAccounts, withModules(inv))
		want := []importing.ModuleRecords{{Module: module.Crypto, Count: 1, Kinds: []string{"security_op"}, Lines: []int{4}}}
		if diff := cmp.Diff(want, plan.ModuleRecords); diff != "" {
			t.Fatalf("module records (-want +got):\n%s", diff)
		}
		res := must(e.svc.ApplyImport(context.Background(), plan))
		if res.SecurityOps != 0 || res.Securities != 0 || len(res.Warnings) != 1 {
			t.Fatalf("result = %+v", res)
		}
	})
	t.Run("investments off asks for crypto", func(t *testing.T) {
		plan := newEnv(nil).plan(t, src, createAccounts)
		if len(plan.ModuleRecords) != 1 || plan.ModuleRecords[0].Module != module.Crypto {
			t.Fatalf("module records = %+v", plan.ModuleRecords)
		}
	})
	t.Run("enabling crypto imports it", func(t *testing.T) {
		e := newEnv(nil)
		plan := e.plan(t, src, createAccounts, withModules(inv))
		res := must(e.svc.ApplyImportWith(context.Background(), plan, importing.ApplyOptions{
			EnableModules: []module.ID{module.Crypto}, Modules: inv, ModuleWriter: &fakeModuleWriter{},
		}))
		if res.SecurityOps != 1 || res.Securities != 1 {
			t.Fatalf("result = %+v", res)
		}
	})
	t.Run("crypto on imports directly", func(t *testing.T) {
		e := newEnv(nil)
		plan := e.plan(t, src, createAccounts, withModules(both))
		if len(plan.ModuleRecords) != 0 {
			t.Fatalf("module records = %+v", plan.ModuleRecords)
		}
		if res := must(e.svc.ApplyImport(context.Background(), plan)); res.SecurityOps != 1 {
			t.Fatalf("result = %+v", res)
		}
	})
}
