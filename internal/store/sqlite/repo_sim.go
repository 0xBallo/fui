package sqlite

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"
)

type savedSimulationRepo struct {
	repos *repos
}

func normalizeSavedSimulation(in app.SavedSimulation) (app.SavedSimulation, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Kind = strings.TrimSpace(in.Kind)
	in.ParamsJSON = strings.TrimSpace(in.ParamsJSON)
	if in.Name == "" {
		return app.SavedSimulation{}, fmt.Errorf("saved simulation name: %w", domain.ErrMissingField)
	}
	if in.Kind == "" {
		return app.SavedSimulation{}, fmt.Errorf("saved simulation kind: %w", domain.ErrMissingField)
	}
	if in.ParamsJSON == "" {
		return app.SavedSimulation{}, fmt.Errorf("saved simulation params: %w", domain.ErrMissingField)
	}
	return in, nil
}

func (r savedSimulationRepo) Insert(ctx context.Context, in app.SavedSimulation) (app.SavedSimulation, error) {
	if err := r.repos.writable(); err != nil {
		return app.SavedSimulation{}, err
	}
	norm, err := normalizeSavedSimulation(in)
	if err != nil {
		return app.SavedSimulation{}, err
	}
	row, err := r.repos.queries.InsertSavedSimulation(ctx, gen.InsertSavedSimulationParams{
		Name:   norm.Name,
		Kind:   norm.Kind,
		Params: norm.ParamsJSON,
	})
	if err != nil {
		return app.SavedSimulation{}, mapConstraint(err)
	}
	return toSavedSimulation(row), nil
}

func (r savedSimulationRepo) Delete(ctx context.Context, id app.SavedSimulationID) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	changed, err := r.repos.queries.DeleteSavedSimulation(ctx, int64(id))
	if err != nil {
		return mapSQLError(err)
	}
	if changed == 0 {
		return fmt.Errorf("saved simulation %d: %w", id, domain.ErrNotFound)
	}
	return nil
}

func (r savedSimulationRepo) Get(ctx context.Context, id app.SavedSimulationID) (app.SavedSimulation, error) {
	row, err := r.repos.queries.GetSavedSimulation(ctx, int64(id))
	if err != nil {
		return app.SavedSimulation{}, mapNotFound(mapSQLError(err), fmt.Sprintf("saved simulation %d", id))
	}
	return toSavedSimulation(row), nil
}

func (r savedSimulationRepo) GetByName(ctx context.Context, name string) (app.SavedSimulation, error) {
	row, err := r.repos.queries.GetSavedSimulationByName(ctx, strings.TrimSpace(name))
	if err != nil {
		return app.SavedSimulation{}, mapNotFound(mapSQLError(err), fmt.Sprintf("saved simulation %q", name))
	}
	return toSavedSimulation(row), nil
}

func (r savedSimulationRepo) List(ctx context.Context) ([]app.SavedSimulation, error) {
	rows, err := r.repos.queries.ListSavedSimulations(ctx)
	if err != nil {
		return nil, mapSQLError(err)
	}
	out := make([]app.SavedSimulation, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSavedSimulation(row))
	}
	return out, nil
}

func toSavedSimulation(row gen.SavedSimulation) app.SavedSimulation {
	return app.SavedSimulation{
		ID:         app.SavedSimulationID(row.ID),
		Name:       row.Name,
		Kind:       row.Kind,
		ParamsJSON: row.Params,
	}
}

type dismissedInsightRepo struct {
	repos *repos
}

func normalizeDismissedInsight(in app.DismissedInsight) (app.DismissedInsight, error) {
	in.Key = strings.TrimSpace(in.Key)
	in.Kind = domain.InsightKind(strings.TrimSpace(string(in.Kind)))
	if in.Key == "" {
		return app.DismissedInsight{}, fmt.Errorf("dismissed insight key: %w", domain.ErrMissingField)
	}
	if in.Kind == "" {
		return app.DismissedInsight{}, fmt.Errorf("dismissed insight kind: %w", domain.ErrMissingField)
	}
	if in.DismissedAt.IsZero() {
		return app.DismissedInsight{}, fmt.Errorf("dismissed insight dismissed_at: %w", domain.ErrMissingField)
	}
	return in, nil
}

func (r dismissedInsightRepo) Dismiss(ctx context.Context, in app.DismissedInsight) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	norm, err := normalizeDismissedInsight(in)
	if err != nil {
		return err
	}
	return mapConstraint(r.repos.queries.UpsertDismissedInsight(ctx, gen.UpsertDismissedInsightParams{
		Key:         norm.Key,
		Kind:        string(norm.Kind),
		DismissedAt: norm.DismissedAt.UTC().Format(time.RFC3339Nano),
	}))
}

func (r dismissedInsightRepo) List(ctx context.Context) ([]app.DismissedInsight, error) {
	rows, err := r.repos.queries.ListDismissedInsights(ctx)
	if err != nil {
		return nil, mapSQLError(err)
	}
	out := make([]app.DismissedInsight, 0, len(rows))
	for _, row := range rows {
		dismissedAt, parseErr := time.Parse(time.RFC3339Nano, row.DismissedAt)
		if parseErr != nil {
			return nil, fmt.Errorf("dismissed insight %q dismissed_at %q: %w", row.Key, row.DismissedAt, parseErr)
		}
		out = append(out, app.DismissedInsight{
			Key:         row.Key,
			Kind:        domain.InsightKind(row.Kind),
			DismissedAt: dismissedAt.UTC(),
		})
	}
	return out, nil
}

var (
	_ app.SavedSimulationRepo  = savedSimulationRepo{}
	_ app.DismissedInsightRepo = dismissedInsightRepo{}
)
