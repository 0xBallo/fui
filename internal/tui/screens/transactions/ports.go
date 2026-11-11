package transactions

import (
	"context"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// UseCases is the subset of app.Service this screen needs, received by interface so tests can
// supply a stub store-free implementation.
type UseCases interface {
	ListTx(ctx context.Context, req app.ListTxRequest) (app.TxPage, error)
	ListAccounts(ctx context.Context, req app.ListAccountsRequest) ([]app.AccountBalance, error)
	ListCategories(ctx context.Context) ([]domain.Category, error)
	PlanAddTx(ctx context.Context, req app.AddTxRequest) (app.AddTxPlan, error)
	ApplyAddTx(ctx context.Context, p app.AddTxPlan) (domain.Transaction, error)
	PlanEditTx(ctx context.Context, req app.EditTxRequest) (app.EditTxPlan, error)
	ApplyEditTx(ctx context.Context, p app.EditTxPlan) (domain.Transaction, error)
	PlanRemoveTx(ctx context.Context, req app.RemoveTxRequest) (app.RemoveTxPlan, error)
	ApplyRemoveTx(ctx context.Context, p app.RemoveTxPlan) error
	PlanCategorizeTx(ctx context.Context, req app.CategorizeTxRequest) (app.CategorizeTxPlan, error)
	ApplyCategorizeTx(ctx context.Context, p app.CategorizeTxPlan) error
}
