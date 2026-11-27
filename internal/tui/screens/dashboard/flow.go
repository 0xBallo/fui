package dashboard

import (
	"context"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/theme"
)

// FlowUseCases is the optional port behind the dashboard flow chart; without it the section is hidden.
type FlowUseCases interface {
	CashFlow(ctx context.Context, p reporting.CashFlowParams) (reporting.CashFlowResult, error)
}

// SetFlowUseCases enables the flow chart by swapping the disabled placeholder for a live section.
func (m *Model) SetFlowUseCases(uc FlowUseCases) {
	for i, s := range m.sections {
		if _, ok := s.(flowSection); ok {
			m.sections[i] = &flowBlock{uc: uc, loc: m.loc, th: m.th}
			return
		}
	}
}

// flowSection is the disabled placeholder used until a FlowUseCases is wired.
type flowSection struct{}

func (flowSection) load(sectionParams) tea.Cmd     { return nil }
func (flowSection) update(tea.Msg) (tea.Cmd, bool) { return nil, false }
func (flowSection) view(int) string                { return "" }
func (flowSection) keys() []key.Binding            { return nil }

// flowBlock charts income, expenses and savings of the selected period.
type flowBlock struct {
	uc  FlowUseCases
	loc domain.Locale
	th  theme.Theme

	period domain.Period
	res    reporting.CashFlowResult
	err    error
	loaded bool
}

type flowMsg struct {
	period domain.Period
	res    reporting.CashFlowResult
	err    error
}

func (s *flowBlock) load(p sectionParams) tea.Cmd {
	s.period = p.Period
	return func() tea.Msg {
		res, err := s.uc.CashFlow(context.Background(), reporting.CashFlowParams{
			Period: p.Period, BaseCurrency: p.Base, MonthStartDay: p.MonthStartDay,
		})
		return flowMsg{period: p.Period, res: res, err: err}
	}
}

func (s *flowBlock) update(msg tea.Msg) (tea.Cmd, bool) {
	m, ok := msg.(flowMsg)
	if !ok {
		return nil, false
	}
	if m.period == s.period {
		s.res, s.err, s.loaded = m.res, m.err, true
	}
	return nil, true
}

func (s *flowBlock) keys() []key.Binding { return nil }

func (s *flowBlock) view(width int) string {
	title := s.th.Header.Render("Cash flow") + "\n"
	if s.err != nil {
		return title + s.th.Muted.Render("n/a ("+s.err.Error()+")")
	}
	if !s.loaded {
		return title + s.th.Muted.Render("loading…")
	}
	chart := components.NewBarChart(s.th)
	chart.SetWidth(width)
	chart.SetItems(s.items())
	return title + chart.View()
}

func (s *flowBlock) items() []components.BarItem {
	r := s.res
	return []components.BarItem{
		{Label: "Income", Value: abs(r.TotalInflow.Minor()), DisplayValue: domain.FormatAmount(r.TotalInflow, s.loc), Style: s.th.Positive},
		{Label: "Expenses", Value: abs(r.TotalOutflow.Minor()), DisplayValue: domain.FormatAmount(r.TotalOutflow, s.loc), Style: s.th.Negative},
		{Label: "Savings", Value: abs(r.TotalNet.Minor()), DisplayValue: domain.FormatAmount(r.TotalNet, s.loc), Style: s.th.Base},
	}
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
