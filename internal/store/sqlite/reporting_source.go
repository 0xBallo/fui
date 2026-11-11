package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"
)

// ListAccounts implements reporting.Source.
func (s *Store) ListAccounts(ctx context.Context, filter reporting.AccountFilter) ([]domain.Account, error) {
	accountIDsJSON, err := encodeIDSlice(filter.AccountIDs)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	db := s.readDB
	s.mu.RUnlock()

	q := gen.New(db)
	rows, err := q.ListAccountsForReport(ctx, gen.ListAccountsForReportParams{
		IncludeArchived: boolToInt64(filter.IncludeArchived),
		AccountIdsJson:  accountIDsJSON,
	})
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

// ListCategories implements reporting.Source.
func (s *Store) ListCategories(ctx context.Context, categoryIDs []domain.CategoryID) ([]domain.Category, error) {
	idsJSON, err := encodeIDSlice(categoryIDs)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	db := s.readDB
	s.mu.RUnlock()

	q := gen.New(db)
	rows, err := q.ListCategoriesByIDs(ctx, idsJSON)
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

// ListTransactions implements reporting.Source.
func (s *Store) ListTransactions(ctx context.Context, filter reporting.TransactionFilter) ([]domain.Transaction, error) {
	accountIDsJSON, err := encodeIDSlice(filter.AccountIDs)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	db := s.readDB
	s.mu.RUnlock()

	q := gen.New(db)
	rows, err := q.ListTransactionsForReport(ctx, gen.ListTransactionsForReportParams{
		FromDate:       filter.Period.From().String(),
		ToDate:         filter.Period.To().String(),
		AccountIdsJson: accountIDsJSON,
	})
	if err != nil {
		return nil, mapSQLError(err)
	}
	return mapTransactions(rows)
}

// BalanceAt implements reporting.Source.
func (s *Store) BalanceAt(ctx context.Context, accountID domain.AccountID, date domain.Date) (domain.Money, error) {
	s.mu.RLock()
	db := s.readDB
	s.mu.RUnlock()

	q := gen.New(db)
	row, err := q.BalanceAt(ctx, gen.BalanceAtParams{
		AccountID: int64(accountID),
		Date:      date.String(),
	})
	if err != nil {
		return domain.Money{}, mapNotFound(mapSQLError(err), "reporting balance at")
	}
	cur, err := domain.ParseCurrency(row.Currency)
	if err != nil {
		return domain.Money{}, err
	}
	money, err := domain.NewMoney(row.BalanceMinor, cur)
	if err != nil {
		return domain.Money{}, err
	}
	return money, nil
}

// FXRateOnOrBefore implements reporting.Source.
func (s *Store) FXRateOnOrBefore(ctx context.Context, from domain.Currency, to domain.Currency, date domain.Date) (domain.Rate, error) {
	if from == to {
		return domain.NewRate(domain.RateScale)
	}

	fromRate, fromFound, err := s.findRateToBase(ctx, from, date)
	if err != nil {
		return 0, err
	}
	toRate, toFound, err := s.findRateToBase(ctx, to, date)
	if err != nil {
		return 0, err
	}

	switch {
	case !fromFound && !toFound:
		return 0, fmt.Errorf("fx %s->%s on %s: %w", from, to, date, domain.ErrNotFound)
	case fromFound && !toFound:
		return fromRate, nil
	case !fromFound && toFound:
		return invertRate(toRate)
	default:
		return divideRates(fromRate, toRate)
	}
}

func (s *Store) findRateToBase(ctx context.Context, currency domain.Currency, date domain.Date) (domain.Rate, bool, error) {
	s.mu.RLock()
	db := s.readDB
	s.mu.RUnlock()

	q := gen.New(db)
	row, err := q.FXRateOnOrBefore(ctx, gen.FXRateOnOrBeforeParams{
		Currency: currency.Code(),
		Date:     date.String(),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			rate, mkErr := domain.NewRate(domain.RateScale)
			if mkErr != nil {
				return 0, false, mkErr
			}
			return rate, false, nil
		}
		return 0, false, mapSQLError(err)
	}
	rate, err := domain.NewRate(row.RateScaled)
	if err != nil {
		return 0, false, err
	}
	return rate, true, nil
}

func invertRate(rate domain.Rate) (domain.Rate, error) {
	num := new(big.Int).Mul(big.NewInt(domain.RateScale), big.NewInt(domain.RateScale))
	den := big.NewInt(rate.Scaled())
	scaled, err := domain.RoundDiv(num, den)
	if err != nil {
		return 0, err
	}
	return domain.NewRate(scaled)
}

func divideRates(fromRate, toRate domain.Rate) (domain.Rate, error) {
	num := new(big.Int).Mul(big.NewInt(fromRate.Scaled()), big.NewInt(domain.RateScale))
	den := big.NewInt(toRate.Scaled())
	scaled, err := domain.RoundDiv(num, den)
	if err != nil {
		return 0, err
	}
	return domain.NewRate(scaled)
}
