package report

import (
	"context"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
)

// resolvePeriod turns text (empty, or a relative period reporting.ValidRelativePeriod accepts)
// into a domain.Period; an empty Period lets the report use case fall back to the current
// accounting month on its own.
func (m *Model) resolvePeriod(text string) (domain.Period, error) {
	if text == "" {
		return domain.Period{}, nil
	}
	if !reporting.ValidRelativePeriod(text) {
		return domain.Period{}, &domain.Error{
			Code: domain.CodeInvalidValue, Message: "unknown relative period " + text,
			Details: map[string]any{"field": "period", "value": text},
		}
	}
	today, err := m.uc.Today()
	if err != nil {
		return domain.Period{}, err
	}
	return reporting.ResolveRelative(text, today, m.monthStartDay)
}

// limitInt parses the limit field, defaulting to zero (the report use case's own default) on an
// empty or invalid value.
func (m *Model) limitInt() int {
	n, err := strconv.Atoi(m.limitText)
	if err != nil {
		return 0
	}
	return n
}

// horizonInt parses the forecast horizon field the same way limitInt parses the limit field.
func (m *Model) horizonInt() int {
	n, err := strconv.Atoi(m.horizonText)
	if err != nil {
		return 0
	}
	return n
}

// runCurrent runs the active predefined report with the params currently set on the screen.
func (m *Model) runCurrent() tea.Cmd {
	k := predefinedKinds()[m.kindIdx]
	period, err := m.resolvePeriod(m.periodText)
	if err != nil {
		return errCmd(err)
	}
	req := reporting.ReportRequest{Period: period, Account: m.accountText, Loan: m.loanText, Limit: m.limitInt()}
	fReq := reporting.ForecastRequest{HorizonDays: m.horizonInt(), MinBalance: m.minBalText}
	kind, label := k.Kind, k.Label
	return func() tea.Msg {
		ctx := context.Background()
		data, err := m.runKind(ctx, kind, req, fReq)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return reportLoadedMsg{result: reportResult{kind: kind, label: label, data: data}}
	}
}

// runKind dispatches to the use case of kind.
func (m *Model) runKind(ctx context.Context, kind string, req reporting.ReportRequest, fReq reporting.ForecastRequest) (any, error) {
	switch kind {
	case reporting.KindBalance:
		return m.uc.BalanceOverTime(ctx, req)
	case reporting.KindNetWorth:
		return m.uc.NetWorth(ctx, req)
	case reporting.KindBudget:
		return m.uc.BudgetVsActual(ctx, req)
	case reporting.KindDebt:
		return m.uc.RemainingDebt(ctx, req)
	case reporting.KindInterest:
		return m.uc.InterestPaid(ctx, req)
	case reporting.KindPayees:
		return m.uc.TopPayees(ctx, req)
	case reporting.KindForecast:
		return m.uc.Forecast(ctx, fReq)
	}
	return nil, &domain.Error{Code: domain.CodeInvalidValue, Message: "unknown report kind " + kind}
}

// errCmd wraps err into the command the root status bar surfaces.
func errCmd(err error) tea.Cmd {
	return func() tea.Msg { return screens.ErrorMsg{Err: err} }
}
