package lists

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

// openForm opens the add form, or the edit form prefilled from cur when non-nil.
func (m *Profiles) openForm(cur *Profile) {
	if cur != nil {
		m.editing = cur.Name
	} else {
		m.editing = ""
	}
	m.buildFormFields(cur)
	m.form = components.NewForm(
		&m.fields.name, &m.fields.dateColumn, &m.fields.amountColumn, &m.fields.debitColumn,
		&m.fields.creditColumn, &m.fields.descriptionColumn, &m.fields.externalIDColumn,
		&m.fields.delimiter, &m.fields.encoding, &m.fields.dateFormat, &m.fields.decimalSeparator,
		&m.fields.skipRows, &m.fields.targetAccount,
	)
	m.mode = profilesForm
	m.form.Focus()
}

func (m *Profiles) buildFormFields(cur *Profile) {
	f := &m.fields
	f.name = components.NewAutocomplete("Name:", "e.g. revolut", m.th)
	f.dateColumn = components.NewAutocomplete("Date column:", "e.g. Date", m.th)
	f.amountColumn = components.NewAutocomplete("Amount column:", "optional, when signed", m.th)
	f.debitColumn = components.NewAutocomplete("Debit column:", "optional, when split", m.th)
	f.creditColumn = components.NewAutocomplete("Credit column:", "optional, when split", m.th)
	f.descriptionColumn = components.NewAutocomplete("Description column:", "e.g. Description", m.th)
	f.externalIDColumn = components.NewAutocomplete("External ID column:", "optional", m.th)
	f.delimiter = components.NewAutocomplete("Delimiter:", ",", m.th)
	f.encoding = components.NewAutocomplete("Encoding:", "utf-8", m.th)
	f.dateFormat = components.NewAutocomplete("Date format:", "e.g. 2006-01-02", m.th)
	f.decimalSeparator = components.NewAutocomplete("Decimal separator:", ".", m.th)
	f.skipRows = components.NewAutocomplete("Skip rows:", "0", m.th)
	f.targetAccount = components.NewAutocomplete("Target account:", "e.g. Checking", m.th)
	if cur != nil {
		m.prefillFormFields(cur)
	}
}

func (m *Profiles) prefillFormFields(cur *Profile) {
	f := &m.fields
	f.name.SetValue(cur.Name)
	f.name.Blur() // name cannot change on edit
	f.dateColumn.SetValue(cur.DateColumn)
	f.amountColumn.SetValue(cur.AmountColumn)
	f.debitColumn.SetValue(cur.DebitColumn)
	f.creditColumn.SetValue(cur.CreditColumn)
	f.descriptionColumn.SetValue(cur.DescriptionColumn)
	f.externalIDColumn.SetValue(cur.ExternalIDColumn)
	f.delimiter.SetValue(cur.Delimiter)
	f.encoding.SetValue(cur.Encoding)
	f.dateFormat.SetValue(cur.DateFormat)
	f.decimalSeparator.SetValue(cur.DecimalSeparator)
	f.skipRows.SetValue(cur.SkipRows)
	f.targetAccount.SetValue(cur.TargetAccount)
}

func (m *Profiles) handleFormKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = profilesListMode
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

func (m *Profiles) submitForm() tea.Cmd {
	f := &m.fields
	name := f.name.Value()
	if m.editing != "" {
		name = m.editing
	}
	p := Profile{
		Name: name, DateColumn: f.dateColumn.Value(), AmountColumn: f.amountColumn.Value(),
		DebitColumn: f.debitColumn.Value(), CreditColumn: f.creditColumn.Value(),
		DescriptionColumn: f.descriptionColumn.Value(), ExternalIDColumn: f.externalIDColumn.Value(),
		Delimiter: f.delimiter.Value(), Encoding: f.encoding.Value(), DateFormat: f.dateFormat.Value(),
		DecimalSeparator: f.decimalSeparator.Value(), SkipRows: f.skipRows.Value(),
		TargetAccount: f.targetAccount.Value(),
	}
	return func() tea.Msg {
		if err := m.cfg.SaveProfile(p); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.reload()
	}
}

func (m *Profiles) viewForm() string {
	title := "Add import profile"
	if m.editing != "" {
		title = "Edit import profile"
	}
	return m.th.Title.Render(title) + "\n\n" + m.form.View()
}
