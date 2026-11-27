package importer

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/settings/lists"
)

// csvRoles are the roles a CSV column can take; balance and other columns are not part of a
// profile and are simply ignored when it is saved.
var csvRoles = []string{
	importing.RoleDate, importing.RoleAmount, importing.RoleDebit, importing.RoleCredit,
	importing.RoleDescription, importing.RoleID, importing.RoleBalance, importing.RoleOther,
}

// csvSamplePreview is how many data rows of the file the mapper shows.
const csvSamplePreview = 5

// csvMapper is the state of the column mapper: the detection it started from, the profile name
// field and one role selector per column.
type csvMapper struct {
	sug        importing.CSVSuggestion
	nameField  components.Autocomplete
	roleFields []components.Autocomplete
	form       components.Form
}

// csvDetectedMsg carries a CSV detection back into Update.
type csvDetectedMsg struct{ sug importing.CSVSuggestion }

// detectCSV asks the use case for the profile it would suggest for the source form's file.
func (m *Model) detectCSV() (screens.Screen, tea.Cmd) {
	file := strings.TrimSpace(m.fileField.Value())
	switch {
	case m.profiles == nil:
		m.notice = "Saving CSV profiles is not available here."
		return m, nil
	case file == "":
		m.notice = "File is required to map its columns."
		return m, nil
	}
	m.notice = ""
	return m, func() tea.Msg {
		sug, err := m.uc.DetectCSV(context.Background(), file)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return csvDetectedMsg{sug: sug}
	}
}

// openCSVMapper builds the form from the detection: each column starts on its guessed role.
func (m *Model) openCSVMapper(sug importing.CSVSuggestion) {
	// The form holds pointers to the fields, so everything is built in place on m.csv.
	m.csv = csvMapper{sug: sug}
	c := &m.csv
	c.nameField = components.NewAutocomplete("Profile name:", "mybank", m.th)
	c.roleFields = make([]components.Autocomplete, len(sug.Columns))
	fields := []*components.Autocomplete{&c.nameField}
	for i, col := range sug.Columns {
		c.roleFields[i] = components.NewAutocomplete(col.Header+":", importing.RoleOther, m.th)
		c.roleFields[i].SetSuggestions(csvRoles)
		c.roleFields[i].SetValue(col.Role)
		fields = append(fields, &c.roleFields[i])
	}
	c.form = components.NewForm(fields...)
	m.notice = ""
	if !sug.HasHeader {
		m.notice = "No header row detected: columns are named by position."
	}
	m.mode = modeCSVMap
	m.csv.form.Focus()
}

func (m *Model) handleCSVMapKey(msg tea.KeyPressMsg) (screens.Screen, tea.Cmd) {
	f := &m.csv.form
	switch msg.String() {
	case "esc":
		m.notice = ""
		m.mode = modeSource
		return m, nil
	case "shift+tab":
		f.Prev()
		return m, nil
	case "enter":
		if f.FocusedOpen() && !m.roleFocusedExact() {
			f.Update(msg)
			return m, nil
		}
		if !f.AtLast() {
			f.Next()
			return m, nil
		}
		m.saveCSVProfile()
		return m, nil
	}
	f.Update(msg)
	return m, nil
}

// roleFocusedExact reports whether the focused field already holds a full role name: enter then
// moves on instead of spending a keypress on accepting the one suggestion left.
func (m *Model) roleFocusedExact() bool {
	for i := range m.csv.roleFields {
		if m.csv.roleFields[i].Focused() {
			return slices.Contains(csvRoles, strings.ToLower(strings.TrimSpace(m.csv.roleFields[i].Value())))
		}
	}
	return false
}

// saveCSVProfile turns the role choices into a profile on top of the detected format (delimiter,
// date format, decimal separator), saves it and selects it on the source form.
func (m *Model) saveCSVProfile() {
	name := strings.TrimSpace(m.csv.nameField.Value())
	if name == "" {
		m.notice = "Profile name is required."
		return
	}
	base := m.csv.sug.Profile
	p := lists.Profile{
		Name: name, Delimiter: base.Delimiter, Encoding: base.Encoding, DateFormat: base.DateFormat,
		DecimalSeparator: base.DecimalSeparator, SkipRows: strconv.Itoa(base.SkipRows),
		TargetAccount: strings.TrimSpace(m.accountField.Value()),
	}
	targets := map[string]*string{
		importing.RoleDate: &p.DateColumn, importing.RoleAmount: &p.AmountColumn,
		importing.RoleDebit: &p.DebitColumn, importing.RoleCredit: &p.CreditColumn,
		importing.RoleDescription: &p.DescriptionColumn, importing.RoleID: &p.ExternalIDColumn,
	}
	for i, col := range m.csv.sug.Columns {
		role := strings.ToLower(strings.TrimSpace(m.csv.roleFields[i].Value()))
		if role == "" {
			role = importing.RoleOther
		}
		if role == importing.RoleBalance || role == importing.RoleOther {
			continue
		}
		dst, ok := targets[role]
		if !ok {
			m.notice = fmt.Sprintf("Unknown role %q for column %s.", role, col.Header)
			return
		}
		if *dst != "" {
			m.notice = fmt.Sprintf("Role %s is assigned to both %s and %s.", role, *dst, col.Header)
			return
		}
		*dst = col.Header
	}
	if p.DateColumn == "" || p.DescriptionColumn == "" || (p.AmountColumn == "" && (p.DebitColumn == "" || p.CreditColumn == "")) {
		m.notice = "Map a date, a description and an amount (or debit and credit) column."
		return
	}
	if err := m.profiles.SaveProfile(p); err != nil {
		m.notice = "Save failed: " + err.Error()
		return
	}
	m.profileField.SetValue(name)
	if strings.TrimSpace(m.connectorField.Value()) == "" {
		m.connectorField.SetValue("csv")
	}
	m.notice = fmt.Sprintf("Profile %q saved and selected.", name)
	m.mode = modeSource
}

func (m *Model) viewCSVMap() string {
	out := m.th.Title.Render("Map CSV columns") + "\n"
	out += m.th.Muted.Render(m.sampleTable()) + "\n\n"
	out += m.csv.form.View()
	out += "\n\n" + m.th.Muted.Render("roles: "+strings.Join(csvRoles, " ")) + "\n"
	out += m.th.Muted.Render("enter next/save · shift+tab back · esc cancel")
	if m.notice != "" {
		out += "\n" + m.th.Warning.Render(m.notice)
	}
	return out
}

// sampleTable renders the header and the first sample rows, one tab-free line each.
func (m *Model) sampleTable() string {
	lines := make([]string, 0, csvSamplePreview+1)
	headers := make([]string, len(m.csv.sug.Columns))
	for i, c := range m.csv.sug.Columns {
		headers[i] = c.Header
	}
	lines = append(lines, strings.Join(headers, " | "))
	for _, row := range m.csv.sug.Sample[:min(csvSamplePreview, len(m.csv.sug.Sample))] {
		lines = append(lines, strings.Join(row, " | "))
	}
	return strings.Join(lines, "\n")
}
