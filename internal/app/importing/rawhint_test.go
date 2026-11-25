package importing_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"pgregory.net/rapid"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/pkg/fuiproto"
)

func withOriginal(amount int64, cur string) func(*fuiproto.TransactionRecord) {
	return func(t *fuiproto.TransactionRecord) {
		t.Original = &fuiproto.OriginalAmount{Amount: amount, Currency: cur}
	}
}

func withRaw(raw map[string]any) func(*fuiproto.TransactionRecord) {
	return func(t *fuiproto.TransactionRecord) { t.Raw = raw }
}

func withHint(h string) func(*fuiproto.TransactionRecord) {
	return func(t *fuiproto.TransactionRecord) { t.CategoryHint = h }
}

func exportFUI(t *testing.T, e env) []fuiproto.Record {
	t.Helper()
	var buf bytes.Buffer
	if _, err := e.svc.Export(context.Background(), importing.ExportRequest{Format: importing.FormatFUI}, &buf); err != nil {
		t.Fatal(err)
	}
	var out []fuiproto.Record
	for r, err := range fuiproto.DecodeJSONL(&buf) {
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func TestOriginalAndRawStoredAndExported(t *testing.T) {
	e := newEnv(nil)
	raw := map[string]any{"z": "last", "a": json.Number("1"), "n": map[string]any{"k": true}}
	e.importAll(t, stream("bank", "", account("a", "Main", "EUR"),
		txRec("a", "x", "2026-10-03", -300, "coffee", withOriginal(-350, "USD"), withRaw(raw)),
		txRec("a", "y", "2026-10-04", -100, "plain"),
	))
	var got domain.Transaction
	for _, tx := range e.txs(t) {
		if tx.ExternalID == "x" {
			got = tx
		}
		if tx.ExternalID == "y" && (tx.Original != nil || tx.Raw != "") {
			t.Fatalf("plain tx carries original/raw: %+v", tx)
		}
	}
	if got.Original == nil || got.Original.Minor() != -350 || got.Original.Currency().Code() != "USD" {
		t.Fatalf("original = %v", got.Original)
	}
	if want := `{"a":1,"n":{"k":true},"z":"last"}`; got.Raw != want {
		t.Fatalf("raw = %s, want %s", got.Raw, want)
	}
	for _, r := range exportFUI(t, e) {
		if tx := r.Transaction; tx != nil && tx.ID == "x" {
			want := &fuiproto.OriginalAmount{Amount: -350, Currency: "USD"}
			if diff := cmp.Diff(want, tx.Original); diff != "" {
				t.Fatalf("exported original: %s", diff)
			}
			if len(tx.Raw) != 3 {
				t.Fatalf("exported raw = %v", tx.Raw)
			}
		}
	}
}

func TestInvalidOriginalIsRowError(t *testing.T) {
	e := newEnv(nil)
	plan := e.plan(t, stream("bank", "", account("a", "Main", "EUR"),
		txRec("a", "x", "2026-10-03", -300, "coffee", withOriginal(-350, "??"))), createAccounts)
	if len(plan.Errors) != 1 || len(plan.Rows) != 0 {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestBalanceMismatches(t *testing.T) {
	bal := func(date string, amount int64) fuiproto.Record {
		return fuiproto.Record{Type: fuiproto.RecordTypeBalance, Balance: &fuiproto.BalanceRecord{Account: "a", Date: date, Amount: amount}}
	}
	tests := []struct {
		name    string
		balance fuiproto.Record
		new     bool
		want    string // suggest text, empty when no mismatch
	}{
		{"matches", bal("2026-10-31", 600), false, ""},
		{"differs", bal("2026-10-31", 999), false, "fui reconcile 'Main account' --date 2026-10-31 --balance 9.99"},
		{"date before new rows", bal("2026-09-30", 1000), false, ""},
		{"new account derives opening", bal("2026-10-31", 999), true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(nil)
			if !tt.new {
				a := e.seedAccount(t, "Main account", "bank", "a")
				e.seedTx(t, domain.Transaction{
					AccountID: a.ID, Date: date("2026-09-01"), Amount: money(1000), Description: "salary",
					Status: domain.StatusBooked, Kind: domain.KindOther, ExternalID: "old",
				})
			}
			src := stream("bank", "", account("a", "Main account", "EUR"),
				txRec("a", "n", "2026-10-03", -400, "coffee"), tt.balance)
			plan := e.plan(t, src, createAccounts)
			if tt.want == "" {
				if len(plan.BalanceMismatches) != 0 {
					t.Fatalf("mismatches = %+v", plan.BalanceMismatches)
				}
				return
			}
			if len(plan.BalanceMismatches) != 1 {
				t.Fatalf("mismatches = %+v", plan.BalanceMismatches)
			}
			m := plan.BalanceMismatches[0]
			if m.Suggest != tt.want || m.Expected != 999 || m.Computed != 600 || m.AccountRef != "a" || m.AccountID <= 0 {
				t.Fatalf("mismatch = %+v", m)
			}
		})
	}
}

func TestBalanceMismatchesFollowInclude(t *testing.T) {
	e := newEnv(nil)
	a := e.seedAccount(t, "Main account", "bank", "a")
	e.seedTx(t, domain.Transaction{
		AccountID: a.ID, Date: date("2026-09-01"), Amount: money(1000), Description: "salary",
		Status: domain.StatusBooked, Kind: domain.KindOther, ExternalID: "old",
	})
	bal := fuiproto.Record{Type: fuiproto.RecordTypeBalance, Balance: &fuiproto.BalanceRecord{Account: "a", Date: "2026-10-31", Amount: 600}}
	plan := e.plan(t, stream("bank", "", account("a", "Main account", "EUR"), txRec("a", "n", "2026-10-03", -400, "coffee"), bal), createAccounts)
	if len(plan.BalanceMismatches) != 0 {
		t.Fatalf("included row: mismatches = %+v", plan.BalanceMismatches)
	}
	if err := plan.SetInclude(0, false); err != nil {
		t.Fatal(err)
	}
	if len(plan.BalanceMismatches) != 1 || plan.BalanceMismatches[0].Computed != 1000 {
		t.Fatalf("excluded row: mismatches = %+v", plan.BalanceMismatches)
	}
	if err := plan.SetInclude(0, true); err != nil {
		t.Fatal(err)
	}
	if len(plan.BalanceMismatches) != 0 {
		t.Fatalf("re-included row: mismatches = %+v", plan.BalanceMismatches)
	}
}

func TestHintOnlyFeedsRules(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name  string
		cond  domain.RuleConditions
		hint  string
		match bool
	}{
		{"equals", domain.RuleConditions{HintEquals: "groceries"}, "Groceries", true},
		{"equals no substring", domain.RuleConditions{HintEquals: "groc"}, "Groceries", false},
		{"contains case", domain.RuleConditions{HintContains: "ROCER"}, "Groceries", true},
		{"no hint", domain.RuleConditions{HintContains: "groc"}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(nil)
			e.seedAccount(t, "Main", "bank", "main")
			err := e.store.WithTx(ctx, func(r app.Repos) error {
				c, err := r.Categories().Insert(ctx, must(domain.NewCategory("Food", domain.CategoryExpense)))
				if err != nil {
					return err
				}
				_, err = r.Rules().Insert(ctx, domain.Rule{
					Name: "h", Enabled: true, Conditions: tt.cond, Actions: domain.RuleActions{CategoryID: c.ID},
				})
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			plan := e.plan(t, stream("bank", "", account("main", "Main", "EUR"),
				txRec("main", "a", "2026-10-10", -300, "shop", withHint(tt.hint))))
			if got := plan.Rows[0].Category == "Food"; got != tt.match {
				t.Fatalf("category = %q, match want %v", plan.Rows[0].Category, tt.match)
			}
		})
	}
}

func TestPropertyHintNeverAppliedWithoutRule(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		hint := rapid.StringMatching(`[A-Za-z]{1,12}`).Draw(t, "hint")
		e := newEnv(nil)
		e.seedAccount(t, "Main", "bank", "main")
		plan := e.plan(t, stream("bank", "", account("main", "Main", "EUR"),
			txRec("main", "a", "2026-10-10", -300, "shop", withHint(hint))))
		if r := plan.Rows[0]; r.Category != "" || r.Hint == "" || len(plan.NewCategories) != 0 {
			t.Fatalf("hint leaked into the row: %+v", r)
		}
	})
}

func TestPropertyExportKeepsOriginalAndRaw(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		amount := rapid.Int64Range(-1_000_000, 1_000_000).Draw(t, "original")
		raw := rapid.MapOfN(rapid.StringMatching(`[a-z]{1,5}`), rapid.StringMatching(`[ -~]{0,8}`), 1, 4).Draw(t, "raw")
		rawAny := make(map[string]any, len(raw))
		for k, v := range raw {
			rawAny[k] = v
		}
		a := newEnv(nil)
		a.importAll(t, stream("bank", "", account("a", "Main", "EUR"),
			txRec("a", "x", "2026-10-03", -300, "coffee", withOriginal(amount, "USD"), withRaw(rawAny))))
		b := newEnv(nil)
		b.importAll(t, fuiSourceFrom(t, a))
		wantTx, gotTx := a.txs(t)[0], b.txs(t)[0]
		if gotTx.Raw != wantTx.Raw || gotTx.Raw == "" || gotTx.Original == nil || *gotTx.Original != *wantTx.Original {
			t.Fatalf("round trip lost original/raw: got %+v want %+v", gotTx, wantTx)
		}
	})
}

func fuiSourceFrom(t tb, e env) importing.Source {
	t.Helper()
	var buf bytes.Buffer
	if _, err := e.svc.Export(context.Background(), importing.ExportRequest{Format: importing.FormatFUI}, &buf); err != nil {
		t.Fatalf("export: %v", err)
	}
	return fuiSource(t, buf.Bytes())
}

func secOpRecAt(kind fuiproto.SecurityOpKind, date, cash string) fuiproto.Record {
	return fuiproto.Record{Type: fuiproto.RecordTypeSecurityOp, SecurityOp: &fuiproto.SecurityOpRecord{
		Account: "r1", Date: date, Kind: kind, Quantity: 1000000, UnitPrice: 100, CashTx: cash,
		Security: fuiproto.SecurityMeta{Name: "World ETF", Ticker: "VWCE", Kind: fuiproto.SecurityKindETF, Currency: "EUR", QuantityScale: 6},
	}}
}

func TestUndoRemovesSecurityOps(t *testing.T) {
	e := newEnv(nil)
	inv := must(module.Validate([]module.ID{module.Core, module.Investments}))
	src := stream("fui", "", account("r1", "Broker", "EUR"),
		txRec("r1", "buy", "2026-10-01", -100, "buy", kind(fuiproto.TransactionKindInvestment)),
		txRec("r1", "div", "2026-10-02", 5, "dividend", kind(fuiproto.TransactionKindInvestment)),
		secOpRecAt(fuiproto.SecurityOpKindBuy, "2026-10-01", "buy"),
		secOpRecAt(fuiproto.SecurityOpKindDividend, "2026-10-02", "div"),
	)
	res := must(e.svc.ApplyImport(context.Background(), e.plan(t, src, createAccounts, withModules(inv))))
	if res.SecurityOps != 2 {
		t.Fatalf("result = %+v", res)
	}
	plan := must(e.svc.PlanUndo(context.Background(), res.Batch.ID))
	if len(plan.SecurityOps) != 2 || len(plan.Transactions) != 2 || len(plan.Changes) != 3 {
		t.Fatalf("undo plan = %+v", plan)
	}
	if err := e.svc.ApplyUndo(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	err := e.store.Read(context.Background(), func(r app.Repos) error {
		ops, err := r.Securities().ListOpsByBatch(context.Background(), res.Batch.ID)
		if err != nil || len(ops) != 0 {
			t.Fatalf("ops left = %v err=%v", ops, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if left := e.txs(t); len(left) != 0 {
		t.Fatalf("transactions left: %+v", left)
	}
}

func TestUndoRefusesReconciledKeepsSecurityOps(t *testing.T) {
	e := newEnv(nil)
	inv := must(module.Validate([]module.ID{module.Core, module.Investments}))
	src := stream("fui", "", account("r1", "Broker", "EUR"),
		txRec("r1", "buy", "2026-10-01", -100, "buy", kind(fuiproto.TransactionKindInvestment)),
		secOpRecAt(fuiproto.SecurityOpKindBuy, "2026-10-01", "buy"),
	)
	res := must(e.svc.ApplyImport(context.Background(), e.plan(t, src, createAccounts, withModules(inv))))
	tx := e.txs(t)[0]
	tx.Status = domain.StatusReconciled
	err := e.store.WithTx(context.Background(), func(r app.Repos) error { return r.Transactions().Update(context.Background(), tx) })
	if err != nil {
		t.Fatal(err)
	}
	before := e.store.Snapshot()
	if _, err := e.svc.PlanUndo(context.Background(), res.Batch.ID); !errors.Is(err, domain.ErrReconciled) {
		t.Fatalf("err = %v", err)
	}
	if err := e.svc.ApplyUndo(context.Background(), importing.UndoPlan{Batch: res.Batch}); err == nil || !strings.Contains(err.Error(), "reconciled") {
		t.Fatalf("ApplyUndo err = %v", err)
	}
	if e.store.Snapshot() != before {
		t.Fatal("refused undo changed the archive")
	}
}
