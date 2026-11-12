package budgeting

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// CapLine is one cap evaluated for the requested month.
type CapLine struct {
	Budget domain.Budget
	domain.CapStatus
}

// EnvelopeLine is one envelope's ledger for the requested month.
type EnvelopeLine struct {
	Budget domain.Budget
	domain.EnvelopeMonth
}

// Status is the state of the budgets in one month. Caps or Envelopes is filled according to
// the configured mode; ToAssign ("Da assegnare") only in envelope mode.
type Status struct {
	Period    domain.Period
	Mode      domain.BudgetMode
	Caps      []CapLine
	Envelopes []EnvelopeLine
	ToAssign  domain.Unassigned
	Uncovered domain.Money // spending in the month covered by no budget ("non coperte")
}

// Status evaluates every budget for month ("YYYY-MM", empty = current). Transactions are
// converted to the base currency at their date, splits count per row, pending rows are
// planned only. History before the month feeds carry-overs.
func (s *Service) Status(ctx context.Context, monthIn string) (Status, error) {
	if err := s.gate(); err != nil {
		return Status{}, err
	}
	p, err := s.month(monthIn)
	if err != nil {
		return Status{}, err
	}
	st := Status{Period: p, Mode: s.cfg.Mode}
	err = s.store.Read(ctx, func(r app.Repos) error {
		budgets, err := r.Budgets().List(ctx)
		if err != nil {
			return fmt.Errorf("list budgets: %w", err)
		}
		rows, err := s.rows(ctx, r)
		if err != nil {
			return err
		}
		if st.Uncovered, err = domain.UncoveredSpending(budgets, p, rows, s.cfg.BaseCurrency); err != nil {
			return err
		}
		if s.cfg.Mode == domain.BudgetCap {
			return s.caps(&st, budgets, rows)
		}
		moves, err := r.Budgets().Moves(ctx, epoch, p.To())
		if err != nil {
			return fmt.Errorf("list moves: %w", err)
		}
		if err := s.envelopes(&st, budgets, rows, moves); err != nil {
			return err
		}
		return s.toAssign(ctx, r, &st)
	})
	return st, err
}

var epoch = func() domain.Date {
	d, err := domain.NewDate(1900, 1, 1)
	if err != nil {
		panic(err)
	}
	return d
}()

// periodsTo returns the consecutive budget months from the earliest row (or p) up to p.
func (s *Service) periodsTo(p domain.Period, rows []domain.BudgetRow, moves []domain.BudgetMove) ([]domain.Period, error) {
	first := p.From()
	for _, r := range rows {
		if r.Date.Before(first) {
			first = r.Date
		}
	}
	for _, m := range moves {
		if m.Date.Before(first) {
			first = m.Date
		}
	}
	n, d := 0, first
	for {
		fp, err := domain.MonthPeriod(d, s.cfg.MonthStartDay)
		if err != nil {
			return nil, err
		}
		if fp.From().Cmp(p.From()) >= 0 {
			break
		}
		n++
		if d, err = fp.To().AddDays(1); err != nil {
			return nil, err
		}
	}
	ps, err := domain.BudgetPeriods(first, n+1, s.cfg.MonthStartDay)
	if err != nil {
		return nil, err
	}
	return ps, nil
}

func (s *Service) caps(st *Status, budgets []domain.Budget, rows []domain.BudgetRow) error {
	ps, err := s.periodsTo(st.Period, rows, nil)
	if err != nil {
		return err
	}
	for _, b := range budgets {
		sts, err := domain.CapStatuses(b, ps, rows, domain.Money{})
		if err != nil {
			return err
		}
		st.Caps = append(st.Caps, CapLine{Budget: b, CapStatus: sts[len(sts)-1]})
	}
	return nil
}

func (s *Service) envelopes(st *Status, budgets []domain.Budget, rows []domain.BudgetRow, moves []domain.BudgetMove) error {
	ps, err := s.periodsTo(st.Period, rows, moves)
	if err != nil {
		return err
	}
	for _, b := range budgets {
		ms, err := domain.EnvelopeLedger(b, ps, rows, moves, domain.Money{})
		if err != nil {
			return err
		}
		st.Envelopes = append(st.Envelopes, EnvelopeLine{Budget: b, EnvelopeMonth: ms[len(ms)-1]})
	}
	return nil
}

// toAssign computes "Da assegnare" from the liquidity balances at the end of the period.
func (s *Service) toAssign(ctx context.Context, r app.Repos, st *Status) error {
	accts, err := r.Accounts().List(ctx)
	if err != nil {
		return fmt.Errorf("list accounts: %w", err)
	}
	var bals []domain.AccountBalance
	for _, a := range accts {
		if !a.Kind.IsLiquidity() {
			continue
		}
		sum, err := r.Transactions().SumUpTo(ctx, a.ID, st.Period.To())
		if err != nil {
			return fmt.Errorf("balance of %q: %w", a.Name, err)
		}
		mv, err := domain.NewMoney(sum, a.Currency)
		if err != nil {
			return err
		}
		bal, err := a.OpeningBalance.Add(mv)
		if err != nil {
			return err
		}
		if bal, err = s.toBase(ctx, r, bal, st.Period.To()); err != nil {
			return err
		}
		bals = append(bals, domain.AccountBalance{Kind: a.Kind, Balance: bal})
	}
	envs := make([]domain.EnvelopeBalance, len(st.Envelopes))
	for i, e := range st.Envelopes {
		envs[i] = domain.EnvelopeBalance{TagOnly: e.Budget.TagOnly(), Balance: e.Balance}
	}
	st.ToAssign, err = domain.ToAssign(s.cfg.BaseCurrency, bals, envs)
	return err
}

// toBase converts m to the base currency at the rate on or before date.
func (s *Service) toBase(ctx context.Context, r app.Repos, m domain.Money, date domain.Date) (domain.Money, error) {
	if m.Currency() == s.cfg.BaseCurrency {
		return m, nil
	}
	fx, err := r.FXRates().OnOrBefore(ctx, m.Currency(), date)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Money{}, &domain.Error{
			Code:    app.CodeRateMissing,
			Message: fmt.Sprintf("no exchange rate on or before %s for %s", date, m.Currency()),
			Details: map[string]any{"currencies": []string{m.Currency().Code()}, "date": date.String()},
		}
	}
	if err != nil {
		return domain.Money{}, fmt.Errorf("look up %s rate: %w", m.Currency(), err)
	}
	return domain.Convert(m, fx.Rate, s.cfg.BaseCurrency)
}

// rows feeds the engine: every transaction as base-currency rows, one per split when split.
func (s *Service) rows(ctx context.Context, r app.Repos) ([]domain.BudgetRow, error) {
	f, err := domain.NewTxFilter(domain.TxFilterSpec{})
	if err != nil {
		return nil, err
	}
	txs, err := r.Transactions().List(ctx, f, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	var out []domain.BudgetRow
	for _, t := range txs {
		tags, err := r.Tags().TagsOf(ctx, t.ID)
		if err != nil {
			return nil, fmt.Errorf("tags of transaction %d: %w", t.ID, err)
		}
		ids := make([]domain.TagID, len(tags))
		for i, tg := range tags {
			ids[i] = tg.ID
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
			amt, err := s.toBase(ctx, r, sp.Amount, t.Date)
			if err != nil {
				return nil, err
			}
			out = append(out, domain.BudgetRow{
				Date: t.Date, Amount: amt, Kind: t.Kind, Status: t.Status, CategoryID: sp.Category, TagIDs: slices.Clone(ids),
			})
		}
	}
	return out, nil
}
