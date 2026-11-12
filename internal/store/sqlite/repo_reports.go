package sqlite

import (
	"context"
	"fmt"
	"strings"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"
)

type savedReportRepo struct {
	repos *repos
}

func normalizeSavedReport(in app.SavedReport) (app.SavedReport, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Kind = strings.TrimSpace(in.Kind)
	in.ParamsJSON = strings.TrimSpace(in.ParamsJSON)
	if in.Name == "" {
		return app.SavedReport{}, fmt.Errorf("saved report name: %w", domain.ErrMissingField)
	}
	if in.Kind == "" {
		return app.SavedReport{}, fmt.Errorf("saved report kind: %w", domain.ErrMissingField)
	}
	if in.ParamsJSON == "" {
		return app.SavedReport{}, fmt.Errorf("saved report params: %w", domain.ErrMissingField)
	}
	return in, nil
}

func (r savedReportRepo) Insert(ctx context.Context, in app.SavedReport) (app.SavedReport, error) {
	if err := r.repos.writable(); err != nil {
		return app.SavedReport{}, err
	}
	norm, err := normalizeSavedReport(in)
	if err != nil {
		return app.SavedReport{}, err
	}
	row, err := r.repos.queries.InsertSavedReport(ctx, gen.InsertSavedReportParams{
		Name:   norm.Name,
		Kind:   norm.Kind,
		Params: norm.ParamsJSON,
	})
	if err != nil {
		return app.SavedReport{}, mapConstraint(err)
	}
	return toSavedReport(row), nil
}

func (r savedReportRepo) Update(ctx context.Context, in app.SavedReport) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	norm, err := normalizeSavedReport(in)
	if err != nil {
		return err
	}
	changed, err := r.repos.queries.UpdateSavedReport(ctx, gen.UpdateSavedReportParams{
		ID:     int64(norm.ID),
		Name:   norm.Name,
		Kind:   norm.Kind,
		Params: norm.ParamsJSON,
	})
	if err != nil {
		return mapConstraint(err)
	}
	if changed == 0 {
		return fmt.Errorf("saved report %d: %w", in.ID, domain.ErrNotFound)
	}
	return nil
}

func (r savedReportRepo) Delete(ctx context.Context, id app.SavedReportID) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	changed, err := r.repos.queries.DeleteSavedReport(ctx, int64(id))
	if err != nil {
		return mapSQLError(err)
	}
	if changed == 0 {
		return fmt.Errorf("saved report %d: %w", id, domain.ErrNotFound)
	}
	return nil
}

func (r savedReportRepo) Get(ctx context.Context, id app.SavedReportID) (app.SavedReport, error) {
	row, err := r.repos.queries.GetSavedReport(ctx, int64(id))
	if err != nil {
		return app.SavedReport{}, mapNotFound(mapSQLError(err), fmt.Sprintf("saved report %d", id))
	}
	return toSavedReport(row), nil
}

func (r savedReportRepo) GetByName(ctx context.Context, name string) (app.SavedReport, error) {
	row, err := r.repos.queries.GetSavedReportByName(ctx, strings.TrimSpace(name))
	if err != nil {
		return app.SavedReport{}, mapNotFound(mapSQLError(err), fmt.Sprintf("saved report %q", name))
	}
	return toSavedReport(row), nil
}

func (r savedReportRepo) List(ctx context.Context) ([]app.SavedReport, error) {
	rows, err := r.repos.queries.ListSavedReports(ctx)
	if err != nil {
		return nil, mapSQLError(err)
	}
	out := make([]app.SavedReport, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSavedReport(row))
	}
	return out, nil
}

func toSavedReport(row gen.SavedReport) app.SavedReport {
	return app.SavedReport{
		ID:         app.SavedReportID(row.ID),
		Name:       row.Name,
		Kind:       row.Kind,
		ParamsJSON: row.Params,
	}
}

var _ app.SavedReportRepo = savedReportRepo{}
