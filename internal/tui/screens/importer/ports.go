// Package importer implements the TUI screen that drives a connector import end to end: pick a
// connector and file, resolve unmapped accounts, preview the rows the plan would write (with
// their dedup class and the rule effects applied to them), and confirm before applying.
package importer

import (
	"context"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/tui/screens/settings/lists"
)

// UseCases is the subset of the importing service (plus the account listing this screen needs
// for CSV account refs and unmapped-account suggestions) received by interface so tests can
// supply a stub store-free implementation.
type UseCases interface {
	ListConnectors(ctx context.Context) ([]importing.ConnectorInfo, error)
	PlanImport(ctx context.Context, req importing.PlanRequest) (importing.ImportPlan, error)
	ApplyImportWith(ctx context.Context, p importing.ImportPlan, opts importing.ApplyOptions) (importing.ImportResult, error)
	DetectCSV(ctx context.Context, file string) (importing.CSVSuggestion, error)
	ListAccounts(ctx context.Context, req app.ListAccountsRequest) ([]app.AccountBalance, error)
}

// ProfileSaver persists a CSV import profile built by the column mapper; the settings import-
// profiles list uses the same shape, so one configuration adapter serves both.
type ProfileSaver interface {
	SaveProfile(p lists.Profile) error
}
