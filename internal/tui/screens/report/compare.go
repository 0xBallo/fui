package report

import (
	"context"
	"strconv"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

// CompareUseCases is what the period comparison view needs; it matches
// internal/app/reporting.Analytics.ComparePeriods. It is separate from UseCases so the view stays
// optional: without it the "c" key is not offered.
type CompareUseCases interface {
	ComparePeriods(ctx context.Context, req reporting.CompareRequest) (reporting.CompareReport, error)
}

// compareKeys are the bindings of the compare view.
type compareKeys struct {
	Open, Params, ByCategory, Refresh, Back key.Binding
}

func defaultCompareKeys() compareKeys {
	return compareKeys{
		Open:       key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "compare periods")),
		Params:     key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "change periods")),
		ByCategory: key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "toggle by category")),
		Refresh:    key.NewBinding(key.WithKeys("r", "enter"), key.WithHelp("r/enter", "run")),
		Back:       key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

// compareState is the optional period comparison view of the report screen.
type compareState struct {
	uc         CompareUseCases
	km         compareKeys
	req        reporting.CompareRequest
	report     reporting.CompareReport
	loaded     bool
	table      components.Table
	periodA    components.Autocomplete
	periodB    components.Autocomplete
	form       components.Form
	formActive bool
}

// compareLoadedMsg carries a comparison back into Update.
type compareLoadedMsg struct{ report reporting.CompareReport }

// SetCompare enables the period comparison view over uc; nil disables it.
func (m *Model) SetCompare(uc CompareUseCases) {
	m.cmp = compareState{uc: uc, km: defaultCompareKeys(), req: reporting.CompareRequest{ByCategory: true}}
	m.cmp.table = components.NewTable(compareColumns(), m.th)
	m.cmp.table.SetHeight(max(3, m.height-10))
	m.cmp.table.SetWidth(m.width)
}

func compareColumns() []components.Column {
	return []components.Column{
		{Title: "Category", Width: 22, Align: lipgloss.Left},
		{Title: "Period A", Width: 14, Align: lipgloss.Right},
		{Title: "Period B", Width: 14, Align: lipgloss.Right},
		{Title: "Delta", Width: 14, Align: lipgloss.Right},
		{Title: "Change", Width: 9, Align: lipgloss.Right},
	}
}

func (m *Model) compareEnabled() bool { return m.cmp.uc != nil }

func (m *Model) compareBindings() []key.Binding {
	if !m.compareEnabled() {
		return nil
	}
	return []key.Binding{m.cmp.km.Open}
}

func (m *Model) openCompare() tea.Cmd {
	m.mode = modeCompare
	return m.runCompare()
}

func (m *Model) runCompare() tea.Cmd {
	req := m.cmp.req
	return func() tea.Msg {
		rep, err := m.cmp.uc.ComparePeriods(context.Background(), req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return compareLoadedMsg{report: rep}
	}
}

func (m *Model) handleCompareKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	if m.cmp.formActive {
		return m.handleCompareFormKey(msg)
	}
	switch {
	case key.Matches(msg, m.cmp.km.Back):
		m.mode = modeReport
	case key.Matches(msg, m.km.Up):
		m.cmp.table.MoveUp(1)
	case key.Matches(msg, m.km.Down):
		m.cmp.table.MoveDown(1)
	case key.Matches(msg, m.cmp.km.ByCategory):
		m.cmp.req.ByCategory = !m.cmp.req.ByCategory
		return m, m.runCompare()
	case key.Matches(msg, m.cmp.km.Refresh):
		return m, m.runCompare()
	case key.Matches(msg, m.cmp.km.Params):
		m.openCompareForm()
	}
	return m, nil
}

func (m *Model) openCompareForm() {
	c := &m.cmp
	c.periodA = components.NewAutocomplete("Period A:", "last-month", m.th)
	c.periodA.SetValue(c.req.A)
	c.periodA.SetSuggestions(relativePeriods())
	c.periodB = components.NewAutocomplete("Period B:", "this-month", m.th)
	c.periodB.SetValue(c.req.B)
	c.periodB.SetSuggestions(relativePeriods())
	c.form = components.NewForm(&c.periodA, &c.periodB)
	c.formActive = true
	c.form.Focus()
}

func (m *Model) handleCompareFormKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	c := &m.cmp
	switch msg.String() {
	case "esc":
		c.formActive = false
	case "shift+tab":
		c.form.Prev()
	case "enter":
		switch {
		case c.form.FocusedOpen():
			c.form.Update(msg)
		case !c.form.AtLast():
			c.form.Next()
		default:
			c.req.A, c.req.B = c.periodA.Value(), c.periodB.Value()
			c.formActive = false
			return m, m.runCompare()
		}
	default:
		c.form.Update(msg)
	}
	return m, nil
}

func (m *Model) applyCompare(rep reporting.CompareReport) {
	m.cmp.report, m.cmp.loaded = rep, true
	rows := make([]components.Row, 0, len(rep.Rows)+1)
	for _, r := range rep.Rows {
		rows = append(rows, m.compareRow(r.Category, r))
	}
	rows = append(rows, m.compareRow("TOTAL", rep.Total))
	m.cmp.table.SetRows(rows)
	m.cmp.table.SetColumns(compareColumns())
}

func (m *Model) compareRow(label string, r reporting.CompareRowView) components.Row {
	return components.Row{ID: label, Cells: []string{
		label, m.fmtAmount(r.A), m.fmtAmount(r.B), m.signedAmount(r.Delta), changeText(r.DeltaPermille),
	}}
}

// signedAmount always prints the sign so an increase and a decrease differ without colour.
func (m *Model) signedAmount(a reporting.Amount) string {
	s := m.fmtAmount(a)
	if a.Minor > 0 {
		return "+" + s
	}
	return s
}

// changeText renders a permille change as a signed percentage with one decimal; "n/a" when the
// base period has no spending.
func changeText(p *int64) string {
	if p == nil {
		return "n/a"
	}
	v := *p
	sign := ""
	switch {
	case v > 0:
		sign = "+"
	case v < 0:
		sign = "-"
		v = -v
	}
	return sign + strconv.FormatInt(v/10, 10) + "." + strconv.FormatInt(v%10, 10) + "%"
}

func (m *Model) viewCompare() string {
	if m.cmp.formActive {
		return components.Modal(m.th, m.width, m.height,
			m.th.Title.Render("Compare periods")+"\n\n"+m.cmp.form.View())
	}
	title := "Reports — Compare periods"
	if m.cmp.loaded {
		title += " — A " + m.cmp.report.A + " vs B " + m.cmp.report.B
	}
	lines := []string{
		m.th.Title.Render(title), "", m.cmp.table.View(), "",
		m.th.Base.Render("p periods   t by category: " + onOff(m.cmp.req.ByCategory) + "   r refresh   esc back"),
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}
