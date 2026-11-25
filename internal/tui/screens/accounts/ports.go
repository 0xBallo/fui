// Package accounts implements the TUI screen that lists accounts with their balances and shows
// the movements of one account.
package accounts

import (
	"context"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/investing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// UseCases is the subset of app.Service this screen needs, received by interface so tests can
// supply a stub store-free implementation.
type UseCases interface {
	ListAccounts(ctx context.Context, req app.ListAccountsRequest) ([]app.AccountBalance, error)
	NetWorth(ctx context.Context, req app.NetWorthRequest) (app.NetWorth, error)
	ShowAccount(ctx context.Context, ref, at string) (app.AccountBalance, error)
	ListTx(ctx context.Context, req app.ListTxRequest) (app.TxPage, error)
	PlanAddAccount(ctx context.Context, req app.AddAccountRequest) (app.AddAccountPlan, error)
	ApplyAddAccount(ctx context.Context, p app.AddAccountPlan) (domain.Account, error)
	PlanArchiveAccount(ctx context.Context, ref string) (app.ArchiveAccountPlan, error)
	ApplyArchiveAccount(ctx context.Context, p app.ArchiveAccountPlan) (domain.Account, error)
	PlanReconcile(ctx context.Context, enabled module.Set, req app.ReconcileRequest) (app.ReconcilePlan, error)
	ApplyReconcile(ctx context.Context, enabled module.Set, p app.ReconcilePlan) error
}

// PositionsUseCases is the optional investing subset this screen can use to show investment
// positions and let users set manual prices from the account detail panel.
type PositionsUseCases interface {
	ListPositions(ctx context.Context, req investing.PositionsRequest) ([]investing.PositionLine, error)
	PlanSetPrice(ctx context.Context, req investing.SetPriceRequest) (investing.SetPricePlan, error)
	ApplySetPrice(ctx context.Context, p investing.SetPricePlan) error
	PlanAddSecurity(ctx context.Context, req investing.AddSecurityRequest) (investing.AddSecurityPlan, error)
	ApplyAddSecurity(ctx context.Context, p investing.AddSecurityPlan) (domain.Security, error)
	PlanBuy(ctx context.Context, req investing.OpRequest) (investing.OpPlan, error)
	PlanSell(ctx context.Context, req investing.OpRequest) (investing.OpPlan, error)
	PlanDividend(ctx context.Context, req investing.OpRequest) (investing.OpPlan, error)
	PlanCoupon(ctx context.Context, req investing.OpRequest) (investing.OpPlan, error)
	ApplyOp(ctx context.Context, p investing.OpPlan) (domain.SecurityOp, error)
	ListOps(ctx context.Context, req investing.ListOpsRequest) ([]investing.OpLine, error)
	PlanEditOp(ctx context.Context, req investing.EditOpRequest) (investing.EditOpPlan, error)
	ApplyEditOp(ctx context.Context, p investing.EditOpPlan) (domain.SecurityOp, error)
	PlanRemoveOp(ctx context.Context, id domain.SecurityOpID) (investing.RemoveOpPlan, error)
	ApplyRemoveOp(ctx context.Context, p investing.RemoveOpPlan) error
}
