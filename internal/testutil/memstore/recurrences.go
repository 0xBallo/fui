package memstore

import (
	"cmp"
	"context"
	"slices"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

type recurrences struct{ *repos }

func normalizeRecurrence(in app.Recurrence) (app.Recurrence, error) {
	rec, err := domain.NewRecurrence(domain.Recurrence{
		AccountID:   in.AccountID,
		CategoryID:  in.CategoryID,
		Description: in.Description,
		Amount:      in.Amount,
		Rule:        in.Rule,
		Start:       in.Start,
		End:         in.End,
		NextDate:    in.NextDate,
	})
	if err != nil {
		return app.Recurrence{}, err
	}
	in.AccountID = rec.AccountID
	in.CategoryID = rec.CategoryID
	in.Description = rec.Description
	in.Amount = rec.Amount
	in.Rule = rec.Rule
	in.Start = rec.Start
	in.End = rec.End
	in.NextDate = rec.NextDate
	return in, nil
}

func (r recurrences) checkReferences(rec app.Recurrence) error {
	if _, ok := r.st.Accounts[rec.AccountID]; !ok {
		return notFound("account", rec.AccountID)
	}
	if rec.CategoryID != 0 {
		if _, ok := r.st.Categories[rec.CategoryID]; !ok {
			return notFound("category", rec.CategoryID)
		}
	}
	return nil
}

func (r recurrences) Insert(_ context.Context, in app.Recurrence) (app.Recurrence, error) {
	if err := r.writable(); err != nil {
		return app.Recurrence{}, err
	}
	rec, err := normalizeRecurrence(in)
	if err != nil {
		return app.Recurrence{}, err
	}
	if err := r.checkReferences(rec); err != nil {
		return app.Recurrence{}, err
	}
	rec.ID = app.RecurrenceID(r.st.nextID("recurrences"))
	r.st.Recurs[rec.ID] = rec
	return rec, nil
}

func (r recurrences) Update(_ context.Context, in app.Recurrence) error {
	if err := r.writable(); err != nil {
		return err
	}
	if _, ok := r.st.Recurs[in.ID]; !ok {
		return notFound("recurrence", in.ID)
	}
	rec, err := normalizeRecurrence(in)
	if err != nil {
		return err
	}
	if err := r.checkReferences(rec); err != nil {
		return err
	}
	r.st.Recurs[rec.ID] = rec
	return nil
}

func (r recurrences) Delete(_ context.Context, id app.RecurrenceID) error {
	if err := r.writable(); err != nil {
		return err
	}
	if _, ok := r.st.Recurs[id]; !ok {
		return notFound("recurrence", id)
	}
	delete(r.st.Recurs, id)
	return nil
}

func (r recurrences) Get(_ context.Context, id app.RecurrenceID) (app.Recurrence, error) {
	rec, ok := r.st.Recurs[id]
	if !ok {
		return app.Recurrence{}, notFound("recurrence", id)
	}
	return rec, nil
}

func (r recurrences) List(_ context.Context) ([]app.Recurrence, error) {
	return sortedValues(r.st.Recurs), nil
}

func (r recurrences) Due(_ context.Context, until domain.Date) ([]app.Recurrence, error) {
	if until.IsZero() {
		return nil, domain.ErrMissingField
	}
	out := make([]app.Recurrence, 0, len(r.st.Recurs))
	for _, rec := range r.st.Recurs {
		if !rec.NextDate.After(until) {
			out = append(out, rec)
		}
	}
	slices.SortFunc(out, func(a, b app.Recurrence) int {
		return cmp.Or(a.NextDate.Cmp(b.NextDate), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}

func (r recurrences) PendingOf(_ context.Context, id app.RecurrenceID) ([]domain.Transaction, error) {
	if _, ok := r.st.Recurs[id]; !ok {
		return nil, notFound("recurrence", id)
	}
	out := make([]domain.Transaction, 0, len(r.st.Txs))
	for _, tx := range r.st.Txs {
		if tx.RecurrenceID == domain.RecurrenceID(id) && tx.Status == domain.StatusPending {
			out = append(out, tx)
		}
	}
	slices.SortFunc(out, func(a, b domain.Transaction) int {
		return cmp.Or(recurrenceDueDate(a).Cmp(recurrenceDueDate(b)), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}

var _ app.RecurrenceRepo = recurrences{}

func recurrenceDueDate(tx domain.Transaction) domain.Date {
	if !tx.DueDate.IsZero() {
		return tx.DueDate
	}
	return tx.Date
}
