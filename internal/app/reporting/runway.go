package reporting

import (
	"context"
	"errors"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
)

// Runway computes the runway KPI.
type Runway struct {
	deps      dependencies
	positions PositionsValuer
}

// PositionsValuer values the security positions at a date in the base currency.
type PositionsValuer interface {
	// PositionsValue returns the market value of the priced positions at date.
	PositionsValue(ctx context.Context, at domain.Date) (domain.Money, error)
}

// WithPositions returns a Runway able to honour RunwayParams.IncludeInvestments.
func (useCase Runway) WithPositions(valuer PositionsValuer) Runway {
	useCase.positions = valuer
	return useCase
}

// NewRunway creates a Runway use case.
func NewRunway(source Source, clock Clock) (Runway, error) {
	deps, err := validateDependencies(source, clock)
	if err != nil {
		return Runway{}, err
	}
	return Runway{deps: deps}, nil
}

// RunwayParams are the query parameters.
type RunwayParams struct {
	AsOf          domain.Date
	MonthStartDay int
	AccountIDs    []domain.AccountID
	BaseCurrency  domain.Currency
	// IncludeInvestments adds the positions value to the liquidity; it requires WithPositions.
	IncludeInvestments bool
}

// RunwayResult is the structured runway KPI.
type RunwayResult struct {
	AsOf                     domain.Date
	Currency                 domain.Currency
	Liquidity                domain.Money
	ReferenceMonthlySpending domain.Money
	Months                   int
	Days                     int
	ZeroDate                 domain.Date
}

// Run executes the runway KPI computation.
func (useCase Runway) Run(ctx context.Context, params RunwayParams) (RunwayResult, error) {
	asOf, err := useCase.resolveAsOf(params.AsOf)
	if err != nil {
		return RunwayResult{}, err
	}
	balances, missingFromBalances, err := useCase.loadBalances(ctx, asOf, params)
	if err != nil {
		return RunwayResult{}, err
	}
	movements, missingFromMovements, err := useCase.loadMovements(ctx, asOf, params)
	if err != nil {
		return RunwayResult{}, err
	}
	balances, err = useCase.withPositions(ctx, asOf, params, balances)
	if err != nil {
		return RunwayResult{}, err
	}
	missing := mergeMissingRates(missingFromBalances, missingFromMovements)
	if len(missing) > 0 {
		return RunwayResult{}, missingRates(missing)
	}
	computed, err := domain.Runway(asOf, params.MonthStartDay, balances, movements)
	if err != nil {
		return RunwayResult{}, fmt.Errorf("compute runway: %w", err)
	}
	return RunwayResult{
		AsOf:                     asOf,
		Currency:                 params.BaseCurrency,
		Liquidity:                computed.Liquidity,
		ReferenceMonthlySpending: computed.ReferenceMonthlySpending,
		Months:                   computed.Months,
		Days:                     computed.Days,
		ZeroDate:                 computed.ZeroDate,
	}, nil
}

func (useCase Runway) withPositions(ctx context.Context, asOf domain.Date, params RunwayParams, balances []domain.RunwayBalance) ([]domain.RunwayBalance, error) {
	if !params.IncludeInvestments {
		return balances, nil
	}
	if useCase.positions == nil {
		return nil, fmt.Errorf("missing positions valuer: %w", domain.ErrMissingField)
	}
	value, err := useCase.positions.PositionsValue(ctx, asOf)
	if err != nil {
		return nil, fmt.Errorf("positions value: %w", err)
	}
	// Positions are liquid assets for the KPI, so they count like a savings balance.
	return append(balances, domain.RunwayBalance{Kind: domain.AccountSavings, Balance: value}), nil
}

func (useCase Runway) resolveAsOf(asOf domain.Date) (domain.Date, error) {
	if !asOf.IsZero() {
		return asOf, nil
	}
	now := useCase.deps.clock.Now()
	today, err := domain.FromTime(now)
	if err != nil {
		return domain.Date{}, fmt.Errorf("resolve as-of date: %w", err)
	}
	return today, nil
}

func (useCase Runway) loadBalances(ctx context.Context, asOf domain.Date, params RunwayParams) ([]domain.RunwayBalance, map[domain.Currency]domain.Date, error) {
	accounts, err := useCase.deps.source.ListAccounts(ctx, AccountFilter{
		AccountIDs:      params.AccountIDs,
		IncludeArchived: false,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list accounts: %w", err)
	}
	items := make([]domain.RunwayBalance, 0, len(accounts))
	missing := map[domain.Currency]domain.Date{}
	for _, account := range accounts {
		balance, err := useCase.deps.source.BalanceAt(ctx, account.ID, asOf)
		if err != nil {
			return nil, nil, fmt.Errorf("balance of account %d: %w", account.ID, err)
		}
		converted, err := useCase.convertAt(ctx, balance, params.BaseCurrency, asOf)
		if errors.Is(err, errRateMissing) {
			addMissingRate(missing, account.Currency, asOf)
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		items = append(items, domain.RunwayBalance{
			Kind:    account.Kind,
			Balance: converted,
		})
	}
	return items, missing, nil
}

func (useCase Runway) loadMovements(ctx context.Context, asOf domain.Date, params RunwayParams) ([]domain.RunwayMovement, map[domain.Currency]domain.Date, error) {
	period, err := runwayPeriod(asOf, params.MonthStartDay)
	if err != nil {
		return nil, nil, fmt.Errorf("runway period: %w", err)
	}
	transactions, err := useCase.deps.source.ListTransactions(ctx, TransactionFilter{
		Period:     period,
		AccountIDs: params.AccountIDs,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list transactions: %w", err)
	}
	movements := make([]domain.RunwayMovement, 0, len(transactions))
	missing := map[domain.Currency]domain.Date{}
	for _, tx := range transactions {
		converted, err := useCase.convertAt(ctx, tx.Amount, params.BaseCurrency, tx.Date)
		if errors.Is(err, errRateMissing) {
			addMissingRate(missing, tx.Amount.Currency(), tx.Date)
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		movements = append(movements, domain.RunwayMovement{
			Date:   tx.Date,
			Amount: converted,
			Kind:   tx.Kind,
		})
	}
	return movements, missing, nil
}

func runwayPeriod(asOf domain.Date, monthStartDay int) (domain.Period, error) {
	current, err := domain.MonthPeriod(asOf, monthStartDay)
	if err != nil {
		return domain.Period{}, err
	}
	end, err := current.From().AddDays(-1)
	if err != nil {
		return domain.Period{}, err
	}
	start, err := current.From().AddMonths(-12)
	if err != nil {
		return domain.Period{}, err
	}
	return domain.NewPeriod(start, end)
}

func mergeMissingRates(first, second map[domain.Currency]domain.Date) map[domain.Currency]domain.Date {
	merged := make(map[domain.Currency]domain.Date, len(first)+len(second))
	for currency, date := range first {
		merged[currency] = date
	}
	for currency, date := range second {
		addMissingRate(merged, currency, date)
	}
	return merged
}

func addMissingRate(missing map[domain.Currency]domain.Date, currency domain.Currency, date domain.Date) {
	current, ok := missing[currency]
	if !ok || date.Before(current) {
		missing[currency] = date
	}
}
