package lists

import (
	"context"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/app/rules"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/theme"
)

// rulesMode tracks which panel of the rules sub-view is active.
type rulesMode int

const (
	rulesList rulesMode = iota
	rulesForm
	rulesConfirm
	rulesTest
)

// Rules is the settings sub-view that lists categorization rules and lets the user add, edit,
// remove and test them against a transaction or a hypothetical text/account/amount.
type Rules struct {
	uc RulesUseCases
	th theme.Theme
	km KeyMap

	width, height int
	mode          rulesMode

	rules []domain.Rule
	table components.Table

	accounts      []domain.Account
	categories    []domain.Category
	payees        []domain.Payee
	tags          []domain.Tag
	accountNames  []string
	categoryNames []string
	payeeNames    []string
	tagNames      []string

	form    components.Form
	fields  ruleFormFields
	editing string // name of the rule being edited, empty when adding

	confirm components.ConfirmDialog
	pending rules.RulePlan

	testForm    components.Form
	testText    components.Autocomplete
	testAccount components.Autocomplete
	testAmount  components.Autocomplete
	testResult  string

	err error // last submit error, shown inline until the next state change
}

// ruleFormFields groups the add/edit form's autocomplete fields so they can be built and read
// together without a long parameter list.
type ruleFormFields struct {
	name, priority, enabled         components.Autocomplete
	descContains, descEquals        components.Autocomplete
	minAbs, maxAbs, sign            components.Autocomplete
	accounts, category, payee, tags components.Autocomplete
}

var _ screens.Screen = (*Rules)(nil)

// NewRules builds the rules sub-view over uc.
func NewRules(uc RulesUseCases, th theme.Theme) *Rules {
	m := &Rules{uc: uc, th: th, km: DefaultKeyMap()}
	m.table = components.NewTable([]components.Column{
		{Title: "Rule", Width: 20, Align: lipgloss.Left},
		{Title: "Priority", Width: 8, Align: lipgloss.Right},
		{Title: "Enabled", Width: 8, Align: lipgloss.Left},
		{Title: "Summary", Width: 36, Align: lipgloss.Left},
	}, th)
	m.confirm = components.NewConfirmDialog(th)
	return m
}

// Init loads the rules and the reference lists used for suggestions and name resolution.
func (m *Rules) Init() tea.Cmd { return m.load() }

// SetSize adapts the table to the available terminal size.
func (m *Rules) SetSize(w, h int) {
	m.width, m.height = w, h
	m.table.SetHeight(max(3, h-8))
	m.table.SetWidth(w)
}

// Title identifies the sub-view in the settings shell.
func (m *Rules) Title() string { return "Rules" }

// KeyMap exposes the bindings for the global help view, including Test.
func (m *Rules) KeyMap() []key.Binding { return m.km.BindingsWithTest() }

// InputActive reports whether a form is capturing keystrokes.
func (m *Rules) InputActive() bool { return m.mode == rulesForm || m.mode == rulesTest }

type rulesLoadedMsg struct {
	rules      []domain.Rule
	accounts   []domain.Account
	categories []domain.Category
	payees     []domain.Payee
	tags       []domain.Tag
}

func (m *Rules) load() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		rs, err := m.uc.List(ctx)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		accts, err := m.uc.ListAccounts(ctx)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		cats, err := m.uc.ListCategories(ctx)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		payees, err := m.uc.ListPayees(ctx)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		tags, err := m.uc.ListTags(ctx)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return rulesLoadedMsg{rules: rs, accounts: accts, categories: cats, payees: payees, tags: tags}
	}
}

// Update advances the sub-view state; it performs no I/O itself.
func (m *Rules) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case rulesLoadedMsg:
		m.applyLoaded(msg)
		return m, nil
	case rulePlannedMsg:
		m.err = nil
		m.pending = msg.plan
		m.confirm.Show("Delete rule", "Delete rule "+quote(msg.plan.Rule.Name)+"?")
		m.mode = rulesConfirm
		return m, nil
	case ruleTestedMsg:
		m.testResult = m.formatTestResult(msg.result)
		return m, nil
	case screens.ErrorMsg:
		m.err = msg.Err
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Rules) handleKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch m.mode {
	case rulesForm:
		return m.handleFormKey(msg)
	case rulesConfirm:
		return m.handleConfirmKey(msg)
	case rulesTest:
		return m.handleTestKey(msg)
	case rulesList:
		return m.handleListKey(msg)
	}
	return m.handleListKey(msg)
}

// applyLoaded stores the loaded rules and reference lists, deriving the suggestion name slices.
func (m *Rules) applyLoaded(msg rulesLoadedMsg) {
	m.err = nil
	m.rules, m.accounts, m.categories, m.payees, m.tags = msg.rules, msg.accounts, msg.categories, msg.payees, msg.tags
	m.accountNames = accountNames(msg.accounts)
	m.categoryNames = domainCategoryNames(msg.categories)
	m.payeeNames = payeeNames(msg.payees)
	m.tagNames = domainTagNames(msg.tags)
	m.table.SetRows(m.rows())
	m.mode = rulesList
}

func (m *Rules) handleListKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch {
	case key.Matches(msg, m.km.Up):
		m.table.MoveUp(1)
	case key.Matches(msg, m.km.Down):
		m.table.MoveDown(1)
	case key.Matches(msg, m.km.Add):
		m.openForm(nil)
	case key.Matches(msg, m.km.Edit):
		if row := m.table.SelectedRow(); row != nil {
			m.openForm(m.ruleByName(row.ID))
		}
	case key.Matches(msg, m.km.Remove):
		if row := m.table.SelectedRow(); row != nil {
			return m, m.planRemove(row.ID)
		}
	case key.Matches(msg, m.km.Test):
		m.openTestForm()
	}
	return m, nil
}

func (m *Rules) handleConfirmKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	yes, no := m.confirm.HandleKey(msg)
	switch {
	case yes:
		return m, m.submitRemove()
	case no:
		m.mode = rulesList
	}
	return m, nil
}

func (m *Rules) ruleByName(name string) *domain.Rule {
	for i := range m.rules {
		if m.rules[i].Name == name {
			return &m.rules[i]
		}
	}
	return nil
}

func (m *Rules) rows() []components.Row {
	rows := make([]components.Row, len(m.rules))
	for i, r := range m.rules {
		enabled := "no"
		if r.Enabled {
			enabled = "yes"
		}
		rows[i] = components.Row{ID: r.Name, Cells: []string{
			r.Name, strconv.Itoa(r.Priority), enabled, ruleSummary(r),
		}}
	}
	return rows
}

// ruleSummary renders a one-line description of a rule's conditions and actions for the list.
func ruleSummary(r domain.Rule) string {
	var parts []string
	if r.Conditions.DescriptionContains != "" {
		parts = append(parts, "contains "+quote(r.Conditions.DescriptionContains))
	}
	if r.Conditions.DescriptionEquals != "" {
		parts = append(parts, "equals "+quote(r.Conditions.DescriptionEquals))
	}
	if r.Actions.CategoryID != 0 {
		parts = append(parts, "-> category")
	}
	if r.Actions.PayeeID != 0 {
		parts = append(parts, "-> payee")
	}
	if len(r.Actions.TagIDs) > 0 {
		parts = append(parts, "-> tags")
	}
	return strings.Join(parts, ", ")
}

// View renders the active panel.
func (m *Rules) View() string {
	var body string
	switch m.mode {
	case rulesForm:
		body = components.Modal(m.th, m.width, m.height, m.viewForm())
	case rulesConfirm:
		body = components.Modal(m.th, m.width, m.height, m.confirm.View())
	case rulesTest:
		body = components.Modal(m.th, m.width, m.height, m.viewTest())
	case rulesList:
		body = m.viewList()
	default:
		body = m.viewList()
	}
	return withError(m.th, body, m.err)
}

func (m *Rules) viewList() string {
	return m.th.Title.Render("Rules") + "\n\n" + m.table.View()
}

type rulePlannedMsg struct{ plan rules.RulePlan }

func (m *Rules) planRemove(name string) tea.Cmd {
	return func() tea.Msg {
		plan, err := m.uc.PlanRemove(context.Background(), name)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return rulePlannedMsg{plan: plan}
	}
}

func (m *Rules) submitRemove() tea.Cmd {
	plan := m.pending
	return func() tea.Msg {
		ctx := context.Background()
		if err := m.uc.ApplyRemove(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.reload(ctx)
	}
}

func (m *Rules) reload(ctx context.Context) tea.Msg {
	rs, err := m.uc.List(ctx)
	if err != nil {
		return screens.ErrorMsg{Err: err}
	}
	return rulesLoadedMsg{rules: rs, accounts: m.accounts, categories: m.categories, payees: m.payees, tags: m.tags}
}

func quote(s string) string { return "\"" + s + "\"" }

func accountNames(accts []domain.Account) []string {
	names := make([]string, len(accts))
	for i, a := range accts {
		names[i] = a.Name
	}
	return names
}

func domainCategoryNames(cats []domain.Category) []string {
	names := make([]string, len(cats))
	for i, c := range cats {
		names[i] = c.Name
	}
	return names
}

func payeeNames(payees []domain.Payee) []string {
	names := make([]string, len(payees))
	for i, p := range payees {
		names[i] = p.Name
	}
	return names
}

func domainTagNames(tags []domain.Tag) []string {
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Name
	}
	return names
}
