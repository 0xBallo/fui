package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/0xballo/fui/internal/domain"
)

// InitPlan lists the default categories that are still missing.
type InitPlan struct {
	Plan
	Categories []domain.Category
}

// InitResult reports what ApplyInit created.
type InitResult struct {
	Created int
}

// PlanInit plans the creation of the default categories that do not exist yet (by name,
// case-insensitively), so running it twice is harmless.
func (s *Service) PlanInit(ctx context.Context) (InitPlan, error) {
	var plan InitPlan
	err := s.store.Read(ctx, func(r Repos) error {
		existing, err := r.Categories().List(ctx)
		if err != nil {
			return fmt.Errorf("list categories: %w", err)
		}
		have := make(map[string]bool, len(existing))
		for _, c := range existing {
			have[strings.ToLower(c.Name)] = true
		}
		for _, c := range domain.DefaultCategories() {
			if have[strings.ToLower(c.Name)] {
				continue
			}
			plan.Categories = append(plan.Categories, c)
			plan.Changes = append(plan.Changes, categoryCreate(c))
		}
		return nil
	})
	return plan, err
}

// ApplyInit creates the planned categories.
func (s *Service) ApplyInit(ctx context.Context, p InitPlan) (InitResult, error) {
	err := s.apply(ctx, p.Bulk, func(r Repos) error {
		for _, c := range p.Categories {
			if _, err := r.Categories().Insert(ctx, c); err != nil {
				return fmt.Errorf("create category %q: %w", c.Name, err)
			}
		}
		return nil
	})
	if err != nil {
		return InitResult{}, err
	}
	return InitResult{Created: len(p.Categories)}, nil
}
