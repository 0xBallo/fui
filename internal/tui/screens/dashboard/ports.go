// Package dashboard implements the TUI home screen: calculation cycles, net worth, and a quick-add shortcut for transactions.
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
	// ListTx pages transactions; the dashboard uses it to find the oldest one (lifetime cycle).
	ListTx(ctx context.Context, req app.ListTxRequest) (app.TxPage, error)
	// Runway runs the runway KPI (internal/app/reporting.Runway.Run); it fails with
	// domain.ErrNotEnoughData when fewer than 3 complete months of history are available
	// (the median outflow is taken over up to 12 complete months).
	Runway(ctx context.Context, params reporting.RunwayParams) (reporting.RunwayResult, error)
}

// NetWorthUseCases is the optional reporting subset this screen can use to show the net-worth
// breakdown (positions and assets) when those modules are enabled.
type NetWorthUseCases interface {
	NetWorthAt(ctx context.Context, at domain.Date) (reporting.NetWorthPoint, error)
}
