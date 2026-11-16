package memstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/testutil/memstore"
)

func TestSavedSimulationsAndDismissedInsights(t *testing.T) {
	store := memstore.New()
	ctx := context.Background()

	var sim app.SavedSimulation
	if err := store.WithTx(ctx, func(r app.Repos) error {
		var err error
		sim, err = r.SavedSimulations().Insert(ctx, app.SavedSimulation{
			Name:       "Plan A",
			Kind:       "investment",
			ParamsJSON: `{"years":20}`,
		})
		if err != nil {
			return err
		}
		if _, err := r.SavedSimulations().Insert(ctx, app.SavedSimulation{
			Name:       " plan a ",
			Kind:       "pac",
			ParamsJSON: `{}`,
		}); !errors.Is(err, app.ErrAlreadyExists) {
			t.Fatalf("duplicate name err = %v, want already_exists", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("insert saved simulation: %v", err)
	}

	if err := store.Read(ctx, func(r app.Repos) error {
		got, err := r.SavedSimulations().GetByName(ctx, "plan a")
		if err != nil || got.ID != sim.ID {
			t.Fatalf("GetByName = %+v, %v", got, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	dismissedAt := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	if err := store.WithTx(ctx, func(r app.Repos) error {
		if err := r.DismissedInsights().Dismiss(ctx, app.DismissedInsight{
			Key:         "k1",
			Kind:        domain.InsightSubscript,
			DismissedAt: dismissedAt,
		}); err != nil {
			return err
		}
		return r.DismissedInsights().Dismiss(ctx, app.DismissedInsight{
			Key:         "k1",
			Kind:        domain.InsightPriceChange,
			DismissedAt: dismissedAt.Add(time.Hour),
		})
	}); err != nil {
		t.Fatalf("dismiss insight: %v", err)
	}

	if err := store.Read(ctx, func(r app.Repos) error {
		list, err := r.DismissedInsights().List(ctx)
		if err != nil {
			return err
		}
		if len(list) != 1 || list[0].Kind != domain.InsightPriceChange {
			t.Fatalf("dismissed list = %+v", list)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := store.WithTx(ctx, func(r app.Repos) error {
		if err := r.SavedSimulations().Delete(ctx, sim.ID); err != nil {
			return err
		}
		if err := r.SavedSimulations().Delete(ctx, sim.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("delete missing simulation err = %v, want not_found", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
