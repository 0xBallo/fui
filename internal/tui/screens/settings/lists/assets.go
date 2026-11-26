package lists

import (
	"context"
	"strconv"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/app/assetting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/theme"
)

// assetsMode tracks which panel of the assets sub-view is active.
type assetsMode int

const (
	assetsListMode assetsMode = iota
	assetsDetail
	assetsForm
	assetsConfirm
)

// Assets is the settings sub-view that lists assets with their latest value, debt and net value,
// shows an asset's valuation history, and lets the user add, edit and remove assets, record and
// remove valuations, and link or unlink a loan. Requires the assets module to be enabled.
type Assets struct {
	uc      AssetsUseCases
	enabled module.Set
	loc     domain.Locale
	th      theme.Theme
	km      KeyMap

	width, height int
	mode          assetsMode

	items []assetting.Item
	table components.Table

	view *assetting.View // non-nil while the detail panel is open
	hist components.Table

	form    components.Form
	title   string
	fields  assetFormFields
	submit  func() tea.Cmd
	initial assetFormValues // edit form values as shown, to send only what changed

	confirm   components.ConfirmDialog
	onConfirm func(context.Context) error

	err error // last submit error, shown inline until the next state change
}

var _ screens.Screen = (*Assets)(nil)

// NewAssets builds the assets sub-view over uc; enabled gates every use case on the assets module.
func NewAssets(uc AssetsUseCases, enabled module.Set, loc domain.Locale, th theme.Theme) *Assets {
	m := &Assets{uc: uc, enabled: enabled, loc: loc, th: th, km: DefaultKeyMap()}
	m.table = components.NewTable([]components.Column{
		{Title: "Name", Width: 12, Align: lipgloss.Left},
		{Title: "Kind", Width: 8, Align: lipgloss.Left},
		{Title: "Latest", Width: 15, Align: lipgloss.Right},
		{Title: "Loan", Width: 9, Align: lipgloss.Left},
		{Title: "Debt", Width: 15, Align: lipgloss.Right},
		{Title: "Net", Width: 15, Align: lipgloss.Right},
	}, th)
	m.hist = components.NewTable([]components.Column{
		{Title: "Date", Width: 12, Align: lipgloss.Left},
		{Title: "Value", Width: 16, Align: lipgloss.Right},
	}, th)
	m.confirm = components.NewConfirmDialog(th)
	return m
}

// Init loads the assets.
func (m *Assets) Init() tea.Cmd {
	return func() tea.Msg { return m.reload(context.Background(), "") }
}

// SetSize adapts the tables to the available terminal size.
func (m *Assets) SetSize(w, h int) {
	m.width, m.height = w, h
	m.table.SetHeight(max(3, h-8))
	m.table.SetWidth(w)
	m.hist.SetHeight(max(3, h-14))
	m.hist.SetWidth(w)
}

// Title identifies the sub-view in the settings shell.
func (m *Assets) Title() string { return "Assets" }

// KeyMap exposes the bindings for the global help view.
func (m *Assets) KeyMap() []key.Binding { return m.km.BindingsWithAssets() }

// InputActive reports whether a form is capturing keystrokes.
func (m *Assets) InputActive() bool { return m.mode == assetsForm }

type assetsLoadedMsg struct {
	items []assetting.Item
	view  *assetting.View
}

type assetsConfirmMsg struct {
	title, prompt string
	details       []string
	apply         func(context.Context) error
}

type assetsLoansMsg struct {
	asset string
	loans []string
}

// reload lists the assets and, when detail is not empty, refreshes that asset's history.
func (m *Assets) reload(ctx context.Context, detail string) tea.Msg {
	items, err := m.uc.List(ctx, m.enabled, "")
	if err != nil {
		return screens.ErrorMsg{Err: err}
	}
	out := assetsLoadedMsg{items: items}
	if detail != "" {
		v, err := m.uc.Show(ctx, m.enabled, detail, "")
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		out.view = &v
	}
	return out
}

// detailName is the asset the detail panel shows, or "".
func (m *Assets) detailName() string {
	if m.view == nil {
		return ""
	}
	return m.view.Asset.Name
}

// Update advances the sub-view state; it performs no I/O itself.
func (m *Assets) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case assetsLoadedMsg:
		m.err = nil
		m.items, m.view = msg.items, msg.view
		m.table.SetRows(m.rows())
		m.mode = assetsListMode
		if m.view != nil {
			m.hist.SetRows(m.histRows())
			m.mode = assetsDetail
		}
		return m, nil
	case assetsConfirmMsg:
		m.err = nil
		m.onConfirm = msg.apply
		m.confirm.Show(msg.title, msg.prompt, msg.details...)
		m.mode = assetsConfirm
		return m, nil
	case assetsLoansMsg:
		m.openLinkForm(msg.asset, msg.loans)
		return m, nil
	case screens.ErrorMsg:
		m.err = msg.Err
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Assets) handleKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch m.mode {
	case assetsForm:
		return m.handleFormKey(msg)
	case assetsConfirm:
		return m.handleConfirmKey(msg)
	case assetsDetail:
		return m.handleDetailKey(msg)
	case assetsListMode:
		return m.handleListKey(msg)
	}
	return m.handleListKey(msg)
}

func (m *Assets) handleListKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	if key.Matches(msg, m.km.Add) {
		m.openForm(nil)
		return m, nil
	}
	item := m.selected()
	switch {
	case key.Matches(msg, m.km.Up):
		m.table.MoveUp(1)
	case key.Matches(msg, m.km.Down):
		m.table.MoveDown(1)
	case item == nil:
	case key.Matches(msg, m.km.Open):
		name := item.Asset.Name
		return m, func() tea.Msg { return m.reload(context.Background(), name) }
	case key.Matches(msg, m.km.Edit):
		m.openForm(item)
	case key.Matches(msg, m.km.Remove):
		return m, m.planRemove(item.Asset.Name)
	default:
		return m, m.handleAssetAction(msg, item.Asset.Name)
	}
	return m, nil
}

func (m *Assets) handleDetailKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	name := m.detailName()
	switch {
	case key.Matches(msg, m.km.Back):
		m.view = nil
		m.mode = assetsListMode
	case key.Matches(msg, m.km.Up):
		m.hist.MoveUp(1)
	case key.Matches(msg, m.km.Down):
		m.hist.MoveDown(1)
	case key.Matches(msg, m.km.Edit):
		for i := range m.items {
			if m.items[i].Asset.Name == name {
				m.openForm(&m.items[i])
			}
		}
	case key.Matches(msg, m.km.Remove):
		if row := m.hist.SelectedRow(); row != nil {
			return m, m.planRemoveValuation(name, row.ID)
		}
	default:
		return m, m.handleAssetAction(msg, name)
	}
	return m, nil
}

// handleAssetAction runs the value, link and unlink keys shared by the list and the detail panel.
func (m *Assets) handleAssetAction(msg tea.KeyMsg, name string) tea.Cmd {
	switch {
	case key.Matches(msg, m.km.Value):
		m.openValueForm(name)
	case key.Matches(msg, m.km.Link):
		return m.loadLoans(name)
	case key.Matches(msg, m.km.Unlink):
		return m.unlink(name)
	}
	return nil
}

func (m *Assets) handleConfirmKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	yes, no := m.confirm.HandleKey(msg)
	switch {
	case yes:
		apply, detail := m.onConfirm, m.detailName()
		return m, func() tea.Msg {
			ctx := context.Background()
			if err := apply(ctx); err != nil {
				return screens.ErrorMsg{Err: err}
			}
			return m.reload(ctx, detail)
		}
	case no:
		m.mode = m.base()
	}
	return m, nil
}

// base is the panel a form or confirmation returns to.
func (m *Assets) base() assetsMode {
	if m.view != nil {
		return assetsDetail
	}
	return assetsListMode
}

// selected returns the asset under the list cursor, or nil.
func (m *Assets) selected() *assetting.Item {
	row := m.table.SelectedRow()
	if row == nil {
		return nil
	}
	id, err := strconv.ParseInt(row.ID, 10, 64)
	if err != nil {
		return nil
	}
	for i := range m.items {
		if m.items[i].Asset.ID == domain.AssetID(id) {
			return &m.items[i]
		}
	}
	return nil
}

func (m *Assets) money(v domain.Money) string {
	return domain.FormatAmount(v, m.loc) + " " + v.Currency().Code()
}

func (m *Assets) rows() []components.Row {
	rows := make([]components.Row, len(m.items))
	for i, it := range m.items {
		loan, debt := "-", "-"
		if it.Loan != "" {
			loan, debt = it.Loan, m.money(it.Debt)
		}
		rows[i] = components.Row{ID: strconv.FormatInt(int64(it.Asset.ID), 10), Cells: []string{
			it.Asset.Name, string(it.Asset.Kind), m.money(it.Latest.Value), loan, debt, m.money(it.Net),
		}}
	}
	return rows
}

func (m *Assets) histRows() []components.Row {
	rows := make([]components.Row, len(m.view.History))
	for i, v := range m.view.History {
		rows[i] = components.Row{ID: v.Date.String(), Cells: []string{v.Date.String(), m.money(v.Value)}}
	}
	return rows
}

// View renders the active panel.
func (m *Assets) View() string {
	var body string
	switch m.mode {
	case assetsForm:
		body = components.Modal(m.th, m.width, m.height, m.th.Title.Render(m.title)+"\n\n"+m.form.View())
	case assetsConfirm:
		body = components.Modal(m.th, m.width, m.height, m.confirm.View())
	case assetsDetail:
		body = m.viewDetail()
	case assetsListMode:
		body = m.viewList()
	default:
		body = m.viewList()
	}
	return withError(m.th, body, m.err)
}

func (m *Assets) viewList() string {
	return m.th.Title.Render("Assets") + "\n\n" + m.table.View()
}

func (m *Assets) viewDetail() string {
	a := m.view.Asset
	loan := "none"
	if m.view.Loan != "" {
		loan = m.view.Loan + " (debt " + m.money(m.view.Debt) + ")"
	}
	info := m.th.Muted.Render(string(a.Kind)+" · bought "+a.PurchaseDate.String()+" for "+m.money(a.PurchaseValue)) + "\n" +
		m.th.Muted.Render("loan: "+loan+" · net "+m.money(m.view.Net))
	return m.th.Title.Render("Asset: "+a.Name) + "\n" + info + "\n\n" +
		m.th.Header.Render("Valuation history") + "\n" + m.hist.View()
}

func (m *Assets) planRemove(name string) tea.Cmd {
	return func() tea.Msg {
		plan, err := m.uc.PlanRemove(context.Background(), m.enabled, name)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return assetsConfirmMsg{
			title: "Delete asset", prompt: "Delete asset " + plan.Asset.Name + "?",
			details: []string{strconv.Itoa(plan.Valuations) + " valuation(s) will be removed"},
			apply:   func(ctx context.Context) error { return m.uc.ApplyRemove(ctx, plan) },
		}
	}
}

func (m *Assets) planRemoveValuation(name, date string) tea.Cmd {
	return func() tea.Msg {
		plan, err := m.uc.PlanRemoveValuation(context.Background(), m.enabled, name, date)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return assetsConfirmMsg{
			title: "Delete valuation", prompt: "Delete the valuation of " + date + "?",
			apply: func(ctx context.Context) error { return m.uc.ApplyRemoveValuation(ctx, plan) },
		}
	}
}

func (m *Assets) unlink(name string) tea.Cmd {
	return func() tea.Msg {
		plan, err := m.uc.PlanUnlink(context.Background(), m.enabled, name)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return assetsConfirmMsg{
			title: "Unlink loan", prompt: "Unlink the loan from " + plan.Asset.Name + "?",
			apply: func(ctx context.Context) error { return m.uc.ApplyUnlink(ctx, plan) },
		}
	}
}

func (m *Assets) loadLoans(name string) tea.Cmd {
	return func() tea.Msg {
		loans, err := m.uc.ListLoanNames(context.Background(), m.enabled)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return assetsLoansMsg{asset: name, loans: loans}
	}
}
