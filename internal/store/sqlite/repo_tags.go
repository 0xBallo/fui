package sqlite

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"
)

type tagRepo struct {
	repos *repos
}

func toTag(row gen.Tag) domain.Tag {
	return domain.Tag{ID: domain.TagID(row.ID), Name: row.Name}
}

func (r tagRepo) Insert(ctx context.Context, t domain.Tag) (domain.Tag, error) {
	if err := r.repos.writable(); err != nil {
		return domain.Tag{}, err
	}
	row, err := r.repos.queries.InsertTag(ctx, t.Name)
	if err != nil {
		return domain.Tag{}, mapConstraint(err)
	}
	return toTag(row), nil
}

func (r tagRepo) Update(ctx context.Context, t domain.Tag) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	changed, err := r.repos.queries.UpdateTag(ctx, gen.UpdateTagParams{ID: int64(t.ID), Name: t.Name})
	if err != nil {
		return mapConstraint(err)
	}
	if changed == 0 {
		return fmt.Errorf("tag %d: %w", t.ID, domain.ErrNotFound)
	}
	return nil
}

func (r tagRepo) Delete(ctx context.Context, id domain.TagID) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	changed, err := r.repos.queries.DeleteTag(ctx, int64(id))
	if err != nil {
		return mapSQLError(err)
	}
	if changed == 0 {
		return fmt.Errorf("tag %d: %w", id, domain.ErrNotFound)
	}
	return nil
}

func (r tagRepo) Get(ctx context.Context, id domain.TagID) (domain.Tag, error) {
	row, err := r.repos.queries.GetTag(ctx, int64(id))
	if err != nil {
		return domain.Tag{}, mapNotFound(mapSQLError(err), fmt.Sprintf("tag %d", id))
	}
	return toTag(row), nil
}

func (r tagRepo) List(ctx context.Context) ([]domain.Tag, error) {
	rows, err := r.repos.queries.ListTags(ctx)
	if err != nil {
		return nil, mapSQLError(err)
	}
	return mapTags(rows), nil
}

func (r tagRepo) TagsOf(ctx context.Context, tx domain.TransactionID) ([]domain.Tag, error) {
	rows, err := r.repos.queries.TagsOfTransaction(ctx, int64(tx))
	if err != nil {
		return nil, mapSQLError(err)
	}
	return mapTags(rows), nil
}

func (r tagRepo) SetTags(ctx context.Context, tx domain.TransactionID, ids []domain.TagID) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	if _, err := r.repos.queries.GetTransaction(ctx, int64(tx)); err != nil {
		return mapNotFound(mapSQLError(err), fmt.Sprintf("transaction %d", tx))
	}
	if err := r.repos.queries.ClearTransactionTags(ctx, int64(tx)); err != nil {
		return mapSQLError(err)
	}
	seen := map[domain.TagID]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		if _, err := r.repos.queries.GetTag(ctx, int64(id)); err != nil {
			return mapNotFound(mapSQLError(err), fmt.Sprintf("tag %d", id))
		}
		if err := r.repos.queries.AddTransactionTag(ctx, gen.AddTransactionTagParams{TransactionID: int64(tx), TagID: int64(id)}); err != nil {
			return mapConstraint(err)
		}
	}
	return nil
}

func mapTags(rows []gen.Tag) []domain.Tag {
	out := make([]domain.Tag, 0, len(rows))
	for _, row := range rows {
		out = append(out, toTag(row))
	}
	return out
}

var _ app.TagRepo = tagRepo{}
