package dashboard

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/budgeting"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/theme"
)

// AlertBudgetUseCases is the optional budget source of the alerts; it matches
// internal/app/budgeting.Service.Status. Without it budget alerts are skipped.
type AlertBudgetUseCases interface {
	Status(ctx context.Context, month string) (budgeting.Status, error)
}

// AlertRecurringUseCases is the optional recurrence source of the alerts. Without it upcoming
// recurrences are skipped.
type AlertRecurringUseCases interface {
	ListRecurrences(ctx context.Context) ([]app.Recurrence, error)
}

// SetAlertBudget enables budget alerts over uc; nil disables them.
func (m *Model) SetAlertBudget(uc AlertBudgetUseCases) { m.al.budget = uc }

// SetAlertRecurring enables upcoming-recurrence alerts over uc; nil disables them.
func (m *Model) SetAlertRecurring(uc AlertRecurringUseCases) { m.al.rec = uc }

// alertLevel orders alerts from the most to the least urgent.
type alertLevel int

const (
	levelDanger alertLevel = iota
	levelWarning
	levelInfo
)

// alertsMaxRows bounds the list so the panel stays small; the cursor scrolls the window.
const alertsMaxRows = 6

type alert struct {
	level   alertLevel
	text    string
	insight *reporting.InsightView // set for entries that can be dismissed/accepted
}

type alertKeys struct {
	Focus, Up, Down, Dismiss, Accept, Leave key.Binding
}

func defaultAlertKeys() alertKeys {
	return alertKeys{
		Focus:   key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "alerts")),
		Up:      key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
		Down:    key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
		Dismiss: key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "dismiss insight")),
		Accept:  key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "accept subscription")),
		Leave:   key.NewBinding(key.WithKeys("esc", "i"), key.WithHelp("esc", "leave alerts")),
	}
}

// alertsSection lists budget, recurrence and insight alerts, most urgent first. Quick add owns
// "a" on the dashboard, so the list is entered with "i" and "a" only accepts while it is focused.
type alertsSection struct {
	loc domain.Locale
	th  theme.Theme
	km  alertKeys

	budget AlertBudgetUseCases
	rec    AlertRecurringUseCases
	ins    InsightUseCases

	asOf     domain.Date
	budgets  []alert
	upcoming []alert
	insights []reporting.InsightView

	entries []alert
	cursor  int
	focused bool
}

func newAlertsSection(loc domain.Locale, th theme.Theme) *alertsSection {
	return &alertsSection{loc: loc, th: th, km: defaultAlertKeys()}
}

type budgetAlertsMsg struct {
	asOf   domain.Date
	alerts []alert
}

type recurAlertsMsg struct {
	asOf   domain.Date
	alerts []alert
}

func (s *alertsSection) load(p sectionParams) tea.Cmd {
	s.asOf = p.AsOf
	var cmds []tea.Cmd
	if uc := s.budget; uc != nil {
		cmds = append(cmds, func() tea.Msg { return loadBudgetAlerts(uc, s.loc, p) })
	}
	if uc := s.rec; uc != nil {
		cmds = append(cmds, func() tea.Msg { return loadRecurAlerts(uc, s.loc, p) })
	}
	cmds = append(cmds, s.loadInsights())
	return tea.Batch(cmds...)
}

func loadBudgetAlerts(uc AlertBudgetUseCases, loc domain.Locale, p sectionParams) tea.Msg {
	// Any day of the accounting month selects it; the month of the period end always lies inside it.
	per, err := domain.MonthPeriod(p.AsOf, p.MonthStartDay)
	if err != nil {
		return screens.ErrorMsg{Err: err}
	}
	to := per.To()
	st, err := uc.Status(context.Background(), fmt.Sprintf("%04d-%02d", to.Year(), to.Month()))
	if err != nil {
		if isModuleDisabled(err) {
			return budgetAlertsMsg{asOf: p.AsOf}
		}
		return screens.ErrorMsg{Err: err}
	}
	return budgetAlertsMsg{asOf: p.AsOf, alerts: budgetAlerts(st, loc)}
}

func budgetAlerts(st budgeting.Status, loc domain.Locale) []alert {
	var out []alert
	for _, c := range st.Caps {
		switch c.State {
		case domain.BudgetOver:
			out = append(out, alert{level: levelDanger, text: fmt.Sprintf("%s over budget: %s of %s (%d%%)",
				c.Budget.Name, domain.FormatAmount(c.Spent, loc), domain.FormatAmount(c.Available, loc), c.Percent)})
		case domain.BudgetOK:
		case domain.BudgetWarning:
			out = append(out, alert{level: levelWarning, text: fmt.Sprintf("%s near its cap: %s of %s (%d%%)",
				c.Budget.Name, domain.FormatAmount(c.Spent, loc), domain.FormatAmount(c.Available, loc), c.Percent)})
		}
	}
	for _, e := range st.Envelopes {
		if e.Overspend.Sign() > 0 {
			out = append(out, alert{level: levelDanger, text: fmt.Sprintf("%s overspent by %s",
				e.Budget.Name, domain.FormatAmount(e.Overspend, loc))})
		}
	}
	return out
}

func isModuleDisabled(err error) bool {
	return errors.Is(err, app.ErrModuleDisabled)
}

func loadRecurAlerts(uc AlertRecurringUseCases, loc domain.Locale, p sectionParams) tea.Msg {
	recs, err := uc.ListRecurrences(context.Background())
	if err != nil {
		if isModuleDisabled(err) {
			return recurAlertsMsg{asOf: p.AsOf}
		}
		return screens.ErrorMsg{Err: err}
	}
	per, err := domain.MonthPeriod(p.Today, p.MonthStartDay)
	if err != nil {
		return screens.ErrorMsg{Err: err}
	}
	return recurAlertsMsg{asOf: p.AsOf, alerts: recurrenceAlerts(recs, p.Today, per.To(), loc)}
}

// recurrenceAlerts lists the expense recurrences with an occurrence in [today, until].
func recurrenceAlerts(recs []app.Recurrence, today, until domain.Date, loc domain.Locale) []alert {
	type dated struct {
		on domain.Date
		a  alert
	}
	var found []dated
	for _, r := range recs {
		if r.Amount.Sign() >= 0 {
			continue
		}
		dr := domain.Recurrence{
			AccountID: r.AccountID, CategoryID: r.CategoryID, Description: r.Description, Amount: r.Amount,
			Rule: r.Rule, Start: r.Start, End: r.End, NextDate: r.NextDate,
		}
		for _, on := range domain.Expand(dr, today, until) {
			found = append(found, dated{on, alert{level: levelInfo, text: fmt.Sprintf("%s %s on %s",
				r.Description, domain.FormatAmount(r.Amount, loc), on)}})
		}
	}
	slices.SortStableFunc(found, func(a, b dated) int { return a.on.Cmp(b.on) })
	out := make([]alert, len(found))
	for i, f := range found {
		out[i] = f.a
	}
	return out
}

// rebuild merges the sources into entries sorted danger → warning → info.
func (s *alertsSection) rebuild() {
	var out []alert
	out = append(out, s.budgets...)
	out = append(out, s.upcoming...)
	for i := range s.insights {
		it := s.insights[i]
		lvl := levelInfo
		if it.Kind == string(domain.InsightAnomaly) {
			lvl = levelWarning
		}
		out = append(out, alert{level: lvl, text: s.insightText(it), insight: &it})
	}
	slices.SortStableFunc(out, func(a, b alert) int { return int(a.level) - int(b.level) })
	s.entries = out
	s.cursor = min(s.cursor, max(0, len(out)-1))
	if len(out) == 0 {
		s.focused = false
	}
}

func (s *alertsSection) update(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case budgetAlertsMsg:
		if msg.asOf == s.asOf {
			s.budgets = msg.alerts
			s.rebuild()
		}
		return nil, true
	case recurAlertsMsg:
		if msg.asOf == s.asOf {
			s.upcoming = msg.alerts
			s.rebuild()
		}
		return nil, true
	case insightsMsg:
		if msg.disabled {
			s.ins = nil
		}
		s.insights = msg.items
		s.rebuild()
		return nil, true
	case insightDoneMsg:
		return tea.Batch(s.loadInsights(), func() tea.Msg { return screens.StatusMsg{Text: msg.status} }), true
	case tea.KeyPressMsg:
		return s.handleKey(msg)
	}
	return nil, false
}

func (s *alertsSection) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	if !s.focused {
		if key.Matches(msg, s.km.Focus) && len(s.entries) > 0 {
			s.focused = true
			return nil, true
		}
		return nil, false
	}
	switch {
	case key.Matches(msg, s.km.Leave):
		s.focused = false
	case key.Matches(msg, s.km.Up):
		s.cursor = max(0, s.cursor-1)
	case key.Matches(msg, s.km.Down):
		s.cursor = min(len(s.entries)-1, s.cursor+1)
	case key.Matches(msg, s.km.Dismiss), key.Matches(msg, s.km.Accept):
		return s.act(key.Matches(msg, s.km.Accept)), true
	}
	return nil, true
}

// act dismisses or accepts the focused entry; only insight entries support it.
func (s *alertsSection) act(accept bool) tea.Cmd {
	it := s.entries[s.cursor].insight
	if it == nil || s.ins == nil {
		return func() tea.Msg { return screens.StatusMsg{Text: "Only insight alerts can be dismissed or accepted"} }
	}
	if accept {
		return s.accept(*it)
	}
	return s.dismiss(*it)
}

func (s *alertsSection) keys() []key.Binding {
	if len(s.entries) == 0 {
		return nil
	}
	if s.ins == nil {
		return []key.Binding{s.km.Focus}
	}
	return []key.Binding{s.km.Focus, s.km.Dismiss, s.km.Accept}
}

func (s *alertsSection) view(width int) string {
	inner := max(1, width-4)
	var lines []string
	if len(s.entries) == 0 {
		lines = append(lines, s.th.Muted.Render("all clear"))
	}
	start := 0
	if s.cursor >= alertsMaxRows {
		start = s.cursor - alertsMaxRows + 1
	}
	end := min(len(s.entries), start+alertsMaxRows)
	for i := start; i < end; i++ {
		lines = append(lines, s.entryLine(s.entries[i], inner, s.focused && i == s.cursor))
	}
	if more := len(s.entries) - end; more > 0 {
		lines = append(lines, s.th.Muted.Render(fmt.Sprintf("(+%d more)", more)))
	}
	// lipgloss v2 widths include the border.
	return s.th.Header.Render("Alerts") + "\n" +
		s.th.Border.Width(max(1, width-2)+s.th.Border.GetHorizontalBorderSize()).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// entryLine renders "icon level text"; the level word keeps the meaning without colour.
func (s *alertsSection) entryLine(a alert, width int, selected bool) string {
	icon, name, st := "✗", "danger", s.th.Error
	switch a.level {
	case levelDanger:
	case levelWarning:
		icon, name, st = "!", "warning", s.th.Warning
	case levelInfo:
		icon, name, st = "i", "info", s.th.Info
	}
	head := icon + " " + name
	text := ansi.Truncate(a.text, max(1, width-lipgloss.Width(head)-3), "…")
	if selected {
		return s.th.Selected.Render("> " + head + " " + text)
	}
	return "  " + st.Render(head) + " " + text
}
