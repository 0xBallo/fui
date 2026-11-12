package sqlite

import (
	"context"
	"errors"
	"fmt"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"
)

type categoryRepo struct {
	repos *repos
}

func (r categoryRepo) Insert(ctx context.Context, c domain.Category) (domain.Category, error) {
	if err := r.repos.writable(); err != nil {
		return domain.Category{}, err
	}
	row, err := r.repos.queries.InsertCategory(ctx, gen.InsertCategoryParams{
		Name: c.Name,
		Kind: string(c.Kind),
	})
	if err != nil {
		return domain.Category{}, mapConstraint(err)
	}
	return toCategory(row)
}

func (r categoryRepo) Update(ctx context.Context, c domain.Category) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	changed, err := r.repos.queries.UpdateCategory(ctx, gen.UpdateCategoryParams{
		ID:   int64(c.ID),
		Name: c.Name,
		Kind: string(c.Kind),
	})
	if err != nil {
		return mapConstraint(err)
	}
	if changed == 0 {
		return fmt.Errorf("category %d: %w", c.ID, domain.ErrNotFound)
	}
	return nil
}

func (r categoryRepo) Delete(ctx context.Context, id domain.CategoryID) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	changed, err := r.repos.queries.DeleteCategory(ctx, int64(id))
	if err != nil {
		mapped := mapConstraint(err)
		if errors.Is(mapped, app.ErrInUse) {
			return fmt.Errorf("category %d: %w", id, app.ErrInUse)
		}
		return mapped
	}
	if changed == 0 {
		return fmt.Errorf("category %d: %w", id, domain.ErrNotFound)
	}
	return nil
}

func (r categoryRepo) Get(ctx context.Context, id domain.CategoryID) (domain.Category, error) {
	row, err := r.repos.queries.GetCategory(ctx, int64(id))
	if err != nil {
		return domain.Category{}, mapNotFound(mapSQLError(err), "get category")
	}
	return toCategory(row)
}

func (r categoryRepo) List(ctx context.Context) ([]domain.Category, error) {
	rows, err := r.repos.queries.ListCategories(ctx)
	if err != nil {
		return nil, mapSQLError(err)
	}
	out := make([]domain.Category, 0, len(rows))
	for _, row := range rows {
		category, convErr := toCategory(row)
		if convErr != nil {
			return nil, convErr
		}
		out = append(out, category)
	}
	return out, nil
}

func (r categoryRepo) Usage(ctx context.Context, id domain.CategoryID) (int, error) {
	n, err := r.repos.queries.CountCategoryUsage(ctx, toNullInt64(id))
	if err != nil {
		return 0, mapSQLError(err)
	}
	splits, err := r.repos.queries.CountCategorySplitUsage(ctx, int64(id))
	if err != nil {
		return 0, mapSQLError(err)
	}
	return int(n + splits), nil
}

func (r categoryRepo) Reassign(ctx context.Context, from, to domain.CategoryID) (int, error) {
	if err := r.repos.writable(); err != nil {
		return 0, err
	}
	if _, err := r.repos.queries.GetCategory(ctx, int64(to)); err != nil {
		return 0, mapNotFound(mapSQLError(err), "target category")
	}
	moved, err := r.repos.queries.ReassignCategory(ctx, gen.ReassignCategoryParams{
		FromCategoryID: toNullInt64(from),
		ToCategoryID:   toNullInt64(to),
	})
	if err != nil {
		return 0, mapConstraint(err)
	}
	movedSplits, err := r.repos.queries.ReassignSplitCategory(ctx, gen.ReassignSplitCategoryParams{
		FromCategoryID: int64(from),
		ToCategoryID:   int64(to),
	})
	if err != nil {
		return 0, mapConstraint(err)
	}
	return int(moved + movedSplits), nil
}
