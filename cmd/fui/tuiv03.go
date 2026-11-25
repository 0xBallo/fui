package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/investing"
	"github.com/0xballo/fui/internal/app/lending"
	"github.com/0xballo/fui/internal/app/recurring"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/app/simulating"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// The adapters below back the v0.3 TUI features. Like the v0.2 ones they build their service per
// call, because the module gate lives in the service and Settings can change the set live.

// positionsUseCases adapts investing.Service to accounts.PositionsUseCases.
type positionsUseCases struct{ build func() *investing.Service }

func (u positionsUseCases) ListPositions(ctx context.Context, req investing.PositionsRequest) ([]investing.PositionLine, error) {
	return u.build().ListPositions(ctx, req)
}

func (u positionsUseCases) PlanSetPrice(ctx context.Context, req investing.SetPriceRequest) (investing.SetPricePlan, error) {
	return u.build().PlanSetPrice(ctx, req)
}

func (u positionsUseCases) ApplySetPrice(ctx context.Context, p investing.SetPricePlan) error {
	return u.build().ApplySetPrice(ctx, p)
}

// netWorthUseCases adapts reporting.Analytics to dashboard.NetWorthUseCases.
type netWorthUseCases struct{ analytics func() *reporting.Analytics }

func (u netWorthUseCases) NetWorthAt(ctx context.Context, at domain.Date) (reporting.NetWorthPoint, error) {
	return u.analytics().NetWorthAt(ctx, at)
}

// compareUseCases adapts reporting.Analytics to report.CompareUseCases.
type compareUseCases struct{ analytics func() *reporting.Analytics }

func (u compareUseCases) ComparePeriods(ctx context.Context, req reporting.CompareRequest) (reporting.CompareReport, error) {
	return u.analytics().ComparePeriods(ctx, req)
}

// insightUseCases adapts reporting.Insights to dashboard.InsightUseCases.
type insightUseCases struct{ build func() *reporting.Insights }

func (u insightUseCases) Run(ctx context.Context) ([]reporting.InsightView, error) {
	return u.build().Run(ctx)
}

func (u insightUseCases) Dismiss(ctx context.Context, key string) error {
	return u.build().Dismiss(ctx, key)
}

func (u insightUseCases) AcceptSubscription(ctx context.Context, key string) (reporting.AcceptResult, error) {
	return u.build().AcceptSubscription(ctx, key)
}

// simUseCases adapts simulating.Service, binding the live enabled-module set, to report.SimUseCases.
type simUseCases struct {
	svc     *simulating.Service
	modules func() module.Set
	base    domain.Currency
}

func (u simUseCases) Run(ctx context.Context, req simulating.RunRequest) (simulating.RunResult, error) {
	return u.svc.Run(ctx, u.modules(), req)
}

func (u simUseCases) PlanSave(ctx context.Context, name string, req simulating.RunRequest) (simulating.SavePlan, error) {
	return u.svc.PlanSave(ctx, u.modules(), name, req)
}

func (u simUseCases) ApplySave(ctx context.Context, p simulating.SavePlan) (app.SavedSimulation, error) {
	return u.svc.ApplySave(ctx, p)
}

func (u simUseCases) List(ctx context.Context) ([]app.SavedSimulation, error) {
	return u.svc.List(ctx, u.modules())
}

func (u simUseCases) Rerun(ctx context.Context, ref string) (simulating.RerunResult, error) {
	return u.svc.Rerun(ctx, u.modules(), ref)
}

func (u simUseCases) Compare(ctx context.Context, refs []string) (simulating.CompareResult, error) {
	return u.svc.Compare(ctx, u.modules(), refs)
}

func (u simUseCases) BaseCurrency() domain.Currency { return u.base }

// flowUseCases adapts the CashFlow report to dashboard.FlowUseCases.
type flowUseCases struct {
	run func(context.Context, reporting.CashFlowParams) (reporting.CashFlowResult, error)
}

func (u flowUseCases) CashFlow(ctx context.Context, p reporting.CashFlowParams) (reporting.CashFlowResult, error) {
	return u.run(ctx, p)
}

// recurrenceAlerts adapts recurring.Service.List to dashboard.AlertRecurringUseCases.
type recurrenceAlerts struct {
	svc     *recurring.Service
	modules func() module.Set
}

func (u recurrenceAlerts) ListRecurrences(ctx context.Context) ([]app.Recurrence, error) {
	return u.svc.List(ctx, u.modules())
}

// pendingRecurring and pendingLoans are the parts of recurring.Service and lending.Service that
// resolve pending transactions, so the dispatch can be tested without a store.
type pendingRecurring interface {
	PlanConfirm(ctx context.Context, enabled module.Set, id domain.TransactionID) (recurring.ConfirmPlan, error)
	ApplyConfirm(ctx context.Context, p recurring.ConfirmPlan) error
}

type pendingLoans interface {
	PlanConfirm(ctx context.Context, enabled module.Set, id domain.TransactionID) (lending.ConfirmPlan, error)
	ApplyConfirm(ctx context.Context, p lending.ConfirmPlan) error
}

type pendingTxLookup interface {
	ShowTx(ctx context.Context, id domain.TransactionID) (app.TxView, error)
	PlanRemoveTx(ctx context.Context, req app.RemoveTxRequest) (app.RemoveTxPlan, error)
	ApplyRemoveTx(ctx context.Context, p app.RemoveTxPlan) error
}

// pendingActions adapts the recurring and lending services to transactions.PendingActions: a
// pending transaction is confirmed by the module that created it, found from its recurrence link
// or, failing that, from the loan schedule.
type pendingActions struct {
	txs       pendingTxLookup
	recurring pendingRecurring
	loans     pendingLoans
	modules   func() module.Set
}

const (
	noticeNotPending  = "not a recurring or loan pending transaction"
	noticeNoSkip      = "skip is only supported for recurring transactions (loan installments cannot be skipped yet)"
	noticeRecurBooked = "recurring transaction confirmed"
	noticeLoanBooked  = "loan installment confirmed"
	noticeRecurSkip   = "pending recurring transaction skipped"
)

// Confirm books the pending transaction through the module it belongs to.
func (a pendingActions) Confirm(ctx context.Context, id domain.TransactionID) (string, error) {
	view, err := a.txs.ShowTx(ctx, id)
	if err != nil {
		return "", fmt.Errorf("show transaction %d: %w", id, err)
	}
	enabled := a.modules()
	if view.Transaction.RecurrenceID != 0 {
		plan, err := a.recurring.PlanConfirm(ctx, enabled, id)
		if err != nil {
			return "", fmt.Errorf("confirm recurring transaction: %w", err)
		}
		if err := a.recurring.ApplyConfirm(ctx, plan); err != nil {
			return "", fmt.Errorf("confirm recurring transaction: %w", err)
		}
		return noticeRecurBooked, nil
	}
	plan, err := a.loans.PlanConfirm(ctx, enabled, id)
	if err != nil {
		if notLoanInstallment(err) {
			return noticeNotPending, nil
		}
		return "", fmt.Errorf("confirm loan installment: %w", err)
	}
	if err := a.loans.ApplyConfirm(ctx, plan); err != nil {
		return "", fmt.Errorf("confirm loan installment: %w", err)
	}
	return noticeLoanBooked, nil
}

// Skip discards the pending transaction of a recurrence, like tx rm, leaving the recurrence
// untouched: catch-up has already advanced its next date past this occurrence. Loan installments
// have no skip yet, and the loan link is only visible to lending, so any transaction without a
// recurrence gets the same notice.
func (a pendingActions) Skip(ctx context.Context, id domain.TransactionID) (string, error) {
	view, err := a.txs.ShowTx(ctx, id)
	if err != nil {
		return "", fmt.Errorf("show transaction %d: %w", id, err)
	}
	if view.Transaction.RecurrenceID == 0 {
		return noticeNoSkip, nil
	}
	if err := app.WrapModuleError(a.modules().Require(module.Recurring)); err != nil {
		return "", fmt.Errorf("skip recurring transaction: %w", err)
	}
	plan, err := a.txs.PlanRemoveTx(ctx, app.RemoveTxRequest{ID: id})
	if err != nil {
		return "", fmt.Errorf("skip recurring transaction: %w", err)
	}
	if err := a.txs.ApplyRemoveTx(ctx, plan); err != nil {
		return "", fmt.Errorf("skip recurring transaction: %w", err)
	}
	return noticeRecurSkip, nil
}

// notLoanInstallment reports whether lending rejected the transaction for not being a pending loan
// payment, as opposed to a real failure.
func notLoanInstallment(err error) bool {
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.CodeInvalidValue {
		return false
	}
	field, _ := de.Details["field"].(string)
	return field == "transaction" || field == "status"
}
