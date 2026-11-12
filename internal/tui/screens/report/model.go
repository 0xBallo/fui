package report

import (
	"context"
	"strconv"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/theme"
)

// mode tracks which panel of the screen is active.
type mode int

const (
	modeReport mode = iota
	modeSaved
	modeParamsForm
	modeSaveForm
	modeExportForm
	modeConfirmRemove
)

// reportResult is the currently displayed report: its kind, a title label, the saved report name
// when it was run from the saved list ("" for a predefined, ad-hoc run), and its typed data.
type reportResult struct {
	kind  string
	label string
	saved string
	data  any
}

// Model is the analytics (reports) screen: it runs the predefined reports and saved reports of
// internal/app/reporting, lets the user change the period and other params, save the report
// shown, manage saved reports, and export the underlying transactions. Update and View never
// touch the store or the filesystem directly: every use case call and every file write runs
// inside a tea.Cmd.
type Model struct {
	uc            UseCases
	loc           domain.Locale
	monthStartDay int
	th            theme.Theme
	km            KeyMap

	width, height int
	mode          mode

	kindIdx int
	result  reportResult

	periodText  string
	accountText string
	loanText    string
	limitText   string
	horizonText string
	minBalText  string

	table components.Table
	chart components.BarChart
	note  string // extra context line under the table (loan name, forecast thresholds, ...)

	accounts []app.AccountBalance

	saved      []app.SavedReport
	savedTable components.Table

	form   components.Form
	fields formFields

	confirm components.ConfirmDialog
}

var _ screens.Screen = (*Model)(nil)

// formFields holds every Autocomplete the params, save and export forms can use; each form
// builds m.form over the subset it needs.
type formFields struct {
	period, account, loan, limit, horizon, minBalance components.Autocomplete
	name                                              components.Autocomplete
	path, format                                      components.Autocomplete
}

// New builds the report screen over uc, formatting amounts with loc and resolving relative
// periods ("last-month", "this-year", ...) against accounting months starting on
// monthStartDay (1..31).
func New(uc UseCases, loc domain.Locale, monthStartDay int, th theme.Theme) *Model {
	if monthStartDay == 0 {
		monthStartDay = 1
	}
	m := &Model{uc: uc, loc: loc, monthStartDay: monthStartDay, th: th, km: DefaultKeyMap()}
	m.table = components.NewTable(nil, th)
	m.chart = components.NewBarChart(th)
	m.savedTable = components.NewTable(savedColumns(), th)
	m.confirm = components.NewConfirmDialog(th)
	return m
}

// Init loads the account suggestions and runs the first predefined report.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.loadAccounts(), m.runCurrent())
}

// SetSize adapts the tables to the available terminal size.
func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	m.table.SetHeight(max(3, h-14))
	m.table.SetWidth(w)
	m.savedTable.SetHeight(max(3, h-10))
	m.savedTable.SetWidth(w)
	m.chart.SetWidth(w)
}

// Title identifies the screen in the tab bar.
func (m *Model) Title() string { return "Reports" }

// KeyMap exposes the bindings for the global help view.
func (m *Model) KeyMap() []key.Binding { return m.km.Bindings() }

// InputActive reports whether a form is capturing keystrokes.
func (m *Model) InputActive() bool {
	return m.mode == modeParamsForm || m.mode == modeSaveForm || m.mode == modeExportForm
}

// accountsLoadedMsg carries the accounts back into Update, for the account field's suggestions
// and to resolve a name to an ID for export.
type accountsLoadedMsg struct{ rows []app.AccountBalance }

func (m *Model) loadAccounts() tea.Cmd {
	return func() tea.Msg {
		rows, err := m.uc.ListAccounts(context.Background(), app.ListAccountsRequest{})
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return accountsLoadedMsg{rows: rows}
	}
}

// accountNames returns the account names, for the account field's autocomplete suggestions.
func (m *Model) accountNames() []string {
	names := make([]string, len(m.accounts))
	for i, a := range m.accounts {
		names[i] = a.Account.Name
	}
	return names
}

// reportLoadedMsg carries a freshly run report back into Update.
type reportLoadedMsg struct{ result reportResult }

// savedListLoadedMsg carries the saved report definitions back into Update.
type savedListLoadedMsg struct{ rows []app.SavedReport }

// savedRemovedMsg confirms a saved report was deleted.
type savedRemovedMsg struct{}

// exportDoneMsg confirms a transaction export finished.
type exportDoneMsg struct {
	path  string
	count int
}

// Update advances the screen state; it performs no I/O itself.
func (m *Model) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case accountsLoadedMsg:
		m.accounts = msg.rows
		return m, nil
	case reportLoadedMsg:
		m.result = msg.result
		m.refreshResultView()
		m.mode = modeReport
		return m, nil
	case savedListLoadedMsg:
		m.saved = msg.rows
		m.refreshSavedTable()
		m.mode = modeSaved
		return m, nil
	case savedRemovedMsg:
		return m, m.loadSaved()
	case exportDoneMsg:
		return m, func() tea.Msg {
			return screens.StatusMsg{Text: exportStatus(msg)}
		}
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func exportStatus(msg exportDoneMsg) string {
	if msg.count == 1 {
		return "Exported 1 transaction to " + msg.path
	}
	return "Exported " + strconv.Itoa(msg.count) + " transactions to " + msg.path
}

func (m *Model) handleKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch m.mode {
	case modeParamsForm, modeSaveForm, modeExportForm:
		return m.handleFormKey(msg)
	case modeConfirmRemove:
		return m.handleConfirmKey(msg)
	case modeSaved:
		return m.handleSavedKey(msg)
	case modeReport:
		return m.handleReportKey(msg)
	}
	return m.handleReportKey(msg)
}

func (m *Model) handleReportKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch {
	case key.Matches(msg, m.km.Up):
		m.table.MoveUp(1)
	case key.Matches(msg, m.km.Down):
		m.table.MoveDown(1)
	case key.Matches(msg, m.km.PrevKind):
		return m, m.cycleKind(-1)
	case key.Matches(msg, m.km.NextKind):
		return m, m.cycleKind(1)
	case key.Matches(msg, m.km.Refresh):
		return m, m.runCurrent()
	case key.Matches(msg, m.km.Params):
		m.openParamsForm()
	case key.Matches(msg, m.km.Save):
		m.openSaveForm()
	case key.Matches(msg, m.km.Saved):
		return m, m.loadSaved()
	case key.Matches(msg, m.km.Export):
		m.openExportForm()
	}
	return m, nil
}

// cycleKind moves to the next or previous predefined report and runs it, leaving period and
// other params untouched; it clears the saved-report origin of the previous result.
func (m *Model) cycleKind(delta int) tea.Cmd {
	n := len(predefinedKinds())
	m.kindIdx = (m.kindIdx + delta + n) % n
	return m.runCurrent()
}
