package reporting_test

import (
	"context"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/lending"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/clock"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// seedWealth adds a priced ETF position (10 units at 50.00), a house valued 200000.00 and an
// asset-linked mortgage; the ledger is otherwise the default fixture.
func seedWealth(t *testing.T, f *fx) {
	t.Helper()
	ctx := context.Background()
	ls := lending.New(f.store, clock.NewFixed(date("2026-03-15").In(time.UTC)), noBackup{}, lending.Config{Location: time.UTC})
	plan, err := ls.PlanAdd(ctx, f.cfg.Modules, lending.AddRequest{
		Name: "Mortgage", Kind: "mortgage", Principal: "12000", Disbursed: "2026-01-01", TermMonths: 12, RateType: "fixed", Rate: "3,5", Debit: "Main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ls.ApplyAdd(ctx, plan); err != nil {
		t.Fatal(err)
	}
	err = f.store.WithTx(ctx, func(r app.Repos) error {
		loans, err := r.Loans().List(ctx)
		if err != nil {
			return err
		}
		house, err := r.Assets().Insert(ctx, domain.Asset{
			Name: "House", Kind: domain.AssetKindProperty, PurchaseDate: date("2026-01-01"), PurchaseValue: money(15_000_00), LoanID: loans[0].ID,
		})
		if err != nil {
			return err
		}
		if err := r.Assets().SetValuation(ctx, domain.Valuation{AssetID: house.ID, Date: date("2026-02-01"), Value: money(20_000_00)}); err != nil {
			return err
		}
		sec, err := r.Securities().Insert(ctx, domain.Security{Name: "World ETF", Ticker: "VWCE", Kind: domain.SecurityETF, Currency: eur(), QuantityScale: 2})
		if err != nil {
			return err
		}
		qty := must(domain.NewQuantity(1000, 2))
		if _, err := r.Securities().InsertOp(ctx, domain.SecurityOp{
			Date: date("2026-01-10"), Account: f.main.ID, Security: sec.ID, Kind: domain.OpBuy, Quantity: qty, UnitPrice: money(4_000), Fees: money(0),
		}); err != nil {
			return err
		}
		return r.Securities().UpsertPrice(ctx, domain.SecurityPrice{Security: sec.ID, Date: date("2026-02-01"), Price: money(5_000)})
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestNetWorthReportGolden(t *testing.T) {
	f := newFx(t, module.Loans, module.Investments, module.Assets)
	seedWealth(t, f)
	got, err := f.svc.NetWorthReport(context.Background(), reporting.ReportRequest{
		Period: must(domain.NewPeriod(date("2026-01-01"), date("2026-03-31"))),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "net_worth_report", got)
}

func TestNetWorthReportModuleOffRemovesComponent(t *testing.T) {
	tests := []struct {
		name                       string
		mods                       []module.ID
		positions, assets, debtsOn bool
	}{
		{"all off", nil, false, false, false},
		{"investments", []module.ID{module.Investments}, true, false, false},
		{"assets", []module.ID{module.Assets}, false, true, false},
		{"loans", []module.ID{module.Loans}, false, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFx(t, tt.mods...)
			p, err := f.svc.NetWorthAt(context.Background(), date("2026-03-01"))
			if err != nil {
				t.Fatal(err)
			}
			if (p.Positions != nil) != tt.positions || (p.Assets != nil) != tt.assets || (p.Debts != nil) != tt.debtsOn {
				t.Fatalf("components = %+v", p)
			}
			if p.Total.Minor != 160_000 {
				t.Fatalf("total = %d, want 160000", p.Total.Minor)
			}
		})
	}
}

func TestNetWorthReportTotalsAndGates(t *testing.T) {
	f := newFx(t, module.Loans, module.Investments, module.Assets)
	seedWealth(t, f)
	ctx := context.Background()
	p, err := f.svc.NetWorthAt(ctx, date("2026-03-01"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Positions.Minor != 500_00 || p.Assets.Minor != 20_000_00 || p.Debts.Minor >= 0 {
		t.Fatalf("point = %+v %+v %+v", p.Positions, p.Assets, p.Debts)
	}
	if want := p.Liquidity.Minor + p.Positions.Minor + p.Assets.Minor + p.Debts.Minor; p.Total.Minor != want {
		t.Fatalf("total = %d, want %d", p.Total.Minor, want)
	}
	if _, err := f.svc.NetWorthAt(ctx, domain.Date{}); err != nil {
		t.Fatal(err)
	}
	off := reporting.NewAnalytics(f.store, clock.NewFixed(date("2026-03-15").In(time.UTC)), memSource{f.store}, reporting.Config{BaseCurrency: eur(), Location: time.UTC})
	if _, err := off.NetWorthReport(ctx, reporting.ReportRequest{}); err == nil {
		t.Fatal("analytics off: want module error")
	}
}

func TestNetWorthReportUnchangedByFeelessTransfer(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		f := newFx(t, module.Loans, module.Investments, module.Assets)
		seedWealth(t, f)
		amount := rapid.Int64Range(1, 90_000).Draw(rt, "amount")
		at := time.Date(2026, 3, rapid.IntRange(1, 28).Draw(rt, "day"), 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		end := date("2026-03-31")
		before, err := f.svc.NetWorthAt(context.Background(), end)
		if err != nil {
			rt.Fatal(err)
		}
		f.tx(t, f.main, at, -amount, func(x *domain.Transaction) { x.Kind = domain.KindTransfer })
		f.tx(t, f.sav, at, amount, func(x *domain.Transaction) { x.Kind = domain.KindTransfer })
		after, err := f.svc.NetWorthAt(context.Background(), end)
		if err != nil {
			rt.Fatal(err)
		}
		if before.Total != after.Total {
			rt.Fatalf("net worth %v -> %v", before.Total, after.Total)
		}
	})
}

func TestRunwayIncludeInvestments(t *testing.T) {
	f := newFx(t, module.Loans, module.Investments, module.Assets)
	seedWealth(t, f)
	seedHistory(t, f)
	clk := clock.NewFixed(date("2026-03-15").In(time.UTC))
	uc, err := reporting.NewRunway(balanceSource{memSource{f.store}}, clk)
	if err != nil {
		t.Fatal(err)
	}
	params := reporting.RunwayParams{AsOf: date("2026-03-15"), MonthStartDay: 1, BaseCurrency: eur()}
	base, err := uc.Run(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	params.IncludeInvestments = true
	if _, err := uc.Run(context.Background(), params); err == nil {
		t.Fatal("want error without positions valuer")
	}
	got, err := uc.WithPositions(f.svc).Run(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if d := got.Liquidity.Minor() - base.Liquidity.Minor(); d != 500_00 {
		t.Fatalf("liquidity delta = %d, want 50000", d)
	}
}

// balanceSource computes balances from the memstore, as the SQLite source does.
type balanceSource struct{ memSource }

func (b balanceSource) BalanceAt(ctx context.Context, id domain.AccountID, at domain.Date) (domain.Money, error) {
	var out domain.Money
	err := b.read(func(r app.Repos) error {
		acct, err := r.Accounts().Get(ctx, id)
		if err != nil {
			return err
		}
		out, err = balanceAt(ctx, r, acct, at)
		return err
	})
	return out, err
}

func balanceAt(ctx context.Context, r app.Repos, acct domain.Account, at domain.Date) (domain.Money, error) {
	sum, err := r.Transactions().SumUpTo(ctx, acct.ID, at)
	if err != nil {
		return domain.Money{}, err
	}
	return acct.OpeningBalance.Add(money(sum))
}
