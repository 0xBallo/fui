package transactions

import (
	"context"
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

type splitRowInput struct {
	category components.Autocomplete
	amount   components.Autocomplete
}

type splitSeedRow struct {
	category string
	amount   string
}

type splitLoadedMsg struct {
	id           domain.TransactionID
	total        domain.Money
	baseCategory string
	rows         []splitSeedRow
}

func (m *Model) loadSplitEditor() tea.Cmd {
	row := m.selectedTx()
	if row == nil {
		return nil
	}
	id, amount, baseCategory := row.Transaction.ID, row.Transaction.Amount, row.Category
	uc := m.uc
	return func() tea.Msg {
		ctx := context.Background()
		splits, err := uc.ListSplits(ctx, id)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		cats, err := uc.ListCategories(ctx)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		catNameByID := make(map[domain.CategoryID]string, len(cats))
		for _, c := range cats {
			catNameByID[c.ID] = c.Name
		}
		rows := make([]splitSeedRow, len(splits))
		for i, s := range splits {
			rows[i] = splitSeedRow{
				category: catNameByID[s.Category],
				amount:   domain.FormatMachine(s.Amount),
			}
		}
		return splitLoadedMsg{id: id, total: amount, baseCategory: baseCategory, rows: rows}
	}
}

func (m *Model) openSplitEditor(msg splitLoadedMsg) {
	m.splitTxID = msg.id
	m.splitTotal = msg.total
	m.splitNotice = ""

	if len(msg.rows) >= 2 {
		m.splitRows = make([]splitRowInput, len(msg.rows))
		for i, row := range msg.rows {
			m.splitRows[i] = m.newSplitRowInput(i, row.category, row.amount)
		}
	} else {
		m.splitRows = []splitRowInput{
			m.newSplitRowInput(1, msg.baseCategory, domain.FormatMachine(msg.total)),
			m.newSplitRowInput(2, "", ""),
		}
	}
	m.rebuildSplitForm()
	m.mode = modeSplit
	m.splitForm.Focus()
}

func (m *Model) newSplitRowInput(idx int, category, amount string) splitRowInput {
	cat := components.NewAutocomplete(fmt.Sprintf("Row %d category:", idx), "", m.th)
	cat.SetSuggestions(m.categories)
	cat.SetValue(category)
	amt := components.NewAutocomplete(fmt.Sprintf("Row %d amount:", idx), "e.g. -12.34", m.th)
	amt.SetValue(amount)
	return splitRowInput{category: cat, amount: amt}
}

func (m *Model) rebuildSplitForm() {
	fields := make([]*components.Autocomplete, 0, len(m.splitRows)*2)
	for i := range m.splitRows {
		fields = append(fields, &m.splitRows[i].category, &m.splitRows[i].amount)
	}
	m.splitForm = components.NewForm(fields...)
}

func (m *Model) addSplitRow() {
	next := len(m.splitRows) + 1
	m.splitRows = append(m.splitRows, m.newSplitRowInput(next, "", ""))
	m.rebuildSplitForm()
	_ = m.splitForm.Focus()
}

func (m *Model) removeSplitRow() {
	if len(m.splitRows) <= 2 {
		m.splitNotice = "A split needs at least two rows."
		return
	}
	m.splitRows = m.splitRows[:len(m.splitRows)-1]
	m.rebuildSplitForm()
	_ = m.splitForm.Focus()
}

func (m *Model) handleSplitKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		return m, nil
	case "shift+tab":
		m.splitForm.Prev()
		return m, nil
	case "ctrl+n":
		m.addSplitRow()
		return m, nil
	case "ctrl+x":
		m.removeSplitRow()
		return m, nil
	case "enter":
		if m.splitForm.FocusedOpen() {
			m.splitForm.Update(msg)
			return m, nil
		}
		if !m.splitForm.AtLast() {
			m.splitForm.Next()
			return m, nil
		}
		return m, m.submitSplit()
	}
	m.splitForm.Update(msg)
	return m, nil
}

func (m *Model) submitSplit() tea.Cmd {
	rows := make([]app.SplitRowRequest, len(m.splitRows))
	sum, err := domain.NewMoney(0, m.splitTotal.Currency())
	if err != nil {
		return func() tea.Msg { return screens.ErrorMsg{Err: err} }
	}
	for i := range m.splitRows {
		category := strings.TrimSpace(m.splitRows[i].category.Value())
		amountIn := strings.TrimSpace(m.splitRows[i].amount.Value())
		if category == "" {
			m.splitNotice = fmt.Sprintf("Row %d category is required.", i+1)
			return nil
		}
		if amountIn == "" {
			m.splitNotice = fmt.Sprintf("Row %d amount is required.", i+1)
			return nil
		}
		amount, parseErr := domain.ParseAmount(amountIn, m.loc, m.splitTotal.Currency())
		if parseErr != nil {
			m.splitNotice = fmt.Sprintf("Row %d amount is invalid.", i+1)
			return nil
		}
		sum, err = sum.Add(amount)
		if err != nil {
			return func() tea.Msg { return screens.ErrorMsg{Err: err} }
		}
		rows[i] = app.SplitRowRequest{Category: category, Amount: amountIn}
	}
	if cmp, cmpErr := sum.Cmp(m.splitTotal); cmpErr != nil || cmp != 0 {
		m.splitNotice = fmt.Sprintf("Remaining must be 0 (%s total).", domain.FormatAmount(m.splitTotal, m.loc))
		if cmpErr != nil {
			return func() tea.Msg { return screens.ErrorMsg{Err: cmpErr} }
		}
		return nil
	}

	m.splitNotice = ""
	req := app.SplitTxRequest{ID: m.splitTxID, Rows: rows}
	uc, listReq := m.uc, m.req
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := uc.PlanSplitTx(ctx, req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if err := uc.ApplySplitTx(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return reload(ctx, uc, listReq, false)
	}
}

func (m *Model) splitRemainingLine() string {
	sum, err := domain.NewMoney(0, m.splitTotal.Currency())
	if err != nil {
		return m.th.Error.Render(err.Error())
	}
	invalid := 0
	for i := range m.splitRows {
		amountIn := strings.TrimSpace(m.splitRows[i].amount.Value())
		if amountIn == "" {
			continue
		}
		amount, parseErr := domain.ParseAmount(amountIn, m.loc, m.splitTotal.Currency())
		if parseErr != nil {
			invalid++
			continue
		}
		sum, err = sum.Add(amount)
		if err != nil {
			return m.th.Error.Render(err.Error())
		}
	}
	remaining, err := m.splitTotal.Sub(sum)
	if err != nil {
		return m.th.Error.Render(err.Error())
	}
	remainingText := "Remaining: " + domain.FormatAmount(remaining, m.loc) + " " + remaining.Currency().Code()
	if remaining.IsZero() && invalid == 0 {
		remainingText = m.th.StatusBar.Render(remainingText)
	} else {
		remainingText = m.th.Warning.Render(remainingText)
	}
	if invalid > 0 {
		remainingText += "\n" + m.th.Warning.Render(fmt.Sprintf("%d row amount(s) are invalid.", invalid))
	}
	return remainingText
}

func (m *Model) viewSplit() string {
	out := m.th.Title.Render("Split transaction") + "\n\n" + m.splitForm.View()
	out += "\n\n" + m.splitRemainingLine()
	out += "\n" + m.th.Muted.Render("ctrl+n add row · ctrl+x remove row · enter save · esc cancel")
	if m.splitNotice != "" {
		out += "\n" + m.th.Warning.Render(m.splitNotice)
	}
	return out
}

func (m *Model) openTagsEditor() {
	row := m.selectedTx()
	if row == nil {
		return
	}
	m.tagTxID = row.Transaction.ID
	m.tagNotice = ""

	m.tagAddField = components.NewAutocomplete("Add tags (comma-separated):", "", m.th)
	m.tagAddField.SetSuggestions(m.tags)
	m.tagDropField = components.NewAutocomplete("Remove tags (comma-separated):", "", m.th)
	m.tagDropField.SetSuggestions(m.tags)
	m.tagForm = components.NewForm(&m.tagAddField, &m.tagDropField)
	m.mode = modeTags
	m.tagForm.Focus()
}

func (m *Model) handleTagsKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		return m, nil
	case "shift+tab":
		m.tagForm.Prev()
		return m, nil
	case "enter":
		if m.tagForm.FocusedOpen() {
			m.tagForm.Update(msg)
			return m, nil
		}
		if !m.tagForm.AtLast() {
			m.tagForm.Next()
			return m, nil
		}
		return m, m.submitTags()
	}
	m.tagForm.Update(msg)
	return m, nil
}

func (m *Model) submitTags() tea.Cmd {
	add := parseTagList(m.tagAddField.Value())
	remove := parseTagList(m.tagDropField.Value())
	if len(add) == 0 && len(remove) == 0 {
		m.tagNotice = "Type at least one tag to add or remove."
		return nil
	}

	filteredAdd := add[:0]
	for _, name := range add {
		if !slices.Contains(remove, name) {
			filteredAdd = append(filteredAdd, name)
		}
	}
	add = filteredAdd
	m.tagNotice = ""

	id := m.tagTxID
	uc, listReq := m.uc, m.req
	return func() tea.Msg {
		ctx := context.Background()
		if len(add) > 0 {
			p, err := uc.PlanTagTransaction(ctx, app.TagTransactionRequest{ID: id, Add: add})
			if err != nil {
				return screens.ErrorMsg{Err: err}
			}
			if err := uc.ApplyTagTransaction(ctx, p); err != nil {
				return screens.ErrorMsg{Err: err}
			}
		}
		if len(remove) > 0 {
			p, err := uc.PlanUntagTransaction(ctx, app.UntagTransactionRequest{ID: id, Remove: remove})
			if err != nil {
				return screens.ErrorMsg{Err: err}
			}
			if err := uc.ApplyUntagTransaction(ctx, p); err != nil {
				return screens.ErrorMsg{Err: err}
			}
		}
		return reload(ctx, uc, listReq, false)
	}
}

func (m *Model) viewTags() string {
	out := m.th.Title.Render("Edit transaction tags") + "\n\n" + m.tagForm.View()
	out += "\n\n" + m.th.Muted.Render("enter save · esc cancel")
	if m.tagNotice != "" {
		out += "\n" + m.th.Warning.Render(m.tagNotice)
	}
	return out
}

func parseTagList(in string) []string {
	parts := strings.FieldsFunc(in, func(r rune) bool {
		return r == ',' || r == ';'
	})
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, raw := range parts {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, name)
	}
	return out
}
