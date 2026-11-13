package report

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/tui/components"
)

// View renders the active panel.
func (m *Model) View() string {
	switch m.mode {
	case modeParamsForm, modeSaveForm, modeExportForm:
		return components.Modal(m.th, m.width, m.height, m.viewForm())
	case modeConfirmRemove:
		return components.Modal(m.th, m.width, m.height, m.confirm.View())
	case modeSaved:
		return m.viewSaved()
	case modeReport:
		return m.viewReport()
	}
	return m.viewReport()
}

func (m *Model) viewReport() string {
	title := m.th.Title.Render(m.reportTitle())
	lines := []string{title, "", m.table.View()}
	if chart := m.chart.View(); chart != "" {
		lines = append(lines, "", chart)
	}
	if m.note != "" {
		lines = append(lines, "", m.th.Muted.Render(m.note))
	}
	lines = append(lines, "", m.viewFooter())
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *Model) reportTitle() string {
	label := m.result.label
	if label == "" {
		label = predefinedKinds()[m.kindIdx].Label
	}
	title := "Reports — " + label
	if m.result.saved != "" {
		title += " (saved: " + m.result.saved + ")"
	}
	if m.periodText != "" {
		title += " — " + m.periodText
	}
	return title
}

func (m *Model) viewFooter() string {
	return m.th.Base.Render("h/l report   p params   s save   v saved   e export   r refresh")
}

func (m *Model) viewSaved() string {
	title := m.th.Title.Render("Saved reports")
	lines := []string{title, "", m.savedTable.View(), "", m.th.Base.Render("enter run   d delete   esc back")}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *Model) viewForm() string {
	return m.th.Title.Render(m.formTitle()) + "\n\n" + m.form.View()
}

func (m *Model) formTitle() string {
	switch m.mode {
	case modeSaveForm:
		return "Save report"
	case modeExportForm:
		return "Export transactions"
	case modeParamsForm, modeReport, modeSaved, modeConfirmRemove:
		return "Report params"
	}
	return "Report params"
}
