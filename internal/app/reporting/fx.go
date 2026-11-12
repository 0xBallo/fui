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
		return deps.source.FXRateOnOrBefore(ctx, from, to, at)
	}
}

func (useCase Runway) convertAt(ctx context.Context, amount domain.Money, base domain.Currency, at domain.Date) (domain.Money, error) {
	if amount.Currency() == base {
		return amount, nil
	}
	rate, err := useCase.deps.source.FXRateOnOrBefore(ctx, amount.Currency(), base, at)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Money{}, missingRates([]domain.Currency{amount.Currency()}, at)
	}
	if err != nil {
		return domain.Money{}, fmt.Errorf("fx rate %s->%s on %s: %w", amount.Currency(), base, at, err)
	}
	converted, err := domain.Convert(amount, rate, base)
	if err != nil {
		return domain.Money{}, fmt.Errorf("convert %s to %s on %s: %w", amount.Currency(), base, at, err)
	}
	return converted, nil
}

func missingRates(currencies []domain.Currency, date domain.Date) error {
	codes := make([]string, len(currencies))
	for i, currency := range currencies {
		codes[i] = currency.Code()
	}
	slices.Sort(codes)
	verb, hintCurrency := "rate", codes[0]
	if len(codes) > 1 {
		verb, hintCurrency = "rates", "<CURRENCY>"
	}
	message := fmt.Sprintf(
		"missing exchange %s for %s on %s (set one with: fui fx set %s <rate> --date %s)",
		verb, strings.Join(codes, ", "), date, hintCurrency, date,
	)
	return &domain.Error{
		Code:    errRateMissing.Code,
		Message: message,
		Details: map[string]any{"currencies": codes, "date": date.String()},
	}
}
