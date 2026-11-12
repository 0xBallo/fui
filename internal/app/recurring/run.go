package recurring

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// RunRequest asks to materialise every due occurrence through Until (today when empty) as a
// pending transaction.
type RunRequest struct {
	Until string
}

// RunPlan is the plan to create Created and advance the next date of the Recurrences behind
// them. Running again with the same or an earlier Until creates nothing new, because each
// recurrence's next date only ever advances past what was already created.
type RunPlan struct {
	app.Plan
	Created     []domain.Transaction
	Recurrences []app.Recurrence
}

// PlanRun expands every recurrence due on or before Until into pending transactions, without
// writing.
func (s *Service) PlanRun(ctx context.Context, enabled module.Set, req RunRequest) (RunPlan, error) {
	if err := app.WrapModuleError(enabled.Require(module.Recurring)); err != nil {
		return RunPlan{}, err
	}
	until, err := s.date(req.Until)
	if err != nil {
		return RunPlan{}, err
	}
	var out RunPlan
	err = s.store.Read(ctx, func(r app.Repos) error {
		due, err := r.Recurrences().Due(ctx, until)
		if err != nil {
			return fmt.Errorf("list due recurrences: %w", err)
		}
		for _, rec := range due {
			if rec.NextDate.IsZero() {
				continue // exhausted: Due() may still list it since the zero Date sorts first
			}
			existing, err := existingOccurrenceDates(ctx, r, rec)
			if err != nil {
				return err
			}
			created, next, err := occurrences(rec, until, existing)
			if err != nil {
				return err
			}
			if len(created) == 0 {
				continue
			}
			rec.NextDate = next
			out.Created = append(out.Created, created...)
			out.Recurrences = append(out.Recurrences, rec)
			out.Changes = append(out.Changes, occurrenceChanges(rec, created)...)
		}
		out.Bulk = len(out.Created) > 0
		return nil
	})
	return out, err
}

// existingOccurrenceDates returns, keyed by due date, the transactions already linked to rec
// (of any status): once the last in-range occurrence of an End-bound recurrence has been
// created, its NextDate stays capped at that occurrence (see occurrences), so Expand keeps
// re-proposing the same date on every later run; this lets occurrences recognise and skip it.
func existingOccurrenceDates(ctx context.Context, r app.Repos, rec app.Recurrence) (map[domain.Date]bool, error) {
	f, err := domain.NewTxFilter(domain.TxFilterSpec{AccountIDs: []domain.AccountID{rec.AccountID}})
	if err != nil {
		return nil, fmt.Errorf("existing occurrences filter: %w", err)
	}
	txs, err := r.Transactions().List(ctx, f, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("list existing occurrences: %w", err)
	}
	want := domain.RecurrenceID(rec.ID)
	out := map[domain.Date]bool{}
	for _, tx := range txs {
		if tx.RecurrenceID != want {
			continue
		}
		d := tx.DueDate
		if d.IsZero() {
			d = tx.Date
		}
		out[d] = true
	}
	return out, nil
}

// occurrences expands rec from its next date through until and returns the pending transactions
// to create (skipping dates already in existing) plus the next date to persist afterwards.
//
// domain.Advance documents that it zeroes NextDate once a recurrence is exhausted by End, but
// domain.NewRecurrence (revalidated by the repo on every Update) rejects a zero NextDate, so that
// state can never be persisted. occurrences works around this: once advancing would exhaust the
// recurrence, NextDate is instead capped at the date of that last occurrence (itself always a
// valid, in-range date) rather than moved past it. Expand then keeps re-proposing that same date
// on every later run, which existingOccurrenceDates recognises so it is never recreated. See
// REQ in .task-report.md: domain.NewRecurrence should accept a zero NextDate.
func occurrences(rec app.Recurrence, until domain.Date, existing map[domain.Date]bool) ([]domain.Transaction, domain.Date, error) {
	dom := toDomain(rec)
	dates := domain.Expand(dom, dom.NextDate, until)
	if len(dates) == 0 {
		return nil, dom.NextDate, nil
	}
	created := make([]domain.Transaction, 0, len(dates))
	cur, next := dom, dom.NextDate
	for _, d := range dates {
		if !existing[d] {
			tx, err := domain.NewTransaction(domain.Transaction{
				AccountID: rec.AccountID, Date: d, DueDate: d, Amount: rec.Amount, Description: rec.Description,
				CategoryID: rec.CategoryID, Status: domain.StatusPending, Kind: domain.KindPayment,
				RecurrenceID: domain.RecurrenceID(rec.ID),
			})
			if err != nil {
				return nil, domain.Date{}, fmt.Errorf("occurrence of recurrence %d on %s: %w", rec.ID, d, err)
			}
			created = append(created, tx)
		}
		advanced, err := domain.Advance(cur)
		if err != nil {
			return nil, domain.Date{}, fmt.Errorf("advance recurrence %d: %w", rec.ID, err)
		}
		if advanced.NextDate.IsZero() {
			next = d // exhausted: cap at the last occurrence instead of persisting a zero date
			break
		}
		cur, next = advanced, advanced.NextDate
	}
	return created, next, nil
}

func occurrenceChanges(rec app.Recurrence, created []domain.Transaction) []app.Change {
	out := make([]app.Change, 0, len(created))
	for _, tx := range created {
		f := map[string]any{
			"account_id": int64(tx.AccountID), "recurrence_id": int64(rec.ID),
			"date": tx.Date.String(), "due_date": tx.DueDate.String(), "description": tx.Description,
		}
		moneyFields(f, "amount", tx.Amount)
		out = append(out, app.Change{
			Op: app.OpCreate, Entity: app.EntityTransaction, Fields: f,
			Summary: fmt.Sprintf("create pending transaction for recurrence %d on %s", rec.ID, tx.Date),
		})
	}
	return out
}

// ApplyRun creates the pending transactions of the plan and advances the recurrences behind
// them, after a backup. It returns how many transactions were created.
func (s *Service) ApplyRun(ctx context.Context, p RunPlan) (int, error) {
	if len(p.Created) == 0 {
		return 0, nil
	}
	if _, err := s.backup.Backup(ctx, app.BackupReasonBulk); err != nil {
		return 0, fmt.Errorf("backup before bulk change: %w", err)
	}
	err := s.write(ctx, func(r app.Repos) error {
		for _, tx := range p.Created {
			if _, err := r.Transactions().Insert(ctx, tx); err != nil {
				return fmt.Errorf("create pending transaction: %w", err)
			}
		}
		for _, rec := range p.Recurrences {
			if err := r.Recurrences().Update(ctx, rec); err != nil {
				return fmt.Errorf("advance recurrence %d: %w", rec.ID, err)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return len(p.Created), nil
}

// SkipPlan is the plan to advance a recurrence's next date past its next occurrence without
// creating a pending transaction for it.
type SkipPlan struct {
	app.Plan
	Recurrence app.Recurrence
}

// PlanSkip plans skipping the next occurrence of recurrence id.
func (s *Service) PlanSkip(ctx context.Context, enabled module.Set, id app.RecurrenceID) (SkipPlan, error) {
	if err := app.WrapModuleError(enabled.Require(module.Recurring)); err != nil {
		return SkipPlan{}, err
	}
	var out SkipPlan
	err := s.store.Read(ctx, func(r app.Repos) error {
		rec, err := r.Recurrences().Get(ctx, id)
		if err != nil {
			return fmt.Errorf("get recurrence %d: %w", id, err)
		}
		if rec.NextDate.IsZero() {
			return &domain.Error{
				Code: domain.CodeInvalidValue, Message: "recurrence has no next occurrence",
				Details: map[string]any{"field": "next_date"},
			}
		}
		skipped := rec.NextDate
		next, err := domain.Advance(toDomain(rec))
		if err != nil {
			return fmt.Errorf("advance recurrence %d: %w", id, err)
		}
		if next.NextDate.IsZero() {
			// domain.NewRecurrence rejects a zero NextDate (see occurrences' doc in run.go), so
			// skipping past the recurrence's last occurrence cannot be persisted: refuse it.
			return &domain.Error{
				Code: domain.CodeInvalidValue, Message: "recurrence has no further occurrence to skip to",
				Details: map[string]any{"field": "next_date"},
			}
		}
		rec.NextDate = next.NextDate
		out = SkipPlan{
			Plan: app.Plan{Changes: []app.Change{{
				Op: app.OpUpdate, Entity: EntityRecurrence,
				Summary: fmt.Sprintf("skip occurrence of recurrence %d on %s", id, skipped),
				Fields:  map[string]any{"id": int64(id), "skipped": skipped.String(), "next_date": rec.NextDate.String()},
			}}},
			Recurrence: rec,
		}
		return nil
	})
	return out, err
}

// ApplySkip stores the recurrence of the plan with its advanced next date.
func (s *Service) ApplySkip(ctx context.Context, p SkipPlan) error {
	return s.write(ctx, func(r app.Repos) error { return r.Recurrences().Update(ctx, p.Recurrence) })
}
