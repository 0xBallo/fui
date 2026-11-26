package dashboard

import (
	"context"
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
)

// InsightUseCases is what the insight alerts need; it matches internal/app/reporting.Insights.
// It is separate from UseCases so the source stays optional: without it no insight is shown.
type InsightUseCases interface {
	Run(ctx context.Context) ([]reporting.InsightView, error)
	Dismiss(ctx context.Context, key string) error
	AcceptSubscription(ctx context.Context, key string) (reporting.AcceptResult, error)
}

// SetInsights enables the insight alerts over uc; nil disables them.
func (m *Model) SetInsights(uc InsightUseCases) { m.al.ins = uc }

// insightsMsg carries fresh insights back into Update.
type insightsMsg struct {
	items    []reporting.InsightView
	disabled bool
}

// insightDoneMsg reports a finished dismiss/accept, followed by a reload.
type insightDoneMsg struct{ status string }

func (s *alertsSection) loadInsights() tea.Cmd {
	uc := s.ins
	if uc == nil {
		return nil
	}
	return func() tea.Msg {
		items, err := uc.Run(context.Background())
		switch {
		case errors.Is(err, app.ErrModuleDisabled):
			return insightsMsg{disabled: true}
		case err != nil:
			return screens.ErrorMsg{Err: err}
		}
		return insightsMsg{items: items}
	}
}

func (s *alertsSection) dismiss(it reporting.InsightView) tea.Cmd {
	uc := s.ins
	return func() tea.Msg {
		if err := uc.Dismiss(context.Background(), it.Key); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return insightDoneMsg{status: "Dismissed insight: " + it.Label}
	}
}

func (s *alertsSection) accept(it reporting.InsightView) tea.Cmd {
	uc := s.ins
	if it.Kind != string(domain.InsightSubscript) {
		return func() tea.Msg {
			return screens.StatusMsg{Text: "Only subscription insights can be accepted"}
		}
	}
	return func() tea.Msg {
		res, err := uc.AcceptSubscription(context.Background(), it.Key)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if !res.Created {
			return insightDoneMsg{status: "Recurrence already exists for " + it.Label}
		}
		return insightDoneMsg{status: "Created recurrence for " + it.Label}
	}
}

// insightText renders one insight; the kind is always spelled out, never conveyed by colour alone.
func (s *alertsSection) insightText(it reporting.InsightView) string {
	out := "[" + it.Kind + "] " + it.Label
	if it.Amount.Currency != "" {
		if c, err := domain.ParseCurrency(it.Amount.Currency); err == nil {
			if m, err := domain.NewMoney(it.Amount.Minor, c); err == nil {
				out += " " + domain.FormatAmount(m, s.loc) + " " + it.Amount.Currency
			}
		}
	}
	if it.Detail != "" {
		out += " — " + it.Detail
	}
	return out
}
