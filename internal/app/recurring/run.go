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
			created, next, err := occurrences(rec, until)
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

// occurrences expands rec from its next date through until and returns the pending transactions
// to create plus the next date to persist afterwards (the zero Date once the recurrence is
// exhausted by its End; see domain.Advance and domain.Recurrence.Exhausted).
func occurrences(rec app.Recurrence, until domain.Date) ([]domain.Transaction, domain.Date, error) {
	dom := toDomain(rec)
	dates := domain.Expand(dom, dom.NextDate, until)
	if len(dates) == 0 {
		return nil, dom.NextDate, nil
	}
	created := make([]domain.Transaction, 0, len(dates))
	cur := dom
	for _, d := range dates {
		tx, err := domain.NewTransaction(domain.Transaction{
			AccountID: rec.AccountID, Date: d, DueDate: d, Amount: rec.Amount, Description: rec.Description,
			CategoryID: rec.CategoryID, Status: domain.StatusPending, Kind: domain.KindPayment,
			RecurrenceID: domain.RecurrenceID(rec.ID),
		})
		if err != nil {
			return nil, domain.Date{}, fmt.Errorf("occurrence of recurrence %d on %s: %w", rec.ID, d, err)
		}
		created = append(created, tx)
		advanced, err := domain.Advance(cur)
		if err != nil {
			return nil, domain.Date{}, fmt.Errorf("advance recurrence %d: %w", rec.ID, err)
		}
		cur = advanced
		if cur.Exhausted() {
			break
		}
	}
	return created, cur.NextDate, nil
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
		rec.NextDate = next.NextDate
		fields := map[string]any{"id": int64(id), "skipped": skipped.String()}
		if rec.NextDate.IsZero() {
			fields["exhausted"] = true
		} else {
			fields["next_date"] = rec.NextDate.String()
		}
		out = SkipPlan{
			Plan: app.Plan{Changes: []app.Change{{
				Op: app.OpUpdate, Entity: EntityRecurrence,
				Summary: fmt.Sprintf("skip occurrence of recurrence %d on %s", id, skipped),
				Fields:  fields,
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
