package importing_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/pkg/fuiproto"
)

func TestPlanClassifiesRows(t *testing.T) {
	e := newEnv(nil)
	acct := e.seedAccount(t, "Main", "bank", "main")
	e.seedTx(t, domain.Transaction{
		AccountID: acct.ID, Date: date("2026-10-01"), Amount: money(-1000), Description: "Coffee",
		Status: domain.StatusBooked, Kind: domain.KindPayment, ExternalID: "x1",
	})
	e.seedTx(t, domain.Transaction{
		AccountID: acct.ID, Date: date("2026-10-02"), Amount: money(-2000), Description: "Lunch",
		Status: domain.StatusBooked, Kind: domain.KindPayment,
	})
	e.seedTx(t, domain.Transaction{
		AccountID: acct.ID, Date: date("2026-10-05"), Amount: money(-3000), Description: "Cinema",
		Status: domain.StatusBooked, Kind: domain.KindPayment,
	})
	src := stream("bank", "sha1",
		account("main", "Main", "EUR"),
		txRec("main", "x1", "2026-10-01", -1000, "  Coffee  "), // same external id
		txRec("main", "x2", "2026-10-02", -2000, "LUNCH"),      // same fingerprint, new id
		txRec("main", "x3", "2026-10-06", -3000, "Movie"),      // same amount a day away
		txRec("main", "x4", "2026-10-07", -4000, "Rent"),       // new
		txRec("main", "x4", "2026-10-07", -4000, "Rent again"), // same id twice in the file
	)
	plan := e.plan(t, src)
	var got []domain.DedupClass
	for _, r := range plan.Rows {
		got = append(got, r.Class)
	}
	want := []domain.DedupClass{
		domain.DedupDuplicate, domain.DedupDuplicate, domain.DedupDoubtful, domain.DedupNew, domain.DedupDuplicate,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("classes (-want +got):\n%s", diff)
	}
	if c := plan.Counts(); c.Included != 2 || c.Duplicate != 3 || c.Doubtful != 1 || c.New != 1 {
		t.Fatalf("counts = %+v", c)
	}
	if plan.Rows[0].Line != 3 || plan.Rows[0].Tx.Description != "Coffee" {
		t.Fatalf("row 0 = line %d %q", plan.Rows[0].Line, plan.Rows[0].Tx.Description)
	}
}

func TestPlanDoesNotWrite(t *testing.T) {
	e := newEnv(nil)
	e.seedAccount(t, "Main", "bank", "main")
	before := e.store.Snapshot()
	plan := e.plan(t, stream("bank", "", account("main", "Main", "EUR"), txRec("main", "a", "2026-10-01", -100, "x")))
	if e.store.Snapshot() != before || len(e.backup.reasons) != 0 {
		t.Fatal("PlanImport wrote or backed up")
	}
	if !plan.Bulk || len(plan.Changes) == 0 {
		t.Fatalf("plan = %+v", plan.Plan)
	}
}

func TestPlanAccountResolution(t *testing.T) {
	t.Run("unknown ref asks for a mapping", func(t *testing.T) {
		e := newEnv(nil)
		plan := e.plan(t, stream("bank", "", account("r1", "Bank R1", "EUR"), txRec("r1", "a", "2026-10-01", -100, "x")))
		if len(plan.Unmapped) != 1 || plan.Unmapped[0].Ref != "r1" {
			t.Fatalf("unmapped = %+v", plan.Unmapped)
		}
		_, err := e.svc.ApplyImport(context.Background(), plan)
		if !errors.Is(err, importing.ErrUnmappedAccounts) {
			t.Fatalf("ApplyImport err = %v, want ErrUnmappedAccounts", err)
		}
	})
	t.Run("create accounts", func(t *testing.T) {
		e := newEnv(nil)
		plan := e.plan(t, stream("bank", "", account("r1", "Bank R1", "EUR"), txRec("r1", "a", "2026-10-01", -100, "x")), createAccounts)
		if len(plan.Unmapped) != 0 || len(plan.NewAccounts) != 1 || plan.NewAccounts[0].Account.Name != "Bank R1" {
			t.Fatalf("plan = %+v", plan)
		}
	})
	t.Run("name clash while creating asks for a mapping", func(t *testing.T) {
		e := newEnv(nil)
		e.seedAccount(t, "Bank R1", "other", "")
		plan := e.plan(t, stream("bank", "", account("r1", "bank r1", "EUR")), createAccounts)
		if len(plan.Unmapped) != 1 || plan.Unmapped[0].Suggestion != "Bank R1" {
			t.Fatalf("unmapped = %+v", plan.Unmapped)
		}
	})
	t.Run("explicit mapping is saved", func(t *testing.T) {
		e := newEnv(nil)
		acct := e.seedAccount(t, "Checking", "", "")
		src := stream("bank", "", account("r1", "Bank R1", "EUR"), txRec("r1", "a", "2026-10-01", -100, "x"))
		plan := e.plan(t, src, func(r *importing.PlanRequest) { r.Mappings = map[string]string{"r1": "check"} })
		res := must(e.svc.ApplyImport(context.Background(), plan))
		if res.Transactions != 1 || e.txs(t)[0].AccountID != acct.ID {
			t.Fatalf("result = %+v", res)
		}
		again := e.plan(t, src)
		if len(again.Unmapped) != 0 || again.Rows[0].Class != domain.DedupDuplicate {
			t.Fatalf("second plan = %+v", again)
		}
	})
	t.Run("unknown mapping target fails", func(t *testing.T) {
		e := newEnv(nil)
		_, err := e.svc.PlanImport(context.Background(), importing.PlanRequest{
			Source: stream("bank", "", account("r1", "R", "EUR")), Mappings: map[string]string{"r1": "nope"},
		})
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("err = %v, want not found", err)
		}
	})
	t.Run("ref equal to an account name binds it", func(t *testing.T) {
		e := newEnv(nil)
		acct := e.seedAccount(t, "Main", "", "")
		plan := e.plan(t, stream("fui", "", account("main", "Main", "EUR"), txRec("main", "a", "2026-10-01", -100, "x")))
		if len(plan.Unmapped) != 0 || plan.Rows[0].Tx.AccountID != acct.ID || len(plan.Bindings) != 1 {
			t.Fatalf("plan = %+v", plan)
		}
	})
	t.Run("bound account in another currency rejects its rows", func(t *testing.T) {
		e := newEnv(nil)
		e.seedAccount(t, "Main", "bank", "main")
		plan := e.plan(t, stream("bank", "", account("main", "Main", "USD"),
			txRec("main", "a", "2026-10-01", -100, "x", func(r *fuiproto.TransactionRecord) { r.Currency = "USD" })))
		if len(plan.Rows) != 0 || len(plan.Errors) != 1 {
			t.Fatalf("plan rows=%d errors=%+v", len(plan.Rows), plan.Errors)
		}
	})
}

func TestPlanRowErrors(t *testing.T) {
	e := newEnv(nil)
	e.seedAccount(t, "Main", "bank", "main")
	src := stream("bank", "",
		account("main", "Main", "EUR"),
		txRec("main", "a", "2026-13-40", -100, "bad date"),
		txRec("main", "b", "2026-10-01", -100, "ok"),
		fuiproto.Record{Type: fuiproto.RecordTypeError, Error: &fuiproto.ErrorRecord{Line: 7, Message: "cannot parse"}},
		txRec("main", "c", "2026-10-02", -100, "weird kind", kind("nonsense")),
		txRec("main", "d", "2026-10-02", -100, "bad category", category("")),
	)
	plan := e.plan(t, src)
	if len(plan.Rows) != 2 || len(plan.Errors) != 3 {
		t.Fatalf("rows=%d errors=%+v", len(plan.Rows), plan.Errors)
	}
}

func TestPlanAbortsOnBrokenStream(t *testing.T) {
	e := newEnv(nil)
	e.seedAccount(t, "Main", "bank", "main")
	good := stream("bank", "", account("main", "Main", "EUR"), txRec("main", "a", "2026-10-01", -100, "x"))
	t.Run("truncated", func(t *testing.T) {
		truncated := sliceSource{recs: good.recs[:len(good.recs)-1], stderr: "boom"}
		plan, err := e.svc.PlanImport(context.Background(), importing.PlanRequest{Source: truncated})
		if !errors.Is(err, importing.ErrInvalidStream) || !errors.Is(err, fuiproto.ErrTruncatedStream) {
			t.Fatalf("err = %v", err)
		}
		if plan.Stderr != "boom" {
			t.Fatalf("stderr = %q", plan.Stderr)
		}
	})
	t.Run("source failure", func(t *testing.T) {
		failing := good
		failing.recs, failing.failAfter = good.recs[:2], io.ErrUnexpectedEOF
		_, err := e.svc.PlanImport(context.Background(), importing.PlanRequest{Source: failing})
		if !errors.Is(err, importing.ErrConnectorFailed) || !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("no source", func(t *testing.T) {
		if _, err := e.svc.PlanImport(context.Background(), importing.PlanRequest{}); !errors.Is(err, domain.ErrMissingField) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestPlanAlreadyImported(t *testing.T) {
	e := newEnv(nil)
	src := stream("bank", "deadbeef", account("r1", "Bank R1", "EUR"), txRec("r1", "a", "2026-10-01", -100, "x"))
	first := e.plan(t, src, createAccounts)
	if first.AlreadyImported != nil {
		t.Fatal("first plan flagged")
	}
	must(e.svc.ApplyImport(context.Background(), first))
	second := e.plan(t, src, createAccounts)
	if second.AlreadyImported == nil || second.AlreadyImported.SourceSHA256 != "deadbeef" {
		t.Fatalf("second plan AlreadyImported = %+v", second.AlreadyImported)
	}
}

func TestPlanBalanceDiscrepancies(t *testing.T) {
	e := newEnv(nil)
	src := stream("bank", "",
		account("r1", "Bank R1", "EUR"),
		// newest first, as Widiba lists them; the last movement does not follow the previous balance.
		txRec("r1", "c", "2026-10-03", -300, "c", balanceAfter(400)),
		txRec("r1", "b", "2026-10-02", -200, "b", balanceAfter(800)),
		txRec("r1", "a", "2026-10-01", -100, "a", balanceAfter(1000)),
	)
	plan := e.plan(t, src, createAccounts)
	want := []importing.BalanceDiscrepancy{{
		AccountRef: "r1", Line: 3, Date: date("2026-10-03"), Expected: 500, Actual: 400,
	}}
	if diff := cmp.Diff(want, plan.Discrepancies, cmp.Comparer(func(a, b domain.Date) bool { return a == b })); diff != "" {
		t.Fatalf("discrepancies (-want +got):\n%s", diff)
	}
}

// TestPlanBalanceDiscrepanciesSkipsPending covers the bank-balance convention (e.g. Revolut's
// Balance column): a pending movement's balance_after does not reflect it, so it must neither
// advance the chain nor be checked against it.
func TestPlanBalanceDiscrepanciesSkipsPending(t *testing.T) {
	e := newEnv(nil)
	src := stream("bank", "",
		account("r1", "Bank R1", "EUR"),
		txRec("r1", "a", "2026-10-01", -100, "a", balanceAfter(1000)),
		// A pending incoming transfer: the bank balance excludes it, so it stays at 1000 even
		// though the movement itself is +200.
		txRec("r1", "p", "2026-10-02", 200, "pending transfer", balanceAfter(1000), pending()),
		txRec("r1", "b", "2026-10-03", -300, "b", balanceAfter(700)),
	)
	plan := e.plan(t, src, createAccounts)
	if len(plan.Discrepancies) != 0 {
		t.Fatalf("discrepancies = %+v, want none (the pending row must be skipped)", plan.Discrepancies)
	}
}

func TestPlanCategories(t *testing.T) {
	e := newEnv(nil)
	err := e.store.WithTx(context.Background(), func(r app.Repos) error {
		_, err := r.Categories().Insert(context.Background(), domain.Category{Name: "Groceries", Kind: domain.CategoryExpense})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	src := stream("fui", "", account("r1", "R1", "EUR"),
		txRec("r1", "a", "2026-10-01", -100, "x", category("groceries")),
		txRec("r1", "b", "2026-10-01", 5000, "y", category("Bonus")),
		txRec("r1", "c", "2026-10-01", -50, "z", category("BONUS")),
	)
	plan := e.plan(t, src, createAccounts)
	if len(plan.NewCategories) != 1 || plan.NewCategories[0].Name != "Bonus" || plan.NewCategories[0].Kind != domain.CategoryIncome {
		t.Fatalf("new categories = %+v", plan.NewCategories)
	}
	if plan.Rows[0].Category != "Groceries" {
		t.Fatalf("row category = %q, want canonical name", plan.Rows[0].Category)
	}
	res := must(e.svc.ApplyImport(context.Background(), plan))
	if res.Categories != 1 || res.Transactions != 3 {
		t.Fatalf("result = %+v", res)
	}
	for _, tx := range e.txs(t) {
		if tx.CategoryID == 0 {
			t.Fatalf("transaction %d has no category", tx.ID)
		}
	}
}

func TestPlanAndApplySplitRows(t *testing.T) {
	e := newEnv(nil)
	src := stream("fui", "", account("r1", "R1", "EUR"),
		txRec("r1", "a", "2026-10-01", -100, "x",
			withSplits(
				fuiproto.TransactionSplit{Category: "Food", Amount: -40, Note: "food"},
				fuiproto.TransactionSplit{Category: "Home", Amount: -60, Note: "home"},
			),
			payeeName("Shop"), withTags("weekly")))
	plan := e.plan(t, src, createAccounts)
	if len(plan.Rows) != 1 || len(plan.Rows[0].Splits) != 2 {
		t.Fatalf("plan rows = %+v", plan.Rows)
	}
	if len(plan.NewCategories) != 2 || len(plan.NewPayees) != 1 || len(plan.NewTags) != 1 {
		t.Fatalf("plan new entities = cats:%+v payees:%+v tags:%+v", plan.NewCategories, plan.NewPayees, plan.NewTags)
	}
	res := must(e.svc.ApplyImport(context.Background(), plan))
	if res.Transactions != 1 || res.Categories != 2 || res.Payees != 1 || res.Tags != 1 {
		t.Fatalf("result = %+v", res)
	}
	err := e.store.Read(context.Background(), func(r app.Repos) error {
		txs, err := r.Transactions().List(context.Background(), must(domain.NewTxFilter(domain.TxFilterSpec{})), nil, 0)
		if err != nil {
			return err
		}
		if len(txs) != 1 {
			t.Fatalf("transactions = %d, want 1", len(txs))
		}
		splits, err := r.Splits().SplitsOf(context.Background(), txs[0].ID)
		if err != nil {
			return err
		}
		if len(splits) != 2 || splits[0].Note != "food" || splits[1].Note != "home" {
			t.Fatalf("splits = %+v", splits)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPlanSecurityOpsModuleGate(t *testing.T) {
	e := newEnv(nil)
	src := stream("fui", "", account("r1", "Broker", "EUR"),
		txRec("r1", "cash-1", "2026-10-01", -100, "buy cash leg"),
		fuiproto.Record{Type: fuiproto.RecordTypeSecurityOp, SecurityOp: &fuiproto.SecurityOpRecord{
			Account:   "r1",
			Date:      "2026-10-01",
			Kind:      fuiproto.SecurityOpKindBuy,
			Quantity:  1000000,
			UnitPrice: 100,
			Fees:      0,
			CashTx:    "cash-1",
			Security: fuiproto.SecurityMeta{
				Name: "World ETF", Ticker: "VWCE", Kind: fuiproto.SecurityKindETF, Currency: "EUR", QuantityScale: 6,
			},
		}},
	)
	plan := e.plan(t, src, createAccounts)
	if len(plan.SecurityOps) != 0 {
		t.Fatalf("security ops = %+v, want none when investments is off", plan.SecurityOps)
	}
	if len(plan.Warnings) != 1 || plan.Warnings[0].Line == 0 {
		t.Fatalf("warnings = %+v, want one line warning", plan.Warnings)
	}
}

func TestPlanAndApplySecurityOps(t *testing.T) {
	e := newEnv(nil)
	investments := must(module.Validate([]module.ID{module.Core, module.Investments}))
	src := stream("fui", "", account("r1", "Broker", "EUR"),
		txRec("r1", "cash-1", "2026-10-01", -100, "buy cash leg", kind(fuiproto.TransactionKindInvestment)),
		fuiproto.Record{Type: fuiproto.RecordTypeSecurityOp, SecurityOp: &fuiproto.SecurityOpRecord{
			Account:   "r1",
			Date:      "2026-10-01",
			Kind:      fuiproto.SecurityOpKindBuy,
			Quantity:  1000000,
			UnitPrice: 100,
			Fees:      1,
			CashTx:    "cash-1",
			Security: fuiproto.SecurityMeta{
				Name: "World ETF", Ticker: "VWCE", Kind: fuiproto.SecurityKindETF, Currency: "EUR", QuantityScale: 6,
			},
		}},
	)
	plan := e.plan(t, src, createAccounts, withModules(investments))
	if len(plan.NewSecurities) != 1 || len(plan.SecurityOps) != 1 || !plan.SecurityOps[0].Include {
		t.Fatalf("plan securities=%+v ops=%+v", plan.NewSecurities, plan.SecurityOps)
	}
	res := must(e.svc.ApplyImport(context.Background(), plan))
	if res.Transactions != 1 || res.Securities != 1 || res.SecurityOps != 1 {
		t.Fatalf("result = %+v", res)
	}

	err := e.store.Read(context.Background(), func(r app.Repos) error {
		secs, err := r.Securities().List(context.Background())
		if err != nil {
			return err
		}
		if len(secs) != 1 || secs[0].Ticker != "VWCE" {
			t.Fatalf("securities = %+v", secs)
		}
		ops, err := r.Securities().ListOpsBySecurity(context.Background(), secs[0].ID, domain.Date{}, domain.Date{})
		if err != nil {
			return err
		}
		if len(ops) != 1 || ops[0].CashTx == 0 {
			t.Fatalf("security ops = %+v", ops)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestApplyWritesBatchAndBacksUp(t *testing.T) {
	e := newEnv(nil)
	src := stream("bank", "abc", account("r1", "Bank R1", "EUR"),
		txRec("r1", "a", "2026-10-01", -100, "x"), txRec("r1", "b", "2026-10-02", -200, "y"),
		fuiproto.Record{Type: fuiproto.RecordTypeBalance, Balance: &fuiproto.BalanceRecord{Account: "r1", Date: "2026-10-31", Amount: 1000}})
	plan := e.plan(t, src, createAccounts)
	if got := plan.NewAccounts[0].Account.OpeningBalance.Minor(); got != 1300 {
		t.Fatalf("derived opening balance = %d, want 1300", got)
	}
	if got := plan.NewAccounts[0].Account.OpenedOn.String(); got != "2026-10-01" {
		t.Fatalf("opened on %s", got)
	}
	res := must(e.svc.ApplyImport(context.Background(), plan))
	if res.Transactions != 2 || res.Accounts != 1 || res.Batch.ID == 0 || res.BackupPath == "" {
		t.Fatalf("result = %+v", res)
	}
	if got := e.backup.reasons; len(got) != 1 || got[0] != app.BackupReasonImport {
		t.Fatalf("backups = %v", got)
	}
	if res.Batch.SourceSHA256 != "abc" || res.Batch.Connector != "bank" || res.Batch.ImportedAt.IsZero() {
		t.Fatalf("batch = %+v", res.Batch)
	}
	for _, tx := range e.txs(t) {
		if tx.ImportBatchID != res.Batch.ID || tx.Fingerprint == "" {
			t.Fatalf("transaction %+v", tx)
		}
	}
}

func TestApplyHonoursIncludeOverrides(t *testing.T) {
	e := newEnv(nil)
	src := stream("bank", "", account("r1", "Bank R1", "EUR"),
		txRec("r1", "a", "2026-10-01", -100, "x"), txRec("r1", "b", "2026-10-02", -200, "y"))
	plan := e.plan(t, src, createAccounts)
	if err := plan.SetInclude(0, false); err != nil {
		t.Fatal(err)
	}
	if err := plan.SetInclude(9, true); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("SetInclude out of range = %v", err)
	}
	res := must(e.svc.ApplyImport(context.Background(), plan))
	if res.Transactions != 1 || e.txs(t)[0].ExternalID != "b" {
		t.Fatalf("result = %+v txs = %+v", res, e.txs(t))
	}
}

func TestApplyWithNothingIncludedWritesNothing(t *testing.T) {
	e := newEnv(nil)
	src := stream("bank", "", account("r1", "Bank R1", "EUR"), txRec("r1", "a", "2026-10-01", -100, "x"))
	e.importAll(t, src)
	snap := e.store.Snapshot()
	backups := len(e.backup.reasons)
	res := must(e.svc.ApplyImport(context.Background(), e.plan(t, src, createAccounts)))
	if res.Batch.ID != 0 || e.store.Snapshot() != snap || len(e.backup.reasons) != backups {
		t.Fatalf("second import wrote something: %+v", res)
	}
}

func TestApplyIsAtomic(t *testing.T) {
	e := newEnv(nil)
	e.seedAccount(t, "Main", "bank", "main")
	plan := e.plan(t, stream("bank", "", account("main", "Main", "EUR"),
		txRec("main", "a", "2026-10-01", -100, "x"), txRec("main", "b", "2026-10-02", -200, "y")))
	// Make the second insert fail on the unique external id after the first succeeded.
	plan.Rows[1].Tx.ExternalID = "a"
	before := e.store.Snapshot()
	if _, err := e.svc.ApplyImport(context.Background(), plan); !errors.Is(err, app.ErrAlreadyExists) {
		t.Fatalf("err = %v, want already exists", err)
	}
	if e.store.Snapshot() != before {
		t.Fatal("failed import left changes")
	}
}
