package dashboard

import (
	"context"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/theme"
)

// recentLimit is how many transactions the dashboard lists.
const recentLimit = 10

// recentSection lists the newest transactions of the selected period.
type recentSection struct {
	uc  UseCases
	loc domain.Locale
	th  theme.Theme
	all key.Binding

	period domain.Period
	loaded bool
	items  []app.TxView
	err    error
}

type recentMsg struct {
	period domain.Period
	items  []app.TxView
	err    error
}

func newRecentSection(uc UseCases, loc domain.Locale, th theme.Theme) *recentSection {
	return &recentSection{
		uc: uc, loc: loc, th: th,
		all: key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "all transactions")),
	}
}

func (s *recentSection) load(p sectionParams) tea.Cmd {
	s.period = p.Period
	return func() tea.Msg {
		page, err := s.uc.ListTx(context.Background(), app.ListTxRequest{
			From: p.Period.From().String(), To: p.Period.To().String(), Limit: recentLimit,
		})
		return recentMsg{period: p.Period, items: page.Items, err: err}
	}
}

func (s *recentSection) update(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case recentMsg:
		if msg.period == s.period {
			s.items, s.err, s.loaded = msg.items, msg.err, true
		}
		return nil, true
	case tea.KeyMsg:
		if key.Matches(msg, s.all) {
			return func() tea.Msg { return screens.SwitchScreenMsg{Target: screens.TabTransactions} }, true
		}
	}
	return nil, false
}

func (s *recentSection) keys() []key.Binding { return []key.Binding{s.all} }

func (s *recentSection) view(width int) string {
	if !s.loaded {
		return ""
	}
	out := s.th.Header.Render("Recent transactions") + "\n"
	if s.err != nil {
		return out + s.th.Muted.Render("n/a ("+s.err.Error()+")")
	}
	if len(s.items) == 0 {
		return out + s.th.Muted.Render("no transactions in this period")
	}
	tbl := components.NewTable([]components.Column{
		{Title: "Date", Width: 12, Align: lipgloss.Left},
		{Title: "Description", Width: 30, Align: lipgloss.Left},
		{Title: "Category", Width: 16, Align: lipgloss.Left},
		{Title: "Amount", Width: 14, Align: lipgloss.Right},
	}, s.th)
	tbl.SetWidth(width)
	tbl.SetHeight(len(s.items) + 3)
	tbl.SetRows(s.rows())
	return out + tbl.View() + "\n" + s.th.Muted.Render("→ View all transactions (t)")
}

func (s *recentSection) rows() []components.Row {
	rows := make([]components.Row, len(s.items))
	for i, v := range s.items {
		tx := v.Transaction
		rows[i] = components.Row{
			Cells: []string{tx.Date.Format(s.loc), tx.Description, v.Category, domain.FormatAmount(tx.Amount, s.loc)},
			Style: s.th.FlowStyle(recentFlow(v), tx.Status == domain.StatusPending),
		}
	}
	return rows
}

// recentFlow classifies a row for coloring; the sign stays in the amount text so color is never the only cue.
func recentFlow(v app.TxView) theme.Flow {
	switch {
	case v.Transfer != nil:
		return theme.FlowTransfer
	case v.Transaction.Amount.Sign() < 0:
		return theme.FlowExpense
	case v.Transaction.Amount.Sign() > 0:
		return theme.FlowIncome
	}
	return theme.FlowTransfer
}
