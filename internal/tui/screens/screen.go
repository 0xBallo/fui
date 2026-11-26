// Package screens defines top-level view contracts and router coordination for the TUI.
package screens

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
)

// ID designates a top-level screen routed by numeric keys.
type ID int

const (
	// TabDashboard is the default summary view (key 1).
	TabDashboard ID = iota
	// TabTransactions is the transaction register (key 2).
	TabTransactions
	// TabAccounts lists accounts and ledger movements (key 3).
	TabAccounts
	// TabBudget represents the envelope/ceiling planning view (v0.2).
	TabBudget
	// TabReports represents analytics and trends (v0.2).
	TabReports
	// TabImport represents bank file parsing and reconciliation (v0.2).
	TabImport
	// TabSettings represents category and rule management (v0.2).
	TabSettings
)

// Screen defines the standard lifecycle, input handling, and rendering interface for TUI views.
type Screen interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (Screen, tea.Cmd)
	View() string
	KeyMap() []key.Binding
	Title() string
	SetSize(width, height int)
	// InputActive reports whether a text field is currently capturing every keystroke (a form,
	// search box, or filter), so the root router must forward keys straight through instead of
	// intercepting its own global bindings (quit, help, tab switch).
	InputActive() bool
}

// Targeted is implemented by the result message of an async command so the root router can
// deliver it to the screen that started it, even when the user has switched to another tab since.
// Messages without it (keys, window size) go to the active screen only.
type Targeted interface {
	ScreenID() ID
}

// ErrorMsg propagates an operational failure to be surfaced on the global status bar.
type ErrorMsg struct {
	Err error
}

// StatusMsg conveys an informational message to the global status bar.
type StatusMsg struct {
	Text string
}

// SwitchScreenMsg asks the root router to navigate to another tab. A non-nil Filter is handed to
// the target screen when it implements FilterReceiver and ignored otherwise.
type SwitchScreenMsg struct {
	Target ID
	Filter *app.ListTxRequest
}

// FilterReceiver is implemented by screens that can be opened already narrowed to a transaction
// filter (for example a drill-down from a report). ApplyFilter replaces any filter the screen
// holds and returns the command that reloads it, which also serves as the screen's Init.
type FilterReceiver interface {
	ApplyFilter(req app.ListTxRequest) tea.Cmd
}
