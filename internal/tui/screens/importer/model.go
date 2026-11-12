package importer

import (
	"context"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/theme"
)

// mode tracks which panel of the screen is active.
type mode int

const (
	modeSource mode = iota
	modeMapping
	modePreview
	modeConfirm
	modeResult
)

// createAccountOption is the mapping suggestion meaning "create a new account for this ref",
// mirroring the CLI's interactive unmapped-account prompt (see internal/cli/import_unmapped.go).
const createAccountOption = "+ create new account"

// Model is the import screen: a source form (connector, file, profile, account ref, since),
// an optional unmapped-accounts mapping form, a row preview with include/exclude toggling, a
// confirmation dialog and a result summary. Update and View never touch the store; every use
// case call runs inside a tea.Cmd.
type Model struct {
	uc  UseCases
	loc domain.Locale
	th  theme.Theme
	km  KeyMap

	width, height int
	mode          mode
	notice        string

	connectors []importing.ConnectorInfo

	connectorField components.Autocomplete
	fileField      components.Autocomplete
	profileField   components.Autocomplete
	accountField   components.Autocomplete
	sinceField     components.Autocomplete
	sourceForm     components.Form

	req importing.PlanRequest

	mappingForm   components.Form
	mappingFields []components.Autocomplete

	plan  importing.ImportPlan
	table components.Table

	confirm components.ConfirmDialog

	result importing.ImportResult
}

var _ screens.Screen = (*Model)(nil)

// New builds the import screen over uc, formatting amounts and dates with loc.
func New(uc UseCases, loc domain.Locale, th theme.Theme) *Model {
	m := &Model{uc: uc, loc: loc, th: th, km: DefaultKeyMap()}
	m.table = components.NewTable(previewColumns(), th)
	m.confirm = components.NewConfirmDialog(th)
	m.resetSourceForm()
	return m
}

// Init loads the discovered connectors, to suggest them on the source form.
func (m *Model) Init() tea.Cmd { return m.loadConnectors() }

// SetSize adapts the preview table to the available terminal size.
func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	rows := max(3, h-10)
	m.table.SetHeight(rows)
	m.table.SetWidth(w)
}

// Title identifies the screen in the tab bar.
func (m *Model) Title() string { return "Import" }

// KeyMap exposes the bindings for the global help view.
func (m *Model) KeyMap() []key.Binding { return m.km.Bindings() }

// InputActive reports whether the source or mapping form is capturing keystrokes.
func (m *Model) InputActive() bool { return m.mode == modeSource || m.mode == modeMapping }

// connectorsLoadedMsg carries the discovered connectors back into Update.
type connectorsLoadedMsg struct{ connectors []importing.ConnectorInfo }

func (m *Model) loadConnectors() tea.Cmd {
	return func() tea.Msg {
		cs, err := m.uc.ListConnectors(context.Background())
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return connectorsLoadedMsg{connectors: cs}
	}
}

// Update advances the screen state; it performs no I/O itself.
func (m *Model) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case connectorsLoadedMsg:
		m.connectors = msg.connectors
		m.connectorField.SetSuggestions(connectorNames(msg.connectors))
		return m, nil
	case planLoadedMsg:
		return m.handlePlanLoaded(msg)
	case mappingAccountsMsg:
		m.openMappingForm(msg.accounts)
		return m, nil
	case resultMsg:
		m.result = msg.result
		m.mode = modeResult
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch m.mode {
	case modeSource:
		return m.handleSourceKey(msg)
	case modeMapping:
		return m.handleMappingKey(msg)
	case modePreview:
		return m.handlePreviewKey(msg)
	case modeConfirm:
		return m.handleConfirmKey(msg)
	case modeResult:
		return m.handleResultKey(msg)
	}
	return m, nil
}

func (m *Model) handleConfirmKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	yes, no := m.confirm.HandleKey(msg)
	switch {
	case yes:
		return m, m.submitApply()
	case no:
		m.mode = modePreview
	}
	return m, nil
}

func (m *Model) handleResultKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	if key.Matches(msg, m.km.New) || key.Matches(msg, m.km.Confirm) {
		m.resetSourceForm()
		m.mode = modeSource
	}
	return m, nil
}

func (m *Model) submitApply() tea.Cmd {
	plan := m.plan
	return func() tea.Msg {
		res, err := m.uc.ApplyImport(context.Background(), plan)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return resultMsg{result: res}
	}
}

// resultMsg carries what ApplyImport wrote back into Update.
type resultMsg struct{ result importing.ImportResult }

// View renders the active panel.
func (m *Model) View() string {
	switch m.mode {
	case modeSource:
		return m.viewSource()
	case modeMapping:
		return m.viewMapping()
	case modePreview:
		return m.viewPreview()
	case modeConfirm:
		return m.viewPreview() + "\n\n" + m.confirm.View()
	case modeResult:
		return m.viewResult()
	}
	return m.viewSource()
}
