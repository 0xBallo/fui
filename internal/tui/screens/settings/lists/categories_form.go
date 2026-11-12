package lists

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

// openForm opens the add form, or the rename form prefilled with name when non-empty.
func (m *Categories) openForm(name string) {
	m.editing = name
	m.nameField = components.NewAutocomplete("Name:", "e.g. Groceries", m.th)
	if name != "" {
		m.nameField.SetValue(name)
	}
	fields := []*components.Autocomplete{&m.nameField}
	if name == "" {
		m.kindField = components.NewAutocomplete("Kind:", "income/expense", m.th)
		m.kindField.SetSuggestions([]string{string(domain.CategoryIncome), string(domain.CategoryExpense)})
		fields = append(fields, &m.kindField)
	}
	m.form = components.NewForm(fields...)
	m.mode = categoriesForm
	m.form.Focus()
}

func (m *Categories) handleFormKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = categoriesList
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

func (m *Categories) submitForm() tea.Cmd {
	if m.editing != "" {
		return m.submitRename(m.editing, m.nameField.Value())
	}
	return m.submitAdd(m.nameField.Value(), m.kindField.Value())
}

func (m *Categories) submitAdd(name, kind string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := m.uc.PlanAddCategory(ctx, app.AddCategoryRequest{Name: name, Kind: kind})
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if _, err := m.uc.ApplyAddCategory(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.reload(ctx)
	}
}

func (m *Categories) submitRename(current, newName string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := m.uc.PlanRenameCategory(ctx, app.RenameCategoryRequest{Category: current, NewName: newName})
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if _, err := m.uc.ApplyRenameCategory(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.reload(ctx)
	}
}

func (m *Categories) reload(ctx context.Context) tea.Msg {
	cats, err := m.uc.ListCategories(ctx)
	if err != nil {
		return screens.ErrorMsg{Err: err}
	}
	return categoriesLoadedMsg{cats: cats}
}

func (m *Categories) viewForm() string {
	title := "Add category"
	if m.editing != "" {
		title = "Rename category"
	}
	return m.th.Title.Render(title) + "\n\n" + m.form.View()
}

// openReassignForm opens the prompt for a replacement category, asked when removing one still
// used by transactions.
func (m *Categories) openReassignForm(plan app.RemoveCategoryPlan) {
	m.pending = plan
	m.removeTarget = plan.Category.Name
	m.reassignField = components.NewAutocomplete("Reassign to:", "replacement category", m.th)
	m.reassignField.SetSuggestions(otherCategoryNames(m.cats, plan.Category.Name))
	m.form = components.NewForm(&m.reassignField)
	m.mode = categoriesReassign
	m.form.Focus()
}

func (m *Categories) handleReassignKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = categoriesList
		return m, nil
	case "enter":
		if m.form.FocusedOpen() {
			m.form.Update(msg)
			return m, nil
		}
		return m, m.planReassignRemove()
	}
	m.form.Update(msg)
	return m, nil
}

func (m *Categories) planReassignRemove() tea.Cmd {
	name := m.removeTarget
	target := m.reassignField.Value()
	return func() tea.Msg {
		plan, err := m.uc.PlanRemoveCategory(context.Background(), app.RemoveCategoryRequest{Category: name, ReassignTo: target})
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return removePlannedMsg{plan: plan}
	}
}

func (m *Categories) viewReassign() string {
	return m.th.Title.Render("Reassign and delete "+m.removeTarget) + "\n\n" + m.form.View()
}

// openRemoveConfirm shows the final yes/no confirmation once a remove plan is ready (with or
// without a reassignment).
func (m *Categories) openRemoveConfirm(plan app.RemoveCategoryPlan) {
	m.pending = plan
	prompt := fmt.Sprintf("Delete category %q?", plan.Category.Name)
	if plan.Moved > 0 {
		prompt = fmt.Sprintf("Delete category %q, moving %d transactions?", plan.Category.Name, plan.Moved)
	}
	m.confirm.Show("Delete category", prompt)
	m.mode = categoriesConfirm
}
