package lists

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/theme"
)

// profilesMode tracks which panel of the import-profiles sub-view is active.
type profilesMode int

const (
	profilesListMode profilesMode = iota
	profilesForm
	profilesConfirm
)

// Profiles is the settings sub-view that lists CSV import profiles and lets the user add, edit
// and remove them.
type Profiles struct {
	cfg ProfilesConfig
	th  theme.Theme
	km  KeyMap

	width, height int
	mode          profilesMode

	profiles []Profile
	table    components.Table

	form    components.Form
	fields  profileFormFields
	editing string // name of the profile being edited, empty when adding

	confirm components.ConfirmDialog
	pending string // name pending removal
}

// profileFormFields groups the add/edit form's autocomplete fields.
type profileFormFields struct {
	name, dateColumn, amountColumn, debitColumn, creditColumn components.Autocomplete
	descriptionColumn, externalIDColumn                       components.Autocomplete
	delimiter, encoding, dateFormat, decimalSeparator         components.Autocomplete
	skipRows, targetAccount                                   components.Autocomplete
}

var _ screens.Screen = (*Profiles)(nil)

// NewProfiles builds the import-profiles sub-view over cfg.
func NewProfiles(cfg ProfilesConfig, th theme.Theme) *Profiles {
	m := &Profiles{cfg: cfg, th: th, km: DefaultKeyMap()}
	m.table = components.NewTable([]components.Column{
		{Title: "Profile", Width: 20, Align: lipgloss.Left},
		{Title: "Target account", Width: 20, Align: lipgloss.Left},
	}, th)
	m.confirm = components.NewConfirmDialog(th)
	return m
}

// Init loads the saved profiles.
func (m *Profiles) Init() tea.Cmd { return m.load() }

// SetSize adapts the table to the available terminal size.
func (m *Profiles) SetSize(w, h int) {
	m.width, m.height = w, h
	m.table.SetHeight(max(3, h-8))
	m.table.SetWidth(w)
}

// Title identifies the sub-view in the settings shell.
func (m *Profiles) Title() string { return "Import profiles" }

// KeyMap exposes the bindings for the global help view.
func (m *Profiles) KeyMap() []key.Binding { return m.km.Bindings() }

// InputActive reports whether the add/edit form is capturing keystrokes.
func (m *Profiles) InputActive() bool { return m.mode == profilesForm }

type profilesLoadedMsg struct{ profiles []Profile }

func (m *Profiles) load() tea.Cmd {
	return func() tea.Msg {
		profiles, err := m.cfg.ListProfiles()
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return profilesLoadedMsg{profiles: profiles}
	}
}

// Update advances the sub-view state; it performs no I/O itself.
func (m *Profiles) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case profilesLoadedMsg:
		m.profiles = msg.profiles
		m.table.SetRows(m.rows())
		m.mode = profilesListMode
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Profiles) handleKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch m.mode {
	case profilesForm:
		return m.handleFormKey(msg)
	case profilesConfirm:
		return m.handleConfirmKey(msg)
	case profilesListMode:
		return m.handleListKey(msg)
	}
	return m.handleListKey(msg)
}

func (m *Profiles) handleListKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch {
	case key.Matches(msg, m.km.Up):
		m.table.MoveUp(1)
	case key.Matches(msg, m.km.Down):
		m.table.MoveDown(1)
	case key.Matches(msg, m.km.Add):
		m.openForm(nil)
	case key.Matches(msg, m.km.Edit):
		if row := m.table.SelectedRow(); row != nil {
			m.openForm(m.profileByName(row.ID))
		}
	case key.Matches(msg, m.km.Remove):
		if row := m.table.SelectedRow(); row != nil {
			m.pending = row.ID
			m.confirm.Show("Delete profile", fmt.Sprintf("Delete import profile %q?", row.ID))
			m.mode = profilesConfirm
		}
	}
	return m, nil
}

func (m *Profiles) handleConfirmKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	yes, no := m.confirm.HandleKey(msg)
	switch {
	case yes:
		return m, m.submitRemove()
	case no:
		m.mode = profilesListMode
	}
	return m, nil
}

func (m *Profiles) profileByName(name string) *Profile {
	for i := range m.profiles {
		if m.profiles[i].Name == name {
			return &m.profiles[i]
		}
	}
	return nil
}

func (m *Profiles) rows() []components.Row {
	rows := make([]components.Row, len(m.profiles))
	for i, p := range m.profiles {
		rows[i] = components.Row{ID: p.Name, Cells: []string{p.Name, p.TargetAccount}}
	}
	return rows
}

// View renders the active panel.
func (m *Profiles) View() string {
	switch m.mode {
	case profilesForm:
		return m.viewForm()
	case profilesConfirm:
		return m.viewList() + "\n\n" + m.confirm.View()
	case profilesListMode:
		return m.viewList()
	}
	return m.viewList()
}

func (m *Profiles) viewList() string {
	return m.th.Title.Render("Import profiles") + "\n\n" + m.table.View()
}

func (m *Profiles) submitRemove() tea.Cmd {
	name := m.pending
	return func() tea.Msg {
		if err := m.cfg.DeleteProfile(name); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.reload()
	}
}

func (m *Profiles) reload() tea.Msg {
	profiles, err := m.cfg.ListProfiles()
	if err != nil {
		return screens.ErrorMsg{Err: err}
	}
	return profilesLoadedMsg{profiles: profiles}
}
