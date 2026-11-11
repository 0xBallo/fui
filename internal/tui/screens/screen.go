// Package screens defines top-level view contracts and router coordination for the TUI.
package screens

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
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
}

// ErrorMsg propagates an operational failure to be surfaced on the global status bar.
type ErrorMsg struct {
	Err error
}

// StatusMsg conveys an informational message to the global status bar.
type StatusMsg struct {
	Text string
}

// SwitchScreenMsg asks the root router to navigate to another tab.
type SwitchScreenMsg struct {
	Target ID
}
