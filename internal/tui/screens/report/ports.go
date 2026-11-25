// Package report implements the TUI screen for the analytics module: predefined reports, saved
// reports, the cash-flow forecast, and transaction export. It renders results from
// internal/app/reporting as a table and, where meaningful, a text bar chart
// (internal/tui/components.BarChart).
package report

import (
	"context"
	"io"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
)

// UseCases is the subset of the reporting and importing layers this screen needs, received by
// interface so tests can supply a stub store-free implementation. The report-running and saved
// report methods match internal/app/reporting.Analytics; Export matches
// internal/app/importing.Service.Export.
type UseCases interface {
	// BalanceOverTime, NetWorth, RemainingDebt, InterestPaid, TopPayees, BudgetVsActual and
	// Forecast run the predefined reports (internal/app/reporting.Analytics).
	BalanceOverTime(ctx context.Context, req reporting.ReportRequest) (reporting.SeriesResult, error)
	NetWorth(ctx context.Context, req reporting.ReportRequest) (reporting.NetWorthResult, error)
	RemainingDebt(ctx context.Context, req reporting.ReportRequest) (reporting.DebtResult, error)
	InterestPaid(ctx context.Context, req reporting.ReportRequest) (reporting.InterestResult, error)
	TopPayees(ctx context.Context, req reporting.ReportRequest) (reporting.PayeesResult, error)
	BudgetVsActual(ctx context.Context, req reporting.ReportRequest) (reporting.BudgetResult, error)
	Forecast(ctx context.Context, req reporting.ForecastRequest) (reporting.ForecastResult, error)

	// Spending and CashFlow run the grouped spending and cash-flow reports.
	Spending(ctx context.Context, req reporting.ReportRequest) (reporting.GroupedResult, error)
	CashFlow(ctx context.Context, req reporting.ReportRequest) (reporting.GroupedResult, error)

	// List, Run, PlanSave, ApplySave, PlanRemove and ApplyRemove manage saved reports.
	List(ctx context.Context) ([]app.SavedReport, error)
	Run(ctx context.Context, ref string) (reporting.SavedResult, error)
	PlanSave(ctx context.Context, name, kind string, spec reporting.SavedSpec) (reporting.SavePlan, error)
	ApplySave(ctx context.Context, p reporting.SavePlan) (app.SavedReport, error)
	PlanRemove(ctx context.Context, ref string) (reporting.RemovePlan, error)
	ApplyRemove(ctx context.Context, p reporting.RemovePlan) error

	// Today resolves relative periods ("last-month", "this-year", ...) against the current date
	// (internal/app/reporting.ResolveRelative).
	Today() (domain.Date, error)

	// ListAccounts feeds the account field's autocomplete suggestions.
	ListAccounts(ctx context.Context, req app.ListAccountsRequest) ([]app.AccountBalance, error)

	// Export writes the transactions matching req to w (internal/app/importing.Service.Export).
	Export(ctx context.Context, req importing.ExportRequest, w io.Writer) (int, error)

	// ResolveTxFilter resolves a transaction filter request (account and category names, amount
	// range) into the filter Export takes (internal/app.Service.ResolveTxFilter).
	ResolveTxFilter(ctx context.Context, req app.ListTxRequest) (domain.TxFilter, error)

	// ExportResult writes a report result (the data of a reportResult) to w as "csv" or "json".
	// It is a port so this package does not depend on the CLI renderers.
	ExportResult(w io.Writer, format string, data any) error
}
