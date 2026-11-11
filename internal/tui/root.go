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

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/accounts"
	"github.com/0xballo/fui/internal/tui/screens/dashboard"
	"github.com/0xballo/fui/internal/tui/screens/transactions"
	"github.com/0xballo/fui/internal/tui/theme"
)

// minWidth and minHeight are the smallest terminal size the TUI supports; below them Root shows
// only a warning.
const (
	minWidth  = 80
	minHeight = 24
)

// reservedRows is the vertical space Root reserves for the tab bar and the status bar, leaving
// the rest to the active screen.
const reservedRows = 4

// UseCases is the union of every MVP screen's needs: the app layer (shared with the CLI) plus the
// reporting layer the dashboard uses for spending and runway.
type UseCases interface {
	accounts.UseCases
	transactions.UseCases
	dashboard.UseCases
}

// Config holds the user-facing settings Root applies: the locale used to format amounts and
// dates, the theme name ("default" or "high-contrast"), and the first day of the accounting month
// (1..31, see domain.MonthPeriod) used by the dashboard's spending and runway reports.
type Config struct {
	Locale        domain.Locale
	Theme         string
	MonthStartDay int
}

// Root is the top-level Bubble Tea model: it routes between the MVP screens (keys 1/2/3),
// surfaces errors and status messages on a status bar, and renders a help overlay from every
// screen's keymap.
type Root struct {
	loc domain.Locale
	th  theme.Theme
	km  KeyMap

	tabs   [3]screens.Screen
	active screens.ID

	width, height int
	status        string
	err           error
	showHelp      bool
}

var _ tea.Model = (*Root)(nil)

// New builds Root over uc, the MVP screens, and cfg. Locale defaults to en-US and Theme to
// "default" when unset.
func New(uc UseCases, cfg Config) *Root {
	loc := cfg.Locale
	if loc == "" {
		loc = domain.LocaleENUS
	}
	th := theme.ByName(cfg.Theme)
	r := &Root{loc: loc, th: th, km: DefaultKeyMap()}
	r.tabs[screens.TabDashboard] = dashboard.New(uc, loc, th, cfg.MonthStartDay)
	r.tabs[screens.TabTransactions] = transactions.New(uc, loc, th)
	r.tabs[screens.TabAccounts] = accounts.New(uc, loc, th)
	return r
}

// Run starts the program over uc and cfg and blocks until the user quits or ctx is cancelled.
func Run(ctx context.Context, uc UseCases, cfg Config) error {
	if _, err := tea.NewProgram(New(uc, cfg), tea.WithContext(ctx)).Run(); err != nil {
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
		h := max(1, r.height-reservedRows)
		for _, scr := range r.tabs {
			scr.SetSize(r.width, h)
		}
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
		}
	}
	return r.forward(msg)
}

// switchTo activates target (ignoring the v0.2 tabs, which have no screen yet) and reloads its
// data, since screens never keep optimistic state between visits.
func (r *Root) switchTo(target screens.ID) (tea.Model, tea.Cmd) {
	if target < 0 || int(target) >= len(r.tabs) {
		r.status = "that screen is not available yet"
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
	if r.showHelp {
		body = r.viewHelp()
	}
	return r.viewTabs() + "\n\n" + body + "\n\n" + r.viewStatus()
}

// tabNames are the seven screens in tab order; indexes beyond len(r.tabs) have no Screen yet
// (v0.2) and are rendered disabled.
var tabNames = [...]string{"Dashboard", "Transactions", "Accounts", "Budget", "Reports", "Import", "Settings"}

func (r *Root) viewTabs() string {
	parts := make([]string, len(tabNames))
	for i, name := range tabNames {
		label := fmt.Sprintf("%d %s", i+1, name)
		switch {
		case i >= len(r.tabs):
			parts[i] = r.th.DisabledTab.Render(label)
		case screens.ID(i) == r.active:
			parts[i] = r.th.ActiveTab.Render(label)
		default:
			parts[i] = r.th.InactiveTab.Render(label)
		}
	}
	return strings.Join(parts, "  ")
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

func (r *Root) viewHelp() string {
	bindings := append(append([]key.Binding{}, r.km.Bindings()...), r.ActiveKeyMap()...)
	lines := make([]string, 0, len(bindings)+1)
	lines = append(lines, r.th.Title.Render("Help — "+r.tabs[r.active].Title()))
	for _, b := range bindings {
		h := b.Help()
		if h.Key == "" {
			continue
		}
		lines = append(lines, r.th.HelpKey.Render(h.Key)+"  "+r.th.HelpDesc.Render(h.Desc))
	}
	return strings.Join(lines, "\n")
}
