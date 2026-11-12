package reporting

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// historyMonths is how many complete months feed the variable-spending estimate.
const historyMonths = 3

// AccountProjection is the projected daily balance of one liquidity account, in its currency.
type AccountProjection struct {
	Account        string  `json:"account"`
	Currency       string  `json:"currency"`
	Series         []Point `json:"series"`
	FirstBelowZero string  `json:"first_below_zero,omitempty"`
}

// ForecastResult is the cash-flow forecast: per-account daily balances and their total in the
// base currency.
type ForecastResult struct {
	Start          string              `json:"start"`
	Days           int                 `json:"days"`
	Accounts       []AccountProjection `json:"accounts"`
	Total          []Point             `json:"total"`
	FirstBelowZero string              `json:"first_below_zero,omitempty"`
}

// Forecast projects the balance of every active liquidity account for horizonDays from today
// (Config.HorizonDays when zero). It starts from today's balances and adds future-dated
// transactions, the occurrences of recurrences not yet generated, the installments of loans
// (when the loans module is on) that have no transaction yet, and the estimated variable
// spending: the cap budgets (when the budget module is on) and, for what they do not cover, the
// average of the last three complete months. Spending is spread evenly over each month.
func (a *Analytics) Forecast(ctx context.Context, horizonDays int) (ForecastResult, error) {
	if err := a.gate(); err != nil {
		return ForecastResult{}, err
	}
	if horizonDays == 0 {
		horizonDays = a.cfg.HorizonDays
	}
	start, err := a.today()
	if err != nil {
		return ForecastResult{}, err
	}
	end, err := start.AddDays(horizonDays)
	if err != nil {
		return ForecastResult{}, fmt.Errorf("forecast horizon: %w", err)
	}
	var out ForecastResult
	err = a.store.Read(ctx, func(r app.Repos) error {
		all, err := r.Accounts().List(ctx)
		if err != nil {
			return fmt.Errorf("list accounts: %w", err)
		}
		accts := slices.DeleteFunc(all, func(ac domain.Account) bool { return ac.Archived || !ac.Kind.IsLiquidity() })
		openings := make([]domain.ForecastOpening, len(accts))
		for i, ac := range accts {
			bal, err := balanceAt(ctx, r, ac, start)
			if err != nil {
				return err
			}
			openings[i] = domain.ForecastOpening{AccountID: ac.ID, Balance: bal}
		}
		flows, err := a.flows(ctx, r, accts, start, end)
		if err != nil {
			return err
		}
		proj, err := domain.Forecast(start, horizonDays, openings, flows, nil)
		if err != nil {
			return fmt.Errorf("forecast: %w", err)
		}
		out, err = a.assemble(ctx, r, accts, proj, start, horizonDays)
		return err
	})
	return out, err
}

func (a *Analytics) assemble(ctx context.Context, r app.Repos, accts []domain.Account, proj []domain.AccountForecast, start domain.Date, days int) (ForecastResult, error) {
	out := ForecastResult{Start: start.String(), Days: days, Accounts: []AccountProjection{}, Total: []Point{}}
	totals := make([]domain.Money, days+1)
	for i := range totals {
		totals[i] = zeroMoney(a.cfg.BaseCurrency)
	}
	byID := make(map[domain.AccountID]domain.Account, len(accts))
	for _, ac := range accts {
		byID[ac.ID] = ac
	}
	for _, pr := range proj {
		ac := byID[pr.AccountID]
		ap := AccountProjection{Account: ac.Name, Currency: ac.Currency.Code(), Series: make([]Point, len(pr.Series))}
		if pr.HasFirstBelowZero {
			ap.FirstBelowZero = pr.FirstBelowZero.String()
		}
		for i, pt := range pr.Series {
			ap.Series[i] = Point{Date: pt.Date.String(), Amount: amountOf(pt.Balance)}
			// The total uses the rate in force on the start date for the whole horizon.
			base, err := a.toBase(ctx, r, pt.Balance, start)
			if err != nil {
				return ForecastResult{}, err
			}
			if totals[i], err = totals[i].Add(base); err != nil {
				return ForecastResult{}, err
			}
		}
		out.Accounts = append(out.Accounts, ap)
	}
	for i, t := range totals {
		d, err := start.AddDays(i)
		if err != nil {
			return ForecastResult{}, err
		}
		out.Total = append(out.Total, Point{Date: d.String(), Amount: amountOf(t)})
		if out.FirstBelowZero == "" && t.Sign() < 0 {
			out.FirstBelowZero = d.String()
		}
	}
	return out, nil
}

// flows gathers every projected movement of the liquidity accounts in (start, end].
func (a *Analytics) flows(ctx context.Context, r app.Repos, accts []domain.Account, start, end domain.Date) ([]domain.ForecastFlow, error) {
	liquid := make(map[domain.AccountID]domain.Account, len(accts))
	for _, ac := range accts {
		liquid[ac.ID] = ac
	}
	first, err := start.AddDays(1)
	if err != nil {
		return nil, err
	}
	var out []domain.ForecastFlow
	// Future-dated transactions (pending installments and recurrences included).
	f, err := domain.NewTxFilter(domain.TxFilterSpec{From: first, To: end})
	if err != nil {
		return nil, err
	}
	txs, err := r.Transactions().List(ctx, f, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("list future transactions: %w", err)
	}
	for _, t := range txs {
		if _, ok := liquid[t.AccountID]; ok {
			out = append(out, domain.ForecastFlow{AccountID: t.AccountID, Date: t.Date, Amount: t.Amount})
		}
	}
	recs, err := r.Recurrences().List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list recurrences: %w", err)
	}
	for _, rec := range recs {
		if _, ok := liquid[rec.AccountID]; !ok {
			continue
		}
		dr := domain.Recurrence{
			AccountID: rec.AccountID, CategoryID: rec.CategoryID, Description: rec.Description, Amount: rec.Amount,
			Rule: rec.Rule, Start: rec.Start, End: rec.End, NextDate: rec.NextDate,
		}
		for _, d := range domain.Expand(dr, first, end) {
			out = append(out, domain.ForecastFlow{AccountID: rec.AccountID, Date: d, Amount: rec.Amount})
		}
	}
	if a.cfg.Modules.Has(module.Loans) {
		inst, err := a.installmentFlows(ctx, r, liquid, first, end)
		if err != nil {
			return nil, err
		}
		out = append(out, inst...)
	}
	est, err := a.variableFlows(ctx, r, accts, start, end)
	if err != nil {
		return nil, err
	}
	return append(out, est...), nil
}

// installmentFlows are the installments due in [from, to] that no transaction covers yet.
func (a *Analytics) installmentFlows(ctx context.Context, r app.Repos, liquid map[domain.AccountID]domain.Account, from, to domain.Date) ([]domain.ForecastFlow, error) {
	loans, err := r.Loans().List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list loans: %w", err)
	}
	var out []domain.ForecastFlow
	for _, loan := range loans {
		if _, ok := liquid[loan.DebitAccount]; !ok {
			continue
		}
		starts, err := versionStarts(ctx, r, loan)
		if err != nil {
			return nil, err
		}
		insts, err := scheduledInstallments(ctx, r, loan)
		if err != nil {
			return nil, err
		}
		for _, inst := range insts {
			if inst.Due.Before(from) || inst.Due.After(to) {
				continue
			}
			if linked, err := installmentLinked(ctx, r, loan.ID, starts, inst.Due); err != nil || linked {
				if err != nil {
					return nil, err
				}
				continue
			}
			total, err := inst.Total()
			if err != nil {
				return nil, err
			}
			neg, err := total.Neg()
			if err != nil {
				return nil, err
			}
			out = append(out, domain.ForecastFlow{AccountID: loan.DebitAccount, Date: inst.Due, Amount: neg})
		}
	}
	return out, nil
}

func installmentLinked(ctx context.Context, r app.Repos, id domain.LoanID, starts []domain.Date, due domain.Date) (bool, error) {
	for _, from := range starts {
		_, err := r.Loans().TransactionOfInstallment(ctx, id, from, due)
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, domain.ErrNotFound) {
			return false, fmt.Errorf("transaction of installment %s: %w", due, err)
		}
	}
	return false, nil
}

// spendKey is the history bucket of the variable estimate.
type spendKey struct {
	account  domain.AccountID
	category domain.CategoryID
}

// variableFlows estimates the spending that is neither recurring nor an installment nor a
// transfer: budgeted categories follow their cap (attributed to the account that spent most on
// them, in a base-currency account only); the rest follows the history average per account.
func (a *Analytics) variableFlows(ctx context.Context, r app.Repos, accts []domain.Account, start, end domain.Date) ([]domain.ForecastFlow, error) {
	if len(accts) == 0 {
		return nil, nil
	}
	hist, err := a.history(ctx, r, accts, start)
	if err != nil {
		return nil, err
	}
	out, budgeted, err := a.capFlows(ctx, r, accts, hist, start, end)
	if err != nil {
		return nil, err
	}
	perAccount := map[domain.AccountID]int64{}
	for k, minor := range hist {
		if !budgeted[k.category] {
			perAccount[k.account] += minor
		}
	}
	for _, ac := range accts {
		monthly := perAccount[ac.ID] / historyMonths
		if monthly <= 0 {
			continue
		}
		out, err = appendSpread(out, start, end, ac, func(domain.MonthKey) int64 { return monthly }, ac.Currency)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// capFlows spreads the cap budgets and returns the categories they cover.
func (a *Analytics) capFlows(ctx context.Context, r app.Repos, accts []domain.Account, hist map[spendKey]int64, start, end domain.Date) ([]domain.ForecastFlow, map[domain.CategoryID]bool, error) {
	budgeted := map[domain.CategoryID]bool{}
	var out []domain.ForecastFlow
	if !a.cfg.Modules.Has(module.Budget) {
		return out, budgeted, nil
	}
	budgets, err := r.Budgets().List(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list budgets: %w", err)
	}
	for _, b := range budgets {
		if b.Mode != domain.BudgetCap {
			continue
		}
		owner, ok := capOwner(b, hist, accts, a.cfg.BaseCurrency)
		if !ok {
			continue
		}
		for _, t := range b.Targets {
			if t.CategoryID != 0 {
				budgeted[t.CategoryID] = true
			}
		}
		out, err = appendSpread(out, start, end, owner, func(k domain.MonthKey) int64 { return b.AmountFor(k).Minor() }, b.Amount.Currency())
		if err != nil {
			return nil, nil, err
		}
	}
	return out, budgeted, nil
}

// capOwner picks the account a cap's spending is attributed to: the liquidity account with the
// largest history in the cap's categories, else the first account in the base currency.
func capOwner(b domain.Budget, hist map[spendKey]int64, accts []domain.Account, base domain.Currency) (domain.Account, bool) {
	var best domain.Account
	var bestSum int64
	for _, ac := range accts {
		if ac.Currency != base {
			continue
		}
		var sum int64
		for _, t := range b.Targets {
			sum += hist[spendKey{ac.ID, t.CategoryID}]
		}
		if best.ID == 0 || sum > bestSum {
			best, bestSum = ac, sum
		}
	}
	return best, best.ID != 0
}

// history sums, per account and category, the expenses of the last complete months that are
// neither recurring, transfers, exchanges, investments nor loan installments, as positive minor
// units in the account currency. Splits count in their own category.
func (a *Analytics) history(ctx context.Context, r app.Repos, accts []domain.Account, start domain.Date) (map[spendKey]int64, error) {
	cur, err := domain.MonthPeriod(start, a.cfg.MonthStartDay)
	if err != nil {
		return nil, err
	}
	from, err := cur.From().AddMonths(-historyMonths)
	if err != nil {
		return nil, err
	}
	to, err := cur.From().AddDays(-1)
	if err != nil {
		return nil, err
	}
	f, err := domain.NewTxFilter(domain.TxFilterSpec{From: from, To: to, AccountIDs: accountIDs(accts), Sign: domain.TxSignExpense})
	if err != nil {
		return nil, err
	}
	txs, err := r.Transactions().List(ctx, f, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("list history: %w", err)
	}
	out := map[spendKey]int64{}
	for _, t := range txs {
		switch {
		case t.RecurrenceID != 0, t.Status == domain.StatusPending,
			t.Kind == domain.KindTransfer, t.Kind == domain.KindExchange, t.Kind == domain.KindInvestment:
			continue
		}
		if _, err := r.Loans().InstallmentOfTransaction(ctx, t.ID); err == nil {
			continue
		} else if !errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("installment of transaction %d: %w", t.ID, err)
		}
		splits, err := r.Splits().SplitsOf(ctx, t.ID)
		if err != nil {
			return nil, fmt.Errorf("splits of transaction %d: %w", t.ID, err)
		}
		parts := []domain.Split{{Category: t.CategoryID, Amount: t.Amount}}
		if len(splits) > 0 {
			parts = splits
		}
		for _, sp := range parts {
			out[spendKey{t.AccountID, sp.Category}] -= sp.Amount.Minor()
		}
	}
	return out, nil
}

func accountIDs(accts []domain.Account) []domain.AccountID {
	ids := make([]domain.AccountID, len(accts))
	for i, ac := range accts {
		ids[i] = ac.ID
	}
	return ids
}

// appendSpread adds, for every day in (start, end], the monthly spending of its month as an
// outflow of acct, split evenly with the remainder on the month's last day so the month sums
// exactly. A zero monthly amount adds nothing.
func appendSpread(out []domain.ForecastFlow, start, end domain.Date, acct domain.Account, monthly func(domain.MonthKey) int64, cur domain.Currency) ([]domain.ForecastFlow, error) {
	if cur != acct.Currency {
		return out, nil
	}
	day := start
	for day.Before(end) {
		var err error
		if day, err = day.AddDays(1); err != nil {
			return nil, err
		}
		total := monthly(domain.MonthKey{Year: day.Year(), Month: day.Month()})
		dim := time.Date(day.Year(), time.Month(day.Month())+1, 0, 0, 0, 0, 0, time.UTC).Day()
		share := total / int64(dim)
		if day.Day() == dim {
			share = total - share*int64(dim-1)
		}
		if share == 0 {
			continue
		}
		m, err := domain.NewMoney(-share, acct.Currency)
		if err != nil {
			return nil, err
		}
		out = append(out, domain.ForecastFlow{AccountID: acct.ID, Date: day, Amount: m})
	}
	return out, nil
}
