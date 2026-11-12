// Package importer implements the TUI screen that drives a connector import end to end: pick a
// connector and file, resolve unmapped accounts, preview the rows the plan would write (with
// their dedup class and the rule effects applied to them), and confirm before applying.
package importer

import (
	"context"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
)

// UseCases is the subset of the importing service (plus the account listing this screen needs
// for CSV account refs and unmapped-account suggestions) received by interface so tests can
// supply a stub store-free implementation.
type UseCases interface {
	ListConnectors(ctx context.Context) ([]importing.ConnectorInfo, error)
	PlanImport(ctx context.Context, req importing.PlanRequest) (importing.ImportPlan, error)
	ApplyImport(ctx context.Context, p importing.ImportPlan) (importing.ImportResult, error)
	ListAccounts(ctx context.Context, req app.ListAccountsRequest) ([]app.AccountBalance, error)
}
