package investing_test

import (
	"context"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/investing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

func sp(s string) *string { return &s }

var btc = investing.AddSecurityRequest{Name: "Bitcoin", Ticker: "BTC", Kind: "crypto", Currency: "EUR"}

func (e *env) addWallet(t tb) {
	t.Helper()
	ctx := context.Background()
	err := e.store.WithTx(ctx, func(r app.Repos) error {
		_, err := r.Accounts().Insert(ctx, domain.Account{
			Name: "Wallet", Kind: domain.AccountWallet, Currency: eur,
			OpeningBalance: must(domain.NewMoney(0, eur)), OpenedOn: day("2026-01-01"),
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func (e *env) setPrice(t tb, sec, date, price string) {
	t.Helper()
	p, err := e.svc.PlanSetPrice(context.Background(), investing.SetPriceRequest{Security: sec, Date: date, Price: price})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.ApplySetPrice(context.Background(), p); err != nil {
		t.Fatal(err)
	}
}

func TestEditSecurity(t *testing.T) {
	e := newEnv(t, modules(module.Investments))
	ctx := context.Background()
	e.addSec(t, etf)
	e.addSec(t, investing.AddSecurityRequest{Name: "Other", Ticker: "OTH", Kind: "stock", Currency: "EUR"})

	for _, tc := range []struct {
		name string
		req  investing.EditSecurityRequest
		code domain.Code
	}{
		{"duplicate ticker", investing.EditSecurityRequest{Ref: "vwce", Ticker: sp("oth")}, app.CodeAlreadyExists},
		{"duplicate name", investing.EditSecurityRequest{Ref: "vwce", Name: sp("other")}, app.CodeAlreadyExists},
		{"no identifier left", investing.EditSecurityRequest{Ref: "OTH", Ticker: sp("")}, domain.CodeMissingField},
		{"unknown", investing.EditSecurityRequest{Ref: "nope"}, domain.CodeNotFound},
	} {
		if _, err := e.svc.PlanEditSecurity(ctx, tc.req); !hasCode(err, tc.code) {
			t.Errorf("%s: got %v, want %s", tc.name, err, tc.code)
		}
	}

	before := e.store.Snapshot()
	p, err := e.svc.PlanEditSecurity(ctx, investing.EditSecurityRequest{Ref: "VWCE", Name: sp("All-World ETF"), ISIN: sp("IE00BK5BQT80")})
	if err != nil || e.store.Snapshot() != before || p.Changes[0].Op != app.OpUpdate {
		t.Fatalf("plan = %+v, %v", p, err)
	}
	got, err := e.svc.ApplyEditSecurity(ctx, p)
	if err != nil || got.Name != "All-World ETF" || got.ISIN != "IE00BK5BQT80" || got.Ticker != "VWCE" || got.Kind != domain.SecurityETF {
		t.Fatalf("got %+v, %v", got, err)
	}
	if _, err := e.svc.PlanEditSecurity(ctx, investing.EditSecurityRequest{Ref: "IE00BK5BQT80", Name: sp("All-World ETF")}); err != nil {
		t.Fatalf("keeping own values: %v", err)
	}
}

func TestRemoveSecurity(t *testing.T) {
	e := newEnv(t, modules(module.Investments))
	ctx := context.Background()
	e.addSec(t, etf)
	e.addSec(t, investing.AddSecurityRequest{Name: "Other", Ticker: "OTH", Kind: "stock", Currency: "EUR"})
	e.setPrice(t, "VWCE", "2026-03-01", "100")
	e.setPrice(t, "OTH", "2026-03-01", "10")
	e.setPrice(t, "OTH", "2026-03-02", "11")
	if _, err := e.op(t, investing.OpRequest{Kind: domain.OpBuy, Account: "Broker", Security: "VWCE", Date: "2026-03-01", Quantity: "1", Price: "100"}); err != nil {
		t.Fatal(err)
	}

	if _, err := e.svc.PlanRemoveSecurity(ctx, "VWCE"); !hasCode(err, app.CodeInUse) {
		t.Fatalf("with ops: got %v", err)
	}
	if _, err := e.svc.PlanRemoveSecurity(ctx, "nope"); !hasCode(err, domain.CodeNotFound) {
		t.Fatalf("unknown: got %v", err)
	}
	p, err := e.svc.PlanRemoveSecurity(ctx, "OTH")
	if err != nil || p.Prices != 2 {
		t.Fatalf("plan = %+v, %v", p, err)
	}
	if err := e.svc.ApplyRemoveSecurity(ctx, p); err != nil {
		t.Fatal(err)
	}
	list, _ := e.svc.ListSecurities(ctx)
	if len(list) != 1 {
		t.Fatalf("list = %+v", list)
	}
	err = e.store.Read(ctx, func(r app.Repos) error {
		prices, err := r.Securities().ListPrices(ctx, p.Security.ID)
		if err != nil || len(prices) != 0 {
			t.Errorf("prices left: %v, %v", prices, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestListOps(t *testing.T) {
	e := newEnv(t, modules(module.Investments, module.Crypto))
	ctx := context.Background()
	e.addSec(t, etf)
	e.addSec(t, btc)
	for _, o := range []investing.OpRequest{
		{Kind: domain.OpBuy, Account: "Broker", Security: "VWCE", Date: "2026-02-10", Quantity: "10", Price: "100"},
		{Kind: domain.OpBuy, Account: "Broker2", Security: "VWCE", Date: "2026-02-01", Quantity: "2", Price: "100"},
		{Kind: domain.OpDividend, Account: "Broker", Security: "VWCE", Date: "2026-02-10", Quantity: "10", Price: "1"},
		{Kind: domain.OpBuy, Account: "Broker", Security: "BTC", Date: "2026-02-05", Quantity: "0.5", Price: "40000"},
	} {
		if _, err := e.op(t, o); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name string
		req  investing.ListOpsRequest
		ids  []domain.SecurityOpID
	}{
		{"all by date then id", investing.ListOpsRequest{}, []domain.SecurityOpID{2, 4, 1, 3}},
		{"account", investing.ListOpsRequest{Account: "broker2"}, []domain.SecurityOpID{2}},
		{"security", investing.ListOpsRequest{Security: "btc"}, []domain.SecurityOpID{4}},
		{"kind", investing.ListOpsRequest{Kind: "dividend"}, []domain.SecurityOpID{3}},
		{"combined", investing.ListOpsRequest{Account: "Broker", Security: "VWCE", Kind: "buy"}, []domain.SecurityOpID{1}},
	} {
		lines, err := e.svc.ListOps(ctx, tc.req)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		var ids []domain.SecurityOpID
		for _, l := range lines {
			ids = append(ids, l.Op.ID)
			if l.Security.ID != l.Op.Security || l.Account.ID != l.Op.Account {
				t.Errorf("%s: line %+v mismatched", tc.name, l)
			}
		}
		if len(ids) != len(tc.ids) {
			t.Fatalf("%s: ids = %v, want %v", tc.name, ids, tc.ids)
		}
		for i := range ids {
			if ids[i] != tc.ids[i] {
				t.Fatalf("%s: ids = %v, want %v", tc.name, ids, tc.ids)
			}
		}
	}
	for _, bad := range []investing.ListOpsRequest{{Kind: "nope"}, {Account: "nope"}, {Security: "nope"}} {
		if _, err := e.svc.ListOps(ctx, bad); err == nil {
			t.Errorf("%+v: want error", bad)
		}
	}

	off := investing.New(e.store, fixedClock{}, investing.Config{BaseCurrency: eur, Modules: modules(module.Investments)})
	lines, err := off.ListOps(ctx, investing.ListOpsRequest{})
	if err != nil || len(lines) != 3 {
		t.Fatalf("crypto off: %d lines, %v", len(lines), err)
	}
	if _, err := off.ListOps(ctx, investing.ListOpsRequest{Security: "BTC"}); !hasCode(err, domain.CodeNotFound) {
		t.Fatalf("hidden security: got %v", err)
	}
	secs, _ := off.ListSecurities(ctx)
	if len(secs) != 1 {
		t.Fatalf("securities with crypto off = %+v", secs)
	}
	if _, err := off.PlanRemoveSecurity(ctx, "BTC"); !hasCode(err, domain.CodeNotFound) {
		t.Fatalf("hidden remove: got %v", err)
	}
	if _, err := off.ListSecurityPositions(ctx, ""); err != nil {
		t.Fatal(err)
	}
}

func TestEditOp(t *testing.T) {
	e := newEnv(t, modules(module.Investments))
	ctx := context.Background()
	e.addSec(t, etf)
	buy, err := e.op(t, investing.OpRequest{Kind: domain.OpBuy, Account: "Broker", CashAccount: "Cash", Security: "VWCE", Date: "2026-02-01", Quantity: "10", Price: "100", Fees: "2"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.op(t, investing.OpRequest{Kind: domain.OpSell, Account: "Broker", Security: "VWCE", Date: "2026-02-10", Quantity: "8", Price: "110"}); err != nil {
		t.Fatal(err)
	}

	if _, err := e.svc.PlanEditOp(ctx, investing.EditOpRequest{ID: buy.ID, Quantity: sp("5")}); !hasCode(err, domain.CodeInsufficientQuantity) {
		t.Fatalf("shrinking below a later sell: got %v", err)
	}
	if _, err := e.svc.PlanEditOp(ctx, investing.EditOpRequest{ID: buy.ID, Date: sp("2026-02-20")}); !hasCode(err, domain.CodeInsufficientQuantity) {
		t.Fatalf("moving after the sell: got %v", err)
	}
	for _, bad := range []investing.EditOpRequest{
		{ID: 99}, {ID: buy.ID, Quantity: sp("x")}, {ID: buy.ID, Price: sp("x")}, {ID: buy.ID, Fees: sp("x")}, {ID: buy.ID, Date: sp("junk")}, {ID: buy.ID, Quantity: sp("0")},
	} {
		if _, err := e.svc.PlanEditOp(ctx, bad); err == nil {
			t.Errorf("%+v: want error", bad)
		}
	}

	before := e.store.Snapshot()
	p, err := e.svc.PlanEditOp(ctx, investing.EditOpRequest{ID: buy.ID, Date: sp("2026-02-02"), Quantity: sp("12"), Price: sp("101"), Fees: sp("")})
	if err != nil || e.store.Snapshot() != before || len(p.Changes) != 2 || p.CashTx == nil {
		t.Fatalf("plan = %+v, %v", p, err)
	}
	got, err := e.svc.ApplyEditOp(ctx, p)
	if err != nil || got.Quantity.Units() != 12 || got.UnitPrice.Minor() != 10100 || !got.Fees.IsZero() || got.CashTx != buy.CashTx {
		t.Fatalf("got %+v, %v", got, err)
	}
	err = e.store.Read(ctx, func(r app.Repos) error {
		tx, err := r.Transactions().Get(ctx, buy.CashTx)
		if err != nil {
			return err
		}
		if tx.Amount.Minor() != -121200 || tx.Date != day("2026-02-02") || tx.Kind != domain.KindInvestment {
			t.Errorf("cash tx = %+v", tx)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// A reconciled cash transaction freezes the operation.
	err = e.store.WithTx(ctx, func(r app.Repos) error {
		tx, err := r.Transactions().Get(ctx, buy.CashTx)
		if err != nil {
			return err
		}
		tx.Status = domain.StatusReconciled
		return r.Transactions().Update(ctx, tx)
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.PlanEditOp(ctx, investing.EditOpRequest{ID: buy.ID, Price: sp("90")}); !hasCode(err, domain.CodeReconciled) {
		t.Fatalf("reconciled: got %v", err)
	}
	if _, err := e.svc.ApplyEditOp(ctx, investing.EditOpPlan{Op: got}); !hasCode(err, domain.CodeReconciled) {
		t.Fatalf("apply on reconciled: got %v", err)
	}

	// An operation without cash link edits alone; a stale plan is re-checked on apply.
	sell := must(e.svc.ListOps(ctx, investing.ListOpsRequest{Kind: "sell"}))[0].Op
	p, err = e.svc.PlanEditOp(ctx, investing.EditOpRequest{ID: sell.ID, Quantity: sp("9")})
	if err != nil || p.CashTx != nil || len(p.Changes) != 1 {
		t.Fatalf("sell plan = %+v, %v", p, err)
	}
	p.Op.Quantity = must(domain.NewQuantity(13, 0))
	if _, err := e.svc.ApplyEditOp(ctx, p); !hasCode(err, domain.CodeInsufficientQuantity) {
		t.Fatalf("stale plan: got %v", err)
	}
	p.Op.Quantity = must(domain.NewQuantity(9, 0))
	if _, err := e.svc.ApplyEditOp(ctx, p); err != nil {
		t.Fatal(err)
	}
	off := investing.New(e.store, fixedClock{}, investing.Config{BaseCurrency: eur, Modules: modules()})
	if _, err := off.PlanEditOp(ctx, investing.EditOpRequest{ID: sell.ID}); !hasCode(err, app.CodeModuleDisabled) {
		t.Fatalf("investments off: got %v", err)
	}
}

func TestRemoveOpDeletesCashTx(t *testing.T) {
	e := newEnv(t, modules(module.Investments))
	ctx := context.Background()
	e.addSec(t, etf)
	buy, err := e.op(t, investing.OpRequest{Kind: domain.OpBuy, Account: "Broker", CashAccount: "Cash", Security: "VWCE", Date: "2026-02-01", Quantity: "1", Price: "100"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := e.svc.PlanRemoveOp(ctx, buy.ID)
	if err != nil || len(p.Changes) != 2 {
		t.Fatalf("plan = %+v, %v", p, err)
	}
	if err := e.svc.ApplyRemoveOp(ctx, p); err != nil {
		t.Fatal(err)
	}
	err = e.store.Read(ctx, func(r app.Repos) error {
		if _, err := r.Transactions().Get(ctx, buy.CashTx); !hasCode(err, domain.CodeNotFound) {
			t.Errorf("cash tx still there: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCoupon(t *testing.T) {
	e := newEnv(t, modules(module.Investments, module.Crypto))
	ctx := context.Background()
	e.addSec(t, investing.AddSecurityRequest{Name: "BTP 2030", ISIN: "IT0005438004", Kind: "bond", Currency: "EUR"})
	e.addSec(t, btc)
	if _, err := e.op(t, investing.OpRequest{Kind: domain.OpBuy, Account: "Broker", Security: "BTP 2030", Date: "2026-01-10", Quantity: "100", Price: "100"}); err != nil {
		t.Fatal(err)
	}
	p, err := e.svc.PlanCoupon(ctx, investing.OpRequest{Account: "Broker", CashAccount: "Cash", Security: "BTP 2030", Date: "2026-03-01", Quantity: "100", Price: "1.5", Fees: "1"})
	if err != nil || p.Op.Kind != domain.OpCoupon || p.CashTx == nil || p.CashTx.Amount.Minor() != 14900 || p.CashTx.Kind != domain.KindInterest {
		t.Fatalf("plan = %+v, %v", p, err)
	}
	if _, err := e.svc.ApplyOp(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.PlanCoupon(ctx, investing.OpRequest{Account: "Broker", Security: "BTC", Quantity: "1", Price: "1"}); !hasCode(err, domain.CodeInvalidValue) {
		t.Fatalf("crypto coupon: got %v", err)
	}
}

func TestOpAccountMatrix(t *testing.T) {
	e := newEnv(t, modules(module.Investments, module.Crypto))
	e.addWallet(t)
	e.addSec(t, etf)
	e.addSec(t, btc)
	for _, tc := range []struct {
		name, account, security string
		ok                      bool
	}{
		{"etf on investment", "Broker", "VWCE", true},
		{"crypto on investment", "Broker", "BTC", true},
		{"crypto on wallet", "Wallet", "BTC", true},
		{"etf on wallet", "Wallet", "VWCE", false},
		{"etf on checking", "Cash", "VWCE", false},
		{"crypto on checking", "Cash", "BTC", false},
	} {
		_, err := e.op(t, investing.OpRequest{Kind: domain.OpBuy, Account: tc.account, Security: tc.security, Date: "2026-02-01", Quantity: "1", Price: "10"})
		switch {
		case tc.ok && err != nil:
			t.Errorf("%s: %v", tc.name, err)
		case !tc.ok && !hasCode(err, domain.CodeInvalidValue):
			t.Errorf("%s: got %v, want invalid_value", tc.name, err)
		}
	}

	// A wallet needs the crypto module, even for a security that is not crypto.
	off := investing.New(e.store, fixedClock{}, investing.Config{BaseCurrency: eur, Modules: modules(module.Investments)})
	_, err := off.PlanBuy(context.Background(), investing.OpRequest{Account: "Wallet", Security: "VWCE", Quantity: "1", Price: "10"})
	if !hasCode(err, app.CodeModuleDisabled) {
		t.Fatalf("wallet with crypto off: got %v", err)
	}
}

func TestCryptoSubCentPrice(t *testing.T) {
	e := newEnv(t, modules(module.Investments, module.Crypto))
	ctx := context.Background()
	e.addSec(t, btc)
	half, err := e.op(t, investing.OpRequest{Kind: domain.OpBuy, Account: "Broker", Security: "BTC", Date: "2026-02-01", Quantity: "0.5", Price: "0.00001234"})
	if err != nil || half.UnitPrice.Minor() != 1234 || half.UnitPrice.Scale() != 8 {
		t.Fatalf("op = %+v, %v", half, err)
	}
	if _, err := e.op(t, investing.OpRequest{Kind: domain.OpBuy, Account: "Broker", Security: "BTC", Date: "2026-02-01", Quantity: "1000000", Price: "0.00001234"}); err != nil {
		t.Fatal(err)
	}
	e.setPrice(t, "BTC", "2026-02-02", "0.00001234")
	lines, err := e.svc.ListSecurityPositions(ctx, "2026-02-02")
	if err != nil || len(lines) != 1 {
		t.Fatalf("lines = %+v, %v", lines, err)
	}
	// 1000000.5 × 0.00001234 = 12.3400617 → 12.34.
	if v := lines[0].Value; v.Minor() != 1234 || !lines[0].HasPrice {
		t.Fatalf("value = %s", v)
	}
	// The cash movement is exact too: 0.5 × 0.00001234 rounds to zero cents, the million does not.
	cash, err := e.svc.PlanBuy(ctx, investing.OpRequest{Account: "Broker", CashAccount: "Cash", Security: "BTC", Quantity: "1000000", Price: "0.00001234"})
	if err != nil || cash.CashTx.Amount.Minor() != -1234 {
		t.Fatalf("cash = %+v, %v", cash.CashTx, err)
	}
}

func TestListSecurityPositions(t *testing.T) {
	e := newEnv(t, modules(module.Investments))
	ctx := context.Background()
	e.addSec(t, etf)
	e.addSec(t, investing.AddSecurityRequest{Name: "No Price", Ticker: "NOP", Kind: "stock", Currency: "EUR"})
	e.addSec(t, investing.AddSecurityRequest{Name: "Sold", Ticker: "SLD", Kind: "stock", Currency: "EUR"})
	for _, o := range []investing.OpRequest{
		{Kind: domain.OpBuy, Account: "Broker", Security: "VWCE", Date: "2026-01-10", Quantity: "10", Price: "100"},
		{Kind: domain.OpBuy, Account: "Broker2", Security: "VWCE", Date: "2026-01-11", Quantity: "5", Price: "110", Fees: "5"},
		{Kind: domain.OpBuy, Account: "Broker", Security: "NOP", Date: "2026-01-10", Quantity: "1", Price: "10"},
		{Kind: domain.OpBuy, Account: "Broker", Security: "SLD", Date: "2026-01-10", Quantity: "1", Price: "10"},
		{Kind: domain.OpSell, Account: "Broker", Security: "SLD", Date: "2026-01-11", Quantity: "1", Price: "12"},
	} {
		if _, err := e.op(t, o); err != nil {
			t.Fatal(err)
		}
	}
	e.setPrice(t, "VWCE", "2026-02-01", "120")

	lines, err := e.svc.ListSecurityPositions(ctx, "2026-02-15")
	if err != nil || len(lines) != 3 {
		t.Fatalf("lines = %+v, %v", lines, err)
	}
	v := lines[0]
	if v.Security.Ticker != "VWCE" || v.Quantity.Units() != 15 || v.Accounts != 2 || !v.HasPrice ||
		v.CostBasis.Minor() != 155500 || v.Value.Minor() != 180000 || v.Unrealized.Minor() != 24500 || v.Value.Currency() != eur {
		t.Fatalf("VWCE line = %+v", v)
	}
	if n := lines[1]; n.HasPrice || n.Accounts != 1 || n.CostBasis.Minor() != 1000 {
		t.Fatalf("NOP line = %+v", n)
	}
	if s := lines[2]; s.Accounts != 0 || !s.Quantity.IsZero() || s.HasPrice {
		t.Fatalf("SLD line = %+v", s)
	}
	if early, err := e.svc.ListSecurityPositions(ctx, "2026-01-10"); err != nil || len(early) != 3 || early[0].Accounts != 1 {
		t.Fatalf("early = %+v, %v", early, err)
	}
	if _, err := e.svc.ListSecurityPositions(ctx, "x"); err == nil {
		t.Fatal("bad date: want error")
	}
}
