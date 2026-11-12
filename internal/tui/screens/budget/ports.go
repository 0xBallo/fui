// Package budget implements the TUI screen that shows every cap or envelope's status for a
// month (spent, planned, remaining, a progress bar) plus "Da assegnare" in envelope mode, and
// lets the user set an amount, move between months, copy the previous month, and assign or move
// money between envelopes.
package budget

import (
	"context"

	"github.com/0xballo/fui/internal/app/budgeting"
	"github.com/0xballo/fui/internal/domain"
)

// UseCases is the subset of budgeting.Service this screen needs, received by interface so tests
// can supply a stub store-free implementation.
type UseCases interface {
	Status(ctx context.Context, month string) (budgeting.Status, error)
	PlanSet(ctx context.Context, req budgeting.SetRequest) (budgeting.SetPlan, error)
	ApplySet(ctx context.Context, p budgeting.SetPlan) (domain.Budget, error)
	PlanMove(ctx context.Context, req budgeting.MoveRequest) (budgeting.MovePlan, error)
	ApplyMove(ctx context.Context, p budgeting.MovePlan) error
	PlanCopyPrevious(ctx context.Context, month string) (budgeting.CopyPlan, error)
	ApplyCopyPrevious(ctx context.Context, p budgeting.CopyPlan) error
}
