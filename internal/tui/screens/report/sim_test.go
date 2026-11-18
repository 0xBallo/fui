package report_test

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/simulating"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/domain/simulate"
	scr "github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/report"
	"github.com/0xballo/fui/internal/tui/theme"
)

type stubSim struct {
	runs  []simulating.RunRequest
	saved []string
	list  []app.SavedSimulation
	cmpd  []string
}

func result(base int64) simulate.Result {
	return simulate.Result{
		Rows:          []simulate.YearRow{{Year: 1, Nominal: base / 2}, {Year: 2, Nominal: base}},
		Contributions: 1000000, Nominal: base, Real: base - 1000, NetGain: base - 1000000,
	}
}

func triple() *simulate.Triple {
	return &simulate.Triple{Pessimistic: result(1100000), Base: result(1300000), Optimistic: result(1500000)}
}

func (s *stubSim) Run(_ context.Context, req simulating.RunRequest) (simulating.RunResult, error) {
	s.runs = append(s.runs, req)
	switch req.Kind {
	case simulating.KindPAC:
		return simulating.RunResult{Kind: req.Kind, PAC: triple()}, nil
	case simulating.KindCashFlow:
		return simulating.RunResult{Kind: req.Kind, CashFlow: &simulating.CashFlowResult{
			OpeningMinor: 100000, FirstBelowZero: "2026-03-31", EndDeltaMinor: -5000,
			Months: []simulating.CashFlowMonth{
				{End: "2026-01-31", BaseMinor: 90000, ScenarioMinor: 80000},
				{End: "2026-02-28", BaseMinor: 80000, ScenarioMinor: 20000},
			},
		}}, nil
	}
	return simulating.RunResult{Kind: req.Kind, Investment: triple()}, nil
}

func (s *stubSim) PlanSave(_ context.Context, name string, req simulating.RunRequest) (simulating.SavePlan, error) {
	return simulating.SavePlan{Simulation: app.SavedSimulation{Name: name, Kind: req.Kind}}, nil
}

func (s *stubSim) ApplySave(_ context.Context, p simulating.SavePlan) (app.SavedSimulation, error) {
	s.saved = append(s.saved, p.Simulation.Name)
	return p.Simulation, nil
}

func (s *stubSim) List(context.Context) ([]app.SavedSimulation, error) { return s.list, nil }

func (s *stubSim) Rerun(_ context.Context, ref string) (simulating.RerunResult, error) {
	return simulating.RerunResult{
		Simulation: app.SavedSimulation{Name: ref, Kind: simulating.KindInvestment},
		RunResult:  simulating.RunResult{Kind: simulating.KindInvestment, Investment: triple()},
	}, nil
}

func (s *stubSim) Compare(_ context.Context, refs []string) (simulating.CompareResult, error) {
	s.cmpd = refs
	var out simulating.CompareResult
	for _, r := range refs {
		rr, _ := s.Rerun(context.Background(), r)
		out.Items = append(out.Items, rr)
	}
	return out, nil
}

func (*stubSim) BaseCurrency() domain.Currency {
	c, _ := domain.ParseCurrency("EUR")
	return c
}

func simScreen(t *testing.T, uc report.SimUseCases) scr.Screen {
	t.Helper()
	h := newHarness(t)
	m := report.New(h.uc, domain.LocaleENUS, 1, theme.Default())
	m.SetSimulations(uc)
	m.SetSize(120, 40)
	return run(t, m, m.Init())
}

// drive feeds a key to s and runs the resulting commands, returning the screen and any status or
// error text the screen emitted.
func drive(t *testing.T, s scr.Screen, k tea.KeyMsg) (scr.Screen, string) {
	t.Helper()
	var note string
	var walk func(cmd tea.Cmd)
	walk = func(cmd tea.Cmd) {
		if cmd == nil {
			return
		}
		switch msg := cmd().(type) {
		case tea.BatchMsg:
			for _, c := range msg {
				walk(c)
			}
		case scr.ErrorMsg:
			note = msg.Err.Error()
		case scr.StatusMsg:
			note = msg.Text
		default:
			var next tea.Cmd
			s, next = s.Update(msg)
			walk(next)
		}
	}
	var cmd tea.Cmd
	s, cmd = s.Update(k)
	walk(cmd)
	return s, note
}

func keys(t *testing.T, s scr.Screen, in ...string) (scr.Screen, string) {
	t.Helper()
	var note string
	for _, k := range in {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "space":
			msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		}
		s, note = drive(t, s, msg)
	}
	return s, note
}

func TestSimHiddenWithoutUseCases(t *testing.T) {
	h := newHarness(t)
	s := run(t, newFixture(t, h), nil)
	s, _ = keys(t, s, "i")
	if strings.Contains(s.View(), "Simulation") || strings.Contains(s.View(), "i simulations") {
		t.Fatalf("simulations must be hidden:\n%s", s.View())
	}
}

func TestSimRunKindsAndSparkline(t *testing.T) {
	uc := &stubSim{}
	s := simScreen(t, uc)
	if !offers(s, "i") {
		t.Fatalf("key map should offer simulations: %v", s.KeyMap())
	}
	s, _ = keys(t, s, "i")
	v := s.View()
	for _, want := range []string{"Simulation: investment", "Pessimistic", "13,000.00", "+3,000.00", "Annual rates: 3.00% / 5.00% / 7.00%", "▁█"} {
		if !strings.Contains(v, want) {
			t.Fatalf("investment view missing %q:\n%s", want, v)
		}
	}
	s, _ = keys(t, s, "l")
	if !strings.Contains(s.View(), "Simulation: pac") || uc.runs[len(uc.runs)-1].PAC == nil {
		t.Fatalf("pac not run: %+v\n%s", uc.runs, s.View())
	}
	s, _ = keys(t, s, "l")
	v = s.View()
	for _, want := range []string{"Simulation: cashflow", "2026-02-28", "-600.00", "first below zero 2026-03-31", "Balance by month"} {
		if !strings.Contains(v, want) {
			t.Fatalf("cashflow view missing %q:\n%s", want, v)
		}
	}
	s, _ = keys(t, s, "esc")
	if strings.Contains(s.View(), "Simulation:") {
		t.Fatalf("esc should leave simulations:\n%s", s.View())
	}
}

func TestSimParamsFormBuildsRequest(t *testing.T) {
	uc := &stubSim{}
	s := simScreen(t, uc)
	s, _ = keys(t, s, "i", "p")
	if !s.InputActive() {
		t.Fatal("form should capture input")
	}
	// Enum fields keep their suggestion popup open, so a field may need a second enter.
	for i := 0; i < 20 && s.InputActive(); i++ {
		s, _ = keys(t, s, "enter")
	}
	want := simulating.InvestmentParams{
		InitialMinor: 1000000, Years: 10, Compounding: "monthly",
		Rates: simulating.Rates{PessimisticBP: 300, BaseBP: 500, OptimisticBP: 700},
	}
	got := uc.runs[len(uc.runs)-1]
	if got.Investment == nil || len(uc.runs) != 2 {
		t.Fatalf("runs = %+v", uc.runs)
	}
	if diff := cmp.Diff(want, *got.Investment); diff != "" {
		t.Fatalf("params (-want +got):\n%s", diff)
	}
	if s.InputActive() {
		t.Fatal("form should be closed")
	}
}

func TestSimSaveAndCompare(t *testing.T) {
	uc := &stubSim{list: []app.SavedSimulation{
		{Name: "a", Kind: "investment"}, {Name: "b", Kind: "investment"}, {Name: "c", Kind: "investment"}, {Name: "d", Kind: "investment"},
	}}
	s := simScreen(t, uc)
	s, _ = keys(t, s, "i", "s")
	s, _ = keys(t, s, "r", "e", "t")
	s, note := keys(t, s, "enter")
	if len(uc.saved) != 1 || uc.saved[0] != "ret" || !strings.Contains(note, "ret") {
		t.Fatalf("saved = %v note = %q", uc.saved, note)
	}

	s, _ = keys(t, s, "v")
	if !strings.Contains(s.View(), "Saved simulations") {
		t.Fatalf("saved list not shown:\n%s", s.View())
	}
	_, note = keys(t, s, "c")
	if !strings.Contains(note, "select 2 or 3") {
		t.Fatalf("note = %q", note)
	}
	for _, k := range []string{"space", "down", "space", "down", "space", "down", "space"} {
		s, note = keys(t, s, k)
	}
	if !strings.Contains(note, "at most 3") {
		t.Fatalf("fourth selection must be refused, note = %q", note)
	}
	s, _ = keys(t, s, "c")
	if diff := cmp.Diff([]string{"a", "b", "c"}, uc.cmpd); diff != "" {
		t.Fatalf("compared refs (-want +got):\n%s", diff)
	}
	if v := s.View(); !strings.Contains(v, "Compare simulations") || !strings.Contains(v, "13,000.00") {
		t.Fatalf("compare view:\n%s", v)
	}
	s, _ = keys(t, s, "esc", "enter")
	if v := s.View(); !strings.Contains(v, "saved: d") && !strings.Contains(v, "saved: a") {
		t.Fatalf("enter should show the saved simulation:\n%s", v)
	}
}
