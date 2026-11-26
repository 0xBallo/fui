// Package tui is the composition root of the terminal UI: it owns the screen router, the status
// bar, the global keymap, and the help overlay, and forwards every other message to the active
// screen.
package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/accounts"
	"github.com/0xballo/fui/internal/tui/screens/budget"
	"github.com/0xballo/fui/internal/tui/screens/dashboard"
	"github.com/0xballo/fui/internal/tui/screens/importer"
	"github.com/0xballo/fui/internal/tui/screens/report"
	"github.com/0xballo/fui/internal/tui/screens/settings"
	"github.com/0xballo/fui/internal/tui/screens/transactions"
	"github.com/0xballo/fui/internal/tui/theme"
)

// minWidth and minHeight are the smallest terminal size the TUI supports; below them Root shows
// only a warning.
const (
	minWidth  = 80
	minHeight = 24
)

// reservedRows is the vertical space Root reserves besides the body and the help bar: the tab bar
// (three lines), the blank line below it and the status line. The help bar adds its own height.
const reservedRows = 5

// bannerHeight is the rows the header banner takes above the tab bar: none on short terminals.
func (r *Root) bannerHeight() int {
	if r.height >= bannerMinHeight {
		return bannerRows
	}
	return 0
}

// maxHelpRows caps the height of the expanded help bar.
const maxHelpRows = 10

// UseCases is the union of every MVP screen's needs: the app layer (shared with the CLI) plus the
// reporting layer the dashboard uses for spending and runway.
type UseCases interface {
	accounts.UseCases
	transactions.UseCases
	dashboard.UseCases
	PlanUndo(ctx context.Context) (app.UndoPlan, error)
	ApplyUndo(ctx context.Context, plan app.UndoPlan) error
}

// Config holds the user-facing settings Root applies: the locale used to format amounts and
// dates, the theme ("auto" by default, detecting the terminal background; otherwise one of the
// Catppuccin flavors, "terminal", or "high-contrast"), the light/dark flavors "auto" picks
// between, and the first day of the accounting month (1..31, see domain.MonthPeriod) used by the
// dashboard's spending and runway reports.
//
// Modules is the enabled module set; a v0.2 screen is routed only when its use cases below are
// set and its module is enabled, so a zero Config yields the three MVP tabs.
type Config struct {
	Locale        domain.Locale
	Theme         string
	ThemeLight    string
	ThemeDark     string
	MonthStartDay int
	// Version is the build version shown in the header; "v0.3.0" is displayed as "v0.3".
	Version string

	Modules module.Set
	// Budget backs the Budget tab (module budget); Report the Reports tab and Importer the Import
	// tab (both core). Settings, ModuleConfig and ThemeConfig back the Settings tab (core).
	Budget       budget.UseCases
	Report       report.UseCases
	Importer     importer.UseCases
	Settings     settings.UseCases
	ModuleConfig settings.ModuleConfig
	ThemeConfig  settings.ThemeConfig
	// Optional v0.3 features: each is shown only when its use cases are set (and its module is
	// enabled): Positions (module investments) on Accounts, NetWorth (module analytics) and
	// Insights (module analytics) on the dashboard, Compare (module analytics) and Simulations
	// (module simulations) on Reports.
	Positions   accounts.PositionsUseCases
	NetWorth    dashboard.NetWorthUseCases
	Insights    dashboard.InsightUseCases
	Compare     report.CompareUseCases
	Simulations report.SimUseCases
	// Lists adds the CRUD sections (categories, tags, rules, recurrences, import profiles) to Settings.
	Lists settings.Lists
	// Flow (core), AlertBudget (module budget) and AlertRecurring (module recurring) feed the
	// dashboard flow chart and alerts.
	Flow           dashboard.FlowUseCases
	AlertBudget    dashboard.AlertBudgetUseCases
	AlertRecurring dashboard.AlertRecurringUseCases
	// Pending enables confirming and skipping pending rows on the Transactions tab.
	Pending transactions.PendingActions
}

// Root is the top-level Bubble Tea model: it routes between the screens (fixed keys 1..7),
// surfaces errors and status messages on a status line, and renders a help bar from every
// screen's keymap.
type Root struct {
	uc  UseCases
	cfg Config
	loc domain.Locale
	th  theme.Theme
	km  KeyMap
	// bg is the terminal background Bubble Tea reported, used by the "auto" and "high-contrast"
	// themes.
	bg theme.KnownBackground

	// tabs is indexed by screens.ID; a nil entry is a screen that is not routed (module disabled
	// or no use cases wired) and renders as a disabled tab.
	tabs   [len(tabNames)]screens.Screen
	active screens.ID

	width, height int
	status        string
	err           error
	help          components.HelpBar

	// undo confirms the journal entry in undoPlan before reverting it.
	undo     components.ConfirmDialog
	undoPlan app.UndoPlan
}

// undoPlannedMsg carries the result of PlanUndo back into Update.
type undoPlannedMsg struct {
	plan app.UndoPlan
	err  error
}

// undoneMsg carries the result of ApplyUndo back into Update.
type undoneMsg struct {
	kind string
	err  error
}

func (r *Root) planUndo() tea.Cmd {
	return func() tea.Msg {
		plan, err := r.uc.PlanUndo(context.Background())
		return undoPlannedMsg{plan: plan, err: err}
	}
}

func (r *Root) applyUndo(plan app.UndoPlan) tea.Cmd {
	return func() tea.Msg {
		return undoneMsg{kind: plan.Entry.Kind, err: r.uc.ApplyUndo(context.Background(), plan)}
	}
}

// undoDetails lists each change of the plan and its warnings for the confirmation dialog.
func undoDetails(plan app.UndoPlan) []string {
	var out []string
	for _, c := range plan.Changes {
		out = append(out, "• "+c.Summary)
	}
	for _, w := range plan.Warnings {
		out = append(out, "! "+w)
	}
	return out
}

func (r *Root) undoPlanned(msg undoPlannedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		r.err, r.status = msg.err, ""
		return r, nil
	}
	r.undoPlan, r.err, r.status = msg.plan, nil, ""
	r.undo.Show("Undo "+msg.plan.Entry.Kind, "Revert the last operation?", undoDetails(msg.plan)...)
	return r, nil
}

// undone reloads the active screen, which holds no optimistic state, after a successful undo.
func (r *Root) undone(msg undoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		r.err, r.status = msg.err, ""
		return r, nil
	}
	r.status, r.err = "Undone "+msg.kind, nil
	return r, r.tabs[r.active].Init()
}

// handleUndoKey resolves the open undo confirmation; every other key is swallowed.
func (r *Root) handleUndoKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	yes, _ := r.undo.HandleKey(msg)
	if yes {
		return r, r.applyUndo(r.undoPlan)
	}
	return r, nil
}

var _ tea.Model = (*Root)(nil)

// New builds Root over uc, the MVP screens, and cfg. Locale defaults to en-US and Theme to
// "auto" when unset; "auto" and "high-contrast" start from COLORFGBG (else a dark background) and
// switch once the terminal answers the OSC 11 query sent by Init.
func New(uc UseCases, cfg Config) *Root {
	if cfg.Locale == "" {
		cfg.Locale = domain.LocaleENUS
	}
	r := &Root{uc: uc, cfg: cfg, km: DefaultKeyMap()}
	r.rebuild()
	r.undo = components.NewConfirmDialog(r.th)
	return r
}

// rebuild resolves the theme and rebuilds every routed screen from r.cfg. Screens keep no state
// across visits, so replacing them is how a module or theme change takes effect without a
// restart.
func (r *Root) rebuild() {
	cfg := r.cfg
	r.loc = cfg.Locale
	r.th = theme.Resolve(theme.Config{
		Theme: cfg.Theme, ThemeLight: cfg.ThemeLight, ThemeDark: cfg.ThemeDark,
	}, r.bg)
	r.tabs = [len(tabNames)]screens.Screen{}
	r.tabs[screens.TabDashboard] = r.newDashboard()
	r.tabs[screens.TabTransactions] = r.newTransactions()
	r.tabs[screens.TabAccounts] = r.newAccounts()
	if cfg.Budget != nil && cfg.Modules.Has(module.Budget) {
		r.tabs[screens.TabBudget] = budget.New(cfg.Budget, r.loc, r.th)
	}
	if cfg.Report != nil {
		r.tabs[screens.TabReports] = r.newReport()
	}
	if cfg.Importer != nil {
		imp := importer.New(cfg.Importer, r.loc, r.th)
		imp.SetModules(cfg.Modules, cfg.ModuleConfig)
		imp.SetProfiles(cfg.Lists.Profiles)
		r.tabs[screens.TabImport] = imp
	}
	if cfg.Settings != nil && cfg.ModuleConfig != nil && cfg.ThemeConfig != nil {
		scr := settings.New(cfg.Settings, cfg.ModuleConfig, cfg.ThemeConfig, r.th)
		scr.SetLists(cfg.Lists)
		r.tabs[screens.TabSettings] = scr
	}
	if r.tabs[r.active] == nil {
		r.active = screens.TabDashboard
	}
	r.undo = components.NewConfirmDialog(r.th)
	r.help = components.NewHelpBar(r.th)
	r.help.SetWidth(r.width)
	if r.height > 0 {
		r.resize()
	}
}

// newTransactions builds the register, with the confirm and skip keys when the pending-row port
// is wired.
func (r *Root) newTransactions() *transactions.Model {
	m := transactions.New(r.uc, r.loc, r.th)
	if r.cfg.Pending != nil {
		m.SetPendingActions(r.cfg.Pending)
	}
	return m
}

// newDashboard builds the dashboard with the v0.3 net worth and insights features whose module is
// enabled and whose use cases are wired.
func (r *Root) newDashboard() *dashboard.Model {
	m := dashboard.New(r.uc, r.loc, r.th, r.cfg.MonthStartDay)
	if r.cfg.Flow != nil {
		m.SetFlowUseCases(r.cfg.Flow)
	}
	if r.cfg.AlertBudget != nil && r.cfg.Modules.Has(module.Budget) {
		m.SetAlertBudget(r.cfg.AlertBudget)
	}
	if r.cfg.AlertRecurring != nil && r.cfg.Modules.Has(module.Recurring) {
		m.SetAlertRecurring(r.cfg.AlertRecurring)
	}
	if r.cfg.Modules.Has(module.Analytics) {
		if r.cfg.NetWorth != nil {
			m.SetNetWorthUseCases(r.cfg.NetWorth)
		}
		if r.cfg.Insights != nil {
			m.SetInsights(r.cfg.Insights)
		}
	}
	return m
}

// newAccounts builds the accounts screen, with the positions panel when investments is enabled.
func (r *Root) newAccounts() *accounts.Model {
	m := accounts.New(r.uc, r.loc, r.th)
	if r.cfg.Positions != nil && r.cfg.Modules.Has(module.Investments) {
		m.SetPositionsUseCases(r.cfg.Positions)
	}
	m.SetCryptoEnabled(r.cfg.Modules.Has(module.Crypto))
	return m
}

// newReport builds the reports screen with the comparison (analytics) and simulations views.
func (r *Root) newReport() *report.Model {
	m := report.New(r.cfg.Report, r.loc, r.cfg.MonthStartDay, r.th)
	if r.cfg.Compare != nil && r.cfg.Modules.Has(module.Analytics) {
		m.SetCompare(r.cfg.Compare)
	}
	if r.cfg.Simulations != nil && r.cfg.Modules.Has(module.Simulations) {
		m.SetSimulations(r.cfg.Simulations)
	}
	return m
}

// retheme applies a theme to the running TUI. The settings screen is kept (re-themed in place), as
// it is the one emitting the preview and must not lose its cursor.
func (r *Root) retheme(name string) {
	r.cfg.Theme = name
	prev, _ := r.tabs[screens.TabSettings].(*settings.Model)
	r.rebuild()
	if prev != nil {
		prev.SetTheme(r.th)
		r.tabs[screens.TabSettings] = prev
		prev.SetSize(r.width, r.bodyHeight())
	}
}

// helpView renders the help bar for the active screen.
func (r *Root) helpView() string {
	maxH := min(maxHelpRows, max(1, r.height/2))
	out := r.help.View(r.km.Bindings(), r.ActiveKeyMap(), r.tabs[r.active].Title(), maxH)
	if r.help.Expanded() || r.width == 0 {
		return out
	}
	// bubbles/help overflows the width when the ellipsis itself does not fit after the last
	// item; cut the line here so the collapsed bar is always one row.
	return ansi.Truncate(out, r.width, "…")
}

// bodyHeight is the height left to the active screen below the tab bar and above the status
// line and the help bar.
func (r *Root) bodyHeight() int {
	return max(1, r.height-reservedRows-r.bannerHeight()-lipgloss.Height(r.helpView()))
}

func (r *Root) resize() {
	r.help.SetWidth(r.width)
	h := r.bodyHeight()
	for _, scr := range r.tabs {
		if scr != nil {
			scr.SetSize(r.width, h)
		}
	}
}

// Run starts the program over uc and cfg (full screen and with mouse reporting, see View) and
// blocks until the user quits or ctx is cancelled.
func Run(ctx context.Context, uc UseCases, cfg Config) error {
	if _, err := tea.NewProgram(New(uc, cfg), tea.WithContext(ctx)).Run(); err != nil {
		return fmt.Errorf("run tui: %w", err)
	}
	return nil
}

// Init loads the data of the initially active screen (the dashboard) and asks the terminal for its
// background color.
func (r *Root) Init() tea.Cmd {
	return tea.Batch(tea.RequestBackgroundColor, r.tabs[r.active].Init())
}

// Update advances the router state and, unless a global binding consumes the key, forwards the
// message to the active screen.
func (r *Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.width, r.height = msg.Width, msg.Height
		r.resize()
		return r, nil
	case tea.BackgroundColorMsg:
		return r.setBackground(msg.IsDark())
	case settings.ModulesChangedMsg:
		r.cfg.Modules = msg.Enabled
		r.rebuild()
		return r, r.tabs[r.active].Init()
	case settings.ThemePreviewMsg:
		r.retheme(msg.Theme)
		return r, nil
	case settings.ThemeChangedMsg:
		r.retheme(msg.Theme)
		return r, nil
	case screens.ErrorMsg:
		r.err, r.status = msg.Err, ""
		return r, nil
	case screens.StatusMsg:
		r.status, r.err = msg.Text, nil
		return r, nil
	case screens.SwitchScreenMsg:
		if msg.Filter != nil {
			return r.switchToFiltered(msg.Target, *msg.Filter)
		}
		return r.switchTo(msg.Target)
	case undoPlannedMsg:
		return r.undoPlanned(msg)
	case undoneMsg:
		return r.undone(msg)
	case tea.KeyPressMsg:
		return r.handleKey(msg)
	case tea.MouseMsg:
		return r.handleMouse(msg)
	}
	return r.forward(msg)
}

func (r *Root) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if r.undo.Visible() {
		return r.handleUndoKey(msg)
	}
	if r.help.Expanded() {
		r.help.Toggle()
		r.resize()
		return r, nil
	}
	// A screen with an open form, filter, search box, or categorize field owns every keystroke:
	// the router must not steal single letters (like "q") that are valid input there.
	if !r.tabs[r.active].InputActive() {
		switch {
		case key.Matches(msg, r.km.Quit):
			return r, tea.Quit
		case key.Matches(msg, r.km.Help):
			r.help.Toggle()
			r.resize()
			return r, nil
		case key.Matches(msg, r.km.Undo):
			return r, r.planUndo()
		case key.Matches(msg, r.km.Dashboard):
			return r.switchTo(screens.TabDashboard)
		case key.Matches(msg, r.km.Transactions):
			return r.switchTo(screens.TabTransactions)
		case key.Matches(msg, r.km.Accounts):
			return r.switchTo(screens.TabAccounts)
		case key.Matches(msg, r.km.Budget):
			return r.switchTo(screens.TabBudget)
		case key.Matches(msg, r.km.Reports):
			return r.switchTo(screens.TabReports)
		case key.Matches(msg, r.km.Import):
			return r.switchTo(screens.TabImport)
		case key.Matches(msg, r.km.Settings):
			return r.switchTo(screens.TabSettings)
		}
	}
	return r.forward(msg)
}

// switchTo activates target (ignoring tabs with no screen: module disabled or not wired) and
// reloads its data, since screens never keep optimistic state between visits.
func (r *Root) switchTo(target screens.ID) (tea.Model, tea.Cmd) {
	if target < 0 || int(target) >= len(r.tabs) || r.tabs[target] == nil {
		r.status = "that screen is not available"
		return r, nil
	}
	r.active, r.status, r.err = target, "", nil
	return r, r.tabs[r.active].Init()
}

// switchToFiltered navigates to target and, when that screen can receive a filter, applies it
// instead of the screen's plain Init so the unfiltered list is never loaded; other screens
// behave as in switchTo and ignore the filter.
func (r *Root) switchToFiltered(target screens.ID, req app.ListTxRequest) (tea.Model, tea.Cmd) {
	model, cmd := r.switchTo(target)
	if r.active != target {
		return model, cmd
	}
	if fr, ok := r.tabs[target].(screens.FilterReceiver); ok {
		return r, fr.ApplyFilter(req)
	}
	return model, cmd
}

// forward delivers msg to the active screen, or to the screen it targets when it is a result of an
// async command that may have been started on a tab that is no longer active.
func (r *Root) forward(msg tea.Msg) (tea.Model, tea.Cmd) {
	target := r.active
	if t, ok := msg.(screens.Targeted); ok {
		if id := t.ScreenID(); id >= 0 && int(id) < len(r.tabs) && r.tabs[id] != nil {
			target = id
		} else {
			return r, nil // its screen no longer exists (module disabled meanwhile)
		}
	}
	scr, cmd := r.tabs[target].Update(msg)
	r.tabs[target] = scr
	return r, cmd
}

// ActiveKeyMap returns the key bindings of the currently active screen, for the help overlay and
// for tests that check no screen rebinds a key Root already uses globally.
func (r *Root) ActiveKeyMap() []key.Binding { return r.tabs[r.active].KeyMap() }

// View renders the program full screen (the alternate screen, so the shell is restored on exit)
// with mouse reporting.
func (r *Root) View() tea.View {
	v := tea.NewView(r.render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// render draws the tab bar, the active screen, and the status bar, or just a warning below the
// minimum supported terminal size.
func (r *Root) render() string {
	if r.width > 0 && (r.width < minWidth || r.height < minHeight) {
		return r.th.Warning.Render(fmt.Sprintf(
			"Terminal too small: %dx%d. FUI needs at least %dx%d.", r.width, r.height, minWidth, minHeight))
	}
	helpBar := r.helpView()
	h := max(1, r.height-reservedRows-r.bannerHeight()-lipgloss.Height(helpBar))
	body := r.tabs[r.active].View()
	if r.undo.Visible() {
		body = components.Modal(r.th, r.width, h, r.undo.View())
	}
	if r.height > 0 {
		body = lipgloss.NewStyle().Height(h).MaxHeight(h).Render(body)
	}
	header := r.viewTabs()
	if r.bannerHeight() > 0 {
		header = viewBanner(r.th, r.cfg.Version) + "\n" + header
	}
	return header + "\n\n" + body + "\n" + r.viewStatus() + "\n" + helpBar
}

// tabNames are the seven screens in tab order; a tab with no Screen is rendered disabled.
var tabNames = [...]string{"Dashboard", "Transactions", "Accounts", "Budget", "Reports", "Import", "Settings"}

// Tab borders: the active tab is open at the bottom so it merges with the rule that continues to
// the right edge, as in the lipgloss tabs example.
var (
	tabBorder = lipgloss.Border{
		Top: "─", Bottom: "─", Left: "│", Right: "│",
		TopLeft: "╭", TopRight: "╮", BottomLeft: "┴", BottomRight: "┴",
	}
	activeTabBorder = lipgloss.Border{
		Top: "─", Bottom: " ", Left: "│", Right: "│",
		TopLeft: "╭", TopRight: "╮", BottomLeft: "┘", BottomRight: "└",
	}
)

// viewTabs renders the bordered tab bar, dropping padding and then the inactive tabs' numbers when
// the terminal is too narrow for all seven labels.
func (r *Root) viewTabs() string {
	row := lipgloss.JoinHorizontal(lipgloss.Top, r.tabParts()...)
	gapW := max(0, r.width-lipgloss.Width(row))
	rule := lipgloss.NewStyle().Foreground(r.th.Tokens.Accent).Render(strings.Repeat("─", gapW))
	middle := strings.Repeat(" ", gapW)
	if r.bannerHeight() == 0 {
		// The compact title sits on the middle row of the tab bar, right-aligned; it is dropped
		// when the gap cannot hold it plus a space of margin.
		title := logoStyle(r.th).Render("FinanceUI") + " " + r.th.Muted.Render(byline(r.cfg.Version))
		if w := lipgloss.Width(title); gapW > w {
			middle = strings.Repeat(" ", gapW-w) + title
		}
	}
	gap := strings.Repeat(" ", gapW) + "\n" + middle + "\n" + rule
	return lipgloss.JoinHorizontal(lipgloss.Top, row, gap)
}

// tabParts renders each tab with the roomiest variant that fits the terminal width.
func (r *Root) tabParts() []string {
	var parts []string
	for _, v := range []struct {
		pad      int
		numbered bool
	}{{1, true}, {0, true}, {0, false}} {
		parts = r.renderTabs(v.pad, v.numbered)
		if r.width == 0 || lipgloss.Width(lipgloss.JoinHorizontal(lipgloss.Top, parts...)) <= r.width {
			break
		}
	}
	return parts
}

// tabAt returns the tab under column x of the tab bar.
func (r *Root) tabAt(x int) (screens.ID, bool) {
	left := 0
	for i, part := range r.tabParts() {
		left += lipgloss.Width(part)
		if x < left {
			return screens.ID(i), true
		}
	}
	return 0, false
}

// tabBarRows is the height of the bordered tab bar.
const tabBarRows = 3

// handleMouse switches tabs on a click in the tab bar and turns the wheel into up/down keys for
// the active screen. A click anywhere closes the help; a screen with an open form gets no mouse
// input, as its fields are keyboard-only.
func (r *Root) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	m := msg.Mouse()
	switch msg.(type) {
	case tea.MouseClickMsg:
		return r.handleClick(m)
	case tea.MouseWheelMsg:
		if r.help.Expanded() || r.undo.Visible() || r.tabs[r.active].InputActive() {
			return r, nil
		}
		if m.Button == tea.MouseWheelUp {
			return r.forward(tea.KeyPressMsg{Code: tea.KeyUp})
		}
		if m.Button == tea.MouseWheelDown {
			return r.forward(tea.KeyPressMsg{Code: tea.KeyDown})
		}
	}
	return r, nil
}

// handleClick closes the help on a left click, or switches to the tab clicked in the tab bar.
func (r *Root) handleClick(m tea.Mouse) (tea.Model, tea.Cmd) {
	switch {
	case m.Button != tea.MouseLeft:
	case r.help.Expanded():
		r.help.Toggle()
		r.resize()
	case r.undo.Visible(), r.tabs[r.active].InputActive():
	case m.Y >= r.bannerHeight() && m.Y < r.bannerHeight()+tabBarRows:
		if id, ok := r.tabAt(m.X); ok {
			return r.switchTo(id)
		}
	}
	return r, nil
}

// setBackground re-resolves the theme with the background the terminal reported. Screens are
// rebuilt (and the active one reloaded) only when the theme actually changes, which happens at
// most once, right after startup.
func (r *Root) setBackground(dark bool) (tea.Model, tea.Cmd) {
	r.bg = theme.KnownBackground{Known: true, IsDark: dark}
	next := theme.Resolve(theme.Config{
		Theme: r.cfg.Theme, ThemeLight: r.cfg.ThemeLight, ThemeDark: r.cfg.ThemeDark,
	}, r.bg)
	if next.Name == r.th.Name && next.Tokens.Base == r.th.Tokens.Base {
		return r, nil
	}
	r.rebuild()
	return r, r.tabs[r.active].Init()
}

func (r *Root) renderTabs(pad int, numbered bool) []string {
	tab := lipgloss.NewStyle().Border(tabBorder, true).BorderForeground(r.th.Tokens.Accent).Padding(0, pad)
	parts := make([]string, len(tabNames))
	for i, name := range tabNames {
		label := name
		if numbered || screens.ID(i) == r.active {
			label = fmt.Sprintf("%d %s", i+1, name)
		}
		switch {
		case r.tabs[i] == nil:
			parts[i] = tab.Render(r.th.DisabledTab.Render(label))
		case screens.ID(i) == r.active:
			parts[i] = tab.Border(activeTabBorder, true).Render(r.th.ActiveTab.Render(label))
		default:
			parts[i] = tab.Render(r.th.InactiveTab.Render(label))
		}
	}
	return parts
}

func (r *Root) viewStatus() string {
	switch {
	case r.err != nil:
		return r.th.Error.Render("Error: " + r.err.Error())
	case r.status != "":
		return r.th.StatusBar.Render(r.status)
	default:
		return ""
	}
}
