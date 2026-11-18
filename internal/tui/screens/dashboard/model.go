package dashboard

import (
	"context"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/theme"
)

// mode tracks which panel of the screen is active.
type mode int

const (
	modeView mode = iota
	modeForm
)

// twoColumnWidth is the terminal width from which the sections are laid out in two columns.
const twoColumnWidth = 120

// Model is the dashboard screen: a cycle header, the net worth, the other sections, and a
// quick-add shortcut. Update and View never touch the store; every use case call runs inside a
// tea.Cmd.
type Model struct {
	uc            UseCases
	loc           domain.Locale
	th            theme.Theme
	km            KeyMap
	monthStartDay int

	width, height int
	mode          mode
	vp            viewport.Model

	cycle  domain.Cycle
	anchor domain.Date
	today  domain.Date
	since  domain.Date
	base   domain.Currency
	period domain.Period
	ready  bool

	nw       *netWorthSection
	sections []section

	categories []string
	accounts   []string

	ins insightPanel

	form      components.Form
	accountF  components.Autocomplete
	amountF   components.Autocomplete
	descF     components.Autocomplete
	categoryF components.Autocomplete
}

var _ screens.Screen = (*Model)(nil)

// New builds the dashboard screen over uc, formatting amounts and dates with loc. monthStartDay
// is the first day of the accounting month (1..31, see domain.MonthPeriod); values outside that
// range default to 1.
func New(uc UseCases, loc domain.Locale, th theme.Theme, monthStartDay int) *Model {
	if monthStartDay < 1 || monthStartDay > 31 {
		monthStartDay = 1
	}
	m := &Model{
		uc: uc, loc: loc, th: th, km: DefaultKeyMap(), monthStartDay: monthStartDay,
		cycle: domain.CycleMonth, vp: viewport.New(0, 0),
	}
	m.nw = &netWorthSection{uc: uc, loc: loc, th: th}
	m.sections = []section{m.nw, &runwaySection{uc: uc, th: th}, flowSection{}, newRecentSection(uc, loc, th), alertsSection{}}
	return m
}

// Init loads the dashboard data.
func (m *Model) Init() tea.Cmd { return tea.Batch(m.loadBase(), m.ins.load()) }

// SetSize adapts the panels to the available terminal size.
func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	m.ins.width = w
	m.vp.Width = w
	m.vp.Height = max(1, h-2)
}

// Title identifies the screen in the tab bar.
func (m *Model) Title() string { return "Dashboard" }

// KeyMap exposes the bindings for the global help view.
func (m *Model) KeyMap() []key.Binding {
	out := append(m.km.Bindings(), m.ins.keyBindings()...)
	for _, s := range m.sections {
		out = append(out, s.keys()...)
	}
	return out
}

// InputActive reports whether the quick-add form is capturing keystrokes.
func (m *Model) InputActive() bool { return m.mode == modeForm }

// baseMsg carries the cycle-independent data back into Update.
type baseMsg struct {
	today      domain.Date
	since      domain.Date
	base       domain.Currency
	categories []string
	accounts   []string
}

func (m *Model) loadBase() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		today, err := m.uc.Today()
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		out := baseMsg{today: today}
		bals, err := m.uc.ListAccounts(ctx, app.ListAccountsRequest{})
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		for _, b := range bals {
			out.accounts = append(out.accounts, b.Account.Name)
		}
		cats, err := m.uc.ListCategories(ctx)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		for _, c := range cats {
			out.categories = append(out.categories, c.Name)
		}
		// The base currency is only known once configured; sections cope with it being empty.
		if nw, err := m.uc.NetWorth(ctx, app.NetWorthRequest{}); err == nil {
			out.base = nw.Base
		}
		page, err := m.uc.ListTx(ctx, app.ListTxRequest{Sort: domain.TxSortDate, Limit: 1})
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if len(page.Items) > 0 {
			out.since = page.Items[0].Transaction.Date
		}
		return out
	}
}

// recompute derives the period from the cycle and anchor and reloads every section.
func (m *Model) recompute() tea.Cmd {
	p, err := domain.CyclePeriod(m.cycle, m.anchor, m.monthStartDay, m.since)
	if err != nil {
		return func() tea.Msg { return screens.ErrorMsg{Err: err} }
	}
	m.period = p
	asOf := p.To()
	if m.today.Before(asOf) {
		asOf = m.today
	}
	params := sectionParams{Period: p, AsOf: asOf, Today: m.today, Base: m.base, MonthStartDay: m.monthStartDay}
	cmds := make([]tea.Cmd, 0, len(m.sections))
	for _, s := range m.sections {
		cmds = append(cmds, s.load(params))
	}
	return tea.Batch(cmds...)
}

// atCurrent reports whether the period contains today, so there is no later period to show.
func (m *Model) atCurrent() bool {
	return m.cycle == domain.CycleLifetime || m.period.Contains(m.today)
}

// Update advances the screen state; it performs no I/O itself.
func (m *Model) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	if _, isKey := msg.(tea.KeyMsg); !isKey || m.mode == modeView {
		if cmd, ok := m.ins.update(msg); ok {
			return m, cmd
		}
	}
	if _, isKey := msg.(tea.KeyMsg); !isKey {
		for _, s := range m.sections {
			if cmd, ok := s.update(msg); ok {
				return m, cmd
			}
		}
	}
	switch msg := msg.(type) {
	case baseMsg:
		m.today, m.since, m.base = msg.today, msg.since, msg.base
		m.categories, m.accounts = msg.categories, msg.accounts
		m.mode = modeView
		if !m.ready {
			m.ready = true
			m.anchor = msg.today
		}
		return m, m.recompute()
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	if m.mode == modeForm {
		return m.handleFormKey(msg)
	}
	for _, s := range m.sections {
		if cmd, ok := s.update(msg); ok {
			return m, cmd
		}
	}
	switch {
	case key.Matches(msg, m.km.Add):
		m.openAddForm()
	case !m.ready:
	case key.Matches(msg, m.km.Cycle):
		m.cycle = m.cycle.Next()
		m.anchor = m.today
		return m, m.recompute()
	case key.Matches(msg, m.km.Prev):
		return m, m.shift(-1)
	case key.Matches(msg, m.km.Next):
		if m.atCurrent() {
			return m, nil
		}
		return m, m.shift(1)
	case key.Matches(msg, m.km.Up):
		m.vp.ScrollUp(1)
	case key.Matches(msg, m.km.Down):
		m.vp.ScrollDown(1)
	case key.Matches(msg, m.km.PageUp):
		m.vp.HalfPageUp()
	case key.Matches(msg, m.km.PageDown):
		m.vp.HalfPageDown()
	}
	return m, nil
}

func (m *Model) shift(n int) tea.Cmd {
	if m.cycle == domain.CycleLifetime {
		return nil
	}
	a, err := domain.ShiftCycle(m.cycle, m.anchor, n, m.monthStartDay)
	if err != nil {
		return func() tea.Msg { return screens.ErrorMsg{Err: err} }
	}
	m.anchor = a
	return m.recompute()
}

// View renders the active panel.
func (m *Model) View() string {
	if m.mode == modeForm {
		return components.Modal(m.th, m.width, m.height, m.viewForm())
	}
	m.vp.SetContent(m.content())
	return m.viewHeader() + "\n\n" + m.vp.View()
}

var monthNames = [...]string{
	"January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December",
}

func (m *Model) viewHeader() string {
	if !m.ready {
		return m.th.Muted.Render("loading…")
	}
	from := m.period.From()
	var label string
	switch m.cycle {
	case domain.CycleMonth:
		label = monthNames[from.Month()-1] + " " + strconv.Itoa(from.Year())
	case domain.CycleQuarter:
		label = "Q" + strconv.Itoa((from.Month()-1)/3+1) + " " + strconv.Itoa(from.Year())
	case domain.CycleYear:
		label = strconv.Itoa(from.Year())
	case domain.CycleLifetime:
		label = "since " + from.String()
	}
	prev, next := "◀", "▶"
	if m.atCurrent() {
		next = " "
	}
	if m.cycle == domain.CycleLifetime {
		prev = " "
	}
	return m.th.Header.Render(prev + " " + label + " · " + string(m.cycle) + " " + next)
}

// content lays the sections out in one or two columns, then appends the quick insight panel.
func (m *Model) content() string {
	var out string
	if m.width >= twoColumnWidth {
		colW := m.width/2 - 1
		left := m.column(colW, m.sections[:3])
		right := m.column(colW, m.sections[3:])
		out = lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(colW).Render(left), "  ",
			lipgloss.NewStyle().Width(colW).Render(right))
	} else {
		out = m.column(m.width, m.sections)
	}
	if ins := m.ins.view(); ins != "" {
		out += "\n\n" + ins
	}
	return out
}

func (m *Model) column(width int, secs []section) string {
	var parts []string
	for _, s := range secs {
		if v := s.view(width); v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, "\n\n")
}
