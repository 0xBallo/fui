package reporting

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/budgeting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// ReportRequest carries the parameters of the analytics reports; each report uses the ones it
// needs. A zero Period means the current accounting month.
type ReportRequest struct {
	Period  domain.Period
	Account string // balance-over-time: account name (archived accounts included)
	Loan    string // remaining-debt: loan name; interest-paid: optional filter
	Limit   int    // top-payees: number of rows, default 10
}

func (a *Analytics) period(p domain.Period) (domain.Period, error) {
	if !p.From().IsZero() && !p.To().IsZero() {
		return p, nil
	}
	today, err := a.today()
	if err != nil {
		return domain.Period{}, err
	}
	return domain.MonthPeriod(today, a.cfg.MonthStartDay)
}

// SeriesResult is a monthly series in the base currency (or the account currency for a
// single-account series).
type SeriesResult struct {
	Period string  `json:"period"`
	Points []Point `json:"points"`
}

func periodLabel(p domain.Period) string { return p.From().String() + ".." + p.To().String() }

// BalanceOverTime reports the month-end balance of one account over the period.
func (a *Analytics) BalanceOverTime(ctx context.Context, req ReportRequest) (SeriesResult, error) {
	if err := a.gate(); err != nil {
		return SeriesResult{}, err
	}
	p, err := a.period(req.Period)
	if err != nil {
		return SeriesResult{}, err
	}
	var out SeriesResult
	err = a.store.Read(ctx, func(r app.Repos) error {
		accts, err := r.Accounts().List(ctx)
		if err != nil {
			return fmt.Errorf("list accounts: %w", err)
		}
		acct, err := pickAccount(accts, req.Account)
		if err != nil {
			return err
		}
		out, err = a.series(p, func(at domain.Date) (domain.Money, error) {
			return balanceAt(ctx, r, acct, at)
		})
		return err
	})
	return out, err
}

func (a *Analytics) series(p domain.Period, at func(domain.Date) (domain.Money, error)) (SeriesResult, error) {
	ends, err := a.monthEnds(p)
	if err != nil {
		return SeriesResult{}, err
	}
	out := SeriesResult{Period: periodLabel(p), Points: make([]Point, 0, len(ends))}
	for _, end := range ends {
		m, err := at(end)
		if err != nil {
			return SeriesResult{}, err
		}
		out.Points = append(out.Points, Point{Date: end.String(), Amount: amountOf(m)})
	}
	return out, nil
}

// NetWorth reports the month-end sum of every account, archived ones and liabilities included,
// converted to the base currency at the rate of each month end.
func (a *Analytics) NetWorth(ctx context.Context, req ReportRequest) (SeriesResult, error) {
	if err := a.gate(); err != nil {
		return SeriesResult{}, err
	}
	p, err := a.period(req.Period)
	if err != nil {
		return SeriesResult{}, err
	}
	var out SeriesResult
	err = a.store.Read(ctx, func(r app.Repos) error {
		accts, err := r.Accounts().List(ctx)
		if err != nil {
			return fmt.Errorf("list accounts: %w", err)
		}
		out, err = a.series(p, func(at domain.Date) (domain.Money, error) {
			return a.sumBase(ctx, r, accts, at)
		})
		return err
	})
	return out, err
}

// sumBase is the sum of the balances of accts at date, in the base currency.
func (a *Analytics) sumBase(ctx context.Context, r app.Repos, accts []domain.Account, at domain.Date) (domain.Money, error) {
	total := zeroMoney(a.cfg.BaseCurrency)
	for _, acct := range accts {
		bal, err := balanceAt(ctx, r, acct, at)
		if err != nil {
			return domain.Money{}, err
		}
		if bal, err = a.toBase(ctx, r, bal, at); err != nil {
			return domain.Money{}, err
		}
		if total, err = total.Add(bal); err != nil {
			return domain.Money{}, err
		}
	}
	return total, nil
}

// DebtResult is the remaining debt of one loan over time.
type DebtResult struct {
	Loan string `json:"loan"`
	SeriesResult
}

// RemainingDebt reports the month-end remaining debt of a loan: minus the balance of its
// liability account.
func (a *Analytics) RemainingDebt(ctx context.Context, req ReportRequest) (DebtResult, error) {
	if err := a.gate(module.Loans); err != nil {
		return DebtResult{}, err
	}
	p, err := a.period(req.Period)
	if err != nil {
		return DebtResult{}, err
	}
	var out DebtResult
	err = a.store.Read(ctx, func(r app.Repos) error {
		loan, acct, err := pickLoan(ctx, r, req.Loan)
		if err != nil {
			return err
		}
		out.Loan = loan.Name
		out.SeriesResult, err = a.series(p, func(at domain.Date) (domain.Money, error) {
			bal, err := balanceAt(ctx, r, acct, at)
			if err != nil {
				return domain.Money{}, err
			}
			return bal.Neg()
		})
		return err
	})
	return out, err
}

func pickLoan(ctx context.Context, r app.Repos, ref string) (domain.Loan, domain.Account, error) {
	loans, err := r.Loans().List(ctx)
	if err != nil {
		return domain.Loan{}, domain.Account{}, fmt.Errorf("list loans: %w", err)
	}
	names := make([]string, len(loans))
	for i, l := range loans {
		names[i] = l.Name
	}
	i, err := domain.Resolve(ref, names)
	if err != nil {
		return domain.Loan{}, domain.Account{}, fmt.Errorf("loan: %w", err)
	}
	acct, err := r.Accounts().Get(ctx, loans[i].LiabilityAccountID)
	if err != nil {
		return domain.Loan{}, domain.Account{}, fmt.Errorf("liability account of %q: %w", loans[i].Name, err)
	}
	return loans[i], acct, nil
}

// InterestRow is the interest and fees of installments due in one year of one loan.
type InterestRow struct {
	Loan     string `json:"loan"`
	Year     int    `json:"year"`
	Interest Amount `json:"interest"`
	Fees     Amount `json:"fees"`
}

// InterestResult is the interest-paid report.
type InterestResult struct {
	Period string        `json:"period"`
	Rows   []InterestRow `json:"rows"`
}

// InterestPaid reports, per loan and calendar year, the interest and fees of the scheduled
// installments due within the period and not after today. It reads each schedule version only
// from its own validity start, so installments replaced by a later version are not counted.
func (a *Analytics) InterestPaid(ctx context.Context, req ReportRequest) (InterestResult, error) {
	if err := a.gate(module.Loans); err != nil {
		return InterestResult{}, err
	}
	p, err := a.period(req.Period)
	if err != nil {
		return InterestResult{}, err
	}
	today, err := a.today()
	if err != nil {
		return InterestResult{}, err
	}
	out := InterestResult{Period: periodLabel(p), Rows: []InterestRow{}}
	err = a.store.Read(ctx, func(r app.Repos) error {
		loans, err := r.Loans().List(ctx)
		if err != nil {
			return fmt.Errorf("list loans: %w", err)
		}
		if req.Loan != "" {
			loan, _, err := pickLoan(ctx, r, req.Loan)
			if err != nil {
				return err
			}
			loans = []domain.Loan{loan}
		}
		for _, loan := range loans {
			rows, err := a.interestRows(ctx, r, loan, p, today)
			if err != nil {
				return err
			}
			out.Rows = append(out.Rows, rows...)
		}
		return nil
	})
	return out, err
}

func (a *Analytics) interestRows(ctx context.Context, r app.Repos, loan domain.Loan, p domain.Period, today domain.Date) ([]InterestRow, error) {
	insts, err := scheduledInstallments(ctx, r, loan)
	if err != nil {
		return nil, err
	}
	cur := loan.Principal.Currency()
	byYear := map[int][2]domain.Money{}
	for _, inst := range insts {
		if !p.Contains(inst.Due) || inst.Due.After(today) {
			continue
		}
		y := byYear[inst.Due.Year()]
		if y[0].Currency().IsZero() {
			y = [2]domain.Money{zeroMoney(cur), zeroMoney(cur)}
		}
		if y[0], err = y[0].Add(inst.Interest); err != nil {
			return nil, err
		}
		if y[1], err = y[1].Add(inst.Fees); err != nil {
			return nil, err
		}
		byYear[inst.Due.Year()] = y
	}
	years := make([]int, 0, len(byYear))
	for y := range byYear {
		years = append(years, y)
	}
	slices.Sort(years)
	out := make([]InterestRow, 0, len(years))
	for _, y := range years {
		out = append(out, InterestRow{Loan: loan.Name, Year: y, Interest: amountOf(byYear[y][0]), Fees: amountOf(byYear[y][1])})
	}
	return out, nil
}

// scheduledInstallments returns the installments in force, one version after the other: each
// version contributes the dues from its ValidFrom up to the next version's.
func scheduledInstallments(ctx context.Context, r app.Repos, loan domain.Loan) ([]domain.Installment, error) {
	starts, err := versionStarts(ctx, r, loan)
	if err != nil {
		return nil, err
	}
	var out []domain.Installment
	for i, from := range starts {
		sched, err := r.Loans().ScheduleAt(ctx, loan.ID, from)
		if err != nil {
			return nil, fmt.Errorf("schedule at %s: %w", from, err)
		}
		for _, inst := range sched.Installments {
			if inst.Due.Before(from) || (i+1 < len(starts) && !inst.Due.Before(starts[i+1])) {
				continue
			}
			out = append(out, inst)
		}
	}
	return out, nil
}

// versionStarts returns the ValidFrom dates of the schedule versions of loan, ascending.
func versionStarts(ctx context.Context, r app.Repos, loan domain.Loan) ([]domain.Date, error) {
	events, err := r.Loans().ListEvents(ctx, loan.ID)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	starts := []domain.Date{loan.DisbursedOn}
	for _, e := range events {
		if e.EventDate.After(starts[len(starts)-1]) {
			starts = append(starts, e.EventDate)
		}
	}
	return starts, nil
}

// PayeeRow is the spending with one payee.
type PayeeRow struct {
	Payee string `json:"payee"`
	Total Amount `json:"total"`
	Count int    `json:"count"`
}

// PayeesResult is the top-payees report.
type PayeesResult struct {
	Period string     `json:"period"`
	Total  Amount     `json:"total"`
	Rows   []PayeeRow `json:"rows"`
}

// DefaultTopPayees is the row count of the top-payees report when none is requested.
const DefaultTopPayees = 10

// TopPayees ranks payees by what was spent with them (net of refunds) in the period, in the
// base currency. Transfers, exchanges, investments, pending rows and rows without a payee are
// left out.
func (a *Analytics) TopPayees(ctx context.Context, req ReportRequest) (PayeesResult, error) {
	if err := a.gate(); err != nil {
		return PayeesResult{}, err
	}
	p, err := a.period(req.Period)
	if err != nil {
		return PayeesResult{}, err
	}
	limit := cmp.Or(req.Limit, DefaultTopPayees)
	out := PayeesResult{Period: periodLabel(p), Rows: []PayeeRow{}}
	err = a.store.Read(ctx, func(r app.Repos) error {
		f, err := domain.NewTxFilter(domain.TxFilterSpec{From: p.From(), To: p.To()})
		if err != nil {
			return err
		}
		txs, err := r.Transactions().List(ctx, f, nil, 0)
		if err != nil {
			return fmt.Errorf("list transactions: %w", err)
		}
		payees, err := r.Payees().List(ctx)
		if err != nil {
			return fmt.Errorf("list payees: %w", err)
		}
		names := make(map[domain.PayeeID]string, len(payees))
		for _, py := range payees {
			names[py.ID] = py.Name
		}
		rows, total, err := a.payeeRows(ctx, r, txs, names)
		if err != nil {
			return err
		}
		out.Rows = rows
		out.Total = amountOf(total)
		slices.SortFunc(out.Rows, func(x, y PayeeRow) int {
			return cmp.Or(cmp.Compare(y.Total.Minor, x.Total.Minor), cmp.Compare(x.Payee, y.Payee))
		})
		if len(out.Rows) > limit {
			out.Rows = out.Rows[:limit]
		}
		return nil
	})
	return out, err
}

// payeeRows totals the spending of txs per payee, keeping the payees with a positive total.
func (a *Analytics) payeeRows(ctx context.Context, r app.Repos, txs []domain.Transaction, names map[domain.PayeeID]string) ([]PayeeRow, domain.Money, error) {
	type acc struct {
		total domain.Money
		count int
	}
	by := map[domain.PayeeID]*acc{}
	for _, t := range txs {
		switch {
		case t.PayeeID == 0, t.Status == domain.StatusPending,
			t.Kind == domain.KindTransfer, t.Kind == domain.KindExchange, t.Kind == domain.KindInvestment:
			continue
		}
		base, err := a.toBase(ctx, r, t.Amount, t.Date)
		if err != nil {
			return nil, domain.Money{}, err
		}
		if base, err = base.Neg(); err != nil {
			return nil, domain.Money{}, err
		}
		e := by[t.PayeeID]
		if e == nil {
			e = &acc{total: zeroMoney(a.cfg.BaseCurrency)}
			by[t.PayeeID] = e
		}
		e.count++
		if e.total, err = e.total.Add(base); err != nil {
			return nil, domain.Money{}, err
		}
	}
	total := zeroMoney(a.cfg.BaseCurrency)
	rows := []PayeeRow{}
	for id, e := range by {
		if e.total.Sign() <= 0 {
			continue
		}
		rows = append(rows, PayeeRow{Payee: names[id], Total: amountOf(e.total), Count: e.count})
		var err error
		if total, err = total.Add(e.total); err != nil {
			return nil, domain.Money{}, err
		}
	}
	return rows, total, nil
}

// BudgetLine is one budget evaluated for the month.
type BudgetLine struct {
	Budget    string `json:"budget"`
	Budgeted  Amount `json:"budgeted"`
	Spent     Amount `json:"spent"`
	Remaining Amount `json:"remaining"`
	State     string `json:"state,omitempty"` // caps only
}

// BudgetResult is the budget-vs-actual report.
type BudgetResult struct {
	Period    string       `json:"period"`
	Mode      string       `json:"mode"`
	Lines     []BudgetLine `json:"lines"`
	Uncovered Amount       `json:"uncovered"`
}

// BudgetVsActual compares each budget with the actual spending of the accounting month that
// contains the start of req.Period (the current month when zero). It needs the budget module.
func (a *Analytics) BudgetVsActual(ctx context.Context, req ReportRequest) (BudgetResult, error) {
	if err := a.gate(module.Budget); err != nil {
		return BudgetResult{}, err
	}
	p, err := a.period(req.Period)
	if err != nil {
		return BudgetResult{}, err
	}
	st, err := budgeting.New(a.store, a.clock, budgeting.Config{
		Locale: a.cfg.Locale, BaseCurrency: a.cfg.BaseCurrency, MonthStartDay: a.cfg.MonthStartDay,
		Mode: a.cfg.BudgetMode, Location: a.cfg.Location, Modules: a.cfg.Modules,
	}).Status(ctx, fmt.Sprintf("%04d-%02d", p.From().Year(), p.From().Month()))
	if err != nil {
		return BudgetResult{}, fmt.Errorf("budget status: %w", err)
	}
	out := BudgetResult{Period: periodLabel(st.Period), Mode: string(st.Mode), Lines: []BudgetLine{}, Uncovered: amountOf(st.Uncovered)}
	for _, c := range st.Caps {
		out.Lines = append(out.Lines, BudgetLine{
			Budget: c.Budget.Name, Budgeted: amountOf(c.Available), Spent: amountOf(c.Spent),
			Remaining: amountOf(c.Remaining), State: string(c.State),
		})
	}
	for _, e := range st.Envelopes {
		avail, err := e.Balance.Add(e.Spent)
		if err != nil {
			return BudgetResult{}, err
		}
		out.Lines = append(out.Lines, BudgetLine{
			Budget: e.Budget.Name, Budgeted: amountOf(avail), Spent: amountOf(e.Spent), Remaining: amountOf(e.Balance),
		})
	}
	return out, nil
}
