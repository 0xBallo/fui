package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func TestSavedSimulationRepoCRUD(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	var first app.SavedSimulation
	if err := store.WithTx(ctx, func(r app.Repos) error {
		var err error
		first, err = r.SavedSimulations().Insert(ctx, app.SavedSimulation{
			Name:       "Retirement 2045",
			Kind:       "investment",
			ParamsJSON: `{"years":20}`,
		})
		if err != nil {
			return err
		}
		_, err = r.SavedSimulations().Insert(ctx, app.SavedSimulation{
			Name:       "retirement 2045",
			Kind:       "pac",
			ParamsJSON: `{}`,
		})
		if !errors.Is(err, app.ErrAlreadyExists) {
			t.Fatalf("insert duplicate simulation err = %v, want already_exists", err)
		}
		_, err = r.SavedSimulations().Insert(ctx, app.SavedSimulation{
			Name:       "Invalid",
			Kind:       "simulation",
			ParamsJSON: `{}`,
		})
		if !errors.Is(err, app.ErrAlreadyExists) {
			t.Fatalf("insert invalid kind err = %v, want already_exists", err)
		}
		list, err := r.SavedSimulations().List(ctx)
		if err != nil || len(list) != 1 || list[0].ID != first.ID {
			t.Fatalf("List = %+v, %v", list, err)
		}
		return nil
	}); err != nil {
		t.Fatalf("saved simulations flow: %v", err)
	}

	if err := store.Read(ctx, func(r app.Repos) error {
		got, err := r.SavedSimulations().GetByName(ctx, " retirement 2045 ")
		if err != nil || got.ID != first.ID || got.ParamsJSON != first.ParamsJSON {
			t.Fatalf("GetByName = %+v, %v", got, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := store.WithTx(ctx, func(r app.Repos) error {
		if err := r.SavedSimulations().Delete(ctx, first.ID); err != nil {
			return err
		}
		if err := r.SavedSimulations().Delete(ctx, first.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("delete missing simulation err = %v, want not_found", err)
		}
		if _, err := r.SavedSimulations().Get(ctx, first.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("get deleted simulation err = %v, want not_found", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestDismissedInsightRepoFlow(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	ts := time.Date(2026, 11, 2, 9, 30, 0, 123_000_000, time.UTC)

	if err := store.WithTx(ctx, func(r app.Repos) error {
		if err := r.DismissedInsights().Dismiss(ctx, app.DismissedInsight{
			Key:         "  abc123  ",
			Kind:        domain.InsightSubscript,
			DismissedAt: ts,
		}); err != nil {
			return err
		}
		// Upsert by key keeps the row unique and updates kind/time.
		if err := r.DismissedInsights().Dismiss(ctx, app.DismissedInsight{
			Key:         "abc123",
			Kind:        domain.InsightPriceChange,
			DismissedAt: ts.Add(time.Hour),
		}); err != nil {
			return err
		}
		if err := r.DismissedInsights().Dismiss(ctx, app.DismissedInsight{
			Key:         "def456",
			Kind:        domain.InsightTrend,
			DismissedAt: ts,
		}); err != nil {
			return err
		}
		if err := r.DismissedInsights().Dismiss(ctx, app.DismissedInsight{
			Key:         "",
			Kind:        domain.InsightAnomaly,
			DismissedAt: ts,
		}); !errors.Is(err, domain.ErrMissingField) {
			t.Fatalf("dismiss empty key err = %v, want missing_field", err)
		}
		if err := r.DismissedInsights().Dismiss(ctx, app.DismissedInsight{
			Key:         "k",
			Kind:        domain.InsightAnomaly,
			DismissedAt: time.Time{},
		}); !errors.Is(err, domain.ErrMissingField) {
			t.Fatalf("dismiss empty time err = %v, want missing_field", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("dismissed insights flow: %v", err)
	}

	if err := store.Read(ctx, func(r app.Repos) error {
		list, err := r.DismissedInsights().List(ctx)
		if err != nil {
			return err
		}
		if len(list) != 2 {
			t.Fatalf("len(list) = %d, want 2", len(list))
		}
		if list[0].Key != "abc123" || list[0].Kind != domain.InsightPriceChange || !list[0].DismissedAt.Equal(ts.Add(time.Hour)) {
			t.Fatalf("list[0] = %+v", list[0])
		}
		if list[1].Key != "def456" || list[1].Kind != domain.InsightTrend || !list[1].DismissedAt.Equal(ts) {
			t.Fatalf("list[1] = %+v", list[1])
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
