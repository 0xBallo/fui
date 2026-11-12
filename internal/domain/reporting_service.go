package domain

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// RateLookup returns the rate from one currency to another on a date.
type RateLookup func(from Currency, to Currency, at Date) (Rate, error)

const codeRateMissing Code = "rate_missing"

// CategoryTotal is one line of spending grouped by category.
type CategoryTotal struct {
	CategoryID    CategoryID
	Uncategorized bool
	Total         Money
}

// CashFlowMonth is one accounting-month bucket in the cash-flow report.
type CashFlowMonth struct {
	Period  Period
	Inflow  Money
	Outflow Money
	Net     Money
}

// SpendingByCategory groups outflows in period by category, in base currency.
func SpendingByCategory(period Period, transactions []Transaction, base Currency, rateLookup RateLookup) ([]CategoryTotal, Money, error) {
	if base.IsZero() {
		return nil, Money{}, missingField("currency")
	}
	totals := map[CategoryID]Money{}
	total := Money{cur: base}
	missing := newMissingRateTracker()
	for _, transaction := range transactions {
		include, converted, err := includeAndConvert(transaction, period, base, rateLookup)
		if err != nil {
			if isRateMissing(err) {
				missing.add(transaction.Amount.Currency(), transaction.Date)
				continue
			}
			return nil, Money{}, err
		}
		if !include || converted.Sign() >= 0 {
			continue
		}
		abs, err := converted.Neg()
		if err != nil {
			return nil, Money{}, err
		}
		key := transaction.CategoryID
		current := totals[key]
		if current.Currency().IsZero() {
			current = Money{cur: base}
		}
		current, err = current.Add(abs)
		if err != nil {
			return nil, Money{}, err
		}
		totals[key] = current
		total, err = total.Add(abs)
		if err != nil {
			return nil, Money{}, err
		}
	}
	if missing.hasAny() {
		return nil, Money{}, missing.err()
	}
	return flattenCategoryTotals(totals), total, nil
}

// CashFlowByMonth groups inflow/outflow/net by accounting month in period.
func CashFlowByMonth(period Period, monthStartDay int, transactions []Transaction, base Currency, rateLookup RateLookup) ([]CashFlowMonth, error) {
	if base.IsZero() {
		return nil, missingField("currency")
	}
	months, err := buildCashFlowMonths(period, monthStartDay, base)
	if err != nil {
		return nil, err
	}
	missing := newMissingRateTracker()
	for _, transaction := range transactions {
		include, converted, err := includeAndConvert(transaction, period, base, rateLookup)
		if err != nil {
			if isRateMissing(err) {
				missing.add(transaction.Amount.Currency(), transaction.Date)
				continue
			}
			return nil, err
		}
		if !include {
			continue
		}
		month, err := MonthPeriod(transaction.Date, monthStartDay)
		if err != nil {
			return nil, err
		}
		line := months[month.From()]
		line.Net, err = line.Net.Add(converted)
		if err != nil {
			return nil, err
		}
		if converted.Sign() >= 0 {
			line.Inflow, err = line.Inflow.Add(converted)
		} else {
			out, negErr := converted.Neg()
			if negErr != nil {
				return nil, negErr
			}
			line.Outflow, err = line.Outflow.Add(out)
		}
		if err != nil {
			return nil, err
		}
		months[month.From()] = line
	}
	if missing.hasAny() {
		return nil, missing.err()
	}
	return flattenCashFlow(months), nil
}

func includeAndConvert(transaction Transaction, period Period, base Currency, rateLookup RateLookup) (bool, Money, error) {
	if transaction.Date.IsZero() {
		return false, Money{}, missingField("date")
	}
	if transaction.Amount.Currency().IsZero() {
		return false, Money{}, missingField("amount")
	}
	if transaction.Kind == KindTransfer || transaction.Kind == KindInvestment || !period.Contains(transaction.Date) {
		return false, Money{}, nil
	}
	converted, err := convertToBase(transaction.Amount, transaction.Date, base, rateLookup)
	if err != nil {
		return false, Money{}, err
	}
	return true, converted, nil
}

func convertToBase(amount Money, at Date, base Currency, rateLookup RateLookup) (Money, error) {
	if amount.Currency() == base {
		return amount, nil
	}
	if rateLookup == nil {
		return Money{}, missingField("rate_lookup")
	}
	rate, err := rateLookup(amount.Currency(), base, at)
	if err != nil {
		return Money{}, fmt.Errorf("lookup rate %s->%s on %s: %w", amount.Currency(), base, at, err)
	}
	return Convert(amount, rate, base)
}

func flattenCategoryTotals(totals map[CategoryID]Money) []CategoryTotal {
	rows := make([]CategoryTotal, 0, len(totals))
	for categoryID, total := range totals {
		rows = append(rows, CategoryTotal{
			CategoryID:    categoryID,
			Uncategorized: categoryID == 0,
			Total:         total,
		})
	}
	slices.SortFunc(rows, func(a, b CategoryTotal) int {
		if a.Uncategorized != b.Uncategorized {
			if a.Uncategorized {
				return 1
			}
			return -1
		}
		if a.Total.Minor() != b.Total.Minor() {
			if a.Total.Minor() > b.Total.Minor() {
				return -1
			}
			return 1
		}
		if a.CategoryID < b.CategoryID {
			return -1
		}
		if a.CategoryID > b.CategoryID {
			return 1
		}
		return 0
	})
	return rows
}

func buildCashFlowMonths(period Period, monthStartDay int, base Currency) (map[Date]CashFlowMonth, error) {
	months := map[Date]CashFlowMonth{}
	current, err := MonthPeriod(period.From(), monthStartDay)
	if err != nil {
		return nil, err
	}
	for {
		key := current.From()
		months[key] = CashFlowMonth{
			Period:  current,
			Inflow:  Money{cur: base},
			Outflow: Money{cur: base},
			Net:     Money{cur: base},
		}
		if !current.To().Before(period.To()) {
			return months, nil
		}
		nextStart, err := current.To().AddDays(1)
		if err != nil {
			return nil, err
		}
		current, err = MonthPeriod(nextStart, monthStartDay)
		if err != nil {
			return nil, err
		}
	}
}

func flattenCashFlow(months map[Date]CashFlowMonth) []CashFlowMonth {
	keys := make([]Date, 0, len(months))
	for key := range months {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b Date) int { return a.Cmp(b) })
	rows := make([]CashFlowMonth, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, months[key])
	}
	return rows
}

func isRateMissing(err error) bool {
	var domainErr *Error
	return errors.As(err, &domainErr) && domainErr.Code == codeRateMissing
}

type missingRateTracker struct {
	earliestByCurrency map[Currency]Date
}

func newMissingRateTracker() missingRateTracker {
	return missingRateTracker{earliestByCurrency: map[Currency]Date{}}
}

func (tracker missingRateTracker) hasAny() bool {
	return len(tracker.earliestByCurrency) > 0
}

func (tracker missingRateTracker) add(currency Currency, date Date) {
	if date.IsZero() || currency.IsZero() {
		return
	}
	earliest, ok := tracker.earliestByCurrency[currency]
	if !ok || date.Before(earliest) {
		tracker.earliestByCurrency[currency] = date
	}
}

func (tracker missingRateTracker) err() error {
	codes := make([]string, 0, len(tracker.earliestByCurrency))
	currencyDates := make(map[string]string, len(tracker.earliestByCurrency))
	earliest := Date{}
	for currency, date := range tracker.earliestByCurrency {
		code := currency.Code()
		codes = append(codes, code)
		currencyDates[code] = date.String()
		if earliest.IsZero() || date.Before(earliest) {
			earliest = date
		}
	}
	slices.Sort(codes)
	verb, hintCurrency := "rate", codes[0]
	if len(codes) > 1 {
		verb, hintCurrency = "rates", "<CURRENCY>"
	}
	return &Error{
		Code: codeRateMissing,
		Message: fmt.Sprintf(
			"missing exchange %s for %s on %s (set one with: fui fx set %s <rate> --date %s)",
			verb, strings.Join(codes, ", "), earliest, hintCurrency, earliest,
		),
		Details: map[string]any{
			"currencies":     codes,
			"date":           earliest.String(),
			"currency_dates": currencyDates,
		},
	}
}
