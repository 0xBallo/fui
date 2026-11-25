package transactions

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/bubbles/key"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

func (m *Model) openFilterForm() {
	m.filterAccount = components.NewAutocomplete("Account:", "any", m.th)
	m.filterAccount.SetSuggestions(m.accounts)
	if len(m.req.Accounts) > 0 {
		m.filterAccount.SetValue(m.req.Accounts[0])
	}
	m.filterCategory = components.NewAutocomplete("Category:", "any", m.th)
	m.filterCategory.SetSuggestions(m.categories)
	if len(m.req.Categories) > 0 {
		m.filterCategory.SetValue(m.req.Categories[0])
	}
	m.filterStatus = components.NewAutocomplete("Status:", "any", m.th)
	m.filterStatus.SetSuggestions(txStatuses())
	if len(m.req.Statuses) > 0 {
		m.filterStatus.SetValue(m.req.Statuses[0])
	}
	m.filterKind = components.NewAutocomplete("Kind:", "any", m.th)
	m.filterKind.SetSuggestions(txKinds())
	if len(m.req.Kinds) > 0 {
		m.filterKind.SetValue(m.req.Kinds[0])
	}
	m.filterUncategorized = components.NewAutocomplete("Uncategorized:", "no", m.th)
	m.filterUncategorized.SetSuggestions([]string{"yes", "no"})
	if m.req.Uncategorized {
		m.filterUncategorized.SetValue("yes")
	}
	m.filterSign = components.NewAutocomplete("Income/Expense:", "any", m.th)
	m.filterSign.SetSuggestions([]string{string(domain.TxSignIncome), string(domain.TxSignExpense)})
	if m.req.Sign != "" && m.req.Sign != domain.TxSignAny {
		m.filterSign.SetValue(string(m.req.Sign))
	}
	m.filterFrom = components.NewAutocomplete("From:", "any", m.th)
	m.filterFrom.SetValue(m.req.From)
	m.filterTo = components.NewAutocomplete("To:", "any", m.th)
	m.filterTo.SetValue(m.req.To)
	m.filterMin = components.NewAutocomplete("Min amount:", "any", m.th)
	m.filterMin.SetValue(m.req.MinAmount)
	m.filterMax = components.NewAutocomplete("Max amount:", "any", m.th)
	m.filterMax.SetValue(m.req.MaxAmount)
	m.filterForm = components.NewForm(
		&m.filterAccount, &m.filterCategory, &m.filterStatus, &m.filterKind,
		&m.filterUncategorized, &m.filterSign,
		&m.filterFrom, &m.filterTo, &m.filterMin, &m.filterMax,
	)
	m.mode = modeFilter
	m.filterForm.Focus()
}

// ApplyFilter replaces the current filter with req, as when drilling down from a report, and
// reloads from the top of the list. Paging fields of req are ignored. The filter shows as chips,
// removable with the usual chip keys.
func (m *Model) ApplyFilter(req app.ListTxRequest) tea.Cmd {
	req.After, req.Limit = nil, 0
	m.req = req
	m.chipFocus = 0
	m.mode = modeList
	m.next = nil
	m.table.SetCursor(0)
	return m.load()
}

func (m *Model) handleFilterKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	if m.picker.IsOpen() {
		m.picker, _ = m.picker.Update(msg)
		if v, ok := m.picker.Picked(); ok && m.pickTarget != nil {
			m.pickTarget.SetValue(v)
		}
		if !m.picker.IsOpen() {
			m.pickTarget = nil
		}
		return m, nil
	}
	if key.Matches(msg, m.km.PickDate) && m.openDatePicker() {
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.mode = modeList
		return m, nil
	case "shift+tab":
		m.filterForm.Prev()
		return m, nil
	case "enter":
		if m.filterForm.FocusedOpen() {
			m.filterForm.Update(msg)
			return m, nil
		}
		if !m.filterForm.AtLast() {
			m.filterForm.Next()
			return m, nil
		}
		return m, m.submitFilter()
	}
	m.filterForm.Update(msg)
	return m, nil
}

// openDatePicker opens the picker on the focused From/To field, starting from its value when it is
// a valid date and from today otherwise. It reports whether a date field had focus.
func (m *Model) openDatePicker() bool {
	switch {
	case m.filterFrom.Focused():
		m.pickTarget = &m.filterFrom
	case m.filterTo.Focused():
		m.pickTarget = &m.filterTo
	default:
		return false
	}
	initial := m.today()
	if d, err := domain.ParseDate(m.pickTarget.Value()); err == nil {
		initial = d
	}
	m.picker.Open(initial.In(time.UTC))
	return true
}

// today returns the current date from the use cases (the clock lives in app.Service), or the
// zero date if it cannot be resolved.
func (m *Model) today() domain.Date {
	d, err := m.uc.Today()
	if err != nil {
		return domain.Date{}
	}
	return d
}

// submitFilter rebuilds the request from the form on top of the incoming one, so criteria the form
// has no field for (Month, Year, Tags, Payees, Batch, Text, sort) and extra values of the
// single-value fields survive an edit.
func (m *Model) submitFilter() tea.Cmd {
	prev := m.req
	req := prev
	req.From, req.To = m.filterFrom.Value(), m.filterTo.Value()
	req.MinAmount, req.MaxAmount = m.filterMin.Value(), m.filterMax.Value()
	if (req.From != "" && req.From != prev.From) || (req.To != "" && req.To != prev.To) {
		// A new explicit range replaces the Month/Year convenience period, which excludes it.
		req.Month, req.Year = "", 0
	}
	req.Accounts = formList(m.filterAccount.Value(), prev.Accounts)
	req.Categories = formList(m.filterCategory.Value(), prev.Categories)
	req.Statuses = formList(m.filterStatus.Value(), prev.Statuses)
	req.Kinds = formList(m.filterKind.Value(), prev.Kinds)
	req.Uncategorized = m.filterUncategorized.Value() == "yes"
	switch m.filterSign.Value() {
	case string(domain.TxSignIncome):
		req.Sign = domain.TxSignIncome
	case string(domain.TxSignExpense):
		req.Sign = domain.TxSignExpense
	default:
		if prev.Sign != domain.TxSignAny {
			req.Sign = ""
		}
	}
	m.req = req
	return m.load()
}

// formList returns the filter list a single-value form field stands for: unchanged when the field
// still shows the first incoming value, empty when cleared, otherwise just the typed value.
func formList(v string, prev []string) []string {
	switch {
	case v == "":
		return nil
	case len(prev) > 0 && prev[0] == v:
		return prev
	}
	return []string{v}
}

func (m *Model) viewFilter() string {
	if m.picker.IsOpen() {
		return m.th.Title.Render("Pick a date") + "\n" + m.picker.View()
	}
	hint := ""
	if m.filterFrom.Focused() || m.filterTo.Focused() {
		hint = "\n" + m.th.Muted.Render(m.km.PickDate.Help().Key+" "+m.km.PickDate.Help().Desc)
	}
	return m.th.Title.Render("Filter transactions") + "\n\n" + m.filterForm.View() + hint
}

// filterChip is one active filter rendered as a removable chip in the header above the list.
type filterChip struct {
	kind, label, value string
}

// periodChips lists the date-range chips of m.req.
func (m *Model) periodChips() []filterChip {
	var chips []filterChip
	if m.req.From != "" {
		chips = append(chips, filterChip{"from", "From", m.req.From})
	}
	if m.req.To != "" {
		chips = append(chips, filterChip{"to", "To", m.req.To})
	}
	if m.req.Month != "" {
		chips = append(chips, filterChip{"month", "Month", m.req.Month})
	}
	if m.req.Year != 0 {
		chips = append(chips, filterChip{"year", "Year", strconv.Itoa(m.req.Year)})
	}
	return chips
}

// activeChips lists every non-empty filter field of m.req as a removable chip, in a stable order.
func (m *Model) activeChips() []filterChip {
	chips := m.periodChips()
	for _, a := range m.req.Accounts {
		chips = append(chips, filterChip{"account", "Account", a})
	}
	for _, c := range m.req.Categories {
		chips = append(chips, filterChip{"category", "Category", c})
	}
	for _, p := range m.req.Payees {
		chips = append(chips, filterChip{"payee", "Payee", p})
	}
	for _, t := range m.req.Tags {
		chips = append(chips, filterChip{"tag", "Tag", t})
	}
	for _, s := range m.req.Statuses {
		chips = append(chips, filterChip{"status", "Status", s})
	}
	for _, k := range m.req.Kinds {
		chips = append(chips, filterChip{"kind", "Kind", k})
	}
	if m.req.Uncategorized {
		chips = append(chips, filterChip{"uncategorized", "Uncategorized", "yes"})
	}
	if m.req.Sign != "" && m.req.Sign != domain.TxSignAny {
		chips = append(chips, filterChip{"sign", "Income/Expense", string(m.req.Sign)})
	}
	if m.req.MinAmount != "" {
		chips = append(chips, filterChip{"min", "Min", m.req.MinAmount})
	}
	if m.req.MaxAmount != "" {
		chips = append(chips, filterChip{"max", "Max", m.req.MaxAmount})
	}
	if m.req.Text != "" {
		chips = append(chips, filterChip{"text", "Search", m.req.Text})
	}
	return chips
}

// chipsView renders the active filter chips, highlighting the one with chip focus.
func (m *Model) chipsView() string {
	chips := m.activeChips()
	if len(chips) == 0 {
		return ""
	}
	if m.chipFocus >= len(chips) {
		m.chipFocus = len(chips) - 1
	}
	parts := make([]string, len(chips))
	for i, c := range chips {
		text := fmt.Sprintf("%s: %s ×", c.label, c.value)
		if i == m.chipFocus {
			parts[i] = m.th.Selected.Render(text)
		} else {
			parts[i] = m.th.Muted.Render(text)
		}
	}
	return strings.Join(parts, "  ")
}

// handleChipKey handles the chip-focus and chip-removal bindings in list mode, reporting whether
// msg matched one of them.
func (m *Model) handleChipKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	switch {
	case key.Matches(msg, m.km.PrevChip):
		m.moveChipFocus(-1)
	case key.Matches(msg, m.km.NextChip):
		m.moveChipFocus(1)
	case key.Matches(msg, m.km.RemoveChip):
		return true, m.removeFocusedChip()
	default:
		return false, nil
	}
	return true, nil
}

// moveChipFocus shifts the focused chip by delta, wrapping around the active chip list.
func (m *Model) moveChipFocus(delta int) {
	n := len(m.activeChips())
	if n == 0 {
		m.chipFocus = 0
		return
	}
	m.chipFocus = (m.chipFocus + delta + n) % n
}

// removeFocusedChip clears the filter field the focused chip represents and reloads.
func (m *Model) removeFocusedChip() tea.Cmd {
	chips := m.activeChips()
	if len(chips) == 0 {
		return nil
	}
	if m.chipFocus >= len(chips) {
		m.chipFocus = len(chips) - 1
	}
	c := chips[m.chipFocus]
	m.clearChip(c)
	if m.chipFocus > 0 {
		m.chipFocus--
	}
	return m.load()
}

// clearChip resets the filter field chip c stands for.
func (m *Model) clearChip(c filterChip) {
	switch c.kind {
	case "from":
		m.req.From = ""
	case "to":
		m.req.To = ""
	case "month":
		m.req.Month = ""
	case "year":
		m.req.Year = 0
	case "min":
		m.req.MinAmount = ""
	case "max":
		m.req.MaxAmount = ""
	case "text":
		m.req.Text = ""
	case "uncategorized":
		m.req.Uncategorized = false
	case "sign":
		m.req.Sign = ""
	default:
		m.clearListChip(c)
	}
}

// clearListChip removes the single value chip c stands for from its multi-value filter field.
func (m *Model) clearListChip(c filterChip) {
	switch c.kind {
	case "account":
		m.req.Accounts = removeValue(m.req.Accounts, c.value)
	case "category":
		m.req.Categories = removeValue(m.req.Categories, c.value)
	case "payee":
		m.req.Payees = removeValue(m.req.Payees, c.value)
	case "tag":
		m.req.Tags = removeValue(m.req.Tags, c.value)
	case "status":
		m.req.Statuses = removeValue(m.req.Statuses, c.value)
	case "kind":
		m.req.Kinds = removeValue(m.req.Kinds, c.value)
	}
}

// removeValue returns s without its first occurrence of v, leaving s untouched.
func removeValue(s []string, v string) []string {
	out := make([]string, 0, len(s))
	for _, x := range s {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

func (m *Model) openCategorize() {
	ids := m.table.SelectedIDs()
	if len(ids) == 0 {
		row := m.table.SelectedRow()
		if row == nil {
			return
		}
		m.table.ToggleSelect()
	}
	m.categorizeField = components.NewAutocomplete("New category:", "", m.th)
	m.categorizeField.SetSuggestions(m.categories)
	m.mode = modeCategorize
	m.categorizeField.Focus()
}

func (m *Model) handleCategorizeKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		return m, nil
	case "enter":
		if m.categorizeField.DropdownOpen() && m.categorizeField.Value() != "" {
			updated, cmd := m.categorizeField.Update(msg)
			m.categorizeField = updated
			return m, cmd
		}
		return m, m.submitCategorize(false)
	}
	updated, cmd := m.categorizeField.Update(msg)
	m.categorizeField = updated
	return m, cmd
}

// parseTxIDs parses every table row ID in strs back into the TransactionID it was formatted from.
func parseTxIDs(strs []string) ([]domain.TransactionID, error) {
	ids := make([]domain.TransactionID, len(strs))
	for i, s := range strs {
		id, err := parseTxID(s)
		if err != nil {
			return nil, err
		}
		ids[i] = id
	}
	return ids, nil
}

func (m *Model) submitCategorize(confirmReconciled bool) tea.Cmd {
	ids, parseErr := parseTxIDs(m.table.SelectedIDs())
	category := m.categorizeField.Value()
	uc, listReq := m.uc, m.req
	return func() tea.Msg {
		if parseErr != nil {
			return screens.ErrorMsg{Err: parseErr}
		}
		ctx := context.Background()
		plan, err := uc.PlanCategorizeTx(ctx, app.CategorizeTxRequest{
			IDs: ids, Category: category, ConfirmReconciled: confirmReconciled,
		})
		if err != nil {
			if !confirmReconciled && errors.Is(err, domain.ErrReconciled) {
				return needReconcileMsg{purpose: confirmCategorizeReconciled}
			}
			return screens.ErrorMsg{Err: err}
		}
		if err := uc.ApplyCategorizeTx(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return reload(ctx, uc, listReq, true)
	}
}

func (m *Model) viewCategorize() string {
	return components.Modal(m.th, m.width, m.height, m.th.Title.Render("Categorize")+"\n"+m.categorizeField.View())
}
