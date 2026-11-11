// Package accounts implements the TUI screen that lists accounts with their balances and shows
// the movements of one account.
package accounts

import (
	"context"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// UseCases is the subset of app.Service this screen needs, received by interface so tests can
// supply a stub store-free implementation.
type UseCases interface {
	ListAccounts(ctx context.Context, req app.ListAccountsRequest) ([]app.AccountBalance, error)
	ShowAccount(ctx context.Context, ref, at string) (app.AccountBalance, error)
	ListTx(ctx context.Context, req app.ListTxRequest) (app.TxPage, error)
	PlanAddAccount(ctx context.Context, req app.AddAccountRequest) (app.AddAccountPlan, error)
	ApplyAddAccount(ctx context.Context, p app.AddAccountPlan) (domain.Account, error)
	PlanArchiveAccount(ctx context.Context, ref string) (app.ArchiveAccountPlan, error)
	ApplyArchiveAccount(ctx context.Context, p app.ArchiveAccountPlan) (domain.Account, error)
}
