package reporting

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
)

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
	if err != nil {
		return domain.Money{}, fmt.Errorf("fx rate %s->%s on %s: %w", amount.Currency(), base, at, err)
	}
	converted, err := domain.Convert(amount, rate, base)
	if err != nil {
		return domain.Money{}, fmt.Errorf("convert %s to %s on %s: %w", amount.Currency(), base, at, err)
	}
	return converted, nil
}
