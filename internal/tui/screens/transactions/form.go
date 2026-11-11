package transactions

import (
	"context"
	"errors"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

// confirmPurpose tracks which action the active confirm dialog belongs to, since delete and edit
// can both need a second confirmation when the target transaction is reconciled.
type confirmPurpose int

const (
	confirmNone confirmPurpose = iota
	confirmDelete
	confirmDeleteReconciled
	confirmEditReconciled
)

func (m *Model) openAddForm() {
	m.editing = nil
	m.editingTransferLeg = false
	m.accountF = components.NewAutocomplete("Account:", "e.g. Checking", m.th)
	m.accountF.SetSuggestions(m.accounts)
	m.amountF = components.NewAutocomplete("Amount:", "e.g. -42.50", m.th)
	m.dateF = components.NewAutocomplete("Date:", "today", m.th)
	m.descF = components.NewAutocomplete("Description:", "", m.th)
	m.categoryF = components.NewAutocomplete("Category:", "optional", m.th)
	m.categoryF.SetSuggestions(m.categories)
	m.notesF = components.NewAutocomplete("Notes:", "optional", m.th)
	m.statusF = components.NewAutocomplete("Status:", "booked", m.th)
	m.statusF.SetSuggestions(txStatuses())
	m.kindF = components.NewAutocomplete("Kind:", "payment", m.th)
	m.kindF.SetSuggestions(txKinds())
	m.form = components.NewForm(&m.accountF, &m.amountF, &m.dateF, &m.descF, &m.categoryF, &m.notesF, &m.statusF, &m.kindF)
	m.mode = modeForm
	m.form.Focus()
}

// openEditForm loads the selected transaction fresh (so edits apply to current data) and opens
// the edit form over it; it carries no Account field since EditTxRequest cannot move accounts.
func (m *Model) openEditForm() tea.Cmd {
	row := m.selectedTx()
	if row == nil {
		return nil
	}
	tx := row.Transaction
	id := tx.ID
	return func() tea.Msg {
		return editTargetMsg{tx: tx, id: id}
	}
}

// editTargetMsg carries the transaction to edit back into Update, so opening the form itself
// stays free of I/O.
type editTargetMsg struct {
	tx domain.Transaction
	id domain.TransactionID
}

func (m *Model) startEditForm(tx domain.Transaction) {
	m.editing = &tx.ID
	m.editingTransferLeg = tx.TransferGroupID != 0
	m.amountF = components.NewAutocomplete("Amount:", "", m.th)
	if !m.editingTransferLeg {
		m.amountF.SetValue(domain.FormatMachine(tx.Amount))
	}
	m.dateF = components.NewAutocomplete("Date:", "", m.th)
	m.dateF.SetValue(tx.Date.String())
	m.descF = components.NewAutocomplete("Description:", "", m.th)
	m.descF.SetValue(tx.Description)
	m.categoryF = components.NewAutocomplete("Category:", "optional", m.th)
	m.categoryF.SetSuggestions(m.categories)
	m.notesF = components.NewAutocomplete("Notes:", "optional", m.th)
	m.notesF.SetValue(tx.Notes)
	m.statusF = components.NewAutocomplete("Status:", "", m.th)
	m.statusF.SetValue(string(tx.Status))
	m.statusF.SetSuggestions(txStatuses())
	m.kindF = components.NewAutocomplete("Kind:", "", m.th)
	m.kindF.SetValue(string(tx.Kind))
	m.kindF.SetSuggestions(txKinds())
	fields := []*components.Autocomplete{&m.dateF, &m.descF, &m.categoryF, &m.notesF, &m.statusF, &m.kindF}
	if !m.editingTransferLeg {
		fields = append([]*components.Autocomplete{&m.amountF}, fields...)
	}
	m.form = components.NewForm(fields...)
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
		if m.editing != nil {
			return m, m.submitEdit(false)
		}
		return m, m.submitAdd()
	}
	m.form.Update(msg)
	return m, nil
}

func (m *Model) submitAdd() tea.Cmd {
	req := app.AddTxRequest{
		Account: m.accountF.Value(), Amount: m.amountF.Value(), Date: m.dateF.Value(),
		Description: m.descF.Value(), Category: m.categoryF.Value(), Notes: m.notesF.Value(),
		Status: m.statusF.Value(), Kind: m.kindF.Value(),
	}
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := m.uc.PlanAddTx(ctx, req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if _, err := m.uc.ApplyAddTx(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.reload(ctx)
	}
}

func (m *Model) submitEdit(confirmReconciled bool) tea.Cmd {
	id := *m.editing
	date, desc, notes := m.dateF.Value(), m.descF.Value(), m.notesF.Value()
	category, status, kind := m.categoryF.Value(), m.statusF.Value(), m.kindF.Value()
	var amount *string
	if !m.editingTransferLeg {
		v := m.amountF.Value()
		amount = &v
	}
	req := app.EditTxRequest{
		ID: id, Amount: amount, Date: &date, Description: &desc, Notes: &notes,
		Category: &category, Status: &status, Kind: &kind, ConfirmReconciled: confirmReconciled,
	}
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := m.uc.PlanEditTx(ctx, req)
		if err != nil {
			if !confirmReconciled && errors.Is(err, domain.ErrReconciled) {
				return needReconcileMsg{purpose: confirmEditReconciled}
			}
			return screens.ErrorMsg{Err: err}
		}
		if _, err := m.uc.ApplyEditTx(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.reload(ctx)
	}
}

func (m *Model) viewForm() string {
	title := "Add transaction"
	if m.editing != nil {
		title = "Edit transaction"
	}
	return m.th.Title.Render(title) + "\n\n" + m.form.View()
}

func txStatuses() []string {
	return []string{string(domain.StatusPending), string(domain.StatusBooked), string(domain.StatusReconciled)}
}

func txKinds() []string {
	return []string{
		string(domain.KindPayment), string(domain.KindTransfer), string(domain.KindExchange),
		string(domain.KindFee), string(domain.KindInterest), string(domain.KindRefund),
		string(domain.KindInvestment), string(domain.KindOther),
	}
}
