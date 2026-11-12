package lists

import (
	"context"
	"errors"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/theme"
)

// categoriesMode tracks which panel of the categories sub-view is active.
type categoriesMode int

const (
	categoriesList categoriesMode = iota
	categoriesForm
	categoriesReassign
	categoriesConfirm
)

// Categories is the settings sub-view that lists categories and lets the user add, rename and
// remove them. Removing a category in use requires naming a replacement first.
type Categories struct {
	uc CategoriesUseCases
	th theme.Theme
	km KeyMap

	width, height int
	mode          categoriesMode

	cats  []domain.Category
	table components.Table

	form      components.Form
	nameField components.Autocomplete
	kindField components.Autocomplete
	editing   string // name of the category being renamed, empty when adding

	reassignField components.Autocomplete
	removeTarget  string

	confirm components.ConfirmDialog
	pending app.RemoveCategoryPlan

	err error // last submit error, shown inline until the next state change
}

var _ screens.Screen = (*Categories)(nil)

// NewCategories builds the categories sub-view over uc.
func NewCategories(uc CategoriesUseCases, th theme.Theme) *Categories {
	m := &Categories{uc: uc, th: th, km: DefaultKeyMap()}
	m.table = components.NewTable([]components.Column{
		{Title: "Category", Width: 28, Align: lipgloss.Left},
		{Title: "Kind", Width: 10, Align: lipgloss.Left},
	}, th)
	m.confirm = components.NewConfirmDialog(th)
	return m
}

// Init loads the categories.
func (m *Categories) Init() tea.Cmd { return m.load() }

// SetSize adapts the table to the available terminal size.
func (m *Categories) SetSize(w, h int) {
	m.width, m.height = w, h
	m.table.SetHeight(max(3, h-8))
	m.table.SetWidth(w)
}

// Title identifies the sub-view in the settings shell.
func (m *Categories) Title() string { return "Categories" }

// KeyMap exposes the bindings for the global help view.
func (m *Categories) KeyMap() []key.Binding { return m.km.Bindings() }

// InputActive reports whether a form is capturing keystrokes.
func (m *Categories) InputActive() bool {
	return m.mode == categoriesForm || m.mode == categoriesReassign
}

type categoriesLoadedMsg struct{ cats []domain.Category }

func (m *Categories) load() tea.Cmd {
	return func() tea.Msg {
		cats, err := m.uc.ListCategories(context.Background())
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return categoriesLoadedMsg{cats: cats}
	}
}

// Update advances the sub-view state; it performs no I/O itself.
func (m *Categories) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case categoriesLoadedMsg:
		m.err = nil
		m.cats = msg.cats
		m.table.SetRows(m.rows())
		m.mode = categoriesList
		return m, nil
	case needReassignMsg:
		m.openReassignForm(msg.plan)
		return m, nil
	case removePlannedMsg:
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

func (m *Categories) handleKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch m.mode {
	case categoriesForm:
		return m.handleFormKey(msg)
	case categoriesReassign:
		return m.handleReassignKey(msg)
	case categoriesConfirm:
		return m.handleConfirmKey(msg)
	case categoriesList:
		return m.handleListKey(msg)
	}
	return m.handleListKey(msg)
}

func (m *Categories) handleListKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
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

func (m *Categories) handleConfirmKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	yes, no := m.confirm.HandleKey(msg)
	switch {
	case yes:
		return m, m.submitRemove()
	case no:
		m.mode = categoriesList
	}
	return m, nil
}

func (m *Categories) rows() []components.Row {
	rows := make([]components.Row, len(m.cats))
	for i, c := range m.cats {
		rows[i] = components.Row{ID: c.Name, Cells: []string{c.Name, string(c.Kind)}}
	}
	return rows
}

// View renders the active panel.
func (m *Categories) View() string {
	var body string
	switch m.mode {
	case categoriesForm:
		body = m.viewForm()
	case categoriesReassign:
		body = m.viewReassign()
	case categoriesConfirm:
		body = m.viewList() + "\n\n" + m.confirm.View()
	case categoriesList:
		body = m.viewList()
	default:
		body = m.viewList()
	}
	return withError(m.th, body, m.err)
}

func (m *Categories) viewList() string {
	return m.th.Title.Render("Categories") + "\n\n" + m.table.View()
}

// needReassignMsg carries a removal blocked by in-use transactions back into Update, so the
// screen can prompt for a replacement category before confirming.
type needReassignMsg struct{ plan app.RemoveCategoryPlan }

func (m *Categories) planRemove(name string) tea.Cmd {
	return func() tea.Msg {
		plan, err := m.uc.PlanRemoveCategory(context.Background(), app.RemoveCategoryRequest{Category: name})
		if err != nil {
			if errors.Is(err, app.ErrInUse) {
				return needReassignMsg{plan: plan}
			}
			return screens.ErrorMsg{Err: err}
		}
		return removePlannedMsg{plan: plan}
	}
}

type removePlannedMsg struct{ plan app.RemoveCategoryPlan }

func (m *Categories) submitRemove() tea.Cmd {
	plan := m.pending
	return func() tea.Msg {
		ctx := context.Background()
		if _, err := m.uc.ApplyRemoveCategory(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		cats, err := m.uc.ListCategories(ctx)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return categoriesLoadedMsg{cats: cats}
	}
}

// otherCategoryNames returns the names of every category but self, for the reassign-target
// suggestions.
func otherCategoryNames(cats []domain.Category, self string) []string {
	names := make([]string, 0, len(cats))
	for _, c := range cats {
		if !equalFold(c.Name, self) {
			names = append(names, c.Name)
		}
	}
	return names
}
