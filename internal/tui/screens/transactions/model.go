// Package transactions provides the register view for inspecting, searching and managing transactions.
package transactions

import (
	"context"
	"errors"
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

// pageSize is the number of transactions fetched per ListTx call.
const pageSize = 200

// maxWindow caps how many loaded rows are kept in memory at once (pageSize * 3), trimming the
// oldest loaded rows once the window grows past it.
const maxWindow = pageSize * 3

// nearBottomThreshold triggers loading the next page once the cursor gets this close to the
// bottom of what is currently loaded.
const nearBottomThreshold = pageSize / 4

// mode tracks which panel of the screen is active.
type mode int

const (
	modeList mode = iota
	modeForm
	modeConfirm
	modeFilter
	modeSearch
	modeCategorize
)

// Model is the transactions register: a keyset-paginated, filterable, searchable list, with
// forms to add, edit, delete and bulk-categorize. Update and View never touch the store; every
// use case call runs inside a tea.Cmd.
type Model struct {
	uc  UseCases
	loc domain.Locale
	th  theme.Theme
	km  KeyMap

	width, height int
	mode          mode
	lastErr       error

	req         app.ListTxRequest
	items       []app.TxView
	next        *app.TxCursor
	loadingMore bool
	table       components.Table

	accounts   []string
	categories []string

	form               components.Form
	accountF           components.Autocomplete
	amountF            components.Autocomplete
	dateF              components.Autocomplete
	descF              components.Autocomplete
	categoryF          components.Autocomplete
	notesF             components.Autocomplete
	statusF            components.Autocomplete
	kindF              components.Autocomplete
	editing            *domain.TransactionID
	editingTransferLeg bool

	confirm        components.ConfirmDialog
	confirmPurpose confirmPurpose
	deleteTarget   domain.TransactionID

	filterForm                                              components.Form
	filterAccount, filterCategory, filterStatus, filterKind components.Autocomplete
	filterUncategorized, filterSign                         components.Autocomplete
	filterFrom, filterTo, filterMin, filterMax              components.Autocomplete
	chipFocus                                               int

	searchField  components.Autocomplete
	searchSeq    int
	searchCancel context.CancelFunc
	tick         func(seq int) tea.Cmd

	categorizeField components.Autocomplete
}

var _ screens.Screen = (*Model)(nil)

// New builds the transactions screen over uc, formatting amounts and dates with loc.
func New(uc UseCases, loc domain.Locale, th theme.Theme) *Model {
	m := &Model{uc: uc, loc: loc, th: th, km: DefaultKeyMap(), tick: defaultTick}
	m.table = components.NewTable([]components.Column{
		{Title: "Date", Width: 10, Align: lipgloss.Left},
		{Title: "Account", Width: 16, Align: lipgloss.Left},
		{Title: "Description", Width: 24, Align: lipgloss.Left},
		{Title: "Category", Width: 14, Align: lipgloss.Left},
		{Title: "Status", Width: 10, Align: lipgloss.Left},
		{Title: "Amount", Width: 14, Align: lipgloss.Right},
	}, th)
	m.confirm = components.NewConfirmDialog(th)
	return m
}

// Init loads the first page of transactions along with the account and category names used by
// the forms.
func (m *Model) Init() tea.Cmd { return m.load() }

// SetSize adapts the table to the available terminal size.
func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	m.table.SetHeight(max(3, h-8))
	m.table.SetWidth(w)
}

// Title identifies the screen in the tab bar.
func (m *Model) Title() string { return "Transactions" }

// KeyMap exposes the bindings for the global help view.
func (m *Model) KeyMap() []key.Binding { return m.km.Bindings() }

// InputActive reports whether a form, the filter panel, the quick search box, or the bulk
// categorize field is currently capturing keystrokes.
func (m *Model) InputActive() bool {
	switch m.mode {
	case modeForm, modeFilter, modeSearch, modeCategorize:
		return true
	case modeList, modeConfirm:
		return false
	}
	return false
}

// loadedMsg carries a fresh first page, plus account and category names, back into Update.
// clearSelection is set when the load follows a bulk action that consumed the table's selection.
type loadedMsg struct {
	page           app.TxPage
	accounts       []string
	categories     []string
	clearSelection bool
}

// moreMsg carries one more page, appended to the already loaded window.
type moreMsg struct{ page app.TxPage }

func (m *Model) load() tea.Cmd {
	uc, req := m.uc, m.req
	return func() tea.Msg { return reload(context.Background(), uc, req, false) }
}

func (m *Model) loadMore() tea.Cmd {
	req := m.req
	req.Limit = pageSize
	req.After = m.next
	return func() tea.Msg {
		page, err := m.uc.ListTx(context.Background(), req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return moreMsg{page: page}
	}
}

// Update advances the screen state; it performs no I/O itself.
func (m *Model) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case loadedMsg:
		m.items = msg.page.Items
		m.next = msg.page.Next
		m.accounts = msg.accounts
		m.categories = msg.categories
		m.table.SetRows(m.txRows())
		if msg.clearSelection {
			m.table.ClearSelection()
		}
		m.mode = modeList
		m.lastErr = nil
		return m, nil
	case moreMsg:
		m.loadingMore = false
		m.items = append(m.items, msg.page.Items...)
		m.next = msg.page.Next
		m.trimWindow()
		m.table.SetRows(m.txRows())
		return m, nil
	case editTargetMsg:
		m.startEditForm(msg.tx)
		return m, nil
	case debounceMsg:
		if msg.seq != m.searchSeq {
			return m, nil // superseded by a later keystroke
		}
		return m, m.runSearch(msg.seq)
	case searchResultMsg:
		return m.applySearchResult(msg)
	case needReconcileMsg:
		m.confirmPurpose = msg.purpose
		m.confirm.Show("Reconciled transaction", "This transaction is reconciled. Continue anyway?")
		m.mode = modeConfirm
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// trimWindow drops the oldest loaded rows once the window grows past maxWindow, keeping the
// cursor pointed at the same logical row.
func (m *Model) trimWindow() {
	if len(m.items) <= maxWindow {
		return
	}
	drop := len(m.items) - maxWindow
	m.items = m.items[drop:]
	m.table.SetCursor(m.table.Cursor() - drop)
}

func (m *Model) handleKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch m.mode {
	case modeForm:
		return m.handleFormKey(msg)
	case modeConfirm:
		return m.handleConfirmKey(msg)
	case modeFilter:
		return m.handleFilterKey(msg)
	case modeSearch:
		return m.handleSearchKey(msg)
	case modeCategorize:
		return m.handleCategorizeKey(msg)
	case modeList:
		return m.handleListKey(msg)
	}
	return m.handleListKey(msg)
}

func (m *Model) handleListKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	if handled, cmd := m.handleNavKey(msg); handled {
		return m, cmd
	}
	switch {
	case key.Matches(msg, m.km.Add):
		m.openAddForm()
	case key.Matches(msg, m.km.Edit):
		return m, m.openEditForm()
	case key.Matches(msg, m.km.Delete):
		return m.handleDeleteKey()
	case key.Matches(msg, m.km.Filter):
		m.openFilterForm()
	case key.Matches(msg, m.km.Search):
		m.openSearch()
	case key.Matches(msg, m.km.Select):
		m.table.ToggleSelect()
		m.table.SetShowSelect(true)
	case key.Matches(msg, m.km.Categorize):
		m.openCategorize()
	case key.Matches(msg, m.km.ClearFilter):
		m.req = app.ListTxRequest{}
		return m, m.load()
	}
	if handled, cmd := m.handleChipKey(msg); handled {
		return m, cmd
	}
	return m, nil
}

// handleNavKey handles cursor movement in the table, reporting whether msg matched one of the
// navigation bindings.
func (m *Model) handleNavKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	switch {
	case key.Matches(msg, m.km.Up):
		m.table.MoveUp(1)
	case key.Matches(msg, m.km.Down):
		m.table.MoveDown(1)
		return true, m.maybeLoadMore()
	case key.Matches(msg, m.km.PageUp):
		m.table.PageUp()
	case key.Matches(msg, m.km.PageDown):
		m.table.PageDown()
		return true, m.maybeLoadMore()
	case key.Matches(msg, m.km.Top):
		m.table.MoveTop()
	case key.Matches(msg, m.km.Bottom):
		m.table.MoveBottom()
		return true, m.maybeLoadMore()
	default:
		return false, nil
	}
	return true, nil
}

// maybeLoadMore starts fetching the next page once the cursor nears the loaded bottom.
func (m *Model) maybeLoadMore() tea.Cmd {
	if m.next == nil || m.loadingMore || !m.table.NearBottom(nearBottomThreshold) {
		return nil
	}
	m.loadingMore = true
	return m.loadMore()
}

// handleDeleteKey arms the delete confirmation dialog for the selected row.
func (m *Model) handleDeleteKey() (screens.Screen, tea.Cmd) {
	row := m.selectedTx()
	if row == nil {
		return m, nil
	}
	m.deleteTarget = row.Transaction.ID
	m.confirmPurpose = confirmDelete
	m.confirm.Show("Delete transaction", fmt.Sprintf("Delete %q?", row.Transaction.Description))
	m.mode = modeConfirm
	return m, nil
}

// parseTxID parses a table row ID back into the TransactionID it was formatted from.
func parseTxID(s string) (domain.TransactionID, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse transaction id %q: %w", s, err)
	}
	return domain.TransactionID(n), nil
}

func (m *Model) selectedTx() *app.TxView {
	row := m.table.SelectedRow()
	if row == nil {
		return nil
	}
	for i := range m.items {
		if strconv.FormatInt(int64(m.items[i].Transaction.ID), 10) == row.ID {
			return &m.items[i]
		}
	}
	return nil
}

func (m *Model) handleConfirmKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	yes, no := m.confirm.HandleKey(msg)
	purpose := m.confirmPurpose
	switch {
	case yes:
		m.confirmPurpose = confirmNone
		switch purpose {
		case confirmDelete:
			return m, m.submitDelete(false)
		case confirmDeleteReconciled:
			return m, m.submitDelete(true)
		case confirmEditReconciled:
			return m, m.submitEdit(true)
		case confirmCategorizeReconciled:
			return m, m.submitCategorize(true)
		case confirmNone:
			return m, nil
		}
	case no:
		m.confirmPurpose = confirmNone
		m.mode = modeList
	}
	return m, nil
}

func (m *Model) submitDelete(confirmReconciled bool) tea.Cmd {
	id := m.deleteTarget
	uc, req := m.uc, m.req
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := uc.PlanRemoveTx(ctx, app.RemoveTxRequest{ID: id, ConfirmReconciled: confirmReconciled})
		if err != nil {
			if !confirmReconciled && errors.Is(err, domain.ErrReconciled) {
				return needReconcileMsg{purpose: confirmDeleteReconciled}
			}
			return screens.ErrorMsg{Err: err}
		}
		if err := uc.ApplyRemoveTx(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return reload(ctx, uc, req, false)
	}
}

// needReconcileMsg asks the user to confirm an edit or delete of a reconciled transaction.
type needReconcileMsg struct{ purpose confirmPurpose }

// reload is the shared body of every tea.Cmd that needs a fresh first page afterward. It only
// touches its parameters, which every caller must copy out of the Model synchronously before
// returning the tea.Cmd closure that calls it, keeping all live Model access confined to Update.
func reload(ctx context.Context, uc UseCases, req app.ListTxRequest, clearSelection bool) tea.Msg {
	req.Limit = pageSize
	req.After = nil
	page, err := uc.ListTx(ctx, req)
	if err != nil {
		return screens.ErrorMsg{Err: err}
	}
	accts, err := uc.ListAccounts(ctx, app.ListAccountsRequest{})
	if err != nil {
		return screens.ErrorMsg{Err: err}
	}
	cats, err := uc.ListCategories(ctx)
	if err != nil {
		return screens.ErrorMsg{Err: err}
	}
	names := make([]string, len(accts))
	for i, a := range accts {
		names[i] = a.Account.Name
	}
	catNames := make([]string, len(cats))
	for i, c := range cats {
		catNames[i] = c.Name
	}
	return loadedMsg{page: page, accounts: names, categories: catNames, clearSelection: clearSelection}
}

func (m *Model) txRows() []components.Row {
	rows := make([]components.Row, len(m.items))
	for i, v := range m.items {
		rows[i] = components.Row{
			ID: strconv.FormatInt(int64(v.Transaction.ID), 10),
			Cells: []string{
				v.Transaction.Date.Format(m.loc), v.Account, v.Transaction.Description, v.Category,
				string(v.Transaction.Status), domain.FormatAmount(v.Transaction.Amount, m.loc),
			},
		}
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
	case modeFilter:
		return m.viewFilter()
	case modeSearch:
		return m.viewSearch()
	case modeCategorize:
		return m.viewCategorize()
	case modeList:
		return m.viewList()
	}
	return m.viewList()
}

func (m *Model) viewList() string {
	header := m.th.Title.Render("Transactions")
	if chips := m.chipsView(); chips != "" {
		header += "\n" + chips
	}
	return header + "\n\n" + m.table.View()
}
