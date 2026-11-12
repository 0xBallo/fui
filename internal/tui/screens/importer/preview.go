package importer

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

// previewColumns describes the row preview table: the dedup class is shown as a word-plus-symbol
// label (classLabel), never by color alone, and inclusion as a checkbox-like mark.
func previewColumns() []components.Column {
	return []components.Column{
		{Title: "Line", Width: 5, Align: lipgloss.Right},
		{Title: "Date", Width: 10, Align: lipgloss.Left},
		{Title: "Description", Width: 30, Align: lipgloss.Left},
		{Title: "Amount", Width: 14, Align: lipgloss.Right},
		{Title: "Class", Width: 10, Align: lipgloss.Left},
		{Title: "Inc", Width: 4, Align: lipgloss.Left},
	}
}

// openPreview switches to the row preview once the plan has no unmapped connector ref left.
func (m *Model) openPreview() {
	m.table.SetRows(m.previewRows())
	m.table.SetCursor(0)
	m.mode = modePreview
}

func (m *Model) previewRows() []components.Row {
	rows := make([]components.Row, len(m.plan.Rows))
	for i, row := range m.plan.Rows {
		rows[i] = components.Row{ID: strconv.Itoa(i), Cells: []string{
			strconv.Itoa(row.Line), row.Tx.Date.Format(m.loc), row.Tx.Description,
			domain.FormatAmount(row.Tx.Amount, m.loc), classLabel(row.Class), incMark(row.Include),
		}}
	}
	return rows
}

// classLabel renders the dedup class as a symbol plus word, so it reads the same without color.
func classLabel(c domain.DedupClass) string {
	switch c {
	case domain.DedupNew:
		return "+ new"
	case domain.DedupDuplicate:
		return "= dup"
	case domain.DedupDoubtful:
		return "? dbt"
	default:
		return string(c)
	}
}

func incMark(include bool) string {
	if include {
		return "[x]"
	}
	return "[ ]"
}

func (m *Model) handlePreviewKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch {
	case key.Matches(msg, m.km.Up):
		m.table.MoveUp(1)
	case key.Matches(msg, m.km.Down):
		m.table.MoveDown(1)
	case key.Matches(msg, m.km.Toggle):
		m.toggleInclude()
	case key.Matches(msg, m.km.Confirm):
		return m.openConfirm()
	case key.Matches(msg, m.km.Back):
		m.resetSourceForm()
	}
	return m, nil
}

// toggleInclude flips Include on the row under the cursor and refreshes its displayed mark.
func (m *Model) toggleInclude() {
	i := m.table.Cursor()
	if i < 0 || i >= len(m.plan.Rows) {
		return
	}
	if err := m.plan.SetInclude(i, !m.plan.Rows[i].Include); err != nil {
		return
	}
	m.table.SetRows(m.previewRows())
}

// openConfirm asks for a final yes/no before applying, listing the plan's changes.
func (m *Model) openConfirm() (screens.Screen, tea.Cmd) {
	n := m.plan.Counts()
	if n.Included == 0 {
		m.notice = "No rows included: nothing to import."
		return m, nil
	}
	m.notice = ""
	m.confirm.Show("Confirm import", fmt.Sprintf("Apply %d transaction(s)?", n.Included), changeSummaries(m.plan.Changes)...)
	m.mode = modeConfirm
	return m, nil
}

// changeSummaries lists up to 6 change summaries, with a trailing count of the rest.
func changeSummaries(changes []app.Change) []string {
	limit := min(6, len(changes))
	out := make([]string, limit, limit+1)
	for i := 0; i < limit; i++ {
		out[i] = changes[i].Summary
	}
	if len(changes) > limit {
		out = append(out, fmt.Sprintf("...and %d more change(s)", len(changes)-limit))
	}
	return out
}

func (m *Model) viewPreview() string {
	n := m.plan.Counts()
	out := m.th.Title.Render("Import preview") + "\n"
	out += m.th.Base.Render(fmt.Sprintf("connector=%s source=%s", m.plan.Connector, m.plan.Source))
	out += "\n" + m.th.Base.Render(fmt.Sprintf(
		"new=%d duplicate=%d doubtful=%d errors=%d included=%d", n.New, n.Duplicate, n.Doubtful, n.Errors, n.Included))
	if m.plan.AlreadyImported != nil {
		out += "\n" + m.th.Warning.Render(fmt.Sprintf("already imported as batch #%d", m.plan.AlreadyImported.ID))
	}
	if len(m.plan.Discrepancies) > 0 {
		out += "\n" + m.th.Warning.Render(fmt.Sprintf("%d balance discrepancy(ies)", len(m.plan.Discrepancies)))
	}
	out += "\n\n" + m.table.View()
	if detail := m.rowDetail(); detail != "" {
		out += "\n" + detail
	}
	out += "\n\n" + m.th.Muted.Render("space/i include/exclude · enter confirm · esc new import")
	if m.notice != "" {
		out += "\n" + m.th.Warning.Render(m.notice)
	}
	return out
}

// rowDetail describes the row under the cursor: its category, payee, tags, transfer label and
// which categorization rule (if any) set them (see RuleEffect in internal/app/importing).
func (m *Model) rowDetail() string {
	i := m.table.Cursor()
	if i < 0 || i >= len(m.plan.Rows) {
		return ""
	}
	row := m.plan.Rows[i]
	parts := []string{"line " + strconv.Itoa(row.Line)}
	if row.Category != "" {
		parts = append(parts, "category="+row.Category)
	}
	if row.Payee != "" {
		parts = append(parts, "payee="+row.Payee)
	}
	if len(row.Tags) > 0 {
		parts = append(parts, "tags="+strings.Join(row.Tags, ","))
	}
	if row.Transfer != "" {
		parts = append(parts, "transfer="+row.Transfer)
	}
	if rule := ruleSummary(row.Rules); rule != "" {
		parts = append(parts, rule)
	}
	return m.th.Muted.Render(strings.Join(parts, "  "))
}

func ruleSummary(e importing.RuleEffect) string {
	var parts []string
	if e.CategoryRule != "" {
		parts = append(parts, "category rule="+e.CategoryRule)
	}
	if e.PayeeRule != "" {
		parts = append(parts, "payee rule="+e.PayeeRule)
	}
	if e.TagsRule != "" {
		parts = append(parts, "tags rule="+e.TagsRule)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

func (m *Model) viewResult() string {
	r := m.result
	out := m.th.Title.Render("Import complete") + "\n\n"
	out += m.th.Base.Render(fmt.Sprintf(
		"transactions=%d accounts=%d categories=%d payees=%d tags=%d transfers=%d",
		r.Transactions, r.Accounts, r.Categories, r.Payees, r.Tags, r.Transfers))
	if r.RecurrencesConfirmed > 0 {
		out += "\n" + m.th.Base.Render(fmt.Sprintf("recurrences confirmed=%d", r.RecurrencesConfirmed))
	}
	if r.BackupPath != "" {
		out += "\n" + m.th.Muted.Render("backup: "+r.BackupPath)
	}
	out += "\n\n" + m.th.Muted.Render("n/enter new import")
	return out
}
