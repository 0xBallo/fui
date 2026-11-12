package sqlite

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"

	msqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

type repos struct {
	queries  *gen.Queries
	readOnly bool
}

func newRepos(tx *sql.Tx, readOnly bool) *repos {
	return &repos{
		queries:  newQueries(tx),
		readOnly: readOnly,
	}
}

func (r *repos) Accounts() app.AccountRepo         { return accountRepo{repos: r} }
func (r *repos) Transactions() app.TransactionRepo { return transactionRepo{repos: r} }
func (r *repos) Categories() app.CategoryRepo      { return categoryRepo{repos: r} }
func (r *repos) Rules() app.RuleRepo               { return ruleRepo{repos: r} }
func (r *repos) Transfers() app.TransferRepo       { return transferRepo{repos: r} }
func (r *repos) FXRates() app.FXRateRepo           { return fxRateRepo{repos: r} }
func (r *repos) Imports() app.ImportRepo           { return importRepo{repos: r} }
func (r *repos) Tags() app.TagRepo                 { return tagRepo{repos: r} }
func (r *repos) Payees() app.PayeeRepo             { return payeeRepo{repos: r} }
func (r *repos) Splits() app.SplitRepo             { return splitRepo{repos: r} }
func (r *repos) Budgets() app.BudgetRepo           { return budgetRepo{repos: r} }

func (r *repos) writable() error {
	if r.readOnly {
		return errReadOnly
	}
	return nil
}

func mapNotFound(err error, what string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", what, domain.ErrNotFound)
	}
	return err
}

func mapConstraint(err error) error {
	if err == nil {
		return nil
	}
	mapped := mapSQLError(err)
	if errors.Is(mapped, app.ErrAlreadyExists) || errors.Is(mapped, app.ErrInUse) {
		return mapped
	}
	var sqlErr *msqlite.Error
	if errors.As(err, &sqlErr) && sqlErr.Code() == sqlite3.SQLITE_CONSTRAINT {
		return app.ErrAlreadyExists
	}
	return mapped
}
