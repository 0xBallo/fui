package main

import (
	"context"
	"io"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/budgeting"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
)

// The adapters below satisfy the v0.2 screens' use-case ports. The budget and report ones build
// their service per call from the current configuration: both gate on the enabled-module set, which
// the Settings screen changes while the TUI is running.

// budgetUseCases adapts budgeting.Service to budget.UseCases.
type budgetUseCases struct{ build func() *budgeting.Service }

func (u budgetUseCases) Status(ctx context.Context, month string) (budgeting.Status, error) {
	return u.build().Status(ctx, month)
}

func (u budgetUseCases) PlanSet(ctx context.Context, req budgeting.SetRequest) (budgeting.SetPlan, error) {
	return u.build().PlanSet(ctx, req)
}

func (u budgetUseCases) ApplySet(ctx context.Context, p budgeting.SetPlan) (domain.Budget, error) {
	return u.build().ApplySet(ctx, p)
}

func (u budgetUseCases) PlanMove(ctx context.Context, req budgeting.MoveRequest) (budgeting.MovePlan, error) {
	return u.build().PlanMove(ctx, req)
}

func (u budgetUseCases) ApplyMove(ctx context.Context, p budgeting.MovePlan) error {
	return u.build().ApplyMove(ctx, p)
}

func (u budgetUseCases) PlanCopyPrevious(ctx context.Context, month string) (budgeting.CopyPlan, error) {
	return u.build().PlanCopyPrevious(ctx, month)
}

func (u budgetUseCases) ApplyCopyPrevious(ctx context.Context, p budgeting.CopyPlan) error {
	return u.build().ApplyCopyPrevious(ctx, p)
}

// reportUseCases adapts reporting.Analytics, app.Service and importing.Service to report.UseCases.
type reportUseCases struct {
	analytics func() *reporting.Analytics
	svc       *app.Service
	importing *importing.Service
}

func (u reportUseCases) BalanceOverTime(ctx context.Context, r reporting.ReportRequest) (reporting.SeriesResult, error) {
	return u.analytics().BalanceOverTime(ctx, r)
}

func (u reportUseCases) NetWorth(ctx context.Context, r reporting.ReportRequest) (reporting.SeriesResult, error) {
	return u.analytics().NetWorth(ctx, r)
}

func (u reportUseCases) RemainingDebt(ctx context.Context, r reporting.ReportRequest) (reporting.DebtResult, error) {
	return u.analytics().RemainingDebt(ctx, r)
}

func (u reportUseCases) InterestPaid(ctx context.Context, r reporting.ReportRequest) (reporting.InterestResult, error) {
	return u.analytics().InterestPaid(ctx, r)
}

func (u reportUseCases) TopPayees(ctx context.Context, r reporting.ReportRequest) (reporting.PayeesResult, error) {
	return u.analytics().TopPayees(ctx, r)
}

func (u reportUseCases) BudgetVsActual(ctx context.Context, r reporting.ReportRequest) (reporting.BudgetResult, error) {
	return u.analytics().BudgetVsActual(ctx, r)
}

func (u reportUseCases) Forecast(ctx context.Context, r reporting.ForecastRequest) (reporting.ForecastResult, error) {
	return u.analytics().Forecast(ctx, r)
}

func (u reportUseCases) List(ctx context.Context) ([]app.SavedReport, error) {
	return u.analytics().List(ctx)
}

func (u reportUseCases) Run(ctx context.Context, ref string) (reporting.SavedResult, error) {
	return u.analytics().Run(ctx, ref)
}

func (u reportUseCases) PlanSave(ctx context.Context, name, kind string, spec reporting.SavedSpec) (reporting.SavePlan, error) {
	return u.analytics().PlanSave(ctx, name, kind, spec)
}

func (u reportUseCases) ApplySave(ctx context.Context, p reporting.SavePlan) (app.SavedReport, error) {
	return u.analytics().ApplySave(ctx, p)
}

func (u reportUseCases) PlanRemove(ctx context.Context, ref string) (reporting.RemovePlan, error) {
	return u.analytics().PlanRemove(ctx, ref)
}

func (u reportUseCases) ApplyRemove(ctx context.Context, p reporting.RemovePlan) error {
	return u.analytics().ApplyRemove(ctx, p)
}

func (u reportUseCases) Today() (domain.Date, error) { return u.svc.Today() }

func (u reportUseCases) ListAccounts(ctx context.Context, req app.ListAccountsRequest) ([]app.AccountBalance, error) {
	return u.svc.ListAccounts(ctx, req)
}

func (u reportUseCases) Export(ctx context.Context, req importing.ExportRequest, w io.Writer) (int, error) {
	return u.importing.Export(ctx, req, w)
}

// importUseCases adapts importing.Service and app.Service to importer.UseCases.
type importUseCases struct {
	*importing.Service
	svc *app.Service
}

func (u importUseCases) ListAccounts(ctx context.Context, req app.ListAccountsRequest) ([]app.AccountBalance, error) {
	return u.svc.ListAccounts(ctx, req)
}
