package lists

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

// openForm opens the add form, or the rename form prefilled with name when non-empty.
func (m *Tags) openForm(name string) {
	m.err = nil
	m.editing = name
	m.nameField = components.NewAutocomplete("Name:", "e.g. vacation", m.th)
	if name != "" {
		m.nameField.SetValue(name)
	}
	m.form = components.NewForm(&m.nameField)
	m.mode = tagsForm
	m.form.Focus()
}

func (m *Tags) handleFormKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = tagsList
		return m, nil
	case "enter":
		if m.form.FocusedOpen() {
			m.form.Update(msg)
			return m, nil
		}
		return m, m.submitForm()
	}
	m.form.Update(msg)
	return m, nil
}

func (m *Tags) submitForm() tea.Cmd {
	if m.editing != "" {
		return m.submitRename(m.editing, m.nameField.Value())
	}
	return m.submitAdd(m.nameField.Value())
}

func (m *Tags) submitAdd(name string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := m.uc.PlanAddTag(ctx, app.AddTagRequest{Name: name})
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if _, err := m.uc.ApplyAddTag(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.reload(ctx)
	}
}

func (m *Tags) submitRename(current, newName string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := m.uc.PlanRenameTag(ctx, app.RenameTagRequest{Tag: current, NewName: newName})
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if _, err := m.uc.ApplyRenameTag(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.reload(ctx)
	}
}

func (m *Tags) reload(ctx context.Context) tea.Msg {
	tags, err := m.uc.ListTags(ctx)
	if err != nil {
		return screens.ErrorMsg{Err: err}
	}
	return tagsLoadedMsg{tags: tags}
}

func (m *Tags) viewForm() string {
	title := "Add tag"
	if m.editing != "" {
		title = "Rename tag"
	}
	return m.th.Title.Render(title) + "\n\n" + m.form.View()
}
