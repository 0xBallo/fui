package tui_test

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/investing"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/tui"
	"github.com/0xballo/fui/internal/tui/screens/report"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
)

type v03NetWorth struct{}

func (v03NetWorth) NetWorthAt(context.Context, domain.Date) (reporting.NetWorthPoint, error) {
	return reporting.NetWorthPoint{Positions: &reporting.Amount{Minor: 300_00, Currency: "EUR"}}, nil
}

type v03Insights struct{}

func (v03Insights) Run(context.Context) ([]reporting.InsightView, error) {
	return []reporting.InsightView{{Kind: "subscription", Key: "sub:netflix", Label: "Netflix", Detail: "monthly"}}, nil
}

func (v03Insights) Dismiss(context.Context, string) error { return nil }

func (v03Insights) AcceptSubscription(context.Context, string) (reporting.AcceptResult, error) {
	return reporting.AcceptResult{}, nil
}

type v03Positions struct{}

func (v03Positions) ListPositions(context.Context, investing.PositionsRequest) ([]investing.PositionLine, error) {
	return []investing.PositionLine{{Security: domain.Security{ID: 1, Name: "VWCE ETF"}}}, nil
}

func (v03Positions) PlanSetPrice(context.Context, investing.SetPriceRequest) (investing.SetPricePlan, error) {
	return investing.SetPricePlan{}, nil
}

func (v03Positions) ApplySetPrice(context.Context, investing.SetPricePlan) error { return nil }

type v03Compare struct{}

func (v03Compare) ComparePeriods(context.Context, reporting.CompareRequest) (reporting.CompareReport, error) {
	return reporting.CompareReport{A: "2026-05-01..2026-05-31", B: "2026-06-01..2026-06-30"}, nil
}

// v03Report answers the commands the reports screen runs when it opens; any other call panics.
type v03Report struct{ report.UseCases }

func (v03Report) ListAccounts(context.Context, app.ListAccountsRequest) ([]app.AccountBalance, error) {
	return nil, nil
}

func (v03Report) NetWorth(context.Context, reporting.ReportRequest) (reporting.SeriesResult, error) {
	return reporting.SeriesResult{}, nil
}

func (v03Report) Today() (domain.Date, error) { return domain.NewDate(2026, 6, 15) }

// v03Config wires every v0.3 TUI feature; the modules decide which ones Root enables. The report
// screen gets the v03Report stub; the simulation use cases are a nil-embedding stub, as these tests
// only open the simulations view.
func v03Config(mods ...module.ID) tui.Config {
	cfg := routedConfig(mods...)
	cfg.Report = v03Report{}
	cfg.NetWorth = v03NetWorth{}
	cfg.Insights = v03Insights{}
	cfg.Positions = v03Positions{}
	cfg.Compare = v03Compare{}
	cfg.Simulations = struct{ report.SimUseCases }{}
	return cfg
}

// drain runs cmd and every command it yields, feeding the messages back to m.
func drain(m tea.Model, cmd tea.Cmd) tea.Model {
	for cmd != nil {
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				m = drain(m, c)
			}
			return m
		}
		m, cmd = m.Update(msg)
	}
	return m
}

func newV03Root(t *testing.T, cfg tui.Config) tea.Model {
	t.Helper()
	fx := screentest.New(t)
	fx.Account(t, app.AddAccountRequest{
		Name: "Broker", Kind: "investment", Currency: "EUR", OpeningBalance: "0", OpenedOn: "2026-01-01",
	})
	root := tui.New(useCases{Service: fx.Service, Reporting: fx.NewReporting(t)}, cfg)
	m, _ := root.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return drain(m, m.Init())
}

// press types keys on the active screen and runs the commands they return.
func press(m tea.Model, keys ...string) tea.Model {
	for _, k := range keys {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		if k == "enter" {
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		}
		var cmd tea.Cmd
		m, cmd = m.Update(msg)
		m = drain(m, cmd)
	}
	return m
}

// TestV03FeaturesFollowModules drives Root through the v0.3 keys: each feature is reachable when
// its module is enabled and absent when it is disabled, even with its use cases wired.
func TestV03FeaturesFollowModules(t *testing.T) {
	all := []module.ID{module.Analytics, module.Investments, module.Recurring, module.Simulations}
	tests := []struct {
		name    string
		mods    []module.ID
		keys    []string
		want    []string
		notWant []string
	}{
		{name: "dashboard net worth and insights", mods: all, want: []string{"Positions: 300.00", "Netflix"}},
		{name: "dashboard without analytics", mods: []module.ID{module.Investments, module.Recurring, module.Simulations}, notWant: []string{"Positions: 300.00", "Netflix"}},
		{name: "insights focus", mods: all, keys: []string{"i"}, want: []string{"> i info [subscription] Netflix"}},
		{name: "account positions", mods: all, keys: []string{"3", "enter"}, want: []string{"VWCE ETF"}},
		{name: "account without investments", mods: []module.ID{module.Analytics}, keys: []string{"3", "enter"}, notWant: []string{"VWCE ETF"}},
		{name: "report compare", mods: all, keys: []string{"5", "c"}, want: []string{"Compare periods", "TOTAL"}},
		{name: "report without analytics", mods: []module.ID{module.Recurring, module.Simulations}, keys: []string{"5", "c"}, notWant: []string{"Compare periods", "c compare"}},
		{name: "report simulations", mods: all, keys: []string{"5", "?"}, want: []string{" simulations"}},
		{name: "report without simulations", mods: []module.ID{module.Analytics}, keys: []string{"5", "i"}, notWant: []string{"Simulation", "i simulations"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := press(newV03Root(t, v03Config(tc.mods...)), tc.keys...)
			view := m.View()
			for _, s := range tc.want {
				if !strings.Contains(view, s) {
					t.Errorf("missing %q in:\n%s", s, view)
				}
			}
			for _, s := range tc.notWant {
				if strings.Contains(view, s) {
					t.Errorf("unexpected %q in:\n%s", s, view)
				}
			}
		})
	}
}
