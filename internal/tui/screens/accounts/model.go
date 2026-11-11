package accounts

import (
	"context"
	"fmt"
	"strconv"

	"github.com/charmbracelet/bubbles/key"
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
	modeList mode = iota
	modeDetail
	modeForm
	modeConfirm
)

// Model is the accounts screen: a list of balances, the movement detail of one account, and the
// forms to add or archive an account. Update and View never touch the store; every use case call
// runs inside a tea.Cmd.
type Model struct {
	uc  UseCases
	loc domain.Locale
	th  theme.Theme
	km  KeyMap

	width, height int
	mode          mode

	balances []app.AccountBalance
	table    components.Table

	detail    app.AccountBalance
	detailTbl components.Table

	form      components.Form
	nameField components.Autocomplete
	kindField components.Autocomplete
	currField components.Autocomplete
	openField components.Autocomplete
	dateField components.Autocomplete

	confirm       components.ConfirmDialog
	archiveTarget string
}

var _ screens.Screen = (*Model)(nil)

// New builds the accounts screen over uc, formatting amounts and dates with loc.
func New(uc UseCases, loc domain.Locale, th theme.Theme) *Model {
	m := &Model{uc: uc, loc: loc, th: th, km: DefaultKeyMap()}
	m.table = components.NewTable([]components.Column{
		{Title: "Account", Width: 24, Align: lipgloss.Left},
		{Title: "Kind", Width: 12, Align: lipgloss.Left},
		{Title: "Balance", Width: 16, Align: lipgloss.Right},
	}, th)
	m.detailTbl = components.NewTable([]components.Column{
		{Title: "Date", Width: 10, Align: lipgloss.Left},
		{Title: "Description", Width: 28, Align: lipgloss.Left},
		{Title: "Amount", Width: 14, Align: lipgloss.Right},
	}, th)
	m.confirm = components.NewConfirmDialog(th)
	return m
}

// Init loads the account balances.
func (m *Model) Init() tea.Cmd { return m.load() }

// SetSize adapts the tables to the available terminal size.
func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	rows := max(3, h-8)
	m.table.SetHeight(rows)
	m.table.SetWidth(w)
	m.detailTbl.SetHeight(rows)
	m.detailTbl.SetWidth(w)
}

// Title identifies the screen in the tab bar.
func (m *Model) Title() string { return "Accounts" }

// KeyMap exposes the bindings for the global help view.
func (m *Model) KeyMap() []key.Binding { return m.km.Bindings() }

// InputActive reports whether the add-account form is capturing keystrokes.
func (m *Model) InputActive() bool { return m.mode == modeForm }

// loadedMsg carries a fresh account listing back into Update.
type loadedMsg struct{ balances []app.AccountBalance }

func (m *Model) load() tea.Cmd {
	return func() tea.Msg {
		bals, err := m.uc.ListAccounts(context.Background(), app.ListAccountsRequest{})
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return loadedMsg{balances: bals}
	}
}

// detailLoadedMsg carries one account and its movements back into Update.
type detailLoadedMsg struct {
	bal  app.AccountBalance
	page app.TxPage
}

func (m *Model) loadDetail(name string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		bal, err := m.uc.ShowAccount(ctx, name, "")
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		page, err := m.uc.ListTx(ctx, app.ListTxRequest{Accounts: []string{name}})
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return detailLoadedMsg{bal: bal, page: page}
	}
}

// Update advances the screen state; it performs no I/O itself.
func (m *Model) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case loadedMsg:
		m.balances = msg.balances
		m.table.SetRows(m.accountRows())
		m.mode = modeList
		return m, nil
	case detailLoadedMsg:
		m.detail = msg.bal
		m.detailTbl.SetRows(m.txRows(msg.page.Items))
		m.mode = modeDetail
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch m.mode {
	case modeForm:
		return m.handleFormKey(msg)
	case modeConfirm:
		return m.handleConfirmKey(msg)
	case modeDetail:
		return m.handleDetailKey(msg)
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
	case key.Matches(msg, m.km.Open):
		row := m.table.SelectedRow()
		if row == nil {
			return m, nil
		}
		return m, m.loadDetail(row.ID)
	case key.Matches(msg, m.km.Add):
		m.openAddForm()
	case key.Matches(msg, m.km.Archive):
		row := m.table.SelectedRow()
		if row == nil {
			return m, nil
		}
		m.archiveTarget = row.ID
		m.confirm.Show("Archive account", fmt.Sprintf("Archive %q?", row.ID))
		m.mode = modeConfirm
	}
	return m, nil
}

func (m *Model) handleDetailKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch {
	case key.Matches(msg, m.km.Back):
		m.mode = modeList
	case key.Matches(msg, m.km.Up):
		m.detailTbl.MoveUp(1)
	case key.Matches(msg, m.km.Down):
		m.detailTbl.MoveDown(1)
	}
	return m, nil
}

func (m *Model) handleConfirmKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	yes, no := m.confirm.HandleKey(msg)
	switch {
	case yes:
		return m, m.submitArchive()
	case no:
		m.mode = modeList
	}
	return m, nil
}

func (m *Model) submitArchive() tea.Cmd {
	ref := m.archiveTarget
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := m.uc.PlanArchiveAccount(ctx, ref)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if _, err := m.uc.ApplyArchiveAccount(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		bals, err := m.uc.ListAccounts(ctx, app.ListAccountsRequest{})
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return loadedMsg{balances: bals}
	}
}

func (m *Model) openAddForm() {
	m.nameField = components.NewAutocomplete("Name:", "e.g. Main checking", m.th)
	m.kindField = components.NewAutocomplete("Kind:", "cash/checking/credit_card/savings/investment/loan", m.th)
	m.kindField.SetSuggestions([]string{
		string(domain.AccountCash), string(domain.AccountChecking), string(domain.AccountCreditCard),
		string(domain.AccountSavings), string(domain.AccountInvestment), string(domain.AccountLoan),
	})
	m.currField = components.NewAutocomplete("Currency:", "EUR", m.th)
	m.openField = components.NewAutocomplete("Opening balance:", "0", m.th)
	m.dateField = components.NewAutocomplete("Opened on:", "today", m.th)
	m.form = components.NewForm(&m.nameField, &m.kindField, &m.currField, &m.openField, &m.dateField)
	m.mode = modeForm
	m.form.Focus()
}

func (m *Model) handleFormKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		return m, nil
	case "shift+tab":
		m.form.Prev()
		return m, nil
	case "enter":
		if m.form.FocusedOpen() {
			m.form.Update(msg)
			return m, nil
		}
		if !m.form.AtLast() {
			m.form.Next()
			return m, nil
		}
		return m, m.submitAdd()
	}
	m.form.Update(msg)
	return m, nil
}

func (m *Model) submitAdd() tea.Cmd {
	req := app.AddAccountRequest{
		Name: m.nameField.Value(), Kind: m.kindField.Value(), Currency: m.currField.Value(),
		OpeningBalance: m.openField.Value(), OpenedOn: m.dateField.Value(),
	}
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := m.uc.PlanAddAccount(ctx, req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if _, err := m.uc.ApplyAddAccount(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		bals, err := m.uc.ListAccounts(ctx, app.ListAccountsRequest{})
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return loadedMsg{balances: bals}
	}
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

func (m *Model) txRows(items []app.TxView) []components.Row {
	rows := make([]components.Row, len(items))
	for i, v := range items {
		rows[i] = components.Row{ID: strconv.FormatInt(int64(v.Transaction.ID), 10), Cells: []string{
			v.Transaction.Date.Format(m.loc), v.Transaction.Description, domain.FormatAmount(v.Transaction.Amount, m.loc),
		}}
	}
	return rows
}

// View renders the active panel.
func (m *Model) View() string {
	switch m.mode {
	case modeForm:
		return m.viewForm()
	case modeConfirm:
		return m.viewList() + "\n\n" + m.confirm.View()
	case modeDetail:
		return m.viewDetail()
	case modeList:
		return m.viewList()
	}
	return m.viewList()
}

func (m *Model) viewList() string {
	return m.th.Title.Render("Accounts") + "\n\n" + m.table.View()
}

func (m *Model) viewDetail() string {
	title := m.th.Title.Render("Movements — " + m.detail.Account.Name)
	bal := m.th.Base.Render("Balance: " + domain.FormatAmount(m.detail.Balance, m.loc))
	return title + "\n" + bal + "\n\n" + m.detailTbl.View()
}

func (m *Model) viewForm() string {
	return m.th.Title.Render("Add account") + "\n\n" + m.form.View()
}
