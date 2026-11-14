package dashboard

import (
	"strings"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
)

// SetNetWorthUseCases wires the optional reporting use cases for net-worth components.
func (m *Model) SetNetWorthUseCases(uc NetWorthUseCases) {
	m.netWorthUC = uc
}

func (m *Model) renderNetWorth() string {
	if m.netWorthErr != nil {
		return m.th.Muted.Render("n/a (" + m.netWorthErr.Error() + ")")
	}
	lines := []string{
		m.th.Base.Render(domain.FormatAmount(m.netWorth.Total, m.loc)),
	}
	if m.netWorthPt == nil {
		return strings.Join(lines, "\n")
	}
	lines[0] = m.th.Base.Render("Total: " + domain.FormatAmount(m.netWorth.Total, m.loc))
	if m.netWorthPt.Positions != nil {
		lines = append(lines, m.th.Base.Render("Positions: "+m.formatComponent(*m.netWorthPt.Positions)))
	}
	if m.netWorthPt.Assets != nil {
		lines = append(lines, m.th.Base.Render("Assets: "+m.formatComponent(*m.netWorthPt.Assets)))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) formatComponent(a reporting.Amount) string {
	cur, err := domain.ParseCurrency(a.Currency)
	if err != nil {
		return "?"
	}
	money, err := domain.NewMoney(a.Minor, cur)
	if err != nil {
		return "?"
	}
	return domain.FormatAmount(money, m.loc)
}
