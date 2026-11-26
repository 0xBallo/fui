package lists

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app/assetting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

// assetFormFields groups the autocomplete fields of the asset, valuation and link forms.
type assetFormFields struct {
	name, kind, date, value, currency, loan components.Autocomplete
}

// assetFormValues is the edit form content as prefilled.
type assetFormValues struct {
	name, kind, date, value, currency string
}

func (m *Assets) newFields() {
	f := &m.fields
	f.name = components.NewAutocomplete("Name:", "e.g. Flat", m.th)
	f.kind = components.NewAutocomplete("Kind:", "property/vehicle/other", m.th)
	f.kind.SetSuggestions([]string{string(domain.AssetKindProperty), string(domain.AssetKindVehicle), string(domain.AssetKindOther)})
	f.date = components.NewAutocomplete("Purchase date:", "YYYY-MM-DD, default today", m.th)
	f.value = components.NewAutocomplete("Purchase value:", "e.g. 250000.00", m.th)
	f.currency = components.NewAutocomplete("Currency:", "optional, default base currency", m.th)
}

func (m *Assets) startForm(title string, submit func() tea.Cmd, fields ...*components.Autocomplete) {
	m.err = nil
	m.title, m.submit = title, submit
	m.form = components.NewForm(fields...)
	m.mode = assetsForm
	m.form.Focus()
}

// openForm opens the add form, or the edit form prefilled from cur when non-nil.
func (m *Assets) openForm(cur *assetting.Item) {
	m.newFields()
	f := &m.fields
	if cur == nil {
		m.startForm("Add asset", m.submitAdd, &f.name, &f.kind, &f.date, &f.value, &f.currency)
		return
	}
	m.initial = assetFormValues{
		name: cur.Asset.Name, kind: string(cur.Asset.Kind), date: cur.Asset.PurchaseDate.String(),
		value: domain.FormatAmount(cur.Asset.PurchaseValue, m.loc), currency: cur.Asset.PurchaseValue.Currency().Code(),
	}
	f.name.SetValue(m.initial.name)
	f.kind.SetValue(m.initial.kind)
	f.date.SetValue(m.initial.date)
	f.value.SetValue(m.initial.value)
	f.currency.SetValue(m.initial.currency)
	m.startForm("Edit asset", m.submitEdit, &f.name, &f.kind, &f.date, &f.value, &f.currency)
}

func (m *Assets) openValueForm(name string) {
	m.newFields()
	f := &m.fields
	f.date = components.NewAutocomplete("Date:", "YYYY-MM-DD, default today", m.th)
	f.value = components.NewAutocomplete("Value:", "e.g. 260000.00", m.th)
	m.startForm("Add valuation: "+name, func() tea.Cmd { return m.submitValue(name) }, &f.date, &f.value)
}

func (m *Assets) openLinkForm(name string, loans []string) {
	m.newFields()
	f := &m.fields
	f.loan = components.NewAutocomplete("Loan:", "pick a loan", m.th)
	f.loan.SetSuggestions(loans)
	m.startForm("Link loan: "+name, func() tea.Cmd { return m.submitLink(name) }, &f.loan)
}

func (m *Assets) handleFormKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = m.base()
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
		return m, m.submit()
	}
	m.form.Update(msg)
	return m, nil
}

func (m *Assets) submitAdd() tea.Cmd {
	f := &m.fields
	req := assetting.AddRequest{
		Name: f.name.Value(), Kind: f.kind.Value(), PurchaseDate: f.date.Value(),
		Value: f.value.Value(), Currency: f.currency.Value(),
	}
	name := req.Name
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := m.uc.PlanAdd(ctx, m.enabled, req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if _, err := m.uc.ApplyAdd(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.reload(ctx, m.keepDetail(name))
	}
}

// keepDetail returns the asset whose detail panel must stay open after a change, or "" in list mode.
func (m *Assets) keepDetail(name string) string {
	if m.view == nil {
		return ""
	}
	return name
}

// changed returns a pointer to now when it differs from before, nil otherwise, so an edit sends
// only the fields the user touched.
func changed(before, now string) *string {
	if before == now {
		return nil
	}
	return &now
}

func (m *Assets) submitEdit() tea.Cmd {
	f, old := &m.fields, m.initial
	req := assetting.EditRequest{
		Ref: old.name, Name: changed(old.name, f.name.Value()), Kind: changed(old.kind, f.kind.Value()),
		PurchaseDate: changed(old.date, f.date.Value()), PurchaseValue: changed(old.value, f.value.Value()),
		Currency: changed(old.currency, f.currency.Value()),
	}
	name := f.name.Value()
	if req.Name == nil && req.Kind == nil && req.PurchaseDate == nil && req.PurchaseValue == nil && req.Currency == nil {
		m.mode = m.base()
		return nil
	}
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := m.uc.PlanEdit(ctx, m.enabled, req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if _, err := m.uc.ApplyEdit(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.reload(ctx, m.keepDetail(name))
	}
}

func (m *Assets) submitValue(name string) tea.Cmd {
	f := &m.fields
	req := assetting.ValueRequest{Asset: name, Date: f.date.Value(), Value: f.value.Value()}
	detail := m.detailName()
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := m.uc.PlanValue(ctx, m.enabled, req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		apply := func(ctx context.Context) error { return m.uc.ApplyValue(ctx, plan) }
		if plan.Overwrite {
			return assetsConfirmMsg{
				title: "Replace valuation", prompt: "Replace the valuation of " + plan.Valuation.Date.String() + "?",
				details: []string{"previous value: " + m.money(plan.Previous)}, apply: apply,
			}
		}
		if err := apply(ctx); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.reload(ctx, detail)
	}
}

func (m *Assets) submitLink(name string) tea.Cmd {
	loan, detail := m.fields.loan.Value(), m.detailName()
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := m.uc.PlanLink(ctx, m.enabled, name, loan)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if err := m.uc.ApplyLink(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.reload(ctx, detail)
	}
}
