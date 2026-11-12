package sqlite

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"
)

type budgetRepo struct {
	repos *repos
}

// toBudget assembles a domain.Budget from its row and child rows.
func toBudget(row gen.Budget, targets []gen.BudgetTarget, amounts []gen.BudgetAmount) (domain.Budget, error) {
	mode, err := domain.ParseBudgetMode(row.Mode)
	if err != nil {
		return domain.Budget{}, fmt.Errorf("parse budget mode %q: %w", row.Mode, err)
	}
	cur, err := domain.ParseCurrency(row.Currency)
	if err != nil {
		return domain.Budget{}, fmt.Errorf("parse budget currency %q: %w", row.Currency, err)
	}
	amount, err := domain.NewMoney(row.AmountMinor, cur)
	if err != nil {
		return domain.Budget{}, fmt.Errorf("parse budget amount: %w", err)
	}
	b := domain.Budget{
		ID:     domain.BudgetID(row.ID),
		Name:   row.Name,
		Mode:   mode,
		Amount: amount,
		Carry:  row.Carry != 0,
		Thresholds: domain.BudgetThresholds{
			Warn: int(row.ThresholdWarn),
			Over: int(row.ThresholdOver),
		},
	}
	for _, t := range targets {
		target := domain.BudgetTarget{
			CategoryID: domain.CategoryID(nullInt64OrZero(t.CategoryID)),
			TagID:      domain.TagID(nullInt64OrZero(t.TagID)),
		}
		b.Targets = append(b.Targets, target)
	}
	for _, a := range amounts {
		over, err := domain.NewMoney(a.AmountMinor, cur)
		if err != nil {
			return domain.Budget{}, fmt.Errorf("parse budget override amount: %w", err)
		}
		if b.Overrides == nil {
			b.Overrides = map[domain.MonthKey]domain.Money{}
		}
		b.Overrides[domain.MonthKey{Year: int(a.Year), Month: int(a.Month)}] = over
	}
	return b, nil
}

func (r budgetRepo) checkTargets(ctx context.Context, targets []domain.BudgetTarget) error {
	for _, t := range targets {
		if t.CategoryID != 0 {
			if _, err := r.repos.queries.GetCategory(ctx, int64(t.CategoryID)); err != nil {
				return mapNotFound(mapSQLError(err), fmt.Sprintf("budget target category %d", t.CategoryID))
			}
		}
		if t.TagID != 0 {
			if _, err := r.repos.queries.GetTag(ctx, int64(t.TagID)); err != nil {
				return mapNotFound(mapSQLError(err), fmt.Sprintf("budget target tag %d", t.TagID))
			}
		}
	}
	return nil
}

func (r budgetRepo) setTargets(ctx context.Context, id int64, targets []domain.BudgetTarget) error {
	if err := r.repos.queries.ClearBudgetTargets(ctx, id); err != nil {
		return mapSQLError(err)
	}
	for _, t := range targets {
		if err := r.repos.queries.AddBudgetTarget(ctx, gen.AddBudgetTargetParams{
			BudgetID:   id,
			CategoryID: toNullInt64(t.CategoryID),
			TagID:      toNullInt64(t.TagID),
		}); err != nil {
			return mapConstraint(err)
		}
	}
	return nil
}

func (r budgetRepo) setAmounts(ctx context.Context, id int64, overrides map[domain.MonthKey]domain.Money) error {
	if err := r.repos.queries.ClearBudgetAmounts(ctx, id); err != nil {
		return mapSQLError(err)
	}
	for k, m := range overrides {
		if err := r.repos.queries.AddBudgetAmount(ctx, gen.AddBudgetAmountParams{
			BudgetID:    id,
			Year:        int64(k.Year),
			Month:       int64(k.Month),
			AmountMinor: m.Minor(),
		}); err != nil {
			return mapConstraint(err)
		}
	}
	return nil
}

func (r budgetRepo) Insert(ctx context.Context, b domain.Budget) (domain.Budget, error) {
	if err := r.repos.writable(); err != nil {
		return domain.Budget{}, err
	}
	if err := r.checkTargets(ctx, b.Targets); err != nil {
		return domain.Budget{}, err
	}
	row, err := r.repos.queries.InsertBudget(ctx, gen.InsertBudgetParams{
		Name:          b.Name,
		Mode:          string(b.Mode),
		AmountMinor:   b.Amount.Minor(),
		Currency:      b.Amount.Currency().Code(),
		Carry:         boolToInt64(b.Carry),
		ThresholdWarn: int64(b.Thresholds.Warn),
		ThresholdOver: int64(b.Thresholds.Over),
	})
	if err != nil {
		return domain.Budget{}, mapConstraint(err)
	}
	if err := r.setTargets(ctx, row.ID, b.Targets); err != nil {
		return domain.Budget{}, err
	}
	if err := r.setAmounts(ctx, row.ID, b.Overrides); err != nil {
		return domain.Budget{}, err
	}
	return r.Get(ctx, domain.BudgetID(row.ID))
}

func (r budgetRepo) Update(ctx context.Context, b domain.Budget) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	if err := r.checkTargets(ctx, b.Targets); err != nil {
		return err
	}
	changed, err := r.repos.queries.UpdateBudget(ctx, gen.UpdateBudgetParams{
		ID:            int64(b.ID),
		Name:          b.Name,
		Mode:          string(b.Mode),
		AmountMinor:   b.Amount.Minor(),
		Currency:      b.Amount.Currency().Code(),
		Carry:         boolToInt64(b.Carry),
		ThresholdWarn: int64(b.Thresholds.Warn),
		ThresholdOver: int64(b.Thresholds.Over),
	})
	if err != nil {
		return mapConstraint(err)
	}
	if changed == 0 {
		return fmt.Errorf("budget %d: %w", b.ID, domain.ErrNotFound)
	}
	if err := r.setTargets(ctx, int64(b.ID), b.Targets); err != nil {
		return err
	}
	return r.setAmounts(ctx, int64(b.ID), b.Overrides)
}

func (r budgetRepo) Delete(ctx context.Context, id domain.BudgetID) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	changed, err := r.repos.queries.DeleteBudget(ctx, int64(id))
	if err != nil {
		return mapSQLError(err)
	}
	if changed == 0 {
		return fmt.Errorf("budget %d: %w", id, domain.ErrNotFound)
	}
	return nil
}

func (r budgetRepo) Get(ctx context.Context, id domain.BudgetID) (domain.Budget, error) {
	row, err := r.repos.queries.GetBudget(ctx, int64(id))
	if err != nil {
		return domain.Budget{}, mapNotFound(mapSQLError(err), fmt.Sprintf("budget %d", id))
	}
	targets, err := r.repos.queries.TargetsOfBudget(ctx, row.ID)
	if err != nil {
		return domain.Budget{}, mapSQLError(err)
	}
	amounts, err := r.repos.queries.AmountsOfBudget(ctx, row.ID)
	if err != nil {
		return domain.Budget{}, mapSQLError(err)
	}
	return toBudget(row, targets, amounts)
}

func (r budgetRepo) List(ctx context.Context) ([]domain.Budget, error) {
	rows, err := r.repos.queries.ListBudgets(ctx)
	if err != nil {
		return nil, mapSQLError(err)
	}
	targets, err := r.repos.queries.ListBudgetTargets(ctx)
	if err != nil {
		return nil, mapSQLError(err)
	}
	amounts, err := r.repos.queries.ListBudgetAmounts(ctx)
	if err != nil {
		return nil, mapSQLError(err)
	}
	return r.assemble(rows, targets, amounts)
}

// assemble groups targets and amounts by budget ID and builds the domain budgets, in row order.
func (r budgetRepo) assemble(rows []gen.Budget, targets []gen.BudgetTarget, amounts []gen.BudgetAmount) ([]domain.Budget, error) {
	byBudgetTargets := map[int64][]gen.BudgetTarget{}
	for _, t := range targets {
		byBudgetTargets[t.BudgetID] = append(byBudgetTargets[t.BudgetID], t)
	}
	byBudgetAmounts := map[int64][]gen.BudgetAmount{}
	for _, a := range amounts {
		byBudgetAmounts[a.BudgetID] = append(byBudgetAmounts[a.BudgetID], a)
	}
	out := make([]domain.Budget, 0, len(rows))
	for _, row := range rows {
		b, err := toBudget(row, byBudgetTargets[row.ID], byBudgetAmounts[row.ID])
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}

func (r budgetRepo) ForMonth(ctx context.Context, k domain.MonthKey) ([]domain.Budget, error) {
	rows, err := r.repos.queries.ListBudgets(ctx)
	if err != nil {
		return nil, mapSQLError(err)
	}
	targets, err := r.repos.queries.ListBudgetTargets(ctx)
	if err != nil {
		return nil, mapSQLError(err)
	}
	amounts, err := r.repos.queries.ListBudgetAmountsForMonth(ctx, gen.ListBudgetAmountsForMonthParams{
		Year: int64(k.Year), Month: int64(k.Month),
	})
	if err != nil {
		return nil, mapSQLError(err)
	}
	return r.assemble(rows, targets, amounts)
}

func (r budgetRepo) budgetExists(ctx context.Context, id domain.BudgetID) error {
	if id == 0 {
		return nil // the unassigned pool
	}
	if _, err := r.repos.queries.GetBudget(ctx, int64(id)); err != nil {
		return mapNotFound(mapSQLError(err), fmt.Sprintf("budget %d", id))
	}
	return nil
}

func (r budgetRepo) AddMove(ctx context.Context, m domain.BudgetMove) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	if err := r.budgetExists(ctx, m.From); err != nil {
		return err
	}
	if err := r.budgetExists(ctx, m.To); err != nil {
		return err
	}
	if err := r.repos.queries.InsertBudgetMove(ctx, gen.InsertBudgetMoveParams{
		Date:         m.Date.String(),
		FromBudgetID: toNullInt64(m.From),
		ToBudgetID:   toNullInt64(m.To),
		AmountMinor:  m.Amount.Minor(),
		Currency:     m.Amount.Currency().Code(),
	}); err != nil {
		return mapConstraint(err)
	}
	return nil
}

func (r budgetRepo) Moves(ctx context.Context, from, to domain.Date) ([]domain.BudgetMove, error) {
	rows, err := r.repos.queries.BudgetMovesInRange(ctx, gen.BudgetMovesInRangeParams{
		FromDate: from.String(), ToDate: to.String(),
	})
	if err != nil {
		return nil, mapSQLError(err)
	}
	out := make([]domain.BudgetMove, 0, len(rows))
	for _, row := range rows {
		move, err := toBudgetMove(row)
		if err != nil {
			return nil, err
		}
		out = append(out, move)
	}
	return out, nil
}

func toBudgetMove(row gen.BudgetMove) (domain.BudgetMove, error) {
	cur, err := domain.ParseCurrency(row.Currency)
	if err != nil {
		return domain.BudgetMove{}, fmt.Errorf("parse budget move currency %q: %w", row.Currency, err)
	}
	amount, err := domain.NewMoney(row.AmountMinor, cur)
	if err != nil {
		return domain.BudgetMove{}, fmt.Errorf("parse budget move amount: %w", err)
	}
	date, err := domain.ParseDate(row.Date)
	if err != nil {
		return domain.BudgetMove{}, fmt.Errorf("parse budget move date %q: %w", row.Date, err)
	}
	return domain.BudgetMove{
		Date:   date,
		From:   domain.BudgetID(nullInt64OrZero(row.FromBudgetID)),
		To:     domain.BudgetID(nullInt64OrZero(row.ToBudgetID)),
		Amount: amount,
	}, nil
}

var _ app.BudgetRepo = budgetRepo{}
