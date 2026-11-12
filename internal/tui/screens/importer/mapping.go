package importer

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

// mappingAccountsMsg carries the existing accounts back into Update, to suggest as mapping
// targets for the plan's unmapped connector refs.
type mappingAccountsMsg struct{ accounts []app.AccountBalance }

func (m *Model) loadAccountsForMapping() tea.Cmd {
	return func() tea.Msg {
		accts, err := m.uc.ListAccounts(context.Background(), app.ListAccountsRequest{})
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return mappingAccountsMsg{accounts: accts}
	}
}

// openMappingForm builds one field per unmapped connector ref, suggesting the existing accounts
// plus the option to create a new one.
func (m *Model) openMappingForm(accts []app.AccountBalance) {
	suggestions := make([]string, 0, len(accts)+1)
	suggestions = append(suggestions, createAccountOption)
	for _, a := range accts {
		suggestions = append(suggestions, a.Account.Name)
	}
	refs := m.plan.Unmapped
	m.mappingFields = make([]components.Autocomplete, len(refs))
	fields := make([]*components.Autocomplete, len(refs))
	for i, u := range refs {
		label := fmt.Sprintf("%s (%s, %s):", u.Name, u.Currency, u.Kind)
		m.mappingFields[i] = components.NewAutocomplete(label, createAccountOption, m.th)
		m.mappingFields[i].SetSuggestions(suggestions)
		fields[i] = &m.mappingFields[i]
	}
	m.mappingForm = components.NewForm(fields...)
	m.mode = modeMapping
	m.mappingForm.Focus()
}

func (m *Model) handleMappingKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.resetSourceForm()
		return m, nil
	case "shift+tab":
		m.mappingForm.Prev()
		return m, nil
	case "enter":
		if m.mappingForm.FocusedOpen() {
			m.mappingForm.Update(msg)
			return m, nil
		}
		if !m.mappingForm.AtLast() {
			m.mappingForm.Next()
			return m, nil
		}
		return m, m.submitMapping()
	}
	m.mappingForm.Update(msg)
	return m, nil
}

// submitMapping records each ref's choice (bind to an existing account, or create a new one)
// into the plan request and re-plans the import, mirroring the CLI's interactive unmapped-
// account prompt (internal/cli/import_unmapped.go).
func (m *Model) submitMapping() tea.Cmd {
	if m.req.Mappings == nil {
		m.req.Mappings = map[string]string{}
	}
	for i, u := range m.plan.Unmapped {
		choice := strings.TrimSpace(m.mappingFields[i].Value())
		if choice == "" || choice == createAccountOption {
			m.req.CreateAccounts = true
			continue
		}
		m.req.Mappings[u.Ref] = choice
	}
	return m.planImport()
}

func (m *Model) viewMapping() string {
	out := m.th.Title.Render("Map unmapped accounts") + "\n"
	out += m.th.Base.Render(fmt.Sprintf("%d connector account(s) need a target", len(m.plan.Unmapped)))
	out += "\n\n" + m.mappingForm.View()
	out += "\n\n" + m.th.Muted.Render("enter next/replan · esc cancel import")
	return out
}
