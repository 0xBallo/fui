// Package tui is the composition root of the terminal UI: it owns the screen router, the status
// bar, the global keymap, and the help overlay, and forwards every other message to the active
// screen.
package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

// reservedRows is the vertical space Root reserves for the tab bar (three lines), the status bar
// and the blank line above it, plus one for the rule under each screen title, leaving the rest to
// the active screen.
const reservedRows = 6

// UseCases is the union of every MVP screen's needs: the app layer (shared with the CLI) plus the
// reporting layer the dashboard uses for spending and runway.
type UseCases interface {
	accounts.UseCases
	transactions.UseCases
	dashboard.UseCases
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

	Modules module.Set
	// Budget backs the Budget tab (module budget); Report the Reports tab and Importer the Import
	// tab (both core). Settings, ModuleConfig and ThemeConfig back the Settings tab (core).
	Budget       budget.UseCases
	Report       report.UseCases
	Importer     importer.UseCases
	Settings     settings.UseCases
	ModuleConfig settings.ModuleConfig
	ThemeConfig  settings.ThemeConfig
	// Lists adds the CRUD sections (categories, tags, rules, recurrences, import profiles) to Settings.
	Lists settings.Lists
}

// Root is the top-level Bubble Tea model: it routes between the screens (fixed keys 1..7),
// surfaces errors and status messages on a status bar, and renders a help overlay from every
// screen's keymap.
type Root struct {
	uc  UseCases
	cfg Config
	loc domain.Locale
	th  theme.Theme
	km  KeyMap

	// tabs is indexed by screens.ID; a nil entry is a screen that is not routed (module disabled
	// or no use cases wired) and renders as a disabled tab.
	tabs   [len(tabNames)]screens.Screen
	active screens.ID

	width, height int
	status        string
	err           error
	showHelp      bool
}

var _ tea.Model = (*Root)(nil)

// New builds Root over uc, the MVP screens, and cfg. Locale defaults to en-US and Theme to
// "auto" when unset; "auto" and "high-contrast" detect the terminal's background with
// theme.LipglossDetector (OSC 11, falling back to COLORFGBG, then assuming a dark background).
func New(uc UseCases, cfg Config) *Root {
	if cfg.Locale == "" {
		cfg.Locale = domain.LocaleENUS
	}
	r := &Root{uc: uc, cfg: cfg, km: DefaultKeyMap()}
	r.rebuild()
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
	}, theme.LipglossDetector{})
	r.tabs = [len(tabNames)]screens.Screen{}
	r.tabs[screens.TabDashboard] = dashboard.New(r.uc, r.loc, r.th, cfg.MonthStartDay)
	r.tabs[screens.TabTransactions] = transactions.New(r.uc, r.loc, r.th)
	r.tabs[screens.TabAccounts] = accounts.New(r.uc, r.loc, r.th)
	if cfg.Budget != nil && cfg.Modules.Has(module.Budget) {
		r.tabs[screens.TabBudget] = budget.New(cfg.Budget, r.loc, r.th)
	}
	if cfg.Report != nil {
		r.tabs[screens.TabReports] = report.New(cfg.Report, r.loc, cfg.MonthStartDay, r.th)
	}
	if cfg.Importer != nil {
		r.tabs[screens.TabImport] = importer.New(cfg.Importer, r.loc, r.th)
	}
	if cfg.Settings != nil && cfg.ModuleConfig != nil && cfg.ThemeConfig != nil {
		scr := settings.New(cfg.Settings, cfg.ModuleConfig, cfg.ThemeConfig, r.th)
		scr.SetLists(cfg.Lists)
		r.tabs[screens.TabSettings] = scr
	}
	if r.tabs[r.active] == nil {
		r.active = screens.TabDashboard
	}
	if r.height > 0 {
		r.resize()
	}
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
		prev.SetSize(r.width, max(1, r.height-reservedRows))
	}
}

func (r *Root) resize() {
	h := max(1, r.height-reservedRows)
	for _, scr := range r.tabs {
		if scr != nil {
			scr.SetSize(r.width, h)
		}
	}
}

// Run starts the program full screen (the alternate screen, so the shell is restored on exit) and
// with mouse reporting, over uc and cfg and blocks until the user quits or ctx is cancelled.
func Run(ctx context.Context, uc UseCases, cfg Config) error {
	if _, err := tea.NewProgram(New(uc, cfg), tea.WithContext(ctx), tea.WithAltScreen(), tea.WithMouseCellMotion()).Run(); err != nil {
		return fmt.Errorf("run tui: %w", err)
	}
	return nil
}

// Init loads the data of the initially active screen (the dashboard).
func (r *Root) Init() tea.Cmd { return r.tabs[r.active].Init() }

// Update advances the router state and, unless a global binding consumes the key, forwards the
// message to the active screen.
func (r *Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.width, r.height = msg.Width, msg.Height
		r.resize()
		return r, nil
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
		return r.switchTo(msg.Target)
	case tea.KeyMsg:
		return r.handleKey(msg)
	case tea.MouseMsg:
		return r.handleMouse(msg)
	}
	return r.forward(msg)
}

func (r *Root) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if r.showHelp {
		r.showHelp = false
		return r, nil
	}
	// A screen with an open form, filter, search box, or categorize field owns every keystroke:
	// the router must not steal single letters (like "q") that are valid input there.
	if !r.tabs[r.active].InputActive() {
		switch {
		case key.Matches(msg, r.km.Quit):
			return r, tea.Quit
		case key.Matches(msg, r.km.Help):
			r.showHelp = true
			return r, nil
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

func (r *Root) forward(msg tea.Msg) (tea.Model, tea.Cmd) {
	scr, cmd := r.tabs[r.active].Update(msg)
	r.tabs[r.active] = scr
	return r, cmd
}

// ActiveKeyMap returns the key bindings of the currently active screen, for the help overlay and
// for tests that check no screen rebinds a key Root already uses globally.
func (r *Root) ActiveKeyMap() []key.Binding { return r.tabs[r.active].KeyMap() }

// View renders the tab bar, the active screen, and the status bar, or just a warning below the
// minimum supported terminal size.
func (r *Root) View() string {
	if r.width > 0 && (r.width < minWidth || r.height < minHeight) {
		return r.th.Warning.Render(fmt.Sprintf(
			"Terminal too small: %dx%d. FUI needs at least %dx%d.", r.width, r.height, minWidth, minHeight))
	}
	body := r.tabs[r.active].View()
	// Fill the terminal: the body takes every line the tab bar (three), the blank line and the
	// status bar leave, so the status bar sits on the last row.
	h := max(1, r.height-5)
	if r.showHelp {
		body = components.Modal(r.th, r.width, h, r.viewHelp(h))
	}
	if r.height > 0 {
		body = lipgloss.NewStyle().Height(h).MaxHeight(h).Render(body)
	}
	return r.viewTabs() + "\n" + body + "\n\n" + r.viewStatus()
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
	gap := lipgloss.NewStyle().Border(tabBorder, false, false, true, false).
		BorderForeground(r.th.Tokens.Accent).
		Render(strings.Repeat(" ", max(0, r.width-lipgloss.Width(row))))
	return lipgloss.JoinHorizontal(lipgloss.Bottom, row, gap)
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
	if msg.Action != tea.MouseActionPress {
		return r, nil
	}
	switch {
	case r.showHelp:
		if msg.Button == tea.MouseButtonLeft {
			r.showHelp = false
		}
	case r.tabs[r.active].InputActive():
	case msg.Button == tea.MouseButtonLeft && msg.Y < tabBarRows:
		if id, ok := r.tabAt(msg.X); ok {
			return r.switchTo(id)
		}
	case msg.Button == tea.MouseButtonWheelUp:
		return r.forward(tea.KeyMsg{Type: tea.KeyUp})
	case msg.Button == tea.MouseButtonWheelDown:
		return r.forward(tea.KeyMsg{Type: tea.KeyDown})
	}
	return r, nil
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
		return r.th.StatusBar.Render("? help · q quit")
	}
}

// viewHelp lists the global and active-screen bindings, wrapping them into side-by-side columns
// when they would not fit the h lines of the modal that frames them.
func (r *Root) viewHelp(h int) string {
	bindings := append(append([]key.Binding{}, r.km.Bindings()...), r.ActiveKeyMap()...)
	var keys, descs []string
	for _, b := range bindings {
		if hb := b.Help(); hb.Key != "" {
			keys, descs = append(keys, hb.Key), append(descs, hb.Desc)
		}
	}
	// The modal's border and the title with its rule take four lines.
	perCol := len(keys)
	if r.height > 0 {
		perCol = max(1, h-4)
	}
	var cols []string
	for start := 0; start < len(keys); start += perCol {
		end := min(start+perCol, len(keys))
		keyW := 0
		for _, k := range keys[start:end] {
			keyW = max(keyW, lipgloss.Width(k))
		}
		lines := make([]string, 0, end-start)
		for i := start; i < end; i++ {
			lines = append(lines, r.th.HelpKey.Width(keyW).Render(keys[i])+"  "+r.th.HelpDesc.Render(descs[i]))
		}
		col := strings.Join(lines, "\n")
		if end < len(keys) {
			col = lipgloss.NewStyle().PaddingRight(4).Render(col)
		}
		cols = append(cols, col)
	}
	return r.th.Title.Render("Help — "+r.tabs[r.active].Title()) + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}
