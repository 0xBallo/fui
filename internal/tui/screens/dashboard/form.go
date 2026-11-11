package dashboard

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

// openAddForm opens the quick-add form: account, amount, description, and an optional category —
// the full set of fields lives in the Transactions screen's Add form.
func (m *Model) openAddForm() {
	m.accountF = components.NewAutocomplete("Account:", "e.g. Checking", m.th)
	m.accountF.SetSuggestions(m.accounts)
	m.amountF = components.NewAutocomplete("Amount:", "e.g. -42.50", m.th)
	m.descF = components.NewAutocomplete("Description:", "", m.th)
	m.categoryF = components.NewAutocomplete("Category:", "optional", m.th)
	m.categoryF.SetSuggestions(m.categories)
	m.form = components.NewForm(&m.accountF, &m.amountF, &m.descF, &m.categoryF)
	m.mode = modeForm
	m.form.Focus()
}

func (m *Model) handleFormKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeView
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
		return m, m.submitAdd()
	}
	m.form.Update(msg)
	return m, nil
}

func (m *Model) submitAdd() tea.Cmd {
	req := app.AddTxRequest{
		Account: m.accountF.Value(), Amount: m.amountF.Value(),
		Description: m.descF.Value(), Category: m.categoryF.Value(),
	}
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := m.uc.PlanAddTx(ctx, req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if _, err := m.uc.ApplyAddTx(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.load()()
	}
}

func (m *Model) viewForm() string {
	return m.th.Title.Render("Quick add") + "\n\n" + m.form.View()
}
