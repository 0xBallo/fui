package dashboard

import (
	"context"
	"errors"
	"strconv"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/reporting"
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

// Model is the dashboard screen: account balances, net worth, the runway KPI, current-month
// spending by category, and a quick-add shortcut. Update and View never touch the store; every
// use case call runs inside a tea.Cmd.
type Model struct {
	uc  UseCases
	loc domain.Locale
	th  theme.Theme
	km  KeyMap

	width, height int
	mode          mode

	balances []app.AccountBalance
	table    components.Table

	netWorth    app.NetWorth
	netWorthErr error

	spending reporting.SpendingResult
	chart    components.BarChart

	runway          reporting.RunwayResult
	runwayAvailable bool

	categories []string
	accounts   []string

	form      components.Form
	accountF  components.Autocomplete
	amountF   components.Autocomplete
	descF     components.Autocomplete
	categoryF components.Autocomplete
}

var _ screens.Screen = (*Model)(nil)

// New builds the dashboard screen over uc, formatting amounts and dates with loc.
func New(uc UseCases, loc domain.Locale, th theme.Theme) *Model {
	m := &Model{uc: uc, loc: loc, th: th, km: DefaultKeyMap()}
	m.table = components.NewTable([]components.Column{
		{Title: "Account", Width: 24, Align: lipgloss.Left},
		{Title: "Kind", Width: 12, Align: lipgloss.Left},
		{Title: "Balance", Width: 16, Align: lipgloss.Right},
	}, th)
	m.chart = components.NewBarChart(th)
	return m
}

// Init loads the dashboard data.
func (m *Model) Init() tea.Cmd { return m.load() }

// SetSize adapts the panels to the available terminal size.
func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	rows := len(m.balances)
	if rows > 8 {
		rows = 8
	}
	if rows < 3 {
		rows = 3
	}
	m.table.SetHeight(rows)
	m.table.SetWidth(w)
	m.chart.SetWidth(w)
}

// Title identifies the screen in the tab bar.
func (m *Model) Title() string { return "Dashboard" }

// KeyMap exposes the bindings for the global help view.
func (m *Model) KeyMap() []key.Binding { return m.km.Bindings() }

// loadedMsg carries every piece of fresh dashboard data back into Update.
type loadedMsg struct {
	balances []app.AccountBalance

	netWorth    app.NetWorth
	netWorthErr error

	spending reporting.SpendingResult

	runway          reporting.RunwayResult
	runwayAvailable bool

	categories []string
	accounts   []string
}

func (m *Model) load() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		bals, err := m.uc.ListAccounts(ctx, app.ListAccountsRequest{})
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		out := loadedMsg{balances: bals}
		for _, b := range bals {
			out.accounts = append(out.accounts, b.Account.Name)
		}

		// Net worth needs a configured base currency and fx rates; it is shown as "n/a" rather
		// than failing the whole screen when either is missing.
		out.netWorth, out.netWorthErr = m.uc.NetWorth(ctx, app.NetWorthRequest{})

		cats, err := m.uc.ListCategories(ctx)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		for _, c := range cats {
			out.categories = append(out.categories, c.Name)
		}

		// Accounting months start on the 1st: FUI has no per-user setting for this yet.
		const monthStartDay = 1
		base := out.netWorth.Base
		out.spending, err = m.uc.Spending(ctx, reporting.SpendingParams{BaseCurrency: base, MonthStartDay: monthStartDay})
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}

		out.runway, err = m.uc.Runway(ctx, reporting.RunwayParams{BaseCurrency: base, MonthStartDay: monthStartDay})
		switch {
		case errors.Is(err, domain.ErrNotEnoughData):
			out.runwayAvailable = false
		case err != nil:
			return screens.ErrorMsg{Err: err}
		default:
			out.runwayAvailable = true
		}
		return out
	}
}

// Update advances the screen state; it performs no I/O itself.
func (m *Model) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case loadedMsg:
		m.balances = msg.balances
		m.table.SetRows(m.accountRows())
		m.netWorth, m.netWorthErr = msg.netWorth, msg.netWorthErr
		m.spending = msg.spending
		m.chart.SetItems(spendingBars(msg.spending))
		m.runway, m.runwayAvailable = msg.runway, msg.runwayAvailable
		m.categories, m.accounts = msg.categories, msg.accounts
		m.mode = modeView
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	if m.mode == modeForm {
		return m.handleFormKey(msg)
	}
	if key.Matches(msg, m.km.Add) {
		m.openAddForm()
	}
	return m, nil
}

func (m *Model) accountRows() []components.Row {
	rows := make([]components.Row, len(m.balances))
	for i, b := range m.balances {
		rows[i] = components.Row{ID: b.Account.Name, Cells: []string{
			b.Account.Name, string(b.Account.Kind), domain.FormatAmount(b.Balance, m.loc),
		}}
	}
	return rows
}

// spendingBars converts a spending report into bar chart items. Spending totals are already
// positive absolute outflows (see domain.SpendingByCategory), so no sign handling is needed here.
func spendingBars(r reporting.SpendingResult) []components.BarItem {
	items := make([]components.BarItem, 0, len(r.Rows))
	for _, row := range r.Rows {
		items = append(items, components.BarItem{
			Label: row.CategoryName, Value: row.Total.Minor(),
			DisplayValue: domain.FormatMachine(row.Total),
		})
	}
	return items
}

// View renders the active panel.
func (m *Model) View() string {
	if m.mode == modeForm {
		return m.viewForm()
	}
	return m.viewDashboard()
}

func (m *Model) viewDashboard() string {
	out := m.th.Title.Render("Dashboard") + "\n\n"
	out += m.th.Header.Render("Accounts") + "\n" + m.table.View() + "\n\n"
	out += m.th.Header.Render("Net worth") + "\n" + m.viewNetWorth() + "\n\n"
	out += m.th.Header.Render("Runway") + "\n" + m.viewRunway() + "\n\n"
	out += m.th.Header.Render("Spending this month") + "\n" + m.chart.View()
	return out
}

func (m *Model) viewNetWorth() string {
	if m.netWorthErr != nil {
		return m.th.Muted.Render("n/a (" + m.netWorthErr.Error() + ")")
	}
	return m.th.Base.Render(domain.FormatAmount(m.netWorth.Total, m.loc))
}

func (m *Model) viewRunway() string {
	if !m.runwayAvailable {
		return m.th.Muted.Render("not enough data (needs 12 complete months of history)")
	}
	months, days := m.runway.Months, m.runway.Days
	return m.th.Base.Render(domain.FormatAmount(m.runway.Liquidity, m.loc)) +
		" — " + strconv.Itoa(months) + " months (" + strconv.Itoa(days) + " days), zero around " + m.runway.ZeroDate.String()
}
