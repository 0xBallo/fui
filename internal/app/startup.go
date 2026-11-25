package app

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// CatchupSource is a use case that can materialise what became due since the last run (pending
// recurring occurrences, pending loan installments). It is defined here, by its consumer, so app
// does not import the packages that implement it.
type CatchupSource interface {
	// CatchUp creates everything due through today and returns how many transactions it created.
	// It must be idempotent and a no-op when its module is not enabled.
	CatchUp(ctx context.Context, enabled module.Set, today domain.Date) (int, error)
}

// Catchup runs every source in order and returns the total number of transactions created. It
// stops at the first failing source, returning what was created so far with the error.
func Catchup(ctx context.Context, enabled module.Set, today domain.Date, srcs ...CatchupSource) (int, error) {
	total := 0
	for _, src := range srcs {
		n, err := src.CatchUp(ctx, enabled, today)
		total += n
		if err != nil {
			return total, fmt.Errorf("catch up: %w", err)
		}
	}
	return total, nil
}
