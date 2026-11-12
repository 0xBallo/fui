package recurring

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// ConfirmPlan is the plan to book the pending transaction a recurrence created.
type ConfirmPlan struct {
	app.Plan
	Tx domain.Transaction
}

// PlanConfirm books the pending transaction txID: it must be pending and linked to a recurrence.
// Importing a matching booked movement does the same thing automatically (see
// internal/app/importing/recur.go); PlanConfirm is for the user confirming it by hand.
func (s *Service) PlanConfirm(ctx context.Context, enabled module.Set, txID domain.TransactionID) (ConfirmPlan, error) {
	if err := app.WrapModuleError(enabled.Require(module.Recurring)); err != nil {
		return ConfirmPlan{}, err
	}
	var out ConfirmPlan
	err := s.store.Read(ctx, func(r app.Repos) error {
		tx, err := r.Transactions().Get(ctx, txID)
		if err != nil {
			return fmt.Errorf("get transaction %d: %w", txID, err)
		}
		if tx.RecurrenceID == 0 {
			return &domain.Error{
				Code: domain.CodeInvalidValue, Message: "transaction is not linked to a recurrence",
				Details: map[string]any{"field": "recurrence_id"},
			}
		}
		if tx.Status != domain.StatusPending {
			return &domain.Error{
				Code: domain.CodeInvalidValue, Message: "transaction is not pending",
				Details: map[string]any{"field": "status", "value": string(tx.Status)},
			}
		}
		tx.Status = domain.StatusBooked
		if tx, err = domain.NewTransaction(tx); err != nil {
			return fmt.Errorf("transaction: %w", err)
		}
		out = ConfirmPlan{
			Plan: app.Plan{Changes: []app.Change{{
				Op: app.OpUpdate, Entity: app.EntityTransaction,
				Summary: fmt.Sprintf("confirm recurring transaction %d", txID),
				Fields:  map[string]any{"id": int64(txID), "status": string(domain.StatusBooked)},
			}}},
			Tx: tx,
		}
		return nil
	})
	return out, err
}

// ApplyConfirm stores the transaction of the plan as booked.
func (s *Service) ApplyConfirm(ctx context.Context, p ConfirmPlan) error {
	return s.write(ctx, func(r app.Repos) error { return r.Transactions().Update(ctx, p.Tx) })
}
