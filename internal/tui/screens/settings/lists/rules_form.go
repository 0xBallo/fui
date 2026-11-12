package lists

import (
	"context"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app/rules"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

// openForm opens the add form, or the edit form prefilled from cur when non-nil. Editing a rule
// cannot rename it (rules.EditRequest has no rename field), so the name field is excluded from
// the form and shown as a read-only label instead (see viewForm).
func (m *Rules) openForm(cur *domain.Rule) {
	m.err = nil
	if cur != nil {
		m.editing = cur.Name
	} else {
		m.editing = ""
	}
	m.buildFormFields(cur)
	fields := []*components.Autocomplete{
		&m.fields.priority, &m.fields.enabled,
		&m.fields.descContains, &m.fields.descEquals,
		&m.fields.minAbs, &m.fields.maxAbs, &m.fields.sign,
		&m.fields.accounts, &m.fields.category, &m.fields.payee, &m.fields.tags,
	}
	if m.editing == "" {
		fields = append([]*components.Autocomplete{&m.fields.name}, fields...)
	}
	m.form = components.NewForm(fields...)
	m.mode = rulesForm
	m.form.Focus()
}

// buildFormFields constructs every autocomplete field, prefilled from cur when editing.
func (m *Rules) buildFormFields(cur *domain.Rule) {
	f := &m.fields
	f.name = components.NewAutocomplete("Name:", "e.g. Rent", m.th)
	f.priority = components.NewAutocomplete("Priority:", "0", m.th)
	f.enabled = components.NewAutocomplete("Enabled:", "yes/no", m.th)
	f.enabled.SetSuggestions([]string{"yes", "no"})
	f.descContains = components.NewAutocomplete("Description contains:", "optional", m.th)
	f.descEquals = components.NewAutocomplete("Description equals:", "optional", m.th)
	f.minAbs = components.NewAutocomplete("Min abs amount (minor units):", "optional", m.th)
	f.maxAbs = components.NewAutocomplete("Max abs amount (minor units):", "optional", m.th)
	f.sign = components.NewAutocomplete("Sign:", "any/inflow/outflow", m.th)
	f.sign.SetSuggestions([]string{"any", "inflow", "outflow"})
	f.accounts = components.NewAutocomplete("Accounts (comma-separated):", "optional", m.th)
	f.accounts.SetSuggestions(m.accountNames)
	f.category = components.NewAutocomplete("Category:", "optional", m.th)
	f.category.SetSuggestions(m.categoryNames)
	f.payee = components.NewAutocomplete("Payee:", "optional", m.th)
	f.payee.SetSuggestions(m.payeeNames)
	f.tags = components.NewAutocomplete("Tags (comma-separated):", "optional", m.th)
	f.tags.SetSuggestions(m.tagNames)
	if cur != nil {
		m.prefillFormFields(cur)
	}
}

// prefillFormFields copies cur's values into the already-built fields, for editing.
func (m *Rules) prefillFormFields(cur *domain.Rule) {
	f := &m.fields
	f.name.SetValue(cur.Name)
	f.priority.SetValue(strconv.Itoa(cur.Priority))
	f.enabled.SetValue(yesNo(cur.Enabled))
	f.descContains.SetValue(cur.Conditions.DescriptionContains)
	f.descEquals.SetValue(cur.Conditions.DescriptionEquals)
	f.minAbs.SetValue(ptrString(cur.Conditions.Amount.MinAbsMinor))
	f.maxAbs.SetValue(ptrString(cur.Conditions.Amount.MaxAbsMinor))
	f.sign.SetValue(string(cur.Conditions.Amount.Sign))
	f.accounts.SetValue(idsToNames(cur.Conditions.AccountIDs, m.accounts))
	f.category.SetValue(categoryNameByID(cur.Actions.CategoryID, m.categories))
	f.payee.SetValue(payeeNameByID(cur.Actions.PayeeID, m.payees))
	f.tags.SetValue(tagIDsToNames(cur.Actions.TagIDs, m.tags))
}

func (m *Rules) handleFormKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = rulesList
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
		return m, m.submitForm()
	}
	m.form.Update(msg)
	return m, nil
}

func (m *Rules) conditionsSpec() rules.ConditionsSpec {
	f := &m.fields
	spec := rules.ConditionsSpec{
		DescriptionContains: f.descContains.Value(), DescriptionEquals: f.descEquals.Value(),
		Sign: f.sign.Value(), Accounts: splitList(f.accounts.Value()),
	}
	spec.MinAbsMinor = parseOptionalInt64(f.minAbs.Value())
	spec.MaxAbsMinor = parseOptionalInt64(f.maxAbs.Value())
	return spec
}

func (m *Rules) actionsSpec() rules.ActionsSpec {
	f := &m.fields
	return rules.ActionsSpec{Category: f.category.Value(), Payee: f.payee.Value(), Tags: splitList(f.tags.Value())}
}

func (m *Rules) submitForm() tea.Cmd {
	if m.editing != "" {
		return m.submitEdit()
	}
	return m.submitAdd()
}

func (m *Rules) submitAdd() tea.Cmd {
	f := &m.fields
	priority, _ := strconv.Atoi(f.priority.Value())
	req := rules.AddRequest{
		Name: f.name.Value(), Priority: priority, Disabled: f.enabled.Value() == "no",
		Conditions: m.conditionsSpec(), Actions: m.actionsSpec(),
	}
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := m.uc.PlanAdd(ctx, req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if _, err := m.uc.ApplyAdd(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.reload(ctx)
	}
}

func (m *Rules) submitEdit() tea.Cmd {
	f := &m.fields
	name := m.editing
	priority, _ := strconv.Atoi(f.priority.Value())
	enabled := f.enabled.Value() != "no"
	conditions := m.conditionsSpec()
	actions := m.actionsSpec()
	req := rules.EditRequest{Name: name, Priority: &priority, Enabled: &enabled, Conditions: &conditions, Actions: &actions}
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := m.uc.PlanEdit(ctx, req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if err := m.uc.ApplyEdit(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.reload(ctx)
	}
}

func (m *Rules) viewForm() string {
	title := "Add rule"
	body := m.form.View()
	if m.editing != "" {
		title = "Edit rule"
		body = staticField(m.th, "Name:", m.fields.name.Value()) + "\n" + body
	}
	return m.th.Title.Render(title) + "\n\n" + body
}

// openTestForm opens the preview form: test an existing transaction by ID, or a hypothetical
// text/account/amount.
func (m *Rules) openTestForm() {
	m.err = nil
	m.testText = components.NewAutocomplete("Text:", "description to test", m.th)
	m.testAccount = components.NewAutocomplete("Account:", "e.g. Checking", m.th)
	m.testAccount.SetSuggestions(m.accountNames)
	m.testAmount = components.NewAutocomplete("Amount (signed minor units):", "e.g. -4250", m.th)
	m.testForm = components.NewForm(&m.testText, &m.testAccount, &m.testAmount)
	m.testResult = ""
	m.mode = rulesTest
	m.testForm.Focus()
}

func (m *Rules) handleTestKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = rulesList
		return m, nil
	case "shift+tab":
		m.testForm.Prev()
		return m, nil
	case "enter":
		if m.testForm.FocusedOpen() {
			m.testForm.Update(msg)
			return m, nil
		}
		if !m.testForm.AtLast() {
			m.testForm.Next()
			return m, nil
		}
		return m, m.submitTest()
	}
	m.testForm.Update(msg)
	return m, nil
}

type ruleTestedMsg struct{ result domain.RuleApplyResult }

func (m *Rules) submitTest() tea.Cmd {
	amount, _ := strconv.ParseInt(m.testAmount.Value(), 10, 64)
	req := rules.TestRequest{Text: m.testText.Value(), Account: m.testAccount.Value(), AmountMinor: amount}
	return func() tea.Msg {
		res, err := m.uc.Test(context.Background(), req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return ruleTestedMsg{result: res}
	}
}

// formatTestResult renders the first-match outcome of a test, naming which rule won each field.
func (m *Rules) formatTestResult(res domain.RuleApplyResult) string {
	if res.CategoryRule == "" && res.PayeeRule == "" && res.TagsRule == "" {
		return "no rule matched"
	}
	var lines []string
	if res.CategoryRule != "" {
		lines = append(lines, "category set by rule "+quote(res.CategoryRule))
	}
	if res.PayeeRule != "" {
		lines = append(lines, "payee set by rule "+quote(res.PayeeRule))
	}
	if res.TagsRule != "" {
		lines = append(lines, "tags set by rule "+quote(res.TagsRule))
	}
	return strings.Join(lines, "\n")
}

func (m *Rules) viewTest() string {
	out := m.th.Title.Render("Test rules") + "\n\n" + m.testForm.View()
	if m.testResult != "" {
		out += "\n\n" + m.th.Header.Render("Result:") + "\n" + m.testResult
	}
	return out
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func ptrString(p *int64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatInt(*p, 10)
}

func parseOptionalInt64(s string) *int64 {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return &v
}

func splitList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
