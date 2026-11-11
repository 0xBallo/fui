package app

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
)

// AddCategoryRequest creates a category; an empty Kind means expense.
type AddCategoryRequest struct {
	Name string
	Kind string
}

// AddCategoryPlan is the plan to create Category.
type AddCategoryPlan struct {
	Plan
	Category domain.Category
}

// PlanAddCategory validates the request and plans the creation.
func (s *Service) PlanAddCategory(ctx context.Context, req AddCategoryRequest) (AddCategoryPlan, error) {
	kind, err := domain.ParseCategoryKind(orDefault(req.Kind, string(domain.CategoryExpense)))
	if err != nil {
		return AddCategoryPlan{}, fmt.Errorf("category kind: %w", err)
	}
	c, err := domain.NewCategory(req.Name, kind)
	if err != nil {
		return AddCategoryPlan{}, fmt.Errorf("category: %w", err)
	}
	err = s.store.Read(ctx, func(r Repos) error {
		cats, err := r.Categories().List(ctx)
		if err != nil {
			return fmt.Errorf("list categories: %w", err)
		}
		return checkCategoryName(cats, c.Name, 0)
	})
	if err != nil {
		return AddCategoryPlan{}, err
	}
	return AddCategoryPlan{Plan: Plan{Changes: []Change{categoryCreate(c)}}, Category: c}, nil
}

// ApplyAddCategory creates the category and returns it with its ID.
func (s *Service) ApplyAddCategory(ctx context.Context, p AddCategoryPlan) (domain.Category, error) {
	var out domain.Category
	err := s.apply(ctx, p.Bulk, func(r Repos) (err error) {
		out, err = r.Categories().Insert(ctx, p.Category)
		return err
	})
	return out, err
}

// ListCategories returns every category.
func (s *Service) ListCategories(ctx context.Context) ([]domain.Category, error) {
	var out []domain.Category
	err := s.store.Read(ctx, func(r Repos) (err error) {
		out, err = r.Categories().List(ctx)
		return err
	})
	return out, err
}

// RenameCategoryRequest renames the category named by Category.
type RenameCategoryRequest struct {
	Category string
	NewName  string
}

// RenameCategoryPlan is the plan to store Category.
type RenameCategoryPlan struct {
	Plan
	Category domain.Category
}

// PlanRenameCategory plans the rename.
func (s *Service) PlanRenameCategory(ctx context.Context, req RenameCategoryRequest) (RenameCategoryPlan, error) {
	var out RenameCategoryPlan
	err := s.store.Read(ctx, func(r Repos) error {
		cats, err := r.Categories().List(ctx)
		if err != nil {
			return fmt.Errorf("list categories: %w", err)
		}
		cur, err := pickCategory(cats, req.Category)
		if err != nil {
			return err
		}
		renamed, err := domain.NewCategory(req.NewName, cur.Kind)
		if err != nil {
			return fmt.Errorf("category: %w", err)
		}
		renamed.ID = cur.ID
		if err := checkCategoryName(cats, renamed.Name, cur.ID); err != nil {
			return err
		}
		ch := Change{
			Op: OpUpdate, Entity: EntityCategory,
			Summary: fmt.Sprintf("rename category %q to %q", cur.Name, renamed.Name),
			Fields:  map[string]any{"id": int64(cur.ID), "name": renamed.Name, "previous_name": cur.Name},
		}
		out = RenameCategoryPlan{Plan: Plan{Changes: []Change{ch}}, Category: renamed}
		return nil
	})
	return out, err
}

// ApplyRenameCategory stores the renamed category.
func (s *Service) ApplyRenameCategory(ctx context.Context, p RenameCategoryPlan) (domain.Category, error) {
	err := s.apply(ctx, p.Bulk, func(r Repos) error {
		if err := r.Categories().Update(ctx, p.Category); err != nil {
			return fmt.Errorf("rename category %d: %w", p.Category.ID, err)
		}
		return nil
	})
	return p.Category, err
}

// RemoveCategoryRequest removes Category. A category with transactions is removed only if
// ReassignTo names the category that receives them; otherwise the plan fails with ErrInUse.
type RemoveCategoryRequest struct {
	Category   string
	ReassignTo string
}

// MergeCategoryRequest moves every transaction of Source to Target and removes Source.
type MergeCategoryRequest struct {
	Source string
	Target string
}

// RemoveCategoryPlan deletes Category after moving Moved transactions to Target (zero when none).
// It is Bulk when transactions move.
type RemoveCategoryPlan struct {
	Plan
	Category domain.Category
	Target   domain.CategoryID
	Moved    int
}

// RemoveCategoryResult reports how many transactions were reassigned.
type RemoveCategoryResult struct {
	Reassigned int
}

// PlanRemoveCategory plans the removal, with the reassignment when ReassignTo is set.
func (s *Service) PlanRemoveCategory(ctx context.Context, req RemoveCategoryRequest) (RemoveCategoryPlan, error) {
	var out RemoveCategoryPlan
	err := s.store.Read(ctx, func(r Repos) error {
		cats, err := r.Categories().List(ctx)
		if err != nil {
			return fmt.Errorf("list categories: %w", err)
		}
		cat, err := pickCategory(cats, req.Category)
		if err != nil {
			return err
		}
		used, err := r.Categories().Usage(ctx, cat.ID)
		if err != nil {
			return fmt.Errorf("count transactions of %q: %w", cat.Name, err)
		}
		out = RemoveCategoryPlan{Category: cat}
		if used > 0 && req.ReassignTo == "" {
			return newError(ErrInUse, map[string]any{"category": cat.Name, "transactions": used},
				"category %q has %d transactions: reassign them to another category", cat.Name, used)
		}
		if req.ReassignTo != "" {
			if err := out.reassignTo(cats, req.ReassignTo, used); err != nil {
				return err
			}
		}
		out.Changes = append(out.Changes, Change{
			Op: OpDelete, Entity: EntityCategory, Summary: fmt.Sprintf("delete category %q", cat.Name),
			Fields: map[string]any{"id": int64(cat.ID), "name": cat.Name},
		})
		return nil
	})
	return out, err
}

func (p *RemoveCategoryPlan) reassignTo(cats []domain.Category, ref string, used int) error {
	target, err := pickCategory(cats, ref)
	if err != nil {
		return err
	}
	if target.ID == p.Category.ID {
		return newError(domain.ErrInvalidValue, map[string]any{"field": "reassign_to", "value": ref},
			"cannot reassign category %q to itself", target.Name)
	}
	p.Target = target.ID
	if used == 0 {
		return nil
	}
	p.Moved, p.Bulk = used, true
	p.Changes = append(p.Changes, Change{
		Op: OpUpdate, Entity: EntityTransaction,
		Summary: fmt.Sprintf("move %d transactions from category %q to %q", used, p.Category.Name, target.Name),
		Fields:  map[string]any{"from": p.Category.Name, "to": target.Name, "count": used},
	})
	return nil
}

// PlanMergeCategory plans moving every transaction of Source to Target and deleting Source.
func (s *Service) PlanMergeCategory(ctx context.Context, req MergeCategoryRequest) (RemoveCategoryPlan, error) {
	return s.PlanRemoveCategory(ctx, RemoveCategoryRequest{Category: req.Source, ReassignTo: req.Target})
}

// ApplyRemoveCategory reassigns the transactions (if any), then deletes the category.
func (s *Service) ApplyRemoveCategory(ctx context.Context, p RemoveCategoryPlan) (RemoveCategoryResult, error) {
	var res RemoveCategoryResult
	err := s.apply(ctx, p.Bulk, func(r Repos) error {
		if p.Target != 0 {
			n, err := r.Categories().Reassign(ctx, p.Category.ID, p.Target)
			if err != nil {
				return fmt.Errorf("reassign transactions of %q: %w", p.Category.Name, err)
			}
			res.Reassigned = n
		}
		if err := r.Categories().Delete(ctx, p.Category.ID); err != nil {
			return fmt.Errorf("delete category %q: %w", p.Category.Name, err)
		}
		return nil
	})
	return res, err
}

// ApplyMergeCategory applies a plan made by PlanMergeCategory.
func (s *Service) ApplyMergeCategory(ctx context.Context, p RemoveCategoryPlan) (RemoveCategoryResult, error) {
	return s.ApplyRemoveCategory(ctx, p)
}

// checkCategoryName fails with ErrAlreadyExists when a category other than self has the same name.
func checkCategoryName(cats []domain.Category, name string, self domain.CategoryID) error {
	for _, c := range cats {
		if c.ID != self && equalNames(c.Name, name) {
			return newError(ErrAlreadyExists, map[string]any{"entity": EntityCategory, "name": c.Name},
				"a category named %q already exists", c.Name)
		}
	}
	return nil
}
