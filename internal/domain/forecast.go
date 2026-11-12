package domain

import (
	"slices"
	"strconv"
)

// ForecastOpening is one account opening balance at StartDate.
type ForecastOpening struct {
	AccountID AccountID
	Balance   Money
}

// ForecastFlow is one dated movement to apply during forecasting.
type ForecastFlow struct {
	AccountID AccountID
	Date      Date
	Amount    Money
}

// ForecastThreshold is the warning balance for one account.
type ForecastThreshold struct {
	AccountID AccountID
	Minimum   Money
}

// ForecastPoint is one projected day for one account.
type ForecastPoint struct {
	Date    Date
	Balance Money
}

// AccountForecast is the projection output for a single account.
type AccountForecast struct {
	AccountID            AccountID
	Series               []ForecastPoint
	FirstBelowZero       Date
	FirstBelowThreshold  Date
	HasFirstBelowZero    bool
	HasFirstBelowMinimum bool
}

// Forecast projects daily balances from start for horizonDays.
//
// horizonDays == 0 returns the current balance only. Flows on start are
// considered already reflected in opening balances; projected flows start from
// start+1 day.
func Forecast(start Date, horizonDays int, openings []ForecastOpening, flows []ForecastFlow, thresholds []ForecastThreshold) ([]AccountForecast, error) {
	if start.IsZero() {
		return nil, missingField("date")
	}
	if horizonDays < 0 {
		return nil, invalidValue("horizon_days", strconv.Itoa(horizonDays))
	}
	accounts, err := buildForecastAccounts(openings)
	if err != nil {
		return nil, err
	}
	if err := applyThresholds(accounts, thresholds); err != nil {
		return nil, err
	}
	flowByAccount, err := indexFlows(start, accounts, flows)
	if err != nil {
		return nil, err
	}
	return buildForecastSeries(start, horizonDays, accounts, flowByAccount)
}

type forecastAccountState struct {
	opening      Money
	hasThreshold bool
	threshold    Money
}

func buildForecastAccounts(openings []ForecastOpening) (map[AccountID]forecastAccountState, error) {
	accounts := make(map[AccountID]forecastAccountState, len(openings))
	for _, opening := range openings {
		if opening.AccountID == 0 {
			return nil, missingField("account_id")
		}
		if opening.Balance.Currency().IsZero() {
			return nil, missingField("balance")
		}
		if _, exists := accounts[opening.AccountID]; exists {
			return nil, invalidValue("account_id", "duplicate opening")
		}
		accounts[opening.AccountID] = forecastAccountState{opening: opening.Balance}
	}
	return accounts, nil
}

func applyThresholds(accounts map[AccountID]forecastAccountState, thresholds []ForecastThreshold) error {
	for _, threshold := range thresholds {
		if threshold.AccountID == 0 {
			return missingField("account_id")
		}
		state, ok := accounts[threshold.AccountID]
		if !ok {
			return invalidValue("account_id", "threshold for unknown account")
		}
		if threshold.Minimum.Currency().IsZero() {
			return missingField("minimum")
		}
		if threshold.Minimum.Currency() != state.opening.Currency() {
			return newError(ErrCurrencyMismatch, map[string]any{
				"left":  threshold.Minimum.Currency().Code(),
				"right": state.opening.Currency().Code(),
			}, "currency mismatch: %s vs %s", threshold.Minimum.Currency(), state.opening.Currency())
		}
		state.hasThreshold = true
		state.threshold = threshold.Minimum
		accounts[threshold.AccountID] = state
	}
	return nil
}

func indexFlows(start Date, accounts map[AccountID]forecastAccountState, flows []ForecastFlow) (map[AccountID]map[Date]Money, error) {
	flowByAccount := make(map[AccountID]map[Date]Money, len(accounts))
	for accountID := range accounts {
		flowByAccount[accountID] = map[Date]Money{}
	}
	for _, flow := range flows {
		if flow.AccountID == 0 {
			return nil, missingField("account_id")
		}
		if flow.Date.IsZero() {
			return nil, missingField("date")
		}
		if flow.Date.Cmp(start) <= 0 {
			continue
		}
		state, ok := accounts[flow.AccountID]
		if !ok {
			return nil, invalidValue("account_id", "flow for unknown account")
		}
		if flow.Amount.Currency().IsZero() {
			return nil, missingField("amount")
		}
		if flow.Amount.Currency() != state.opening.Currency() {
			return nil, newError(ErrCurrencyMismatch, map[string]any{
				"left":  flow.Amount.Currency().Code(),
				"right": state.opening.Currency().Code(),
			}, "currency mismatch: %s vs %s", flow.Amount.Currency(), state.opening.Currency())
		}
		bucket := flowByAccount[flow.AccountID]
		current, has := bucket[flow.Date]
		if !has {
			bucket[flow.Date] = flow.Amount
			continue
		}
		next, err := current.Add(flow.Amount)
		if err != nil {
			return nil, err
		}
		bucket[flow.Date] = next
	}
	return flowByAccount, nil
}

func buildForecastSeries(start Date, horizonDays int, accounts map[AccountID]forecastAccountState, flows map[AccountID]map[Date]Money) ([]AccountForecast, error) {
	accountIDs := make([]AccountID, 0, len(accounts))
	for accountID := range accounts {
		accountIDs = append(accountIDs, accountID)
	}
	slices.Sort(accountIDs)
	results := make([]AccountForecast, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		result, err := buildAccountForecast(start, horizonDays, accountID, accounts[accountID], flows[accountID])
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func buildAccountForecast(start Date, horizonDays int, accountID AccountID, state forecastAccountState, flowsByDate map[Date]Money) (AccountForecast, error) {
	series := make([]ForecastPoint, 0, horizonDays+1)
	balance := state.opening
	series = append(series, ForecastPoint{Date: start, Balance: balance})
	firstBelowZero, hasBelowZero := firstBelowThreshold(start, balance, Money{cur: balance.Currency()})
	firstBelowMinimum, hasBelowMinimum := Date{}, false
	if state.hasThreshold {
		firstBelowMinimum, hasBelowMinimum = firstBelowThreshold(start, balance, state.threshold)
	}
	day := start
	for range horizonDays {
		nextDay, err := day.AddDays(1)
		if err != nil {
			return AccountForecast{}, err
		}
		day = nextDay
		delta, has := flowsByDate[day]
		if has {
			balance, err = balance.Add(delta)
			if err != nil {
				return AccountForecast{}, err
			}
		}
		series = append(series, ForecastPoint{Date: day, Balance: balance})
		if !hasBelowZero {
			firstBelowZero, hasBelowZero = firstBelowThreshold(day, balance, Money{cur: balance.Currency()})
		}
		if state.hasThreshold && !hasBelowMinimum {
			firstBelowMinimum, hasBelowMinimum = firstBelowThreshold(day, balance, state.threshold)
		}
	}
	return AccountForecast{
		AccountID:            accountID,
		Series:               series,
		FirstBelowZero:       firstBelowZero,
		FirstBelowThreshold:  firstBelowMinimum,
		HasFirstBelowZero:    hasBelowZero,
		HasFirstBelowMinimum: hasBelowMinimum,
	}, nil
}

func firstBelowThreshold(day Date, balance, threshold Money) (Date, bool) {
	cmp, err := balance.Cmp(threshold)
	if err != nil || cmp >= 0 {
		return Date{}, false
	}
	return day, true
}
