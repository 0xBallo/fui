package budget

import (
	"fmt"
	"slices"

	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/app/budgeting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
)

func capColumns() []components.Column {
	return []components.Column{
		{Title: "Name", Width: 16, Align: lipgloss.Left},
		{Title: "Available", Width: 10, Align: lipgloss.Right},
		{Title: "Spent", Width: 10, Align: lipgloss.Right},
		{Title: "Planned", Width: 10, Align: lipgloss.Right},
		{Title: "Remaining", Width: 10, Align: lipgloss.Right},
		{Title: "Progress", Width: 24, Align: lipgloss.Left},
	}
}

func envelopeColumns(suggested bool) []components.Column {
	cols := []components.Column{
		{Title: "Name", Width: 16, Align: lipgloss.Left},
		{Title: "Carry in", Width: 10, Align: lipgloss.Right},
		{Title: "Assigned", Width: 10, Align: lipgloss.Right},
		{Title: "Spent", Width: 10, Align: lipgloss.Right},
		{Title: "Planned", Width: 10, Align: lipgloss.Right},
		{Title: "Balance", Width: 10, Align: lipgloss.Right},
		{Title: "Progress", Width: 24, Align: lipgloss.Left},
	}
	if suggested {
		cols = append(cols[:len(cols)-1], components.Column{Title: "Per month", Width: 10, Align: lipgloss.Right}, cols[len(cols)-1])
	}
	return cols
}

// budgetLabel is the name shown in a row, tagged with its period when the budget is yearly.
func budgetLabel(b domain.Budget) string {
	if b.Period == domain.BudgetYearly {
		return b.Name + " [" + string(b.Period) + "]"
	}
	return b.Name
}

// wideWidth is the terminal width from which the suggested monthly quota gets its own column;
// narrower screens still show it in the detail line.
const wideWidth = 100

// showSuggested reports whether the suggested monthly quota column is shown: there is a yearly
// envelope and the screen is wide enough to afford it.
func (m *Model) showSuggested() bool {
	return m.width >= wideWidth && slices.ContainsFunc(m.status.Envelopes, func(l budgeting.EnvelopeLine) bool {
		return l.Budget.Period == domain.BudgetYearly
	})
}

// refreshTable rebuilds the table columns and rows from the loaded status: a cap table in cap
// mode, an envelope table in envelope mode.
func (m *Model) refreshTable() {
	if m.status.Mode == envelopeMode {
		m.table.SetColumns(envelopeColumns(m.showSuggested()))
		m.table.SetRows(m.envelopeRows())
		return
	}
	m.table.SetColumns(capColumns())
	m.table.SetRows(m.capRows())
}

func (m *Model) capRows() []components.Row {
	rows := make([]components.Row, len(m.status.Caps))
	for i, l := range m.status.Caps {
		rows[i] = components.Row{ID: l.Budget.Name, Cells: []string{
			budgetLabel(l.Budget),
			domain.FormatAmount(l.Available, m.loc),
			domain.FormatAmount(l.Spent, m.loc),
			domain.FormatAmount(l.Planned, m.loc),
			domain.FormatAmount(l.Remaining, m.loc),
			progressBar(m.th, l.Percent, l.Budget.Thresholds),
		}}
	}
	return rows
}

func (m *Model) envelopeRows() []components.Row {
	suggested := m.showSuggested()
	rows := make([]components.Row, len(m.status.Envelopes))
	for i, l := range m.status.Envelopes {
		cells := []string{
			budgetLabel(l.Budget),
			domain.FormatAmount(l.CarryIn, m.loc),
			domain.FormatAmount(l.Assigned, m.loc),
			domain.FormatAmount(l.Spent, m.loc),
			domain.FormatAmount(l.Planned, m.loc),
			domain.FormatAmount(l.Balance, m.loc),
		}
		if suggested {
			q := ""
			if l.Budget.Period == domain.BudgetYearly {
				q = domain.FormatAmount(l.SuggestedMonthly, m.loc)
			}
			cells = append(cells, q)
		}
		rows[i] = components.Row{ID: l.Budget.Name, Cells: append(cells, progressBar(m.th, envelopePercent(l.EnvelopeMonth), l.Budget.Thresholds))}
	}
	return rows
}

// View renders the active panel.
func (m *Model) View() string {
	if m.help.IsOpen() {
		return m.help.View()
	}
	switch m.mode {
	case modeSetForm, modeAssignForm, modeMoveForm, modeNewForm, modeEditForm:
		return components.Modal(m.th, m.width, m.height, m.viewForm())
	case modeConfirm:
		return components.Modal(m.th, m.width, m.height, m.confirm.View())
	case modeList:
		return m.viewList()
	}
	return m.viewList()
}

func (m *Model) viewList() string {
	title := m.th.Title.Render(fmt.Sprintf("%s (%s)", m.monthIn, m.status.Mode))
	body := m.table.View()
	if m.table.SelectedRow() == nil {
		body = m.th.Muted.Render("No budgets yet: press n to create a budget")
	}
	lines := []string{title, "", body, "", m.viewDetail(), m.viewFooter()}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// viewDetail describes the selected budget: its period and warn/over thresholds, plus the
// suggested monthly quota for a yearly envelope.
func (m *Model) viewDetail() string {
	row := m.table.SelectedRow()
	if row == nil {
		return ""
	}
	var b domain.Budget
	var suggested domain.Money
	for _, l := range m.status.Caps {
		if l.Budget.Name == row.ID {
			b = l.Budget
		}
	}
	for _, l := range m.status.Envelopes {
		if l.Budget.Name == row.ID {
			b, suggested = l.Budget, l.SuggestedMonthly
		}
	}
	period := b.Period
	if period == "" {
		period = domain.BudgetMonthly
	}
	out := fmt.Sprintf("%s: %s, thresholds %d/%d", b.Name, period, b.Thresholds.Warn, b.Thresholds.Over)
	if period == domain.BudgetYearly && m.status.Mode == envelopeMode {
		out += ", suggested " + domain.FormatAmount(suggested, m.loc) + "/month"
	}
	return m.th.Muted.Render(out)
}

func (m *Model) viewFooter() string {
	foot := fmt.Sprintf("Uncovered: %s", domain.FormatAmount(m.status.Uncovered, m.loc))
	if m.status.Mode == envelopeMode {
		foot = fmt.Sprintf("To assign: %s   %s", domain.FormatAmount(m.status.ToAssign.ToAssign, m.loc), foot)
	}
	return m.th.Base.Render(foot)
}

func (m *Model) viewForm() string {
	if m.mode == modeNewForm || m.mode == modeEditForm {
		return m.viewBudgetForm()
	}
	return m.th.Title.Render(formTitle(m.mode)) + "\n\n" + m.form.View()
}

func formTitle(md mode) string {
	switch md {
	case modeAssignForm:
		return "Assign from unassigned pool"
	case modeMoveForm:
		return "Move between envelopes"
	case modeNewForm:
		return "New budget"
	case modeEditForm:
		return "Edit budget"
	case modeSetForm, modeList, modeConfirm:
		return "Set amount"
	}
	return "Set amount"
}
