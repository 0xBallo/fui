package accounts

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/0xballo/fui/internal/app/investing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

// secOpsState holds the forms and the operation list of the account detail panel.
type secOpsState struct {
	form   components.Form
	submit func() tea.Cmd
	title  string
	notice string

	secName, secTicker, secISIN, secKind, secCurrency components.Autocomplete

	opKind, opSecurity, opDate, opQty, opPrice, opFees, opCash components.Autocomplete

	ops      []investing.OpLine
	opsTable components.Table
	editing  investing.OpLine
	initial  editValues
	deleteOp investing.OpLine
}

// editValues are the prefilled values of the edit-operation form, used to send only changes.
type editValues struct{ date, qty, price, fees string }

// opsLoadedMsg carries the operations of the open account back into Update.
type opsLoadedMsg struct{ lines []investing.OpLine }

func (m *Model) initSecOps() {
	m.secops.opsTable = components.NewTable([]components.Column{
		{Title: "Date", Width: 10, Align: lipgloss.Left},
		{Title: "Kind", Width: 9, Align: lipgloss.Left},
		{Title: "Security", Width: 20, Align: lipgloss.Left},
		{Title: "Qty", Width: 10, Align: lipgloss.Right},
		{Title: "Price", Width: 12, Align: lipgloss.Right},
		{Title: "Fees", Width: 10, Align: lipgloss.Right},
	}, m.th)
}

// SetCryptoEnabled tells the screen whether the crypto module is on: it gates the wallet account
// kind and the crypto security kind in the forms.
func (m *Model) SetCryptoEnabled(on bool) { m.crypto = on }

// opsAccount reports whether the open account can hold positions given the enabled modules.
func (m *Model) opsAccount(k domain.AccountKind) bool {
	return k == domain.AccountInvestment || (k == domain.AccountWallet && m.crypto)
}

func (m *Model) startSecForm(title string, submit func() tea.Cmd, fields ...*components.Autocomplete) {
	m.secops.notice = ""
	m.secops.title, m.secops.submit = title, submit
	m.secops.form = components.NewForm(fields...)
	m.secops.form.Focus()
}

func (m *Model) openSecurityForm() {
	if !m.positionsVisible() {
		return
	}
	s := &m.secops
	s.secName = components.NewAutocomplete("Name:", "e.g. VWCE ETF", m.th)
	s.secTicker = components.NewAutocomplete("Ticker:", "optional if ISIN is set", m.th)
	s.secISIN = components.NewAutocomplete("ISIN:", "optional if ticker is set", m.th)
	s.secKind = components.NewAutocomplete("Kind:", m.securityKindHint(), m.th)
	s.secKind.SetSuggestions(m.securityKinds())
	s.secCurrency = components.NewAutocomplete("Currency:", "e.g. EUR", m.th)
	s.secCurrency.SetValue(m.detail.Account.Currency.Code())
	m.startSecForm("Add security", m.submitSecurity, &s.secName, &s.secTicker, &s.secISIN, &s.secKind, &s.secCurrency)
	m.mode = modeSecurityForm
}

func (m *Model) securityKinds() []string {
	kinds := []string{
		string(domain.SecurityStock), string(domain.SecurityETF), string(domain.SecurityBond), string(domain.SecurityFund),
	}
	if m.crypto {
		kinds = append(kinds, string(domain.SecurityCrypto))
	}
	return kinds
}

func (m *Model) securityKindHint() string { return strings.Join(m.securityKinds(), "/") }

func (m *Model) openOpForm() {
	if !m.positionsVisible() {
		return
	}
	s := &m.secops
	s.opKind = components.NewAutocomplete("Kind:", "buy/sell/dividend/coupon", m.th)
	s.opKind.SetSuggestions([]string{
		string(domain.OpBuy), string(domain.OpSell), string(domain.OpDividend), string(domain.OpCoupon),
	})
	s.opSecurity = components.NewAutocomplete("Security:", "name, ticker or ISIN", m.th)
	if line, ok := m.selectedPosition(); ok {
		s.opSecurity.SetValue(line.Security.Name)
	}
	s.opDate = components.NewAutocomplete("Date:", "today", m.th)
	s.opQty = components.NewAutocomplete("Quantity:", "e.g. 10", m.th)
	s.opPrice = components.NewAutocomplete("Price:", "unit price, e.g. 123.45", m.th)
	s.opFees = components.NewAutocomplete("Fees:", "optional", m.th)
	s.opCash = components.NewAutocomplete("Cash account:", "optional, books the cash movement", m.th)
	m.startSecForm("Add operation", m.submitOp,
		&s.opKind, &s.opSecurity, &s.opDate, &s.opQty, &s.opPrice, &s.opFees, &s.opCash)
	m.mode = modeOpForm
}

func (m *Model) openOpEditForm(line investing.OpLine) {
	s := &m.secops
	s.editing = line
	s.initial = editValues{
		date: line.Op.Date.String(), qty: line.Op.Quantity.String(), price: line.Op.UnitPrice.String(),
	}
	if !line.Op.Fees.IsZero() {
		s.initial.fees = domain.FormatMachine(line.Op.Fees)
	}
	s.opDate = components.NewAutocomplete("Date:", "YYYY-MM-DD", m.th)
	s.opQty = components.NewAutocomplete("Quantity:", "e.g. 10", m.th)
	s.opPrice = components.NewAutocomplete("Price:", "unit price, e.g. 123.45", m.th)
	s.opFees = components.NewAutocomplete("Fees:", "optional", m.th)
	s.opDate.SetValue(s.initial.date)
	s.opQty.SetValue(s.initial.qty)
	s.opPrice.SetValue(s.initial.price)
	s.opFees.SetValue(s.initial.fees)
	m.startSecForm("Edit operation", m.submitOpEdit, &s.opDate, &s.opQty, &s.opPrice, &s.opFees)
	m.mode = modeOpEditForm
}

func (m *Model) handleSecFormKey(msg tea.KeyPressMsg, back mode) (screens.Screen, tea.Cmd) {
	f := &m.secops.form
	switch msg.String() {
	case "esc":
		m.mode = back
		return m, nil
	case "shift+tab":
		f.Prev()
		return m, nil
	case "enter":
		if f.FocusedOpen() {
			f.Update(msg)
			return m, nil
		}
		if !f.AtLast() {
			f.Next()
			return m, nil
		}
		return m, m.secops.submit()
	}
	f.Update(msg)
	return m, nil
}

func (m *Model) submitSecurity() tea.Cmd {
	s := &m.secops
	req := investing.AddSecurityRequest{
		Name: strings.TrimSpace(s.secName.Value()), Ticker: strings.TrimSpace(s.secTicker.Value()),
		ISIN: strings.TrimSpace(s.secISIN.Value()), Kind: strings.TrimSpace(s.secKind.Value()),
		Currency: strings.TrimSpace(s.secCurrency.Value()),
	}
	if req.Name == "" {
		s.notice = "Name is required."
		return nil
	}
	s.notice = ""
	account := m.detail.Account.Name
	uc := m.positionsUC
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := uc.PlanAddSecurity(ctx, req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if _, err := uc.ApplyAddSecurity(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.loadDetail(account)()
	}
}

func (m *Model) submitOp() tea.Cmd {
	s := &m.secops
	req := investing.OpRequest{
		Account:     m.detail.Account.Name,
		Security:    strings.TrimSpace(s.opSecurity.Value()),
		Date:        strings.TrimSpace(s.opDate.Value()),
		Quantity:    strings.TrimSpace(s.opQty.Value()),
		Price:       strings.TrimSpace(s.opPrice.Value()),
		Fees:        strings.TrimSpace(s.opFees.Value()),
		CashAccount: strings.TrimSpace(s.opCash.Value()),
	}
	kind := strings.TrimSpace(s.opKind.Value())
	if kind == "" || req.Security == "" || req.Quantity == "" || req.Price == "" {
		s.notice = "Kind, security, quantity and price are required."
		return nil
	}
	s.notice = ""
	uc := m.positionsUC
	var plan func(context.Context, investing.OpRequest) (investing.OpPlan, error)
	switch domain.SecurityOpKind(kind) {
	case domain.OpBuy:
		plan = uc.PlanBuy
	case domain.OpSell:
		plan = uc.PlanSell
	case domain.OpDividend:
		plan = uc.PlanDividend
	case domain.OpCoupon:
		plan = uc.PlanCoupon
	default:
		s.notice = "Kind must be buy, sell, dividend or coupon."
		return nil
	}
	account := req.Account
	return func() tea.Msg {
		ctx := context.Background()
		p, err := plan(ctx, req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if _, err := uc.ApplyOp(ctx, p); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.loadDetail(account)()
	}
}

func (m *Model) submitOpEdit() tea.Cmd {
	s := &m.secops
	req := investing.EditOpRequest{ID: s.editing.Op.ID}
	changed := func(cur, was string) *string {
		cur = strings.TrimSpace(cur)
		if cur == was {
			return nil
		}
		return &cur
	}
	req.Date = changed(s.opDate.Value(), s.initial.date)
	req.Quantity = changed(s.opQty.Value(), s.initial.qty)
	req.Price = changed(s.opPrice.Value(), s.initial.price)
	req.Fees = changed(s.opFees.Value(), s.initial.fees)
	s.notice = ""
	account := m.detail.Account.Name
	uc := m.positionsUC
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := uc.PlanEditOp(ctx, req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if _, err := uc.ApplyEditOp(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.loadOps(account)()
	}
}

func (m *Model) loadOps(account string) tea.Cmd {
	uc := m.positionsUC
	return func() tea.Msg {
		lines, err := uc.ListOps(context.Background(), investing.ListOpsRequest{Account: account})
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return opsLoadedMsg{lines: lines}
	}
}

func (m *Model) setOps(lines []investing.OpLine) {
	s := &m.secops
	s.ops = lines
	rows := make([]components.Row, len(lines))
	for i, l := range lines {
		fees := "-"
		if !l.Op.Fees.IsZero() {
			fees = domain.FormatAmount(l.Op.Fees, m.loc)
		}
		rows[i] = components.Row{ID: fmt.Sprint(i), Cells: []string{
			l.Op.Date.Format(m.loc), string(l.Op.Kind), l.Security.Name,
			l.Op.Quantity.String(), l.Op.UnitPrice.String(), fees,
		}}
	}
	s.opsTable.SetRows(rows)
	s.opsTable.SetWidth(m.width)
	s.opsTable.SetHeight(max(3, m.height-8))
	m.mode = modeOps
}

func (m *Model) selectedOp() (investing.OpLine, bool) {
	row := m.secops.opsTable.SelectedRow()
	if row == nil {
		return investing.OpLine{}, false
	}
	for i, l := range m.secops.ops {
		if fmt.Sprint(i) == row.ID {
			return l, true
		}
	}
	return investing.OpLine{}, false
}

func (m *Model) handleOpsKey(msg tea.KeyPressMsg) (screens.Screen, tea.Cmd) {
	switch {
	case key.Matches(msg, m.km.Back):
		m.mode = modeDetail
	case key.Matches(msg, m.km.Up):
		m.secops.opsTable.MoveUp(1)
	case key.Matches(msg, m.km.Down):
		m.secops.opsTable.MoveDown(1)
	case key.Matches(msg, m.km.OpEdit):
		if line, ok := m.selectedOp(); ok {
			m.openOpEditForm(line)
		}
	case key.Matches(msg, m.km.OpDelete):
		if line, ok := m.selectedOp(); ok {
			m.secops.deleteOp = line
			m.confirm.Show("Delete operation",
				fmt.Sprintf("Delete this %s of %s on %s?", line.Op.Kind, line.Security.Name, line.Op.Date.Format(m.loc)),
				"The linked cash transaction, if any, is removed too.")
			m.mode = modeOpConfirm
		}
	}
	return m, nil
}

func (m *Model) handleOpConfirmKey(msg tea.KeyPressMsg) (screens.Screen, tea.Cmd) {
	yes, no := m.confirm.HandleKey(msg)
	switch {
	case yes:
		return m, m.submitOpDelete()
	case no:
		m.mode = modeOps
	}
	return m, nil
}

func (m *Model) submitOpDelete() tea.Cmd {
	id := m.secops.deleteOp.Op.ID
	account := m.detail.Account.Name
	uc := m.positionsUC
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := uc.PlanRemoveOp(ctx, id)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if err := uc.ApplyRemoveOp(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.loadOps(account)()
	}
}

func (m *Model) viewSecForm(hint string) string {
	s := &m.secops
	out := m.th.Title.Render(s.title) + "\n"
	out += m.th.Base.Render("Account: "+m.detail.Account.Name) + "\n\n" + s.form.View()
	out += "\n\n" + m.th.Muted.Render(hint)
	if s.notice != "" {
		out += "\n" + m.th.Warning.Render(s.notice)
	}
	return out
}

func (m *Model) viewOps() string {
	out := m.th.Title.Render("Operations — "+m.detail.Account.Name) + "\n\n"
	if len(m.secops.ops) == 0 {
		return out + m.th.Muted.Render("(no operations) · esc back")
	}
	return out + m.secops.opsTable.View() + "\n" + m.th.Muted.Render("e edit · d delete · esc back")
}
