package budget

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app/budgeting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/theme"
)

// envelopeMode is a short alias used throughout the screen to compare against status.Mode.
const envelopeMode = domain.BudgetEnvelope

// mode tracks which panel of the screen is active.
type mode int

const (
	modeList mode = iota
	modeSetForm
	modeAssignForm
	modeMoveForm
	modeConfirm
)

// Model is the budget screen: a table of every cap or envelope evaluated for one month, plus the
// forms to set an amount, assign from the unassigned pool, move between envelopes, and the
// confirmation to copy the previous month. Update and View never touch the store: every use case
// call runs inside a tea.Cmd.
type Model struct {
	uc  UseCases
	loc domain.Locale
	th  theme.Theme
	km  KeyMap

	width, height int
	mode          mode

	status  budgeting.Status
	monthIn string // "YYYY-MM" of the month currently shown; "" only before the first load

	table components.Table

	target      string // selected budget name for the active form
	amountField components.Autocomplete
	toField     components.Autocomplete
	form        components.Form

	confirm components.ConfirmDialog
}

var _ screens.Screen = (*Model)(nil)

// New builds the budget screen over uc, formatting amounts with loc.
func New(uc UseCases, loc domain.Locale, th theme.Theme) *Model {
	m := &Model{uc: uc, loc: loc, th: th, km: DefaultKeyMap()}
	m.table = components.NewTable(capColumns(), th)
	m.confirm = components.NewConfirmDialog(th)
	return m
}

// Init loads the status of the current month.
func (m *Model) Init() tea.Cmd { return m.load("") }

// SetSize adapts the table to the available terminal size.
func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	m.table.SetHeight(max(3, h-10))
	m.table.SetWidth(w)
}

// Title identifies the screen in the tab bar.
func (m *Model) Title() string { return "Budget" }

// KeyMap exposes the bindings for the global help view.
func (m *Model) KeyMap() []key.Binding { return m.km.Bindings() }

// InputActive reports whether a form is capturing keystrokes.
func (m *Model) InputActive() bool {
	return m.mode == modeSetForm || m.mode == modeAssignForm || m.mode == modeMoveForm
}

// statusLoadedMsg carries a fresh month status back into Update.
type statusLoadedMsg struct{ st budgeting.Status }

func (m *Model) load(month string) tea.Cmd {
	return func() tea.Msg {
		st, err := m.uc.Status(context.Background(), month)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return statusLoadedMsg{st: st}
	}
}

// monthKey returns the "YYYY-MM" key of the calendar month p starts in.
func monthKey(p domain.Period) string {
	return fmt.Sprintf("%04d-%02d", p.From().Year(), p.From().Month())
}

// shiftMonth returns the "YYYY-MM" key of the month delta calendar months from p's start.
func shiftMonth(p domain.Period, delta int) (string, error) {
	d, err := p.From().AddMonths(delta)
	if err != nil {
		return "", fmt.Errorf("shift month: %w", err)
	}
	return fmt.Sprintf("%04d-%02d", d.Year(), d.Month()), nil
}

// Update advances the screen state; it performs no I/O itself.
func (m *Model) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case statusLoadedMsg:
		m.status = msg.st
		m.monthIn = monthKey(msg.st.Period)
		m.refreshTable()
		m.mode = modeList
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch m.mode {
	case modeSetForm, modeAssignForm, modeMoveForm:
		return m.handleFormKey(msg)
	case modeConfirm:
		return m.handleConfirmKey(msg)
	case modeList:
		return m.handleListKey(msg)
	}
	return m.handleListKey(msg)
}

func (m *Model) handleListKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch {
	case key.Matches(msg, m.km.Up):
		m.table.MoveUp(1)
	case key.Matches(msg, m.km.Down):
		m.table.MoveDown(1)
	case key.Matches(msg, m.km.PrevMonth):
		return m, m.navigate(-1)
	case key.Matches(msg, m.km.NextMonth):
		return m, m.navigate(1)
	case key.Matches(msg, m.km.SetAmount):
		m.openSetForm()
	case key.Matches(msg, m.km.Assign):
		m.openAssignForm()
	case key.Matches(msg, m.km.Move):
		m.openMoveForm()
	case key.Matches(msg, m.km.CopyPrev):
		m.openCopyConfirm()
	}
	return m, nil
}

// navigate loads the month delta calendar months from the one currently shown.
func (m *Model) navigate(delta int) tea.Cmd {
	month, err := shiftMonth(m.status.Period, delta)
	if err != nil {
		return func() tea.Msg { return screens.ErrorMsg{Err: err} }
	}
	return m.load(month)
}

func (m *Model) handleConfirmKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	yes, no := m.confirm.HandleKey(msg)
	switch {
	case yes:
		return m, m.submitCopy()
	case no:
		m.mode = modeList
	}
	return m, nil
}

func (m *Model) openCopyConfirm() {
	m.confirm.Show("Copy previous month", fmt.Sprintf("Copy last month's budgets into %s?", m.monthIn))
	m.mode = modeConfirm
}

func (m *Model) submitCopy() tea.Cmd {
	month := m.monthIn
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := m.uc.PlanCopyPrevious(ctx, month)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if err := m.uc.ApplyCopyPrevious(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		st, err := m.uc.Status(ctx, month)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return statusLoadedMsg{st: st}
	}
}
