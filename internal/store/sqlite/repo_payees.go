package sqlite

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"
	"github.com/0xballo/fui/internal/textnorm"
)

type payeeRepo struct {
	repos *repos
}

func (r payeeRepo) Insert(ctx context.Context, p domain.Payee) (domain.Payee, error) {
	if err := r.repos.writable(); err != nil {
		return domain.Payee{}, err
	}
	row, err := r.repos.queries.InsertPayee(ctx, p.Name)
	if err != nil {
		return domain.Payee{}, mapConstraint(err)
	}
	if err := r.setAliases(ctx, row.ID, p.Aliases); err != nil {
		return domain.Payee{}, err
	}
	return domain.Payee{ID: domain.PayeeID(row.ID), Name: row.Name, Aliases: append([]string(nil), p.Aliases...)}, nil
}

func (r payeeRepo) setAliases(ctx context.Context, id int64, aliases []string) error {
	if err := r.repos.queries.ClearPayeeAliases(ctx, id); err != nil {
		return mapSQLError(err)
	}
	for _, a := range aliases {
		if err := r.repos.queries.AddPayeeAlias(ctx, gen.AddPayeeAliasParams{PayeeID: id, Alias: a}); err != nil {
			return mapConstraint(err)
		}
	}
	return nil
}

func (r payeeRepo) Update(ctx context.Context, p domain.Payee) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	changed, err := r.repos.queries.UpdatePayee(ctx, gen.UpdatePayeeParams{ID: int64(p.ID), Name: p.Name})
	if err != nil {
		return mapConstraint(err)
	}
	if changed == 0 {
		return fmt.Errorf("payee %d: %w", p.ID, domain.ErrNotFound)
	}
	if err := r.setAliases(ctx, int64(p.ID), p.Aliases); err != nil {
		return err
	}
	// The payee name is indexed in the search text of its transactions.
	txs, err := r.repos.queries.ListTransactionsOfPayee(ctx, toNullInt64(p.ID))
	if err != nil {
		return mapSQLError(err)
	}
	for _, row := range txs {
		text := textnorm.SearchText(row.Description, p.Name, row.Notes)
		if err := r.repos.queries.SetTransactionSearchText(ctx, gen.SetTransactionSearchTextParams{ID: row.ID, SearchText: text}); err != nil {
			return mapSQLError(err)
		}
	}
	return nil
}

func (r payeeRepo) Delete(ctx context.Context, id domain.PayeeID) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	changed, err := r.repos.queries.DeletePayee(ctx, int64(id))
	if err != nil {
		return fmt.Errorf("payee %d: %w", id, mapSQLError(err))
	}
	if changed == 0 {
		return fmt.Errorf("payee %d: %w", id, domain.ErrNotFound)
	}
	return nil
}

func (r payeeRepo) Get(ctx context.Context, id domain.PayeeID) (domain.Payee, error) {
	row, err := r.repos.queries.GetPayee(ctx, int64(id))
	if err != nil {
		return domain.Payee{}, mapNotFound(mapSQLError(err), fmt.Sprintf("payee %d", id))
	}
	aliases, err := r.repos.queries.PayeeAliases(ctx, row.ID)
	if err != nil {
		return domain.Payee{}, mapSQLError(err)
	}
	p := domain.Payee{ID: domain.PayeeID(row.ID), Name: row.Name}
	for _, a := range aliases {
		p.Aliases = append(p.Aliases, a.Alias)
	}
	return p, nil
}

func (r payeeRepo) List(ctx context.Context) ([]domain.Payee, error) {
	rows, err := r.repos.queries.ListPayees(ctx)
	if err != nil {
		return nil, mapSQLError(err)
	}
	aliases, err := r.repos.queries.ListPayeeAliases(ctx)
	if err != nil {
		return nil, mapSQLError(err)
	}
	byPayee := map[int64][]string{}
	for _, a := range aliases {
		byPayee[a.PayeeID] = append(byPayee[a.PayeeID], a.Alias)
	}
	out := make([]domain.Payee, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Payee{ID: domain.PayeeID(row.ID), Name: row.Name, Aliases: byPayee[row.ID]})
	}
	return out, nil
}

var _ app.PayeeRepo = payeeRepo{}
