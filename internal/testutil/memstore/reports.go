package memstore

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

type savedReports struct{ *repos }

func (r savedReports) checkName(in app.SavedReport) error {
	for _, other := range r.st.SavedReports {
		if other.ID != in.ID && strings.EqualFold(other.Name, in.Name) {
			return exists("saved report", in.Name)
		}
	}
	return nil
}

func (r savedReports) validate(in app.SavedReport) (app.SavedReport, error) {
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

func (r savedReports) Insert(_ context.Context, in app.SavedReport) (app.SavedReport, error) {
	if err := r.writable(); err != nil {
		return app.SavedReport{}, err
	}
	in, err := r.validate(in)
	if err != nil {
		return app.SavedReport{}, err
	}
	if err := r.checkName(in); err != nil {
		return app.SavedReport{}, err
	}
	in.ID = app.SavedReportID(r.st.nextID("saved_reports"))
	r.st.SavedReports[in.ID] = in
	return in, nil
}

func (r savedReports) Update(_ context.Context, in app.SavedReport) error {
	if err := r.writable(); err != nil {
		return err
	}
	if _, ok := r.st.SavedReports[in.ID]; !ok {
		return notFound("saved report", in.ID)
	}
	var err error
	in, err = r.validate(in)
	if err != nil {
		return err
	}
	if err := r.checkName(in); err != nil {
		return err
	}
	r.st.SavedReports[in.ID] = in
	return nil
}

func (r savedReports) Delete(_ context.Context, id app.SavedReportID) error {
	if err := r.writable(); err != nil {
		return err
	}
	if _, ok := r.st.SavedReports[id]; !ok {
		return notFound("saved report", id)
	}
	delete(r.st.SavedReports, id)
	return nil
}

func (r savedReports) Get(_ context.Context, id app.SavedReportID) (app.SavedReport, error) {
	sr, ok := r.st.SavedReports[id]
	if !ok {
		return app.SavedReport{}, notFound("saved report", id)
	}
	return sr, nil
}

func (r savedReports) GetByName(_ context.Context, name string) (app.SavedReport, error) {
	for _, sr := range r.st.SavedReports {
		if strings.EqualFold(sr.Name, name) {
			return sr, nil
		}
	}
	return app.SavedReport{}, notFound("saved report", name)
}

func (r savedReports) List(_ context.Context) ([]app.SavedReport, error) {
	out := sortedValues(r.st.SavedReports)
	slices.SortStableFunc(out, func(a, b app.SavedReport) int {
		return cmp.Or(
			strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)),
			cmp.Compare(a.ID, b.ID),
		)
	})
	return out, nil
}

var _ app.SavedReportRepo = savedReports{}

type reconciliations struct{ *repos }

func (r reconciliations) Open(_ context.Context, in app.Reconciliation) (app.Reconciliation, error) {
	if err := r.writable(); err != nil {
		return app.Reconciliation{}, err
	}
	if _, ok := r.st.Accounts[in.AccountID]; !ok {
		return app.Reconciliation{}, notFound("account", in.AccountID)
	}
	if in.Date.IsZero() {
		return app.Reconciliation{}, fmt.Errorf("reconciliation date: %w", domain.ErrMissingField)
	}
	if in.StatementBalance.Currency().IsZero() {
		return app.Reconciliation{}, fmt.Errorf("reconciliation statement balance: %w", domain.ErrMissingField)
	}
	k := reconciliationKey{accountID: in.AccountID, date: in.Date}
	if _, existsKey := r.st.Reconciliations[k]; existsKey {
		return app.Reconciliation{}, exists("reconciliation", fmt.Sprintf("%d/%s", in.AccountID, in.Date))
	}
	in.ID = app.ReconciliationID(r.st.nextID("reconciliations"))
	in.ClosedAt = time.Time{}
	r.st.Reconciliations[k] = in
	return in, nil
}

func (r reconciliations) Close(_ context.Context, accountID domain.AccountID, date domain.Date, closedAt time.Time) error {
	if err := r.writable(); err != nil {
		return err
	}
	k := reconciliationKey{accountID: accountID, date: date}
	rr, ok := r.st.Reconciliations[k]
	if !ok {
		return notFound("reconciliation", fmt.Sprintf("%d/%s", accountID, date))
	}
	rr.ClosedAt = closedAt.UTC()
	r.st.Reconciliations[k] = rr
	return nil
}

func (r reconciliations) Get(_ context.Context, accountID domain.AccountID, date domain.Date) (app.Reconciliation, error) {
	rr, ok := r.st.Reconciliations[reconciliationKey{accountID: accountID, date: date}]
	if !ok {
		return app.Reconciliation{}, notFound("reconciliation", fmt.Sprintf("%d/%s", accountID, date))
	}
	return rr, nil
}

func (r reconciliations) LastClosed(_ context.Context, accountID domain.AccountID) (app.Reconciliation, error) {
	var (
		out   app.Reconciliation
		found bool
	)
	for _, rr := range r.st.Reconciliations {
		if rr.AccountID != accountID || rr.ClosedAt.IsZero() {
			continue
		}
		if !found || rr.Date.After(out.Date) || (rr.Date.Cmp(out.Date) == 0 && rr.ID > out.ID) {
			out = rr
			found = true
		}
	}
	if !found {
		return app.Reconciliation{}, notFound("reconciliation", accountID)
	}
	return out, nil
}

var _ app.ReconciliationRepo = reconciliations{}
