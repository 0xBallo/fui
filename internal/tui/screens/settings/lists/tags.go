package lists

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/theme"
)

// tagsMode tracks which panel of the tags sub-view is active.
type tagsMode int

const (
	tagsList tagsMode = iota
	tagsForm
	tagsConfirm
)

// Tags is the settings sub-view that lists tags and lets the user add, rename and remove them.
// Removing a tag in use detaches it from every transaction that carries it (confirmed first).
type Tags struct {
	uc TagsUseCases
	th theme.Theme
	km KeyMap

	width, height int
	mode          tagsMode

	tags  []domain.Tag
	table components.Table

	form      components.Form
	nameField components.Autocomplete
	editing   string // name of the tag being renamed, empty when adding

	confirm components.ConfirmDialog
	pending app.RemoveTagPlan

	err error // last submit error, shown inline until the next state change
}

var _ screens.Screen = (*Tags)(nil)

// NewTags builds the tags sub-view over uc.
func NewTags(uc TagsUseCases, th theme.Theme) *Tags {
	m := &Tags{uc: uc, th: th, km: DefaultKeyMap()}
	m.table = components.NewTable([]components.Column{
		{Title: "Tag", Width: 28, Align: lipgloss.Left},
	}, th)
	m.confirm = components.NewConfirmDialog(th)
	return m
}

// Init loads the tags.
func (m *Tags) Init() tea.Cmd { return m.load() }

// SetSize adapts the table to the available terminal size.
func (m *Tags) SetSize(w, h int) {
	m.width, m.height = w, h
	m.table.SetHeight(max(3, h-8))
	m.table.SetWidth(w)
}

// Title identifies the sub-view in the settings shell.
func (m *Tags) Title() string { return "Tags" }

// KeyMap exposes the bindings for the global help view.
func (m *Tags) KeyMap() []key.Binding { return m.km.Bindings() }

// InputActive reports whether the add/rename form is capturing keystrokes.
func (m *Tags) InputActive() bool { return m.mode == tagsForm }

type tagsLoadedMsg struct{ tags []domain.Tag }

func (m *Tags) load() tea.Cmd {
	return func() tea.Msg {
		tags, err := m.uc.ListTags(context.Background())
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return tagsLoadedMsg{tags: tags}
	}
}

// Update advances the sub-view state; it performs no I/O itself.
func (m *Tags) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tagsLoadedMsg:
		m.err = nil
		m.tags = msg.tags
		m.table.SetRows(m.rows())
		m.mode = tagsList
		return m, nil
	case tagRemovePlannedMsg:
		m.openRemoveConfirm(msg.plan)
		return m, nil
	case screens.ErrorMsg:
		m.err = msg.Err
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Tags) handleKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch m.mode {
	case tagsForm:
		return m.handleFormKey(msg)
	case tagsConfirm:
		return m.handleConfirmKey(msg)
	case tagsList:
		return m.handleListKey(msg)
	}
	return m.handleListKey(msg)
}

func (m *Tags) handleListKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch {
	case key.Matches(msg, m.km.Up):
		m.table.MoveUp(1)
	case key.Matches(msg, m.km.Down):
		m.table.MoveDown(1)
	case key.Matches(msg, m.km.Add):
		m.openForm("")
	case key.Matches(msg, m.km.Edit):
		if row := m.table.SelectedRow(); row != nil {
			m.openForm(row.ID)
		}
	case key.Matches(msg, m.km.Remove):
		if row := m.table.SelectedRow(); row != nil {
			return m, m.planRemove(row.ID)
		}
	}
	return m, nil
}

func (m *Tags) handleConfirmKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	yes, no := m.confirm.HandleKey(msg)
	switch {
	case yes:
		return m, m.submitRemove()
	case no:
		m.mode = tagsList
	}
	return m, nil
}

func (m *Tags) rows() []components.Row {
	rows := make([]components.Row, len(m.tags))
	for i, t := range m.tags {
		rows[i] = components.Row{ID: t.Name, Cells: []string{t.Name}}
	}
	return rows
}

// View renders the active panel.
func (m *Tags) View() string {
	var body string
	switch m.mode {
	case tagsForm:
		body = components.Modal(m.th, m.width, m.height, m.viewForm())
	case tagsConfirm:
		body = components.Modal(m.th, m.width, m.height, m.confirm.View())
	case tagsList:
		body = m.viewList()
	default:
		body = m.viewList()
	}
	return withError(m.th, body, m.err)
}

func (m *Tags) viewList() string {
	return m.th.Title.Render("Tags") + "\n\n" + m.table.View()
}

type tagRemovePlannedMsg struct{ plan app.RemoveTagPlan }

func (m *Tags) planRemove(name string) tea.Cmd {
	return func() tea.Msg {
		plan, err := m.uc.PlanRemoveTag(context.Background(), app.RemoveTagRequest{Tag: name})
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return tagRemovePlannedMsg{plan: plan}
	}
}

func (m *Tags) submitRemove() tea.Cmd {
	plan := m.pending
	return func() tea.Msg {
		ctx := context.Background()
		if err := m.uc.ApplyRemoveTag(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		tags, err := m.uc.ListTags(ctx)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return tagsLoadedMsg{tags: tags}
	}
}

func (m *Tags) openRemoveConfirm(plan app.RemoveTagPlan) {
	m.err = nil
	m.pending = plan
	prompt := fmt.Sprintf("Delete tag %q?", plan.Tag.Name)
	if plan.Detached > 0 {
		prompt = fmt.Sprintf("Delete tag %q, detaching it from %d transactions?", plan.Tag.Name, plan.Detached)
	}
	m.confirm.Show("Delete tag", prompt)
	m.mode = tagsConfirm
}
