package memstore

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

type savedSimulations struct{ *repos }

func (r savedSimulations) checkName(in app.SavedSimulation) error {
	for _, other := range r.st.SavedSimulations {
		if other.ID != in.ID && strings.EqualFold(other.Name, in.Name) {
			return exists("saved simulation", in.Name)
		}
	}
	return nil
}

func (r savedSimulations) validate(in app.SavedSimulation) (app.SavedSimulation, error) {
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

func (r savedSimulations) Insert(_ context.Context, in app.SavedSimulation) (app.SavedSimulation, error) {
	if err := r.writable(); err != nil {
		return app.SavedSimulation{}, err
	}
	in, err := r.validate(in)
	if err != nil {
		return app.SavedSimulation{}, err
	}
	if err := r.checkName(in); err != nil {
		return app.SavedSimulation{}, err
	}
	in.ID = app.SavedSimulationID(r.st.nextID("saved_simulations"))
	r.st.SavedSimulations[in.ID] = in
	return in, nil
}

func (r savedSimulations) Delete(_ context.Context, id app.SavedSimulationID) error {
	if err := r.writable(); err != nil {
		return err
	}
	if _, ok := r.st.SavedSimulations[id]; !ok {
		return notFound("saved simulation", id)
	}
	delete(r.st.SavedSimulations, id)
	return nil
}

func (r savedSimulations) Get(_ context.Context, id app.SavedSimulationID) (app.SavedSimulation, error) {
	sim, ok := r.st.SavedSimulations[id]
	if !ok {
		return app.SavedSimulation{}, notFound("saved simulation", id)
	}
	return sim, nil
}

func (r savedSimulations) GetByName(_ context.Context, name string) (app.SavedSimulation, error) {
	for _, sim := range r.st.SavedSimulations {
		if strings.EqualFold(sim.Name, strings.TrimSpace(name)) {
			return sim, nil
		}
	}
	return app.SavedSimulation{}, notFound("saved simulation", name)
}

func (r savedSimulations) List(_ context.Context) ([]app.SavedSimulation, error) {
	out := sortedValues(r.st.SavedSimulations)
	slices.SortStableFunc(out, func(a, b app.SavedSimulation) int {
		return cmp.Or(
			strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)),
			cmp.Compare(a.ID, b.ID),
		)
	})
	return out, nil
}

var _ app.SavedSimulationRepo = savedSimulations{}

type dismissedInsights struct{ *repos }

func (r dismissedInsights) Dismiss(_ context.Context, in app.DismissedInsight) error {
	if err := r.writable(); err != nil {
		return err
	}
	in.Key = strings.TrimSpace(in.Key)
	in.Kind = domain.InsightKind(strings.TrimSpace(string(in.Kind)))
	if in.Key == "" {
		return fmt.Errorf("dismissed insight key: %w", domain.ErrMissingField)
	}
	if in.Kind == "" {
		return fmt.Errorf("dismissed insight kind: %w", domain.ErrMissingField)
	}
	if in.DismissedAt.IsZero() {
		return fmt.Errorf("dismissed insight dismissed_at: %w", domain.ErrMissingField)
	}
	in.DismissedAt = in.DismissedAt.UTC()
	r.st.Dismissed[in.Key] = in
	return nil
}

func (r dismissedInsights) List(_ context.Context) ([]app.DismissedInsight, error) {
	out := sortedValues(r.st.Dismissed)
	slices.SortFunc(out, func(a, b app.DismissedInsight) int {
		return strings.Compare(a.Key, b.Key)
	})
	return out, nil
}

var _ app.DismissedInsightRepo = dismissedInsights{}
