package reporting

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/assetting"
	"github.com/0xballo/fui/internal/app/investing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// NetWorthPoint is the net worth at one date, broken down by component. A component of a
// disabled module is nil (absent), not a zero amount. Debts is negative.
type NetWorthPoint struct {
	Date      string  `json:"date"`
	Liquidity Amount  `json:"liquidity"`
	Positions *Amount `json:"positions,omitempty"`
	Assets    *Amount `json:"assets,omitempty"`
	Debts     *Amount `json:"debts,omitempty"`
	Total     Amount  `json:"total"`
}

// NetWorthResult is the monthly net-worth series in the base currency.
type NetWorthResult struct {
	Period string          `json:"period"`
	Points []NetWorthPoint `json:"points"`
}

// NetWorthReport composes the month-end net worth from liquidity, security positions
// (investments), latest asset valuations (assets) and loan debts (loans), each component
// present only when its module is enabled. Liquidity is every non-loan account, archived
// included; loan liability accounts count as debts, never as liquidity.
func (a *Analytics) NetWorthReport(ctx context.Context, req ReportRequest) (NetWorthResult, error) {
	if err := a.gate(); err != nil {
		return NetWorthResult{}, err
	}
	p, err := a.period(req.Period)
	if err != nil {
		return NetWorthResult{}, err
	}
	ends, err := a.monthEnds(p)
	if err != nil {
		return NetWorthResult{}, err
	}
	out := NetWorthResult{Period: periodLabel(p), Points: make([]NetWorthPoint, 0, len(ends))}
	for _, end := range ends {
		point, err := a.NetWorthAt(ctx, end)
		if err != nil {
			return NetWorthResult{}, err
		}
		out.Points = append(out.Points, point)
	}
	return out, nil
}

// NetWorthAt composes the net worth at date; a zero date means today.
func (a *Analytics) NetWorthAt(ctx context.Context, at domain.Date) (NetWorthPoint, error) {
	if err := a.gate(); err != nil {
		return NetWorthPoint{}, err
	}
	if at.IsZero() {
		today, err := a.today()
		if err != nil {
			return NetWorthPoint{}, err
		}
		at = today
	}
	mods := domain.NetWorthModules{
		Investments: a.cfg.Modules.Has(module.Investments),
		Assets:      a.cfg.Modules.Has(module.Assets),
		Loans:       a.cfg.Modules.Has(module.Loans),
	}
	in := domain.NetWorthInput{AsOf: at, Modules: mods, Positions: zeroMoney(a.cfg.BaseCurrency)}
	err := a.store.Read(ctx, func(r app.Repos) error {
		var err error
		in.Liquidity, in.LoanDebts, err = a.liquidityAndDebts(ctx, r, at, mods.Loans)
		return err
	})
	if err != nil {
		return NetWorthPoint{}, err
	}
	if mods.Investments {
		if in.Positions, err = a.PositionsValue(ctx, at); err != nil {
			return NetWorthPoint{}, err
		}
	}
	if mods.Assets {
		if in.Assets, err = a.assetInputs(ctx, at); err != nil {
			return NetWorthPoint{}, err
		}
	}
	res, err := domain.NetWorth(in)
	if err != nil {
		return NetWorthPoint{}, fmt.Errorf("compose net worth: %w", err)
	}
	point := NetWorthPoint{Date: at.String(), Liquidity: amountOf(res.Liquidity), Total: amountOf(res.Total)}
	if mods.Investments {
		v := amountOf(res.Positions)
		point.Positions = &v
	}
	if mods.Assets {
		v := amountOf(res.Assets)
		point.Assets = &v
	}
	if mods.Loans {
		v := amountOf(res.Debts)
		point.Debts = &v
	}
	return point, nil
}

// PositionsValue is the market value of every priced security position at date, in the base
// currency; it implements PositionsValuer.
func (a *Analytics) PositionsValue(ctx context.Context, at domain.Date) (domain.Money, error) {
	svc := investing.New(a.store, a.clock, investing.Config{
		Locale: a.cfg.Locale, BaseCurrency: a.cfg.BaseCurrency, Location: a.cfg.Location, Modules: a.cfg.Modules,
	})
	lines, err := svc.ListPositions(ctx, investing.PositionsRequest{Date: at.String()})
	if err != nil {
		return domain.Money{}, fmt.Errorf("list positions: %w", err)
	}
	total := zeroMoney(a.cfg.BaseCurrency)
	for _, line := range lines {
		if !line.Position.HasPrice {
			continue
		}
		if total, err = total.Add(line.Value); err != nil {
			return domain.Money{}, err
		}
	}
	return total, nil
}

func (a *Analytics) liquidityAndDebts(ctx context.Context, r app.Repos, at domain.Date, loans bool) (domain.Money, map[domain.LoanID]domain.Money, error) {
	accts, err := r.Accounts().List(ctx)
	if err != nil {
		return domain.Money{}, nil, fmt.Errorf("list accounts: %w", err)
	}
	liquid := make([]domain.Account, 0, len(accts))
	for _, acct := range accts {
		if acct.Kind != domain.AccountLoan {
			liquid = append(liquid, acct)
		}
	}
	liquidity, err := a.sumBase(ctx, r, liquid, at)
	if err != nil {
		return domain.Money{}, nil, err
	}
	if !loans {
		return liquidity, nil, nil
	}
	debts, err := a.loanDebts(ctx, r, at)
	return liquidity, debts, err
}

// loanDebts is the residual debt (never negative) of every loan at date, in the base currency.
func (a *Analytics) loanDebts(ctx context.Context, r app.Repos, at domain.Date) (map[domain.LoanID]domain.Money, error) {
	loans, err := r.Loans().List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list loans: %w", err)
	}
	out := make(map[domain.LoanID]domain.Money, len(loans))
	for _, loan := range loans {
		acct, err := r.Accounts().Get(ctx, loan.LiabilityAccountID)
		if err != nil {
			return nil, fmt.Errorf("liability account of %q: %w", loan.Name, err)
		}
		bal, err := balanceAt(ctx, r, acct, at)
		if err != nil {
			return nil, err
		}
		debt, err := bal.Neg()
		if err != nil {
			return nil, err
		}
		if debt.Sign() < 0 {
			debt = zeroMoney(acct.Currency)
		}
		if out[loan.ID], err = a.toBase(ctx, r, debt, at); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// assetInputs lists the assets owned at date with their latest valuation converted to the
// base currency, which domain.NetWorth requires.
func (a *Analytics) assetInputs(ctx context.Context, at domain.Date) ([]domain.NetWorthAssetInput, error) {
	svc := assetting.New(a.store, a.clock, nil, assetting.Config{
		Locale: a.cfg.Locale, Location: a.cfg.Location, BaseCurrency: a.cfg.BaseCurrency,
	})
	items, err := svc.List(ctx, a.cfg.Modules, at.String())
	if err != nil {
		return nil, fmt.Errorf("list assets: %w", err)
	}
	var out []domain.NetWorthAssetInput
	err = a.store.Read(ctx, func(r app.Repos) error {
		for _, it := range items {
			if it.Asset.PurchaseDate.After(at) {
				continue
			}
			value, err := a.toBase(ctx, r, it.Latest.Value, at)
			if err != nil {
				return err
			}
			asset := it.Asset
			asset.PurchaseValue = value
			out = append(out, domain.NetWorthAssetInput{
				Asset:      asset,
				Valuations: []domain.Valuation{{AssetID: asset.ID, Date: it.Latest.Date, Value: value}},
			})
		}
		return nil
	})
	return out, err
}
