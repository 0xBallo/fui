package budget

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/0xballo/fui/internal/app/budgeting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

// picker is an Autocomplete that collects several names: picking a suggestion moves it into
// chosen (shown as chips) and clears the field so the next one can be typed.
type picker struct {
	field  components.Autocomplete
	all    []string
	chosen []string
}

func (p *picker) refreshSuggestions() {
	rest := make([]string, 0, len(p.all))
	for _, n := range p.all {
		if !slices.ContainsFunc(p.chosen, func(c string) bool { return strings.EqualFold(c, n) }) {
			rest = append(rest, n)
		}
	}
	p.field.SetSuggestions(rest)
}

// add moves the field's current text into chosen and reopens the suggestions.
func (p *picker) add() {
	v := strings.TrimSpace(p.field.Value())
	if v != "" && !slices.ContainsFunc(p.chosen, func(c string) bool { return strings.EqualFold(c, v) }) {
		p.chosen = append(p.chosen, v)
	}
	p.field.SetValue("")
	p.refreshSuggestions()
	p.field.Focus()
}

// values returns the chosen names plus any comma-separated text still typed in the field.
func (p *picker) values() []string {
	out := slices.Clone(p.chosen)
	for _, part := range strings.Split(p.field.Value(), ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// budgetFields are the inputs of the new/edit budget form.
type budgetFields struct {
	name, period, amount, carry, warn, over components.Autocomplete
	cats, tags                              picker
}

// namesLoadedMsg carries the existing category and tag names for the pickers.
type namesLoadedMsg struct {
	cats, tags       []string
	curCats, curTags []string // names of the targets of the budget being edited
}

func (b *budgetFields) setSuggestions(n namesLoadedMsg) {
	b.cats.all, b.tags.all = n.cats, n.tags
	if len(b.cats.chosen)+len(b.tags.chosen) == 0 {
		b.cats.chosen, b.tags.chosen = n.curCats, n.curTags
	}
	b.cats.refreshSuggestions()
	b.tags.refreshSuggestions()
}

// loadNames fetches the category and tag names off the Update path. A failure only costs the
// suggestions, so it is not reported: typed names are still validated on submit.
func (m *Model) loadNames(current []domain.BudgetTarget) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		cats, _ := m.uc.Categories(ctx)
		tags, _ := m.uc.Tags(ctx)
		msg := namesLoadedMsg{cats: cats, tags: tags}
		if len(current) > 0 {
			msg.curCats, msg.curTags, _ = m.uc.TargetNames(ctx, current)
		}
		return msg
	}
}

// openBudgetForm opens the new (empty) or edit (prefilled from the selected row) budget form.
func (m *Model) openBudgetForm(md mode) tea.Cmd {
	var cur domain.Budget
	if md == modeEditForm {
		row := m.table.SelectedRow()
		if row == nil {
			return nil
		}
		cur = m.budgetNamed(row.ID)
		m.target = cur.Name
	}
	th := m.th
	prev := m.bf
	m.bf = budgetFields{
		name:   components.NewAutocomplete("Name:", "e.g. Groceries", th),
		cats:   picker{field: components.NewAutocomplete("Categories:", "pick or type, enter adds", th), all: prev.cats.all},
		tags:   picker{field: components.NewAutocomplete("Tags:", "pick or type, enter adds", th), all: prev.tags.all},
		period: components.NewAutocomplete("Period:", "month or year", th),
		amount: components.NewAutocomplete("Amount:", "default amount, e.g. 400", th),
		carry:  components.NewAutocomplete("Carry:", "yes or no", th),
		warn:   components.NewAutocomplete("Warn %:", "80", th),
		over:   components.NewAutocomplete("Over %:", "100", th),
	}
	m.bf.period.SetSuggestions([]string{"month", "year"})
	m.bf.carry.SetSuggestions([]string{"yes", "no"})
	m.bf.cats.refreshSuggestions()
	m.bf.tags.refreshSuggestions()

	fields := []*components.Autocomplete{&m.bf.cats.field, &m.bf.tags.field, &m.bf.period, &m.bf.amount, &m.bf.carry, &m.bf.warn, &m.bf.over}
	if md == modeNewForm {
		fields = append([]*components.Autocomplete{&m.bf.name}, fields...)
		m.bf.warn.SetValue(strconv.Itoa(domain.DefaultBudgetThresholds.Warn))
		m.bf.over.SetValue(strconv.Itoa(domain.DefaultBudgetThresholds.Over))
	} else {
		m.prefill(cur)
	}
	m.form = components.NewForm(fields...)
	m.mode = md
	m.form.Focus()
	return m.loadNames(cur.Targets)
}

// prefill fills the edit form from b. Targets are stored as ids: the chips are filled once their
// names are resolved (namesLoadedMsg); until then, or if the user removes them all, an empty
// picker keeps the current targets.
func (m *Model) prefill(b domain.Budget) {
	m.bf.cats.field.SetValue("")
	m.bf.period.SetValue(string(b.Period))
	if !b.Amount.IsZero() {
		m.bf.amount.SetValue(domain.FormatAmount(b.Amount, m.loc))
	}
	carry := "no"
	if b.Carry {
		carry = "yes"
	}
	m.bf.carry.SetValue(carry)
	th := b.Thresholds
	if th == (domain.BudgetThresholds{}) {
		th = domain.DefaultBudgetThresholds
	}
	m.bf.warn.SetValue(strconv.Itoa(th.Warn))
	m.bf.over.SetValue(strconv.Itoa(th.Over))
}

// budgetNamed finds the budget called name in the loaded status.
func (m *Model) budgetNamed(name string) domain.Budget {
	for _, l := range m.status.Caps {
		if l.Budget.Name == name {
			return l.Budget
		}
	}
	for _, l := range m.status.Envelopes {
		if l.Budget.Name == name {
			return l.Budget
		}
	}
	return domain.Budget{}
}

// focusedPicker returns the picker owning the keyboard, or nil.
func (m *Model) focusedPicker() *picker {
	switch {
	case m.bf.cats.field.Focused():
		return &m.bf.cats
	case m.bf.tags.field.Focused():
		return &m.bf.tags
	}
	return nil
}

// handlePickerKey handles enter (add the suggestion or typed name) and backspace on an empty field
// (drop the last chosen name) for the focused picker.
func (m *Model) handlePickerKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	p := m.focusedPicker()
	if p == nil {
		return false, nil
	}
	switch msg.String() {
	case "enter":
		if !m.form.FocusedOpen() {
			return false, nil
		}
		m.form.Update(msg)
		p.add()
		return true, nil
	case "backspace":
		if p.field.Value() == "" && len(p.chosen) > 0 {
			p.chosen = p.chosen[:len(p.chosen)-1]
			p.refreshSuggestions()
			return true, nil
		}
	}
	return false, nil
}

// viewBudgetForm renders the new/edit form, with the chosen targets as chips under their field.
func (m *Model) viewBudgetForm() string {
	title, name := "New budget", ""
	if m.mode == modeEditForm {
		title, name = "Edit budget", m.th.Muted.Render("Name: "+m.target)+"\n"
	}
	var b strings.Builder
	b.WriteString(m.th.Title.Render(title) + "\n\n" + name)
	if m.mode == modeNewForm {
		b.WriteString(m.bf.name.View() + "\n")
	}
	for _, p := range []*picker{&m.bf.cats, &m.bf.tags} {
		b.WriteString(p.field.View() + "\n")
		if chips := components.Chips(m.th, p.chosen, max(10, m.width-20)); chips != "" {
			b.WriteString("  " + chips + "\n")
		}
	}
	for _, f := range []*components.Autocomplete{&m.bf.period, &m.bf.amount, &m.bf.carry, &m.bf.warn, &m.bf.over} {
		b.WriteString(f.View() + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// submitBudget saves the form through PlanSet+ApplySet. Month stays empty: this sets the budget's
// default amount, never a month override (that is the "s" key). On failure the error reaches the
// status bar and the form stays open with its input.
func (m *Model) submitBudget() tea.Cmd {
	req, err := m.budgetRequest()
	if err != nil {
		return func() tea.Msg { return screens.ErrorMsg{Err: err} }
	}
	month := m.monthIn
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := m.uc.PlanSet(ctx, req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if _, err := m.uc.ApplySet(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.reload(ctx, month)
	}
}

func (m *Model) budgetRequest() (budgeting.SetRequest, error) {
	req := budgeting.SetRequest{
		Name:       m.target,
		Categories: m.bf.cats.values(),
		Tags:       m.bf.tags.values(),
		Amount:     strings.TrimSpace(m.bf.amount.Value()),
		Period:     strings.TrimSpace(m.bf.period.Value()),
	}
	if m.mode == modeNewForm {
		req.Name = strings.TrimSpace(m.bf.name.Value())
	}
	var err error
	if req.Warn, err = parsePercent("warn", m.bf.warn.Value()); err != nil {
		return req, err
	}
	if req.Over, err = parsePercent("over", m.bf.over.Value()); err != nil {
		return req, err
	}
	switch strings.ToLower(strings.TrimSpace(m.bf.carry.Value())) {
	case "":
	case "yes", "y":
		t := true
		req.Carry = &t
	case "no", "n":
		f := false
		req.Carry = &f
	default:
		return req, fmt.Errorf("carry must be yes or no, got %q", m.bf.carry.Value())
	}
	return req, nil
}

// parsePercent parses an optional whole percentage; empty keeps the stored value.
func parsePercent(field, in string) (*int, error) {
	in = strings.TrimSpace(in)
	if in == "" {
		return nil, nil
	}
	n, err := strconv.Atoi(in)
	if err != nil {
		return nil, fmt.Errorf("%s threshold must be a whole number, got %q", field, in)
	}
	return &n, nil
}
