package investing_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/investing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/testutil/memstore"
)

// tb is the subset of testing.TB shared by *testing.T and *rapid.T.
type tb interface {
	Helper()
	Fatal(args ...any)
}

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC) }

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

var (
	eur = must(domain.ParseCurrency("EUR"))
	usd = must(domain.ParseCurrency("USD"))
)

func day(s string) domain.Date { return must(domain.ParseDate(s)) }

func modules(ids ...module.ID) module.Set {
	return must(module.Validate(append([]module.ID{module.Core}, ids...)))
}

type env struct {
	store *memstore.Store
	svc   *investing.Service
}

func newEnv(t tb, mods module.Set) *env {
	t.Helper()
	e := &env{store: memstore.New()}
	e.svc = investing.New(e.store, fixedClock{}, investing.Config{
		BaseCurrency: eur, Location: time.UTC, Modules: mods,
	})
	ctx := context.Background()
	err := e.store.WithTx(ctx, func(r app.Repos) error {
		for _, a := range []domain.Account{
			{Name: "Broker", Kind: domain.AccountInvestment, Currency: eur, OpeningBalance: must(domain.NewMoney(0, eur)), OpenedOn: day("2026-01-01")},
			{Name: "Cash", Kind: domain.AccountChecking, Currency: eur, OpeningBalance: must(domain.NewMoney(0, eur)), OpenedOn: day("2026-01-01")},
			{Name: "Dollars", Kind: domain.AccountChecking, Currency: usd, OpeningBalance: must(domain.NewMoney(0, usd)), OpenedOn: day("2026-01-01")},
		} {
			if _, err := r.Accounts().Insert(ctx, a); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *env) addSec(t tb, req investing.AddSecurityRequest) domain.Security {
	t.Helper()
	p, err := e.svc.PlanAddSecurity(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	s, err := e.svc.ApplyAddSecurity(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func (e *env) op(t tb, req investing.OpRequest) (domain.SecurityOp, error) {
	t.Helper()
	p, err := e.svc.PlanOp(context.Background(), req)
	if err != nil {
		return domain.SecurityOp{}, err
	}
	return e.svc.ApplyOp(context.Background(), p)
}

var etf = investing.AddSecurityRequest{Name: "World ETF", Ticker: "vwce", Kind: "etf", Currency: "EUR"}

func hasCode(err error, c domain.Code) bool {
	var de *domain.Error
	return errors.As(err, &de) && de.Code == c
}

func TestModuleGate(t *testing.T) {
	e := newEnv(t, modules())
	if _, err := e.svc.PlanAddSecurity(context.Background(), etf); !hasCode(err, app.CodeModuleDisabled) {
		t.Fatalf("investments off: got %v", err)
	}
	e = newEnv(t, modules(module.Investments))
	if _, err := e.svc.PlanAddSecurity(context.Background(), etf); err != nil {
		t.Fatal(err)
	}
	btc := investing.AddSecurityRequest{Name: "Bitcoin", Ticker: "BTC", Kind: "crypto", Currency: "EUR"}
	if _, err := e.svc.PlanAddSecurity(context.Background(), btc); !hasCode(err, app.CodeModuleDisabled) {
		t.Fatalf("crypto off: got %v", err)
	}
	e = newEnv(t, modules(module.Investments, module.Crypto))
	if s := e.addSec(t, btc); s.QuantityScale != 8 {
		t.Fatalf("crypto scale = %d, want 8", s.QuantityScale)
	}
}

func TestAddSecurity(t *testing.T) {
	e := newEnv(t, modules(module.Investments))
	ctx := context.Background()
	s := e.addSec(t, etf)
	if s.ID == 0 || s.Ticker != "VWCE" {
		t.Fatalf("got %+v", s)
	}
	for _, dup := range []investing.AddSecurityRequest{
		{Name: "world etf", Ticker: "X", Kind: "etf", Currency: "EUR"},
		{Name: "Other", Ticker: "VWCE", Kind: "etf", Currency: "EUR"},
	} {
		if _, err := e.svc.PlanAddSecurity(ctx, dup); !hasCode(err, app.CodeAlreadyExists) {
			t.Fatalf("%+v: got %v", dup, err)
		}
	}
	for _, bad := range []investing.AddSecurityRequest{
		{Name: "A", Ticker: "A", Kind: "nope", Currency: "EUR"},
		{Name: "A", Ticker: "A", Kind: "etf", Currency: "??"},
		{Name: "A", Kind: "etf", Currency: "EUR"},
	} {
		if _, err := e.svc.PlanAddSecurity(ctx, bad); err == nil {
			t.Fatalf("%+v: want error", bad)
		}
	}
	// Dry run: planning wrote nothing.
	before := e.store.Snapshot()
	if _, err := e.svc.PlanAddSecurity(ctx, investing.AddSecurityRequest{Name: "B", Ticker: "B", Kind: "stock", Currency: "EUR"}); err != nil {
		t.Fatal(err)
	}
	if e.store.Snapshot() != before {
		t.Fatal("plan wrote to the store")
	}
	list, err := e.svc.ListSecurities(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %v, %v", list, err)
	}
}

func TestSetPrice(t *testing.T) {
	e := newEnv(t, modules(module.Investments))
	ctx := context.Background()
	e.addSec(t, etf)
	p, err := e.svc.PlanSetPrice(ctx, investing.SetPriceRequest{Security: "vwce", Date: "2026-03-01", Price: "100.50"})
	if err != nil || p.Changes[0].Op != app.OpCreate || p.Price.Price.Minor() != 10050 {
		t.Fatalf("plan = %+v, %v", p, err)
	}
	if err := e.svc.ApplySetPrice(ctx, p); err != nil {
		t.Fatal(err)
	}
	p, err = e.svc.PlanSetPrice(ctx, investing.SetPriceRequest{Security: "World ETF", Date: "2026-03-01", Price: "101"})
	if err != nil || p.Changes[0].Op != app.OpUpdate {
		t.Fatalf("second plan = %+v, %v", p, err)
	}
	for _, bad := range []investing.SetPriceRequest{
		{Security: "nope", Price: "1"}, {Security: "VWCE", Price: "x"}, {Security: "VWCE", Price: "-1"}, {Security: "VWCE", Date: "junk", Price: "1"},
	} {
		if _, err := e.svc.PlanSetPrice(ctx, bad); err == nil {
			t.Fatalf("%+v: want error", bad)
		}
	}
}

func TestOpsAndCashLink(t *testing.T) {
	e := newEnv(t, modules(module.Investments))
	ctx := context.Background()
	e.addSec(t, etf)
	buy := investing.OpRequest{Account: "Broker", CashAccount: "Cash", Security: "VWCE", Date: "2026-02-01", Quantity: "10", Price: "100", Fees: "2"}
	p, err := e.svc.PlanBuy(ctx, buy)
	if err != nil || len(p.Changes) != 2 || p.CashTx.Amount.Minor() != -100200 || p.CashTx.Kind != domain.KindInvestment {
		t.Fatalf("plan = %+v, %v", p, err)
	}
	before := e.store.Snapshot()
	if _, err := e.svc.PlanBuy(ctx, buy); err != nil || e.store.Snapshot() != before {
		t.Fatal("plan must not write")
	}
	op, err := e.svc.ApplyOp(ctx, p)
	if err != nil || op.CashTx == 0 {
		t.Fatalf("op = %+v, %v", op, err)
	}

	// Selling more than held is rejected, with and without the cash link.
	if _, err := e.svc.PlanSell(ctx, investing.OpRequest{Account: "Broker", Security: "VWCE", Date: "2026-02-02", Quantity: "11", Price: "100"}); !hasCode(err, domain.CodeInsufficientQuantity) {
		t.Fatalf("oversell: got %v", err)
	}
	// Another account holds nothing.
	if _, err := e.svc.PlanSell(ctx, investing.OpRequest{Account: "Cash", Security: "VWCE", Date: "2026-02-02", Quantity: "1", Price: "100"}); !hasCode(err, domain.CodeInsufficientQuantity) {
		t.Fatalf("other account: got %v", err)
	}
	sell, err := e.svc.PlanSell(ctx, investing.OpRequest{Account: "Broker", CashAccount: "Cash", Security: "VWCE", Date: "2026-03-01", Quantity: "4", Price: "110", Fees: "1"})
	if err != nil || sell.CashTx.Amount.Minor() != 43900 {
		t.Fatalf("sell = %+v, %v", sell, err)
	}
	sellOp, err := e.svc.ApplyOp(ctx, sell)
	if err != nil {
		t.Fatal(err)
	}
	div, err := e.svc.PlanDividend(ctx, investing.OpRequest{Account: "Broker", CashAccount: "Cash", Security: "VWCE", Date: "2026-03-10", Quantity: "6", Price: "0.5"})
	if err != nil || div.CashTx.Amount.Minor() != 300 || div.CashTx.Kind != domain.KindInterest || sell.CashTx.Kind != domain.KindInvestment {
		t.Fatalf("dividend = %+v, %v", div, err)
	}

	// Removing the buy would leave the sell uncovered; removing the sell works and drops its cash tx.
	rm, err := e.svc.PlanRemoveOp(ctx, sellOp.ID)
	if err != nil || len(rm.Changes) != 2 {
		t.Fatalf("remove plan = %+v, %v", rm, err)
	}
	if _, err := e.svc.PlanRemoveOp(ctx, op.ID); !hasCode(err, domain.CodeInsufficientQuantity) {
		t.Fatalf("remove buy: got %v", err)
	}
	if err := e.svc.ApplyRemoveOp(ctx, rm); err != nil {
		t.Fatal(err)
	}
	err = e.store.Read(ctx, func(r app.Repos) error {
		if _, err := r.Transactions().Get(ctx, sellOp.CashTx); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cash tx still there: %v", err)
		}
		if _, err := r.Securities().GetOp(ctx, sellOp.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("op still there: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Removing the buy is now fine.
	if rm, err = e.svc.PlanRemoveOp(ctx, op.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.ApplyRemoveOp(ctx, rm); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.PlanRemoveOp(ctx, op.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("remove twice: got %v", err)
	}
}

func TestOpErrors(t *testing.T) {
	e := newEnv(t, modules(module.Investments))
	ctx := context.Background()
	e.addSec(t, etf)
	ok := investing.OpRequest{Account: "Broker", Security: "VWCE", Date: "2026-02-01", Quantity: "1", Price: "10"}
	for name, mut := range map[string]func(*investing.OpRequest){
		"unknown security": func(r *investing.OpRequest) { r.Security = "x" },
		"unknown account":  func(r *investing.OpRequest) { r.Account = "x" },
		"unknown cash":     func(r *investing.OpRequest) { r.CashAccount = "x" },
		"cash currency":    func(r *investing.OpRequest) { r.CashAccount = "Dollars" },
		"bad quantity":     func(r *investing.OpRequest) { r.Quantity = "1.5" },
		"zero quantity":    func(r *investing.OpRequest) { r.Quantity = "0" },
		"bad price":        func(r *investing.OpRequest) { r.Price = "x" },
		"bad fees":         func(r *investing.OpRequest) { r.Fees = "x" },
		"bad date":         func(r *investing.OpRequest) { r.Date = "x" },
		"bad kind":         func(r *investing.OpRequest) { r.Kind = "x" },
	} {
		req := ok
		req.Kind = domain.OpBuy
		mut(&req)
		if _, err := e.svc.PlanOp(ctx, req); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
	if _, err := e.svc.PlanRemoveOp(ctx, 99); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("remove unknown: got %v", err)
	}
}

func TestRemoveReconciledCashTx(t *testing.T) {
	e := newEnv(t, modules(module.Investments))
	ctx := context.Background()
	e.addSec(t, etf)
	op, err := e.op(t, investing.OpRequest{Kind: domain.OpBuy, Account: "Broker", CashAccount: "Cash", Security: "VWCE", Date: "2026-02-01", Quantity: "1", Price: "10"})
	if err != nil {
		t.Fatal(err)
	}
	err = e.store.WithTx(ctx, func(r app.Repos) error {
		tx, err := r.Transactions().Get(ctx, op.CashTx)
		if err != nil {
			return err
		}
		tx.Status = domain.StatusReconciled
		return r.Transactions().Update(ctx, tx)
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.PlanRemoveOp(ctx, op.ID); !errors.Is(err, domain.ErrReconciled) {
		t.Fatalf("got %v", err)
	}
}

func TestListPositions(t *testing.T) {
	e := newEnv(t, modules(module.Investments, module.Crypto))
	ctx := context.Background()
	e.addSec(t, etf)
	us := e.addSec(t, investing.AddSecurityRequest{Name: "US Stock", Ticker: "USS", Kind: "stock", Currency: "USD"})
	for _, o := range []investing.OpRequest{
		{Kind: domain.OpBuy, Account: "Broker", Security: "VWCE", Date: "2026-01-10", Quantity: "10", Price: "100"},
		{Kind: domain.OpBuy, Account: "Broker", Security: "USS", Date: "2026-01-10", Quantity: "5", Price: "20"},
		{Kind: domain.OpBuy, Account: "Cash", Security: "VWCE", Date: "2026-01-12", Quantity: "1", Price: "90"},
	} {
		if _, err := e.op(t, o); err != nil {
			t.Fatal(err)
		}
	}
	for _, pr := range []investing.SetPriceRequest{
		{Security: "VWCE", Date: "2026-02-01", Price: "120"}, {Security: "USS", Date: "2026-02-01", Price: "30"},
	} {
		p, err := e.svc.PlanSetPrice(ctx, pr)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.svc.ApplySetPrice(ctx, p); err != nil {
			t.Fatal(err)
		}
	}

	lines, err := e.svc.ListPositions(ctx, investing.PositionsRequest{Account: "Broker", Date: "2026-01-31"})
	if err != nil || len(lines) != 2 || lines[0].Position.HasPrice {
		t.Fatalf("before prices: %+v, %v", lines, err)
	}
	if _, err := e.svc.ListPositions(ctx, investing.PositionsRequest{Account: "Broker", Date: "2026-02-15"}); !hasCode(err, app.CodeRateMissing) {
		t.Fatalf("missing rate: got %v", err)
	}
	err = e.store.WithTx(ctx, func(r app.Repos) error {
		return r.FXRates().Upsert(ctx, domain.FXRate{Currency: usd, Date: day("2026-01-01"), Rate: must(domain.NewRate(900_000_000))})
	})
	if err != nil {
		t.Fatal(err)
	}
	lines, err = e.svc.ListPositions(ctx, investing.PositionsRequest{Date: "2026-02-15"})
	if err != nil || len(lines) != 3 {
		t.Fatalf("lines = %+v, %v", lines, err)
	}
	// Broker/VWCE: 10 × 120 EUR; Broker/USS: 5 × 30 USD × 0.9; Cash/VWCE: 1 × 120.
	want := []int64{120000, 13500, 12000}
	for i, l := range lines {
		if l.Value.Minor() != want[i] || l.Value.Currency() != eur {
			t.Errorf("line %d (%s/%s) value = %s, want %d EUR", i, l.Account.Name, l.Security.Name, l.Value, want[i])
		}
	}
	if lines[1].Security.ID != us.ID {
		t.Fatalf("order: %+v", lines)
	}
	if _, err := e.svc.ListPositions(ctx, investing.PositionsRequest{Account: "nope"}); err == nil {
		t.Fatal("unknown account: want error")
	}
	if _, err := e.svc.ListPositions(ctx, investing.PositionsRequest{Date: "x"}); err == nil {
		t.Fatal("bad date: want error")
	}
}

func TestCryptoHiddenWhenModuleOff(t *testing.T) {
	e := newEnv(t, modules(module.Investments, module.Crypto))
	e.addSec(t, investing.AddSecurityRequest{Name: "Bitcoin", Ticker: "BTC", Kind: "crypto", Currency: "EUR"})
	if _, err := e.op(t, investing.OpRequest{Kind: domain.OpBuy, Account: "Broker", Security: "BTC", Quantity: "0.12345678", Price: "50000"}); err != nil {
		t.Fatal(err)
	}
	off := investing.New(e.store, fixedClock{}, investing.Config{BaseCurrency: eur, Location: time.UTC, Modules: modules(module.Investments)})
	lines, err := off.ListPositions(context.Background(), investing.PositionsRequest{})
	if err != nil || len(lines) != 0 {
		t.Fatalf("lines = %+v, %v", lines, err)
	}
	if _, err := off.PlanBuy(context.Background(), investing.OpRequest{Account: "Broker", Security: "BTC", Quantity: "1", Price: "1"}); !hasCode(err, app.CodeModuleDisabled) {
		t.Fatalf("got %v", err)
	}
}

// Σ cash movements = Σ (−buys − fees + sells − fees + income − fees) over the applied operations.
func TestCashMovementsMatchOpsProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		e := newEnv(rt, modules(module.Investments))
		ctx := context.Background()
		e.addSec(rt, etf)
		var want int64
		held := int64(0)
		for i := range rapid.IntRange(1, 12).Draw(rt, "n") {
			kind := rapid.SampledFrom([]domain.SecurityOpKind{domain.OpBuy, domain.OpSell, domain.OpDividend}).Draw(rt, "kind")
			qty := rapid.Int64Range(1, 50).Draw(rt, "qty")
			price := rapid.Int64Range(0, 100_000).Draw(rt, "price")
			fees := rapid.Int64Range(0, 500).Draw(rt, "fees")
			req := investing.OpRequest{
				Kind: kind, Account: "Broker", CashAccount: "Cash", Security: "VWCE",
				Date: day("2026-01-01").String(), Quantity: strconv.FormatInt(qty, 10), Price: minor(price), Fees: minor(fees),
			}
			if kind == domain.OpDividend && held == 0 {
				continue
			}
			_, err := e.op(rt, req)
			switch {
			case kind == domain.OpSell && qty > held:
				if !hasCode(err, domain.CodeInsufficientQuantity) {
					rt.Fatalf("op %d: oversell not rejected: %v", i, err)
				}
				continue
			case err != nil:
				rt.Fatalf("op %d: %v", i, err)
			}
			switch kind {
			case domain.OpBuy:
				held += qty
				want -= qty*price + fees
			case domain.OpSell:
				held -= qty
				want += qty*price - fees
			case domain.OpDividend, domain.OpCoupon:
				want += qty*price - fees
			}
		}
		var got int64
		_ = e.store.Read(ctx, func(r app.Repos) error {
			txs, err := r.Transactions().List(ctx, domain.TxFilter{}, nil, 1000)
			if err != nil {
				rt.Fatal(err)
			}
			for _, tx := range txs {
				got += tx.Amount.Minor()
			}
			return nil
		})
		if got != want {
			rt.Fatalf("Σ cash = %d, want %d", got, want)
		}
	})
}

func minor(n int64) string { return fmt.Sprintf("%d.%02d", n/100, n%100) }
