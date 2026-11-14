package dashboard

import (
	"context"
	"errors"
	"strconv"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/theme"
)

// InsightUseCases is what the insights panel needs; it matches internal/app/reporting.Insights.
// It is separate from UseCases so the panel stays optional: without it the panel is hidden.
type InsightUseCases interface {
	Run(ctx context.Context) ([]reporting.InsightView, error)
	Dismiss(ctx context.Context, key string) error
	AcceptSubscription(ctx context.Context, key string) (reporting.AcceptResult, error)
}

// insightMaxRows bounds the panel height so it fits the 80x24 layout.
const insightMaxRows = 4

// insightKeys are the panel bindings. Quick add owns "a" on the dashboard itself, so the panel
// is entered with "i" and "a" only means "accept" while it is focused.
type insightKeys struct {
	Focus, Up, Down, Dismiss, Accept, Leave key.Binding
}

func defaultInsightKeys() insightKeys {
	return insightKeys{
		Focus:   key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "insights")),
		Up:      key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
		Down:    key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
		Dismiss: key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "dismiss insight")),
		Accept:  key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "accept subscription")),
		Leave:   key.NewBinding(key.WithKeys("esc", "i"), key.WithHelp("esc", "leave insights")),
	}
}

func (k insightKeys) bindings() []key.Binding {
	return []key.Binding{k.Focus, k.Dismiss, k.Accept}
}

// insightPanel is the optional insights list of the dashboard.
type insightPanel struct {
	uc      InsightUseCases
	km      insightKeys
	items   []reporting.InsightView
	cursor  int
	focused bool
	loc     domain.Locale
	th      theme.Theme
	width   int
}

// insightsLoadedMsg carries fresh insights back into Update.
type insightsLoadedMsg struct {
	items    []reporting.InsightView
	disabled bool
}

// SetInsights enables the insights panel over uc; nil hides it.
func (m *Model) SetInsights(uc InsightUseCases) {
	m.ins = insightPanel{uc: uc, km: defaultInsightKeys(), loc: m.loc, th: m.th, width: m.width}
}

func (p *insightPanel) enabled() bool { return p.uc != nil }

func (p *insightPanel) load() tea.Cmd {
	if !p.enabled() {
		return nil
	}
	return func() tea.Msg {
		items, err := p.uc.Run(context.Background())
		switch {
		case errors.Is(err, app.ErrModuleDisabled):
			return insightsLoadedMsg{disabled: true}
		case err != nil:
			return screens.ErrorMsg{Err: err}
		}
		return insightsLoadedMsg{items: items}
	}
}

// insightDoneMsg reports a finished dismiss/accept, followed by a reload.
type insightDoneMsg struct{ status string }

func (p *insightPanel) update(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case insightsLoadedMsg:
		if msg.disabled {
			p.uc = nil
		}
		p.items = msg.items
		p.cursor = min(p.cursor, max(0, len(p.items)-1))
		if len(p.items) == 0 {
			p.focused = false
		}
		return nil, true
	case insightDoneMsg:
		return tea.Batch(p.load(), func() tea.Msg { return screens.StatusMsg{Text: msg.status} }), true
	case tea.KeyMsg:
		return p.handleKey(msg)
	}
	return nil, false
}

func (p *insightPanel) handleKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	if !p.enabled() {
		return nil, false
	}
	if !p.focused {
		if key.Matches(msg, p.km.Focus) && len(p.items) > 0 {
			p.focused = true
			return nil, true
		}
		return nil, false
	}
	switch {
	case key.Matches(msg, p.km.Leave):
		p.focused = false
	case key.Matches(msg, p.km.Up):
		p.cursor = max(0, p.cursor-1)
	case key.Matches(msg, p.km.Down):
		p.cursor = min(len(p.items)-1, p.cursor+1)
	case key.Matches(msg, p.km.Dismiss):
		return p.dismiss(), true
	case key.Matches(msg, p.km.Accept):
		return p.accept(), true
	}
	return nil, true
}

func (p *insightPanel) dismiss() tea.Cmd {
	it := p.items[p.cursor]
	return func() tea.Msg {
		if err := p.uc.Dismiss(context.Background(), it.Key); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return insightDoneMsg{status: "Dismissed insight: " + it.Label}
	}
}

func (p *insightPanel) accept() tea.Cmd {
	it := p.items[p.cursor]
	if it.Kind != string(domain.InsightSubscript) {
		return func() tea.Msg {
			return screens.StatusMsg{Text: "Only subscription insights can be accepted"}
		}
	}
	return func() tea.Msg {
		res, err := p.uc.AcceptSubscription(context.Background(), it.Key)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if !res.Created {
			return insightDoneMsg{status: "Recurrence already exists for " + it.Label}
		}
		return insightDoneMsg{status: "Created recurrence for " + it.Label}
	}
}

func (p *insightPanel) view() string {
	if !p.enabled() {
		return ""
	}
	out := p.th.Header.Render("Insights") + "\n"
	if len(p.items) == 0 {
		return out + p.th.Muted.Render("no insights")
	}
	start := 0
	if p.cursor >= insightMaxRows {
		start = p.cursor - insightMaxRows + 1
	}
	end := min(len(p.items), start+insightMaxRows)
	for i := start; i < end; i++ {
		line := p.line(p.items[i])
		if p.width > 4 {
			line = ansi.Truncate(line, p.width-2, "…")
		}
		if p.focused && i == p.cursor {
			out += p.th.Selected.Render("> "+line) + "\n"
		} else {
			out += p.th.Base.Render("  "+line) + "\n"
		}
	}
	hint := "i focus"
	if p.focused {
		hint = "j/k move   x dismiss   a accept subscription   esc leave"
	}
	if more := len(p.items) - end; more > 0 {
		hint += "   (+" + strconv.Itoa(more) + " more)"
	}
	return out + p.th.Muted.Render(hint)
}

// line renders one insight; the kind is always spelled out, never conveyed by colour alone.
func (p *insightPanel) line(it reporting.InsightView) string {
	s := "[" + it.Kind + "] " + it.Label
	if it.Amount.Currency != "" {
		if c, err := domain.ParseCurrency(it.Amount.Currency); err == nil {
			if m, err := domain.NewMoney(it.Amount.Minor, c); err == nil {
				s += " " + domain.FormatAmount(m, p.loc) + " " + it.Amount.Currency
			}
		}
	}
	if it.Detail != "" {
		s += " — " + it.Detail
	}
	return s
}

func (p *insightPanel) keyBindings() []key.Binding {
	if !p.enabled() {
		return nil
	}
	return p.km.bindings()
}
