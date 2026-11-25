package accounts

import (
	"context"
	"fmt"
	"strconv"
	"strings"

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
	modeReconcileForm
	modeReconcilePreview
	modeConfirm
	modePriceForm
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
	loans    components.Table
	loanOn   bool // focus is on the loans table
	totals   totals

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

	reconcileAccount      string
	reconcileBalanceField components.Autocomplete
	reconcileDateField    components.Autocomplete
	reconcileForm         components.Form
	reconcilePlan         app.ReconcilePlan
	reconcileNotice       string
	reconcileBalance      string
	reconcileDate         string
	reconcileCands        []domain.Transaction
	reconcileCursor       int
	reconcileBackMode     mode

	positionsUC PositionsUseCases
	positions   positionState
}

var _ screens.Screen = (*Model)(nil)

// New builds the accounts screen over uc, formatting amounts and dates with loc.
func New(uc UseCases, loc domain.Locale, th theme.Theme) *Model {
	m := &Model{uc: uc, loc: loc, th: th, km: DefaultKeyMap()}
	cols := []components.Column{
		{Title: "Account", Width: 24, Align: lipgloss.Left},
		{Title: "Kind", Width: 12, Align: lipgloss.Left},
		{Title: "Currency", Width: 8, Align: lipgloss.Left},
		{Title: "Balance", Width: 16, Align: lipgloss.Right},
	}
	m.table = components.NewTable(cols, th)
	m.loans = components.NewTable(cols, th)
	m.detailTbl = components.NewTable([]components.Column{
		{Title: "Date", Width: 10, Align: lipgloss.Left},
		{Title: "Description", Width: 28, Align: lipgloss.Left},
		{Title: "Amount", Width: 14, Align: lipgloss.Right},
	}, th)
	m.confirm = components.NewConfirmDialog(th)
	m.initPositions()
	return m
}

// layoutLists sizes the two tables, reserving room for the loans section and both total lines.
func (m *Model) layoutLists() {
	avail := max(5, m.height-6)
	n := len(m.loans.Rows())
	if n == 0 {
		m.table.SetHeight(avail - 1)
		return
	}
	loanH := min(n+3, 6)
	m.loans.SetHeight(loanH)
	m.table.SetHeight(max(4, avail-loanH-3))
}

// Init loads the account balances.
func (m *Model) Init() tea.Cmd { return m.load() }

// SetSize adapts the tables to the available terminal size.
func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	rows := max(3, h-8)
	m.layoutLists()
	m.table.SetWidth(w)
	m.loans.SetWidth(w)
	m.detailTbl.SetHeight(rows)
	m.detailTbl.SetWidth(w)
	m.resizePositions(w, h)
}

// Title identifies the screen in the tab bar.
func (m *Model) Title() string { return "Accounts" }

// KeyMap exposes the bindings for the global help view.
func (m *Model) KeyMap() []key.Binding { return m.keyBindings() }

// InputActive reports whether the add-account form is capturing keystrokes.
func (m *Model) InputActive() bool {
	return m.mode == modeForm || m.mode == modeReconcileForm || m.mode == modePriceForm
}

// loadedMsg carries a fresh account listing back into Update.
type loadedMsg struct {
	balances []app.AccountBalance
	netWorth *app.NetWorth // nil when the base currency or a rate is missing
}

// fetch lists the accounts and, best effort, their base-currency values for the section totals.
func (m *Model) fetch(ctx context.Context) tea.Msg {
	bals, err := m.uc.ListAccounts(ctx, app.ListAccountsRequest{})
	if err != nil {
		return screens.ErrorMsg{Err: err}
	}
	msg := loadedMsg{balances: bals}
	if nw, err := m.uc.NetWorth(ctx, app.NetWorthRequest{}); err == nil {
		msg.netWorth = &nw
	}
	return msg
}

func (m *Model) load() tea.Cmd {
	return func() tea.Msg { return m.fetch(context.Background()) }
}

// applyLoaded stores a fresh listing in both tables and recomputes the section totals.
func (m *Model) applyLoaded(msg loadedMsg) {
	m.balances = msg.balances
	m.totals = sectionTotals(msg.netWorth, m.loc)
	wealth, loans := m.accountRows()
	m.table.SetRows(wealth)
	m.loans.SetRows(loans)
	if len(loans) == 0 {
		m.loanOn = false
	}
	m.layoutLists()
	m.mode = modeList
}

// detailLoadedMsg carries one account and its movements back into Update.
type detailLoadedMsg struct {
	bal       app.AccountBalance
	page      app.TxPage
	positions []positionViewRow
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
		positions, err := m.loadDetailPositions(ctx, bal.Account)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return detailLoadedMsg{bal: bal, page: page, positions: positions}
	}
}

// Update advances the screen state; it performs no I/O itself.
func (m *Model) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case loadedMsg:
		m.applyLoaded(msg)
		return m, nil
	case detailLoadedMsg:
		m.detail = msg.bal
		m.detailTbl.SetRows(m.txRows(msg.page.Items))
		m.setPositions(msg.positions)
		m.mode = modeDetail
		return m, nil
	case reconcilePlannedMsg:
		m.setReconcilePlan(msg)
		return m, nil
	case reconcileSavedMsg:
		m.reconcileNotice = msg.notice
		return m, nil
	case reconcileAppliedMsg:
		m.applyLoaded(msg.loaded)
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
	case modeReconcileForm:
		return m.handleReconcileFormKey(msg)
	case modeReconcilePreview:
		return m.handleReconcilePreviewKey(msg)
	case modeConfirm:
		return m.handleConfirmKey(msg)
	case modePriceForm:
		return m.handlePriceFormKey(msg)
	case modeDetail:
		return m.handleDetailKey(msg)
	case modeList:
		return m.handleListKey(msg)
	}
	return m.handleListKey(msg)
}

func (m *Model) handleListKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch {
	case key.Matches(msg, m.km.Switch):
		m.loanOn = !m.loanOn && len(m.loans.Rows()) > 0
	case key.Matches(msg, m.km.Up):
		m.focused().MoveUp(1)
	case key.Matches(msg, m.km.Down):
		m.focused().MoveDown(1)
	case key.Matches(msg, m.km.Open):
		row := m.focused().SelectedRow()
		if row == nil {
			return m, nil
		}
		return m, m.loadDetail(row.ID)
	case key.Matches(msg, m.km.Add):
		m.openAddForm()
	case key.Matches(msg, m.km.Archive):
		row := m.focused().SelectedRow()
		if row == nil {
			return m, nil
		}
		m.archiveTarget = row.ID
		m.confirm.Show("Archive account", fmt.Sprintf("Archive %q?", row.ID))
		m.mode = modeConfirm
	case key.Matches(msg, m.km.Reconcile):
		row := m.focused().SelectedRow()
		if row == nil {
			return m, nil
		}
		m.openReconcileForm(row.ID, modeList)
	}
	return m, nil
}

func (m *Model) focused() *components.Table {
	if m.loanOn {
		return &m.loans
	}
	return &m.table
}

func (m *Model) handleDetailKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch {
	case key.Matches(msg, m.km.Back):
		m.mode = modeList
	case key.Matches(msg, m.km.Up):
		m.detailTbl.MoveUp(1)
		m.positions.table.MoveUp(1)
	case key.Matches(msg, m.km.Down):
		m.detailTbl.MoveDown(1)
		m.positions.table.MoveDown(1)
	case key.Matches(msg, m.km.Reconcile):
		m.openReconcileForm(m.detail.Account.Name, modeDetail)
	case key.Matches(msg, m.km.Price):
		m.openPriceForm()
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
		return m.fetch(ctx)
	}
}

func (m *Model) openReconcileForm(account string, back mode) {
	m.reconcileAccount = account
	m.reconcileBackMode = back
	m.reconcileNotice = ""

	m.reconcileBalanceField = components.NewAutocomplete("Statement balance:", "e.g. 1234.56", m.th)
	m.reconcileDateField = components.NewAutocomplete("Statement date:", "today", m.th)
	m.reconcileForm = components.NewForm(&m.reconcileBalanceField, &m.reconcileDateField)
	m.mode = modeReconcileForm
	m.reconcileForm.Focus()
}

func (m *Model) handleReconcileFormKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = m.reconcileBackMode
		return m, nil
	case "shift+tab":
		m.reconcileForm.Prev()
		return m, nil
	case "enter":
		if m.reconcileForm.FocusedOpen() {
			m.reconcileForm.Update(msg)
			return m, nil
		}
		if !m.reconcileForm.AtLast() {
			m.reconcileForm.Next()
			return m, nil
		}
		return m, m.submitReconcilePlan()
	}
	m.reconcileForm.Update(msg)
	return m, nil
}

func (m *Model) submitReconcilePlan() tea.Cmd {
	balance := strings.TrimSpace(m.reconcileBalanceField.Value())
	if balance == "" {
		m.reconcileNotice = "Statement balance is required."
		return nil
	}
	m.reconcileNotice = ""
	m.reconcileBalance = balance
	m.reconcileDate = strings.TrimSpace(m.reconcileDateField.Value())
	m.reconcileCands = nil
	m.reconcileCursor = 0
	return m.planReconcileCmd(nil, nil)
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
		return m.fetch(ctx)
	}
}

// accountRows splits the balances into the wealth rows (every kind but loan) and the loan rows.
func (m *Model) accountRows() (wealth, loans []components.Row) {
	for _, b := range m.balances {
		row := components.Row{ID: b.Account.Name, Cells: []string{
			b.Account.Name, string(b.Account.Kind), b.Account.Currency.Code(), domain.FormatAmount(b.Balance, m.loc),
		}}
		if b.Account.Kind == domain.AccountLoan {
			loans = append(loans, row)
		} else {
			wealth = append(wealth, row)
		}
	}
	return wealth, loans
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
		return components.Modal(m.th, m.width, m.height, m.viewForm())
	case modeReconcileForm:
		return components.Modal(m.th, m.width, m.height, m.viewReconcileForm())
	case modeReconcilePreview:
		return m.viewReconcilePreview()
	case modeConfirm:
		return components.Modal(m.th, m.width, m.height, m.confirm.View())
	case modePriceForm:
		return components.Modal(m.th, m.width, m.height, m.viewPriceForm())
	case modeDetail:
		return m.viewDetail()
	case modeList:
		return m.viewList()
	}
	return m.viewList()
}

func (m *Model) viewList() string {
	out := m.sectionTitle("Accounts", !m.loanOn) + "\n" + m.table.View() + "\n" + m.th.Base.Render(m.totals.wealth)
	if len(m.loans.Rows()) > 0 {
		out += "\n" + m.sectionTitle("Loans & mortgages", m.loanOn) + "\n" + m.loans.View() + "\n" +
			m.th.Base.Render(m.totals.loans)
	}
	return out
}

func (m *Model) sectionTitle(name string, focus bool) string {
	if focus {
		return m.th.Title.Render("▸ " + name)
	}
	return m.th.Muted.Render("  " + name)
}

func (m *Model) viewDetail() string {
	title := m.th.Title.Render("Movements — " + m.detail.Account.Name)
	bal := m.th.Base.Render("Balance: " + domain.FormatAmount(m.detail.Balance, m.loc))
	out := title + "\n" + bal + "\n\n" + m.detailTbl.View()
	if pos := m.viewPositions(); pos != "" {
		out += "\n\n" + pos
	}
	return out
}

func (m *Model) viewForm() string {
	return m.th.Title.Render("Add account") + "\n\n" + m.form.View()
}

func (m *Model) viewReconcileForm() string {
	out := m.th.Title.Render("Reconcile account") + "\n"
	out += m.th.Base.Render("Account: " + m.reconcileAccount)
	out += "\n\n" + m.reconcileForm.View()
	out += "\n\n" + m.th.Muted.Render("enter preview · esc cancel")
	if m.reconcileNotice != "" {
		out += "\n" + m.th.Warning.Render(m.reconcileNotice)
	}
	return out
}
