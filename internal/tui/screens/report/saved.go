package report

import (
	"context"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

func savedColumns() []components.Column {
	return []components.Column{
		{Title: "Name", Width: 20, Align: lipgloss.Left},
		{Title: "Kind", Width: 20, Align: lipgloss.Left},
		{Title: "Params", Width: 40, Align: lipgloss.Left},
	}
}

// loadSaved fetches the saved report definitions.
func (m *Model) loadSaved() tea.Cmd {
	return func() tea.Msg {
		rows, err := m.uc.List(context.Background())
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return savedListLoadedMsg{rows: rows}
	}
}

func (m *Model) refreshSavedTable() {
	rows := make([]components.Row, len(m.saved))
	for i, s := range m.saved {
		rows[i] = components.Row{ID: s.Name, Cells: []string{s.Name, s.Kind, s.ParamsJSON}}
	}
	m.savedTable.SetRows(rows)
}

func (m *Model) handleSavedKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch {
	case key.Matches(msg, m.km.Up):
		m.savedTable.MoveUp(1)
	case key.Matches(msg, m.km.Down):
		m.savedTable.MoveDown(1)
	case key.Matches(msg, m.km.RunSaved):
		return m, m.runSaved()
	case key.Matches(msg, m.km.DelSaved):
		m.openRemoveConfirm()
	case key.Matches(msg, m.km.Back):
		m.mode = modeReport
	}
	return m, nil
}

// runSaved runs the saved report currently highlighted in the saved-reports table.
func (m *Model) runSaved() tea.Cmd {
	row := m.savedTable.SelectedRow()
	if row == nil {
		return nil
	}
	name := row.ID
	return func() tea.Msg {
		out, err := m.uc.Run(context.Background(), name)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return reportLoadedMsg{result: reportResult{kind: out.Kind, label: out.Name, saved: out.Name, data: out.Data}}
	}
}

func (m *Model) openRemoveConfirm() {
	row := m.savedTable.SelectedRow()
	if row == nil {
		return
	}
	m.confirm.Show("Remove saved report", "Remove saved report \""+row.ID+"\"?")
	m.mode = modeConfirmRemove
}

func (m *Model) handleConfirmKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	yes, no := m.confirm.HandleKey(msg)
	switch {
	case yes:
		return m, m.submitRemove()
	case no:
		m.mode = modeSaved
	}
	return m, nil
}

func (m *Model) submitRemove() tea.Cmd {
	row := m.savedTable.SelectedRow()
	if row == nil {
		return nil
	}
	ref := row.ID
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := m.uc.PlanRemove(ctx, ref)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if err := m.uc.ApplyRemove(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return savedRemovedMsg{}
	}
}

// submitSave validates and stores the current report (kind and params) under the entered name.
func (m *Model) submitSave() tea.Cmd {
	m.mode = modeReport
	name := m.fields.name.Value()
	spec := reporting.SavedSpec{
		Period: m.periodText, Account: m.accountText, Loan: m.loanText,
		Limit: m.limitInt(), MinBalance: m.minBalText,
	}
	kind := m.result.kind
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := m.uc.PlanSave(ctx, name, kind, spec)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if _, err := m.uc.ApplySave(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return screens.StatusMsg{Text: "Saved report \"" + name + "\"."}
	}
}
