package report

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

// openParamsForm opens the period/account/loan/limit/horizon/min-balance fields the active
// predefined report needs.
func (m *Model) openParamsForm() {
	k := predefinedKinds()[m.kindIdx]
	var fields []*components.Autocomplete
	if k.NeedsPeriod {
		m.fields.period = components.NewAutocomplete("Period:", "this-month", m.th)
		m.fields.period.SetValue(m.periodText)
		m.fields.period.SetSuggestions(relativePeriods())
		fields = append(fields, &m.fields.period)
	}
	if k.NeedsAccount {
		m.fields.account = components.NewAutocomplete("Account:", "e.g. Checking", m.th)
		m.fields.account.SetValue(m.accountText)
		m.fields.account.SetSuggestions(m.accountNames())
		fields = append(fields, &m.fields.account)
	}
	if k.NeedsLoan {
		m.fields.loan = components.NewAutocomplete("Loan:", "e.g. Mortgage", m.th)
		m.fields.loan.SetValue(m.loanText)
		fields = append(fields, &m.fields.loan)
	}
	if k.NeedsLimit {
		m.fields.limit = components.NewAutocomplete("Limit:", "default 10", m.th)
		m.fields.limit.SetValue(m.limitText)
		fields = append(fields, &m.fields.limit)
	}
	if k.IsForecast {
		m.fields.horizon = components.NewAutocomplete("Horizon (days):", "default 90", m.th)
		m.fields.horizon.SetValue(m.horizonText)
		m.fields.minBalance = components.NewAutocomplete("Min balance:", "optional", m.th)
		m.fields.minBalance.SetValue(m.minBalText)
		fields = append(fields, &m.fields.horizon, &m.fields.minBalance)
	}
	m.form = components.NewForm(fields...)
	m.mode = modeParamsForm
	m.form.Focus()
}

// relativePeriods are the period field's suggestions: reporting.ValidRelativePeriod accepts
// these plus any "last-N-months".
func relativePeriods() []string {
	return []string{"this-month", "last-month", "this-year", "last-year", "last-3-months", "last-6-months", "last-12-months"}
}

// openSaveForm opens the name field to save the currently displayed report.
func (m *Model) openSaveForm() {
	if m.result.kind == "" {
		return
	}
	m.fields.name = components.NewAutocomplete("Name:", "e.g. monthly-spending", m.th)
	m.form = components.NewForm(&m.fields.name)
	m.mode = modeSaveForm
	m.form.Focus()
}

// openExportForm opens the path and format fields to export the currently displayed report.
func (m *Model) openExportForm() {
	m.fields.path = components.NewAutocomplete("Output path:", "e.g. export.csv", m.th)
	m.fields.format = components.NewAutocomplete("Format:", "csv", m.th)
	m.fields.format.SetSuggestions([]string{"csv", "json"})
	m.form = components.NewForm(&m.fields.path, &m.fields.format)
	m.mode = modeExportForm
	m.form.Focus()
}

func (m *Model) handleFormKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeReport
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

func (m *Model) submitForm() tea.Cmd {
	switch m.mode {
	case modeParamsForm:
		return m.submitParams()
	case modeSaveForm:
		return m.submitSave()
	case modeExportForm:
		return m.submitExport()
	case modeReport, modeSaved, modeConfirmRemove, modeCompare, modeSim:
		return nil
	}
	return nil
}

func (m *Model) submitParams() tea.Cmd {
	m.periodText = m.fields.period.Value()
	m.accountText = m.fields.account.Value()
	m.loanText = m.fields.loan.Value()
	m.limitText = m.fields.limit.Value()
	m.horizonText = m.fields.horizon.Value()
	m.minBalText = m.fields.minBalance.Value()
	return m.runCurrent()
}

// submitExport writes the displayed report result to the entered path as csv or json through
// the ExportResult port.
func (m *Model) submitExport() tea.Cmd {
	m.mode = modeReport
	path := m.fields.path.Value()
	format := m.fields.format.Value()
	if format == "" {
		format = "csv"
	}
	if path == "" {
		return errCmd(&domain.Error{Code: domain.CodeMissingField, Message: "output path is required", Details: map[string]any{"field": "path"}})
	}
	if format != "csv" && format != "json" {
		return errCmd(&domain.Error{
			Code: domain.CodeInvalidValue, Message: "unknown export format " + format + " (use csv or json)",
			Details: map[string]any{"field": "format", "value": format},
		})
	}
	data := m.result.data
	if data == nil {
		return errCmd(&domain.Error{Code: domain.CodeInvalidValue, Message: "no report to export"})
	}
	return func() tea.Msg {
		f, err := os.Create(path) // #nosec G304 -- path is provided by the user through the TUI form.
		if err != nil {
			return screens.ErrorMsg{Err: fmt.Errorf("create output file: %w", err)}
		}
		if err := m.uc.ExportResult(f, format, data); err != nil {
			_ = f.Close()
			return screens.ErrorMsg{Err: err}
		}
		if err := f.Close(); err != nil {
			return screens.ErrorMsg{Err: fmt.Errorf("close output file: %w", err)}
		}
		return exportDoneMsg{path: path}
	}
}
