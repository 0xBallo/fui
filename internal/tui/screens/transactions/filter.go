package transactions

import (
	"context"
	"errors"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

func (m *Model) openFilterForm() {
	m.filterAccount = components.NewAutocomplete("Account:", "any", m.th)
	m.filterAccount.SetSuggestions(m.accounts)
	if len(m.req.Accounts) > 0 {
		m.filterAccount.SetValue(m.req.Accounts[0])
	}
	m.filterCategory = components.NewAutocomplete("Category:", "any", m.th)
	m.filterCategory.SetSuggestions(m.categories)
	if len(m.req.Categories) > 0 {
		m.filterCategory.SetValue(m.req.Categories[0])
	}
	m.filterStatus = components.NewAutocomplete("Status:", "any", m.th)
	m.filterStatus.SetSuggestions(txStatuses())
	if len(m.req.Statuses) > 0 {
		m.filterStatus.SetValue(m.req.Statuses[0])
	}
	m.filterFrom = components.NewAutocomplete("From:", "any", m.th)
	m.filterFrom.SetValue(m.req.From)
	m.filterTo = components.NewAutocomplete("To:", "any", m.th)
	m.filterTo.SetValue(m.req.To)
	m.filterMin = components.NewAutocomplete("Min amount:", "any", m.th)
	m.filterMin.SetValue(m.req.MinAmount)
	m.filterMax = components.NewAutocomplete("Max amount:", "any", m.th)
	m.filterMax.SetValue(m.req.MaxAmount)
	m.filterForm = components.NewForm(
		&m.filterAccount, &m.filterCategory, &m.filterStatus,
		&m.filterFrom, &m.filterTo, &m.filterMin, &m.filterMax,
	)
	m.mode = modeFilter
	m.filterForm.Focus()
}

func (m *Model) handleFilterKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		return m, nil
	case "shift+tab":
		m.filterForm.Prev()
		return m, nil
	case "enter":
		if m.filterForm.FocusedOpen() {
			m.filterForm.Update(msg)
			return m, nil
		}
		if !m.filterForm.AtLast() {
			m.filterForm.Next()
			return m, nil
		}
		return m, m.submitFilter()
	}
	m.filterForm.Update(msg)
	return m, nil
}

func (m *Model) submitFilter() tea.Cmd {
	m.req = app.ListTxRequest{
		From: m.filterFrom.Value(), To: m.filterTo.Value(),
		MinAmount: m.filterMin.Value(), MaxAmount: m.filterMax.Value(),
		Text: m.req.Text, // the quick search field is independent of the filter form
	}
	if v := m.filterAccount.Value(); v != "" {
		m.req.Accounts = []string{v}
	}
	if v := m.filterCategory.Value(); v != "" {
		m.req.Categories = []string{v}
	}
	if v := m.filterStatus.Value(); v != "" {
		m.req.Statuses = []string{v}
	}
	return m.load()
}

func (m *Model) viewFilter() string {
	return m.th.Title.Render("Filter transactions") + "\n\n" + m.filterForm.View()
}

func (m *Model) openSearch() {
	m.searchField = components.NewAutocomplete("Search:", "description contains…", m.th)
	m.searchField.SetValue(m.req.Text)
	m.mode = modeSearch
	m.searchField.Focus()
}

func (m *Model) handleSearchKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		return m, nil
	case "enter":
		m.req.Text = m.searchField.Value()
		return m, m.load()
	}
	updated, cmd := m.searchField.Update(msg)
	m.searchField = updated
	return m, cmd
}

func (m *Model) viewSearch() string {
	return m.viewList() + "\n\n" + m.searchField.View()
}

func (m *Model) openCategorize() {
	ids := m.table.SelectedIDs()
	if len(ids) == 0 {
		row := m.table.SelectedRow()
		if row == nil {
			return
		}
		m.table.ToggleSelect()
	}
	m.categorizeField = components.NewAutocomplete("New category:", "", m.th)
	m.categorizeField.SetSuggestions(m.categories)
	m.mode = modeCategorize
	m.categorizeField.Focus()
}

func (m *Model) handleCategorizeKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		return m, nil
	case "enter":
		if m.categorizeField.DropdownOpen() && m.categorizeField.Value() != "" {
			updated, cmd := m.categorizeField.Update(msg)
			m.categorizeField = updated
			return m, cmd
		}
		return m, m.submitCategorize(false)
	}
	updated, cmd := m.categorizeField.Update(msg)
	m.categorizeField = updated
	return m, cmd
}

// parseTxIDs parses every table row ID in strs back into the TransactionID it was formatted from.
func parseTxIDs(strs []string) ([]domain.TransactionID, error) {
	ids := make([]domain.TransactionID, len(strs))
	for i, s := range strs {
		id, err := parseTxID(s)
		if err != nil {
			return nil, err
		}
		ids[i] = id
	}
	return ids, nil
}

func (m *Model) submitCategorize(confirmReconciled bool) tea.Cmd {
	ids, parseErr := parseTxIDs(m.table.SelectedIDs())
	category := m.categorizeField.Value()
	uc, listReq := m.uc, m.req
	return func() tea.Msg {
		if parseErr != nil {
			return screens.ErrorMsg{Err: parseErr}
		}
		ctx := context.Background()
		plan, err := uc.PlanCategorizeTx(ctx, app.CategorizeTxRequest{
			IDs: ids, Category: category, ConfirmReconciled: confirmReconciled,
		})
		if err != nil {
			if !confirmReconciled && errors.Is(err, domain.ErrReconciled) {
				return needReconcileMsg{purpose: confirmCategorizeReconciled}
			}
			return screens.ErrorMsg{Err: err}
		}
		if err := uc.ApplyCategorizeTx(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return reload(ctx, uc, listReq, true)
	}
}

func (m *Model) viewCategorize() string {
	return m.viewList() + "\n\n" + m.categorizeField.View()
}
