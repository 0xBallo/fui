package reporting

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/0xballo/fui/internal/domain"
)

const codeRateMissing domain.Code = "rate_missing"

var errRateMissing = &domain.Error{Code: codeRateMissing, Message: "exchange rate missing"}

func (deps dependencies) fxLookup(ctx context.Context) domain.RateLookup {
	return func(from domain.Currency, to domain.Currency, at domain.Date) (domain.Rate, error) {
		return deps.lookupRate(ctx, from, to, at)
	}
}

func (deps dependencies) lookupRate(ctx context.Context, from, to domain.Currency, at domain.Date) (domain.Rate, error) {
	rate, err := deps.source.FXRateOnOrBefore(ctx, from, to, at)
	if errors.Is(err, domain.ErrNotFound) {
		return 0, missingRates(map[domain.Currency]domain.Date{from: at})
	}
	if err != nil {
		return 0, fmt.Errorf("fx rate %s->%s on %s: %w", from, to, at, err)
	}
	return rate, nil
}

func (useCase Runway) convertAt(ctx context.Context, amount domain.Money, base domain.Currency, at domain.Date) (domain.Money, error) {
	if amount.Currency() == base {
		return amount, nil
	}
	rate, err := useCase.deps.lookupRate(ctx, amount.Currency(), base, at)
	if err != nil {
		return domain.Money{}, err
	}
	converted, err := domain.Convert(amount, rate, base)
	if err != nil {
		return domain.Money{}, fmt.Errorf("convert %s to %s on %s: %w", amount.Currency(), base, at, err)
	}
	return converted, nil
}

func missingRates(missing map[domain.Currency]domain.Date) error {
	codes := make([]string, 0, len(missing))
	earliest := domain.Date{}
	currencyDates := make(map[string]string, len(missing))
	for currency, date := range missing {
		codes = append(codes, currency.Code())
		currencyDates[currency.Code()] = date.String()
		if earliest.IsZero() || date.Before(earliest) {
			earliest = date
		}
	}
	slices.Sort(codes)
	verb, hintCurrency := "rate", codes[0]
	if len(codes) > 1 {
		verb, hintCurrency = "rates", "<CURRENCY>"
	}
	message := fmt.Sprintf(
		"missing exchange %s for %s on %s (set one with: fui fx set %s <rate> --date %s)",
		verb, strings.Join(codes, ", "), earliest, hintCurrency, earliest,
	)
	return &domain.Error{
		Code:    errRateMissing.Code,
		Message: message,
		Details: map[string]any{
			"currencies":     codes,
			"date":           earliest.String(),
			"currency_dates": currencyDates,
		},
	}
}
