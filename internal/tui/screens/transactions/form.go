package transactions

import (
	"context"
	"errors"
	"slices"
	"strings"

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
	confirmCategorizeReconciled
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
	m.newPayeeTagFields("", nil)
	m.notesF = components.NewAutocomplete("Notes:", "optional", m.th)
	m.statusF = components.NewAutocomplete("Status:", "booked", m.th)
	m.statusF.SetSuggestions(txStatuses())
	m.kindF = components.NewAutocomplete("Kind:", "payment", m.th)
	m.kindF.SetSuggestions(txKinds())
	m.form = components.NewForm(&m.accountF, &m.amountF, &m.dateF, &m.descF, &m.categoryF, &m.payeeF, &m.tagsF, &m.notesF, &m.statusF, &m.kindF)
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
	tags := slices.Clone(row.Tags)
	return func() tea.Msg {
		return editTargetMsg{tx: tx, id: id, tags: tags}
	}
}

// editTargetMsg carries the transaction to edit back into Update, so opening the form itself
// stays free of I/O.
type editTargetMsg struct {
	tx   domain.Transaction
	id   domain.TransactionID
	tags []string
}

func (m *Model) startEditForm(tx domain.Transaction, tags []string) {
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
	m.editPayee = m.payeeName(tx.PayeeID)
	m.editTags = tags
	m.newPayeeTagFields(m.editPayee, tags)
	m.notesF = components.NewAutocomplete("Notes:", "optional", m.th)
	m.notesF.SetValue(tx.Notes)
	m.statusF = components.NewAutocomplete("Status:", "", m.th)
	m.statusF.SetValue(string(tx.Status))
	m.statusF.SetSuggestions(txStatuses())
	m.kindF = components.NewAutocomplete("Kind:", "", m.th)
	m.kindF.SetValue(string(tx.Kind))
	m.kindF.SetSuggestions(txKinds())
	fields := []*components.Autocomplete{&m.dateF, &m.descF, &m.categoryF, &m.payeeF, &m.tagsF, &m.notesF, &m.statusF, &m.kindF}
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
			if m.tagsF.Focused() {
				// Accepting a suggestion leaves the cursor where it was; the next token is typed at the end.
				m.tagsF.Update(tea.KeyMsg{Type: tea.KeyEnd})
			}
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
	if m.tagsF.Focused() {
		m.updateTagsField(msg)
		return m, nil
	}
	m.form.Update(msg)
	return m, nil
}

// newPayeeTagFields builds the Payee and Tags inputs, shared by the add and edit forms.
func (m *Model) newPayeeTagFields(payee string, tags []string) {
	names := make([]string, len(m.payees))
	for i, p := range m.payees {
		names[i] = p.Name
	}
	m.payeeF = components.NewAutocomplete("Payee:", "optional", m.th)
	m.payeeF.SetSuggestions(names)
	m.payeeF.SetValue(payee)
	m.tagsF = components.NewAutocomplete("Tags:", "optional, comma-separated", m.th)
	m.tagsF.SetValue(strings.Join(tags, ", "))
	m.refreshTagSuggestions()
}

func (m *Model) payeeName(id domain.PayeeID) string {
	for _, p := range m.payees {
		if p.ID == id {
			return p.Name
		}
	}
	return ""
}

// updateTagsField forwards a key to the Tags input. Autocomplete only matches the whole value, so
// the suggestions are the known tags re-prefixed with the tokens already typed, which makes each
// comma-separated token completable. Typing reopens the popup that accepting a suggestion closed.
func (m *Model) updateTagsField(msg tea.KeyMsg) {
	switch msg.String() {
	case "up", "down", "tab":
	default:
		m.tagsF.Focus()
	}
	m.tagsF.Update(msg)
	m.refreshTagSuggestions()
}

func (m *Model) refreshTagSuggestions() {
	val := m.tagsF.Value()
	cut := strings.LastIndex(val, ",") + 1
	prefix := val[:cut]
	if cut < len(val) {
		rest := val[cut:]
		prefix += rest[:len(rest)-len(strings.TrimLeft(rest, " "))]
	} else {
		prefix += " "
	}
	used := parseTags(val[:cut])
	var sugg []string
	for _, t := range m.tags {
		if !slices.ContainsFunc(used, func(u string) bool { return strings.EqualFold(u, t) }) {
			sugg = append(sugg, prefix+t)
		}
	}
	m.tagsF.SetSuggestions(sugg)
}

// parseTags splits a comma-separated list, trimming blanks and dropping case-insensitive duplicates.
func parseTags(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		t := strings.TrimSpace(part)
		if t == "" || slices.ContainsFunc(out, func(o string) bool { return strings.EqualFold(o, t) }) {
			continue
		}
		out = append(out, t)
	}
	return out
}

// diffTags returns the tags to add to and remove from current so that it matches want.
func diffTags(current, want []string) (add, remove []string) {
	has := func(list []string, t string) bool {
		return slices.ContainsFunc(list, func(o string) bool { return strings.EqualFold(o, t) })
	}
	for _, t := range want {
		if !has(current, t) {
			add = append(add, t)
		}
	}
	for _, t := range current {
		if !has(want, t) {
			remove = append(remove, t)
		}
	}
	return add, remove
}

// withNotice attaches the plan's implicit-creation notices to a reloaded list message.
func withNotice(msg tea.Msg, notices []string) tea.Msg {
	if l, ok := msg.(loadedMsg); ok {
		l.notice = strings.Join(notices, "; ")
		return l
	}
	return msg
}

func (m *Model) submitAdd() tea.Cmd {
	req := app.AddTxRequest{
		Account: m.accountF.Value(), Amount: m.amountF.Value(), Date: m.dateF.Value(),
		Description: m.descF.Value(), Category: m.categoryF.Value(), Notes: m.notesF.Value(),
		Status: m.statusF.Value(), Kind: m.kindF.Value(),
		Payee: strings.TrimSpace(m.payeeF.Value()), Tags: parseTags(m.tagsF.Value()),
	}
	uc, listReq := m.uc, m.req
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := uc.PlanAddTx(ctx, req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if _, err := uc.ApplyAddTx(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return withNotice(reload(ctx, uc, listReq, false), plan.Notices)
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
	if payee := strings.TrimSpace(m.payeeF.Value()); payee != m.editPayee {
		req.Payee = &payee
	}
	req.AddTags, req.RemoveTags = diffTags(m.editTags, parseTags(m.tagsF.Value()))
	uc, listReq := m.uc, m.req
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := uc.PlanEditTx(ctx, req)
		if err != nil {
			if !confirmReconciled && errors.Is(err, domain.ErrReconciled) {
				return needReconcileMsg{purpose: confirmEditReconciled}
			}
			return screens.ErrorMsg{Err: err}
		}
		if _, err := uc.ApplyEditTx(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return withNotice(reload(ctx, uc, listReq, false), plan.Notices)
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
