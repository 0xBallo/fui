package dashboard

import (
	"context"
	"errors"
	"fmt"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/theme"
)

// runwaySection shows how long the liquidity lasts as a calendar countdown to the zero date.
type runwaySection struct {
	uc UseCases
	th theme.Theme

	asOf      domain.Date // period the pending/loaded result belongs to
	res       reporting.RunwayResult
	available bool
	loaded    bool
}

type runwayMsg struct {
	asOf      domain.Date
	res       reporting.RunwayResult
	available bool
}

func (s *runwaySection) load(p sectionParams) tea.Cmd {
	s.asOf = p.AsOf
	return func() tea.Msg {
		res, err := s.uc.Runway(context.Background(), reporting.RunwayParams{
			AsOf: p.AsOf, BaseCurrency: p.Base, MonthStartDay: p.MonthStartDay,
		})
		switch {
		case errors.Is(err, domain.ErrNotEnoughData):
			return runwayMsg{asOf: p.AsOf}
		case err != nil:
			return screens.ErrorMsg{Err: fmt.Errorf("load runway: %w", err)}
		}
		return runwayMsg{asOf: p.AsOf, res: res, available: true}
	}
}

func (s *runwaySection) update(msg tea.Msg) (tea.Cmd, bool) {
	m, ok := msg.(runwayMsg)
	if !ok {
		return nil, false
	}
	if m.asOf != s.asOf {
		return nil, true // stale response for a previous period
	}
	s.res, s.available, s.loaded = m.res, m.available, true
	return nil, true
}

func (s *runwaySection) view(int) string {
	if !s.loaded {
		return ""
	}
	head := s.th.Header.Render("Runway") + "\n"
	if !s.available {
		return head + s.th.Muted.Render("not enough data (needs 3 complete months)")
	}
	y, m, d := countdown(s.asOf, s.res.ZeroDate)
	return head + s.th.Title.Render(fmt.Sprintf("%dy %dm %dd", y, m, d)) + "\n" +
		s.th.Muted.Render("zero on "+s.res.ZeroDate.String())
}

func (*runwaySection) keys() []key.Binding { return nil }

// countdown is the calendar difference from → to; zero when to is not after from. Whole months
// are counted with end-of-month clamping, the remainder is in days.
func countdown(from, to domain.Date) (y, m, d int) {
	if !to.After(from) {
		return 0, 0, 0
	}
	months := (to.Year()-from.Year())*12 + to.Month() - from.Month()
	if to.Day() < from.Day() {
		months--
	}
	anchor, err := from.AddMonths(months)
	if err != nil {
		return 0, 0, 0
	}
	days := int(to.In(time.UTC).Sub(anchor.In(time.UTC)).Hours() / 24)
	return months / 12, months % 12, days
}
