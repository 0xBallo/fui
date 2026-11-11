package sqlite

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"
)

type fxRateRepo struct {
	repos *repos
}

func (r fxRateRepo) Upsert(ctx context.Context, fx domain.FXRate) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	err := r.repos.queries.UpsertFXRate(ctx, gen.UpsertFXRateParams{
		Currency:   fx.Currency.Code(),
		Date:       fx.Date.String(),
		RateScaled: fx.Rate.Scaled(),
	})
	if err != nil {
		return mapConstraint(err)
	}
	return nil
}

func (r fxRateRepo) List(ctx context.Context, currency domain.Currency) ([]domain.FXRate, error) {
	code := ""
	if !currency.IsZero() {
		code = currency.Code()
	}
	rows, err := r.repos.queries.ListFXRates(ctx, code)
	if err != nil {
		return nil, mapSQLError(err)
	}
	out := make([]domain.FXRate, 0, len(rows))
	for _, row := range rows {
		fx, convErr := toFXRate(row)
		if convErr != nil {
			return nil, convErr
		}
		out = append(out, fx)
	}
	return out, nil
}

func (r fxRateRepo) OnOrBefore(ctx context.Context, currency domain.Currency, date domain.Date) (domain.FXRate, error) {
	row, err := r.repos.queries.FXRateOnOrBefore(ctx, gen.FXRateOnOrBeforeParams{
		Currency: currency.Code(),
		Date:     date.String(),
	})
	if err != nil {
		return domain.FXRate{}, mapNotFound(mapSQLError(err), "fx rate on or before")
	}
	fx, convErr := toFXRate(row)
	if convErr != nil {
		return domain.FXRate{}, fmt.Errorf("fx rate on or before: %w", convErr)
	}
	return fx, nil
}
