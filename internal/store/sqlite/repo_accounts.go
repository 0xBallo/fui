package sqlite

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"
)

type accountRepo struct {
	repos *repos
}

func (r accountRepo) Insert(ctx context.Context, a domain.Account) (domain.Account, error) {
	if err := r.repos.writable(); err != nil {
		return domain.Account{}, err
	}

	row, err := r.repos.queries.InsertAccount(ctx, gen.InsertAccountParams{
		Name:                a.Name,
		Description:         a.Description,
		Kind:                string(a.Kind),
		Currency:            a.Currency.Code(),
		OpeningBalanceMinor: a.OpeningBalance.Minor(),
		OpenedOn:            a.OpenedOn.String(),
		Archived:            boolToInt64(a.Archived),
	})
	if err != nil {
		return domain.Account{}, mapConstraint(err)
	}
	acct, err := toAccount(row)
	if err != nil {
		return domain.Account{}, err
	}
	return acct, nil
}

func (r accountRepo) Update(ctx context.Context, a domain.Account) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	rows, err := r.repos.queries.UpdateAccount(ctx, gen.UpdateAccountParams{
		ID:                  int64(a.ID),
		Name:                a.Name,
		Description:         a.Description,
		Kind:                string(a.Kind),
		Currency:            a.Currency.Code(),
		OpeningBalanceMinor: a.OpeningBalance.Minor(),
		OpenedOn:            a.OpenedOn.String(),
		Archived:            boolToInt64(a.Archived),
	})
	if err != nil {
		return mapConstraint(err)
	}
	if rows == 0 {
		return fmt.Errorf("account %d: %w", a.ID, domain.ErrNotFound)
	}
	return nil
}

func (r accountRepo) Get(ctx context.Context, id domain.AccountID) (domain.Account, error) {
	row, err := r.repos.queries.GetAccount(ctx, int64(id))
	if err != nil {
		return domain.Account{}, mapNotFound(mapSQLError(err), "get account")
	}
	return toAccount(row)
}

func (r accountRepo) List(ctx context.Context) ([]domain.Account, error) {
	rows, err := r.repos.queries.ListAccounts(ctx)
	if err != nil {
		return nil, mapSQLError(err)
	}
	out := make([]domain.Account, 0, len(rows))
	for _, row := range rows {
		acct, convErr := toAccount(row)
		if convErr != nil {
			return nil, convErr
		}
		out = append(out, acct)
	}
	return out, nil
}

func boolToInt64(v bool) int64 {
	if v {
		return 1
	}
	return 0
}
