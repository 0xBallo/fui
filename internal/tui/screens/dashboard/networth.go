package dashboard

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/theme"
)

// SetNetWorthUseCases wires the optional reporting use cases for net-worth components.
func (m *Model) SetNetWorthUseCases(uc NetWorthUseCases) {
	m.nw.components = uc
}

// grossNet returns the net worth before loans (non-loan lines) and after them (gross plus the
// negative loan lines), both in the base currency.
func grossNet(nw app.NetWorth) (gross, net domain.Money, err error) {
	gross, err = domain.NewMoney(0, nw.Base)
	if err != nil {
		return gross, gross, err
	}
	net = gross
	for _, l := range nw.Lines {
		if l.Account.Kind == domain.AccountLoan {
			if net, err = net.Add(l.Converted); err != nil {
				return gross, net, err
			}
			continue
		}
		if gross, err = gross.Add(l.Converted); err != nil {
			return gross, net, err
		}
		if net, err = net.Add(l.Converted); err != nil {
			return gross, net, err
		}
	}
	return gross, net, nil
}

// netWorthSection shows the net worth as of the end of the selected period.
type netWorthSection struct {
	uc         UseCases
	components NetWorthUseCases
	loc        domain.Locale
	th         theme.Theme

	asOf domain.Date
	nw   app.NetWorth
	err  error
	pt   *reporting.NetWorthPoint
}

type netWorthMsg struct {
	asOf domain.Date
	nw   app.NetWorth
	err  error
	pt   *reporting.NetWorthPoint
}

func (s *netWorthSection) load(p sectionParams) tea.Cmd {
	s.asOf = p.AsOf
	return func() tea.Msg {
		ctx := context.Background()
		// Net worth needs a configured base currency and fx rates; it is shown as "n/a" rather
		// than failing the whole screen when either is missing.
		out := netWorthMsg{asOf: p.AsOf}
		out.nw, out.err = s.uc.NetWorth(ctx, app.NetWorthRequest{At: p.AsOf.String()})
		if s.components != nil && out.err == nil {
			pt, err := s.components.NetWorthAt(ctx, p.AsOf)
			if err != nil {
				return screens.ErrorMsg{Err: err}
			}
			out.pt = &pt
		}
		return out
	}
}

func (s *netWorthSection) update(msg tea.Msg) (tea.Cmd, bool) {
	m, ok := msg.(netWorthMsg)
	if !ok {
		return nil, false
	}
	if m.asOf == s.asOf {
		s.nw, s.err, s.pt = m.nw, m.err, m.pt
	}
	return nil, true
}

func (s *netWorthSection) keys() []key.Binding { return nil }

func (s *netWorthSection) view(width int) string {
	out := s.th.Header.Render("Net worth") + "\n"
	if s.err != nil {
		return out + s.th.Muted.Render("n/a ("+s.err.Error()+")")
	}
	gross, net, err := grossNet(s.nw)
	if err != nil {
		return out + s.th.Muted.Render("n/a ("+err.Error()+")")
	}
	amount := domain.FormatAmount(gross, s.loc)
	code := s.nw.Base.Code()
	if components.BigTextWidth(amount)+1+len(code) <= width {
		big := components.BigText(amount, s.th.Header)
		out += lipgloss.JoinHorizontal(lipgloss.Bottom, big, " ", s.th.Muted.Render(code))
	} else {
		out += s.th.Header.Render(amount) + " " + s.th.Muted.Render(code)
	}
	lines := []string{out, s.th.Muted.Render("net of loans " + domain.FormatAmount(net, s.loc))}
	if s.pt != nil {
		if s.pt.Positions != nil {
			lines = append(lines, s.th.Muted.Render("Positions: "+s.formatComponent(*s.pt.Positions)))
		}
		if s.pt.Assets != nil {
			lines = append(lines, s.th.Muted.Render("Assets: "+s.formatComponent(*s.pt.Assets)))
		}
	}
	return strings.Join(lines, "\n")
}

func (s *netWorthSection) formatComponent(a reporting.Amount) string {
	cur, err := domain.ParseCurrency(a.Currency)
	if err != nil {
		return "?"
	}
	money, err := domain.NewMoney(a.Minor, cur)
	if err != nil {
		return "?"
	}
	return domain.FormatAmount(money, s.loc)
}
