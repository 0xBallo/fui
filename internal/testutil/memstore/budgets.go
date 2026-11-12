package memstore

import (
	"context"
	"maps"
	"slices"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

type budgets struct{ *repos }

// cloneBudget deep-copies the target slice and override map so stored and returned values never
// alias state a caller could mutate later.
func cloneBudget(b domain.Budget) domain.Budget {
	b.Targets = slices.Clone(b.Targets)
	if b.Overrides != nil {
		b.Overrides = maps.Clone(b.Overrides)
	}
	return b
}

func (b budgets) checkTargets(targets []domain.BudgetTarget) error {
	for _, t := range targets {
		if t.CategoryID != 0 {
			if _, ok := b.st.Categories[t.CategoryID]; !ok {
				return notFound("budget target category", t.CategoryID)
			}
		}
		if t.TagID != 0 {
			if _, ok := b.st.Tags[t.TagID]; !ok {
				return notFound("budget target tag", t.TagID)
			}
		}
		if t.CategoryID != 0 {
			for id, other := range b.st.Budgets {
				if id == 0 {
					continue
				}
				if slices.ContainsFunc(other.Targets, func(o domain.BudgetTarget) bool { return o.CategoryID == t.CategoryID }) {
					return exists("budget target category", other.Name)
				}
			}
		}
	}
	return nil
}

func (b budgets) Insert(_ context.Context, bgt domain.Budget) (domain.Budget, error) {
	if err := b.writable(); err != nil {
		return domain.Budget{}, err
	}
	if err := b.checkTargets(bgt.Targets); err != nil {
		return domain.Budget{}, err
	}
	bgt.ID = domain.BudgetID(b.st.nextID("budgets"))
	bgt = cloneBudget(bgt)
	b.st.Budgets[bgt.ID] = bgt
	return cloneBudget(bgt), nil
}

func (b budgets) Update(_ context.Context, bgt domain.Budget) error {
	if err := b.writable(); err != nil {
		return err
	}
	if _, ok := b.st.Budgets[bgt.ID]; !ok {
		return notFound("budget", bgt.ID)
	}
	if err := b.checkTargetsExcluding(bgt); err != nil {
		return err
	}
	b.st.Budgets[bgt.ID] = cloneBudget(bgt)
	return nil
}

// checkTargetsExcluding runs checkTargets ignoring bgt's own current targets, so renaming or
// re-saving a budget with the same category target does not trip the one-budget-per-category rule.
func (b budgets) checkTargetsExcluding(bgt domain.Budget) error {
	saved := b.st.Budgets[bgt.ID]
	delete(b.st.Budgets, bgt.ID)
	err := b.checkTargets(bgt.Targets)
	b.st.Budgets[bgt.ID] = saved
	return err
}

func (b budgets) Delete(_ context.Context, id domain.BudgetID) error {
	if err := b.writable(); err != nil {
		return err
	}
	if _, ok := b.st.Budgets[id]; !ok {
		return notFound("budget", id)
	}
	delete(b.st.Budgets, id)
	b.st.Moves = slices.DeleteFunc(slices.Clone(b.st.Moves), func(m domain.BudgetMove) bool {
		return m.From == id || m.To == id
	})
	return nil
}

func (b budgets) Get(_ context.Context, id domain.BudgetID) (domain.Budget, error) {
	bgt, ok := b.st.Budgets[id]
	if !ok {
		return domain.Budget{}, notFound("budget", id)
	}
	return cloneBudget(bgt), nil
}

func (b budgets) List(context.Context) ([]domain.Budget, error) {
	out := sortedValues(b.st.Budgets)
	for i := range out {
		out[i] = cloneBudget(out[i])
	}
	return out, nil
}

// ForMonth returns every budget (a budget has no start or end date), with Overrides holding at
// most the single entry for k, mirroring the SQLite repo's scoped projection.
func (b budgets) ForMonth(_ context.Context, k domain.MonthKey) ([]domain.Budget, error) {
	out := sortedValues(b.st.Budgets)
	for i, bgt := range out {
		scoped := bgt
		scoped.Overrides = nil
		if m, ok := bgt.Overrides[k]; ok {
			scoped.Overrides = map[domain.MonthKey]domain.Money{k: m}
		}
		out[i] = cloneBudget(scoped)
	}
	return out, nil
}

func (b budgets) budgetExists(id domain.BudgetID) error {
	if id == 0 {
		return nil // the unassigned pool
	}
	if _, ok := b.st.Budgets[id]; !ok {
		return notFound("budget", id)
	}
	return nil
}

func (b budgets) AddMove(_ context.Context, m domain.BudgetMove) error {
	if err := b.writable(); err != nil {
		return err
	}
	if err := b.budgetExists(m.From); err != nil {
		return err
	}
	if err := b.budgetExists(m.To); err != nil {
		return err
	}
	b.st.Moves = append(slices.Clone(b.st.Moves), m)
	return nil
}

func (b budgets) Moves(_ context.Context, from, to domain.Date) ([]domain.BudgetMove, error) {
	out := make([]domain.BudgetMove, 0)
	for _, m := range b.st.Moves {
		if !m.Date.Before(from) && !m.Date.After(to) {
			out = append(out, m)
		}
	}
	slices.SortStableFunc(out, func(a, c domain.BudgetMove) int { return a.Date.Cmp(c.Date) })
	return out, nil
}

var _ app.BudgetRepo = budgets{}
