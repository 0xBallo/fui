package budget

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
)

func capColumns() []components.Column {
	return []components.Column{
		{Title: "Name", Width: 18, Align: lipgloss.Left},
		{Title: "Available", Width: 12, Align: lipgloss.Right},
		{Title: "Spent", Width: 12, Align: lipgloss.Right},
		{Title: "Planned", Width: 12, Align: lipgloss.Right},
		{Title: "Remaining", Width: 12, Align: lipgloss.Right},
		{Title: "Progress", Width: 24, Align: lipgloss.Left},
	}
}

func envelopeColumns() []components.Column {
	return []components.Column{
		{Title: "Name", Width: 18, Align: lipgloss.Left},
		{Title: "Carry in", Width: 12, Align: lipgloss.Right},
		{Title: "Assigned", Width: 12, Align: lipgloss.Right},
		{Title: "Spent", Width: 12, Align: lipgloss.Right},
		{Title: "Planned", Width: 12, Align: lipgloss.Right},
		{Title: "Balance", Width: 12, Align: lipgloss.Right},
		{Title: "Progress", Width: 24, Align: lipgloss.Left},
	}
}

// refreshTable rebuilds the table columns and rows from the loaded status: a cap table in cap
// mode, an envelope table in envelope mode.
func (m *Model) refreshTable() {
	if m.status.Mode == envelopeMode {
		m.table.SetColumns(envelopeColumns())
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
			l.Budget.Name,
			domain.FormatAmount(l.Available, m.loc),
			domain.FormatAmount(l.Spent, m.loc),
			domain.FormatAmount(l.Planned, m.loc),
			domain.FormatAmount(l.Remaining, m.loc),
			progressBar(m.th, l.Percent, l.State),
		}}
	}
	return rows
}

func (m *Model) envelopeRows() []components.Row {
	rows := make([]components.Row, len(m.status.Envelopes))
	for i, l := range m.status.Envelopes {
		rows[i] = components.Row{ID: l.Budget.Name, Cells: []string{
			l.Budget.Name,
			domain.FormatAmount(l.CarryIn, m.loc),
			domain.FormatAmount(l.Assigned, m.loc),
			domain.FormatAmount(l.Spent, m.loc),
			domain.FormatAmount(l.Planned, m.loc),
			domain.FormatAmount(l.Balance, m.loc),
			progressBar(m.th, envelopePercent(l.EnvelopeMonth), envelopeState(l.EnvelopeMonth)),
		}}
	}
	return rows
}

// View renders the active panel.
func (m *Model) View() string {
	switch m.mode {
	case modeSetForm, modeAssignForm, modeMoveForm:
		return m.viewForm()
	case modeConfirm:
		return m.viewList() + "\n\n" + m.confirm.View()
	case modeList:
		return m.viewList()
	}
	return m.viewList()
}

func (m *Model) viewList() string {
	title := m.th.Title.Render(fmt.Sprintf("Budget — %s (%s)", m.monthIn, m.status.Mode))
	lines := []string{title, "", m.table.View(), "", m.viewFooter()}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *Model) viewFooter() string {
	foot := fmt.Sprintf("Uncovered: %s", domain.FormatAmount(m.status.Uncovered, m.loc))
	if m.status.Mode == envelopeMode {
		foot = fmt.Sprintf("To assign: %s   %s", domain.FormatAmount(m.status.ToAssign.ToAssign, m.loc), foot)
	}
	return m.th.Base.Render(foot)
}

func (m *Model) viewForm() string {
	return m.th.Title.Render(formTitle(m.mode)) + "\n\n" + m.form.View()
}

func formTitle(md mode) string {
	switch md {
	case modeAssignForm:
		return "Assign from unassigned pool"
	case modeMoveForm:
		return "Move between envelopes"
	case modeSetForm, modeList, modeConfirm:
		return "Set amount"
	}
	return "Set amount"
}
