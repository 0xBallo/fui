package budget

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app/budgeting"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

// openSetForm opens the amount field to set the selected budget's amount for the month shown
// (for a cap, its available cap; for an envelope, its reference target).
func (m *Model) openSetForm() {
	row := m.table.SelectedRow()
	if row == nil {
		return
	}
	m.target = row.ID
	m.amountField = components.NewAutocomplete("Amount:", "e.g. 400", m.th)
	m.form = components.NewForm(&m.amountField)
	m.mode = modeSetForm
	m.form.Focus()
}

// openAssignForm opens the amount field to assign money from the unassigned pool ("Da
// assegnare") to the selected envelope. It needs envelope mode.
func (m *Model) openAssignForm() {
	if m.status.Mode != envelopeMode || m.table.SelectedRow() == nil {
		return
	}
	m.target = m.table.SelectedRow().ID
	m.amountField = components.NewAutocomplete("Amount:", "e.g. 100", m.th)
	m.form = components.NewForm(&m.amountField)
	m.mode = modeAssignForm
	m.form.Focus()
}

// openMoveForm opens the destination and amount fields to move money from the selected envelope
// to another one. It needs envelope mode.
func (m *Model) openMoveForm() {
	if m.status.Mode != envelopeMode || m.table.SelectedRow() == nil {
		return
	}
	m.target = m.table.SelectedRow().ID
	m.toField = components.NewAutocomplete("To:", "destination envelope", m.th)
	m.toField.SetSuggestions(m.otherEnvelopeNames(m.target))
	m.amountField = components.NewAutocomplete("Amount:", "e.g. 50", m.th)
	m.form = components.NewForm(&m.toField, &m.amountField)
	m.mode = modeMoveForm
	m.form.Focus()
}

func (m *Model) otherEnvelopeNames(exclude string) []string {
	var out []string
	for _, l := range m.status.Envelopes {
		if l.Budget.Name != exclude {
			out = append(out, l.Budget.Name)
		}
	}
	return out
}

func (m *Model) handleFormKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		return m, nil
	case "shift+tab":
		m.form.Prev()
		return m, nil
	case "enter":
		if m.form.FocusedOpen() {
			m.form.Update(msg)
			return m, nil
		}
		if !m.form.AtLast() {
			m.form.Next()
			return m, nil
		}
		return m, m.submitForm()
	}
	m.form.Update(msg)
	return m, nil
}

func (m *Model) submitForm() tea.Cmd {
	switch m.mode {
	case modeAssignForm:
		return m.submitMove("", m.target)
	case modeMoveForm:
		return m.submitMove(m.target, m.toField.Value())
	case modeSetForm, modeList, modeConfirm:
		return m.submitSet()
	}
	return m.submitSet()
}

func (m *Model) submitSet() tea.Cmd {
	req := budgeting.SetRequest{Name: m.target, Amount: m.amountField.Value(), Month: m.monthIn}
	month := m.monthIn
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := m.uc.PlanSet(ctx, req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if _, err := m.uc.ApplySet(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.reload(ctx, month)
	}
}

func (m *Model) submitMove(from, to string) tea.Cmd {
	req := budgeting.MoveRequest{From: from, To: to, Amount: m.amountField.Value()}
	month := m.monthIn
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := m.uc.PlanMove(ctx, req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if err := m.uc.ApplyMove(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.reload(ctx, month)
	}
}

// reload re-fetches the status of month after a form submission, converting a lookup failure
// into the same error message the global status bar shows.
func (m *Model) reload(ctx context.Context, month string) tea.Msg {
	st, err := m.uc.Status(ctx, month)
	if err != nil {
		return screens.ErrorMsg{Err: err}
	}
	return statusLoadedMsg{st: st}
}
