package lists

import (
	"context"
	"strconv"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/recurring"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/theme"
)

// recurrencesMode tracks which panel of the recurrences sub-view is active.
type recurrencesMode int

const (
	recurrencesListMode recurrencesMode = iota
	recurrencesForm
	recurrencesConfirm
)

// Recurrences is the settings sub-view that lists recurring transaction templates and lets the
// user add, edit and remove them. Requires the recurring module to be enabled.
type Recurrences struct {
	uc      RecurrencesUseCases
	enabled module.Set
	loc     domain.Locale
	th      theme.Theme
	km      KeyMap

	width, height int
	mode          recurrencesMode

	recs       []app.Recurrence
	accounts   []domain.Account
	categories []domain.Category
	table      components.Table

	form    components.Form
	fields  recurrenceFormFields
	editing *app.RecurrenceID // nil when adding

	confirm components.ConfirmDialog
	pending recurring.RecurrencePlan

	err error // last submit error, shown inline until the next state change
}

// recurrenceFormFields groups the add/edit form's autocomplete fields.
type recurrenceFormFields struct {
	account, category, description components.Autocomplete
	amount, every, unit            components.Autocomplete
	dayOfMonth, start, end         components.Autocomplete
}

var _ screens.Screen = (*Recurrences)(nil)

// NewRecurrences builds the recurrences sub-view over uc; enabled gates every use case on the
// recurring module.
func NewRecurrences(uc RecurrencesUseCases, enabled module.Set, loc domain.Locale, th theme.Theme) *Recurrences {
	m := &Recurrences{uc: uc, enabled: enabled, loc: loc, th: th, km: DefaultKeyMap()}
	m.table = components.NewTable([]components.Column{
		{Title: "Account", Width: 16, Align: lipgloss.Left},
		{Title: "Description", Width: 24, Align: lipgloss.Left},
		{Title: "Amount", Width: 12, Align: lipgloss.Right},
		{Title: "Cadence", Width: 12, Align: lipgloss.Left},
		{Title: "Next", Width: 10, Align: lipgloss.Left},
	}, th)
	m.confirm = components.NewConfirmDialog(th)
	return m
}

// Init loads the recurrences and the reference lists used for suggestions.
func (m *Recurrences) Init() tea.Cmd { return m.load() }

// SetSize adapts the table to the available terminal size.
func (m *Recurrences) SetSize(w, h int) {
	m.width, m.height = w, h
	m.table.SetHeight(max(3, h-8))
	m.table.SetWidth(w)
}

// Title identifies the sub-view in the settings shell.
func (m *Recurrences) Title() string { return "Recurrences" }

// KeyMap exposes the bindings for the global help view.
func (m *Recurrences) KeyMap() []key.Binding { return m.km.Bindings() }

// InputActive reports whether the add/edit form is capturing keystrokes.
func (m *Recurrences) InputActive() bool { return m.mode == recurrencesForm }

type recurrencesLoadedMsg struct {
	recs       []app.Recurrence
	accounts   []domain.Account
	categories []domain.Category
}

func (m *Recurrences) load() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		recs, err := m.uc.List(ctx, m.enabled)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		accts, err := m.uc.ListAccounts(ctx)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		cats, err := m.uc.ListCategories(ctx)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return recurrencesLoadedMsg{recs: recs, accounts: accts, categories: cats}
	}
}

// Update advances the sub-view state; it performs no I/O itself.
func (m *Recurrences) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case recurrencesLoadedMsg:
		m.err = nil
		m.recs, m.accounts, m.categories = msg.recs, msg.accounts, msg.categories
		m.table.SetRows(m.rows())
		m.mode = recurrencesListMode
		return m, nil
	case recurrenceRemovePlannedMsg:
		m.err = nil
		m.pending = msg.plan
		m.confirm.Show("Delete recurrence", "Delete this recurrence?")
		m.mode = recurrencesConfirm
		return m, nil
	case screens.ErrorMsg:
		m.err = msg.Err
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Recurrences) handleKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch m.mode {
	case recurrencesForm:
		return m.handleFormKey(msg)
	case recurrencesConfirm:
		return m.handleConfirmKey(msg)
	case recurrencesListMode:
		return m.handleListKey(msg)
	}
	return m.handleListKey(msg)
}

func (m *Recurrences) handleListKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch {
	case key.Matches(msg, m.km.Up):
		m.table.MoveUp(1)
	case key.Matches(msg, m.km.Down):
		m.table.MoveDown(1)
	case key.Matches(msg, m.km.Add):
		m.openForm(nil)
	case key.Matches(msg, m.km.Edit):
		if row := m.table.SelectedRow(); row != nil {
			m.openForm(m.recByRowID(row.ID))
		}
	case key.Matches(msg, m.km.Remove):
		if row := m.table.SelectedRow(); row != nil {
			if rec := m.recByRowID(row.ID); rec != nil {
				return m, m.planRemove(rec.ID)
			}
		}
	}
	return m, nil
}

func (m *Recurrences) handleConfirmKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	yes, no := m.confirm.HandleKey(msg)
	switch {
	case yes:
		return m, m.submitRemove()
	case no:
		m.mode = recurrencesListMode
	}
	return m, nil
}

func (m *Recurrences) recByRowID(rowID string) *app.Recurrence {
	id, err := strconv.ParseInt(rowID, 10, 64)
	if err != nil {
		return nil
	}
	for i := range m.recs {
		if m.recs[i].ID == app.RecurrenceID(id) {
			return &m.recs[i]
		}
	}
	return nil
}

func (m *Recurrences) rows() []components.Row {
	rows := make([]components.Row, len(m.recs))
	for i, r := range m.recs {
		rows[i] = components.Row{ID: strconv.FormatInt(int64(r.ID), 10), Cells: []string{
			accountNameByID(r.AccountID, m.accounts), r.Description, domain.FormatAmount(r.Amount, m.loc),
			strconv.Itoa(r.Rule.Every) + " " + string(r.Rule.Unit), r.NextDate.String(),
		}}
	}
	return rows
}

// View renders the active panel.
func (m *Recurrences) View() string {
	var body string
	switch m.mode {
	case recurrencesForm:
		body = m.viewForm()
	case recurrencesConfirm:
		body = m.viewList() + "\n\n" + m.confirm.View()
	case recurrencesListMode:
		body = m.viewList()
	default:
		body = m.viewList()
	}
	return withError(m.th, body, m.err)
}

func (m *Recurrences) viewList() string {
	return m.th.Title.Render("Recurrences") + "\n\n" + m.table.View()
}

type recurrenceRemovePlannedMsg struct{ plan recurring.RecurrencePlan }

func (m *Recurrences) planRemove(id app.RecurrenceID) tea.Cmd {
	return func() tea.Msg {
		plan, err := m.uc.PlanRemove(context.Background(), m.enabled, id)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return recurrenceRemovePlannedMsg{plan: plan}
	}
}

func (m *Recurrences) submitRemove() tea.Cmd {
	plan := m.pending
	return func() tea.Msg {
		ctx := context.Background()
		if err := m.uc.ApplyRemove(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.reload(ctx)
	}
}

func (m *Recurrences) reload(ctx context.Context) tea.Msg {
	recs, err := m.uc.List(ctx, m.enabled)
	if err != nil {
		return screens.ErrorMsg{Err: err}
	}
	return recurrencesLoadedMsg{recs: recs, accounts: m.accounts, categories: m.categories}
}

// accountNameByID returns the name of id among accts, or "" when not found.
func accountNameByID(id domain.AccountID, accts []domain.Account) string {
	for _, a := range accts {
		if a.ID == id {
			return a.Name
		}
	}
	return ""
}
