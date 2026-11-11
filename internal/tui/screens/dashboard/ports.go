// Package dashboard implements the TUI home screen: account balances, net worth, the runway KPI,
// current-month spending by category, and a quick-add shortcut for transactions.
package dashboard

import (
	"context"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
)

// UseCases is the subset of the app and reporting layers this screen needs, received by
// interface so tests can supply a stub store-free implementation.
type UseCases interface {
	ListAccounts(ctx context.Context, req app.ListAccountsRequest) ([]app.AccountBalance, error)
	NetWorth(ctx context.Context, req app.NetWorthRequest) (app.NetWorth, error)
	ListCategories(ctx context.Context) ([]domain.Category, error)
	PlanAddTx(ctx context.Context, req app.AddTxRequest) (app.AddTxPlan, error)
	ApplyAddTx(ctx context.Context, p app.AddTxPlan) (domain.Transaction, error)
	Today() (domain.Date, error)
	// Spending runs the spending-by-category report (internal/app/reporting.Spending.Run).
	Spending(ctx context.Context, params reporting.SpendingParams) (reporting.SpendingResult, error)
	// Runway runs the runway KPI (internal/app/reporting.Runway.Run); it fails with
	// domain.ErrNotEnoughData when fewer than 3 complete months of history are available
	// (the median outflow is taken over up to 12 complete months).
	Runway(ctx context.Context, params reporting.RunwayParams) (reporting.RunwayResult, error)
}
