package importer

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/pkg/fuiproto"
)

// resetSourceForm (re)builds the source form, ready for a new import.
func (m *Model) resetSourceForm() {
	m.notice = ""
	m.req = importing.PlanRequest{}
	m.connectorField = components.NewAutocomplete("Connector:", "csv/fui/revolut...", m.th)
	m.connectorField.SetSuggestions(connectorNames(m.connectors))
	m.fileField = components.NewAutocomplete("File:", "statement.csv", m.th)
	m.profileField = components.NewAutocomplete("CSV profile:", "mybank (import.profiles.<name>, csv only)", m.th)
	m.accountField = components.NewAutocomplete("Account ref/name:", "optional (csv connector)", m.th)
	m.sinceField = components.NewAutocomplete("Since:", "optional, YYYY-MM-DD", m.th)
	m.sourceForm = components.NewForm(
		&m.connectorField, &m.fileField, &m.profileField, &m.accountField, &m.sinceField,
	)
	m.mode = modeSource
	m.sourceForm.Focus()
}

func connectorNames(cs []importing.ConnectorInfo) []string {
	names := make([]string, len(cs))
	for i, c := range cs {
		names[i] = c.Name
	}
	return names
}

func (m *Model) handleSourceKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "shift+tab":
		m.sourceForm.Prev()
		return m, nil
	case "enter":
		if m.sourceForm.FocusedOpen() {
			m.sourceForm.Update(msg)
			return m, nil
		}
		if !m.sourceForm.AtLast() {
			m.sourceForm.Next()
			return m, nil
		}
		return m.submitSource()
	}
	m.sourceForm.Update(msg)
	return m, nil
}

// submitSource validates the source form and plans the import. A file ending in .fui.jsonl
// defaults the connector to "fui" when left blank, matching `fui import`.
func (m *Model) submitSource() (screens.Screen, tea.Cmd) {
	file := strings.TrimSpace(m.fileField.Value())
	if file == "" {
		m.notice = "File is required."
		return m, nil
	}
	connector := strings.TrimSpace(m.connectorField.Value())
	if connector == "" {
		if strings.HasSuffix(file, ".fui.jsonl") {
			connector = "fui"
		} else {
			m.notice = "Connector is required (only .fui.jsonl files default to \"fui\")."
			return m, nil
		}
	}
	m.notice = ""
	config := map[string]any{}
	if profile := strings.TrimSpace(m.profileField.Value()); profile != "" {
		config["profile"] = profile
	}
	if account := strings.TrimSpace(m.accountField.Value()); account != "" {
		config["account"] = account
	}
	fetch := fuiproto.FetchRequest{File: &file, Config: config}
	if since := strings.TrimSpace(m.sinceField.Value()); since != "" {
		fetch.Since = &since
	}
	m.req = importing.PlanRequest{Connector: connector, Fetch: fetch}
	return m, m.planImport()
}

// planLoadedMsg carries a fresh plan back into Update.
type planLoadedMsg struct{ plan importing.ImportPlan }

func (m *Model) planImport() tea.Cmd {
	req := m.req
	return func() tea.Msg {
		plan, err := m.uc.PlanImport(context.Background(), req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return planLoadedMsg{plan: plan}
	}
}

func (m *Model) handlePlanLoaded(msg planLoadedMsg) (screens.Screen, tea.Cmd) {
	m.plan = msg.plan
	if len(msg.plan.Unmapped) > 0 {
		m.mode = modeMapping
		return m, m.loadAccountsForMapping()
	}
	m.openPreview()
	return m, nil
}

func (m *Model) viewSource() string {
	out := m.th.Title.Render("Import") + "\n\n" + m.sourceForm.View()
	out += "\n\n" + m.th.Muted.Render("enter next/plan · tab/shift+tab move between fields")
	if m.notice != "" {
		out += "\n" + m.th.Warning.Render(m.notice)
	}
	return out
}
