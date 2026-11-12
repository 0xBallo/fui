package lists

import (
	"context"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/recurring"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

// openForm opens the add form, or the edit form prefilled from cur when non-nil.
func (m *Recurrences) openForm(cur *app.Recurrence) {
	if cur != nil {
		id := cur.ID
		m.editing = &id
	} else {
		m.editing = nil
	}
	m.buildFormFields(cur)
	m.form = components.NewForm(
		&m.fields.account, &m.fields.category, &m.fields.description, &m.fields.amount,
		&m.fields.every, &m.fields.unit, &m.fields.dayOfMonth, &m.fields.start, &m.fields.end,
	)
	m.mode = recurrencesForm
	m.form.Focus()
}

func (m *Recurrences) buildFormFields(cur *app.Recurrence) {
	f := &m.fields
	f.account = components.NewAutocomplete("Account:", "e.g. Checking", m.th)
	f.account.SetSuggestions(accountNames(m.accounts))
	f.category = components.NewAutocomplete("Category:", "optional", m.th)
	f.category.SetSuggestions(domainCategoryNames(m.categories))
	f.description = components.NewAutocomplete("Description:", "e.g. Rent", m.th)
	f.amount = components.NewAutocomplete("Amount:", "e.g. -850.00", m.th)
	f.every = components.NewAutocomplete("Every:", "1", m.th)
	f.unit = components.NewAutocomplete("Unit:", "days/weeks/months", m.th)
	f.unit.SetSuggestions([]string{"days", "weeks", "months"})
	f.dayOfMonth = components.NewAutocomplete("Day of month:", "optional, for month/year", m.th)
	f.start = components.NewAutocomplete("Start date:", "YYYY-MM-DD", m.th)
	f.end = components.NewAutocomplete("End date:", "optional", m.th)
	if cur != nil {
		m.prefillFormFields(cur)
	}
}

// prefillFormFields copies cur's values into every field for display and editing convenience;
// account and start are shown but, per recurring.EditRequest, cannot actually change on submit.
func (m *Recurrences) prefillFormFields(cur *app.Recurrence) {
	f := &m.fields
	f.account.SetValue(accountNameByID(cur.AccountID, m.accounts))
	f.category.SetValue(categoryNameByID(cur.CategoryID, m.categories))
	f.description.SetValue(cur.Description)
	f.amount.SetValue(domain.FormatAmount(cur.Amount, m.loc))
	f.every.SetValue(strconv.Itoa(cur.Rule.Every))
	f.unit.SetValue(string(cur.Rule.Unit))
	f.dayOfMonth.SetValue(strconv.Itoa(cur.Rule.DayOfMonth))
	f.start.SetValue(cur.Start.String())
	if !cur.End.IsZero() {
		f.end.SetValue(cur.End.String())
	}
}

func (m *Recurrences) handleFormKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = recurrencesListMode
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

func (m *Recurrences) submitForm() tea.Cmd {
	if m.editing != nil {
		return m.submitEdit()
	}
	return m.submitAdd()
}

func (m *Recurrences) submitAdd() tea.Cmd {
	f := &m.fields
	every, _ := strconv.Atoi(f.every.Value())
	dayOfMonth, _ := strconv.Atoi(f.dayOfMonth.Value())
	req := recurring.AddRequest{
		Account: f.account.Value(), Category: f.category.Value(), Description: f.description.Value(),
		Amount: f.amount.Value(), Every: every, Unit: f.unit.Value(), DayOfMonth: dayOfMonth,
		Start: f.start.Value(), End: f.end.Value(),
	}
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := m.uc.PlanAdd(ctx, m.enabled, req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if _, err := m.uc.ApplyAdd(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.reload(ctx)
	}
}

func (m *Recurrences) submitEdit() tea.Cmd {
	f := &m.fields
	id := *m.editing
	category, description, amount, unit, end := f.category.Value(), f.description.Value(), f.amount.Value(), f.unit.Value(), f.end.Value()
	every, _ := strconv.Atoi(f.every.Value())
	dayOfMonth, _ := strconv.Atoi(f.dayOfMonth.Value())
	req := recurring.EditRequest{
		ID: id, Category: &category, Description: &description, Amount: &amount,
		Every: &every, Unit: &unit, DayOfMonth: &dayOfMonth, End: &end,
	}
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := m.uc.PlanEdit(ctx, m.enabled, req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if err := m.uc.ApplyEdit(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.reload(ctx)
	}
}

func (m *Recurrences) viewForm() string {
	title := "Add recurrence"
	if m.editing != nil {
		title = "Edit recurrence"
	}
	return m.th.Title.Render(title) + "\n\n" + m.form.View()
}
