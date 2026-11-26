package report_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	scr "github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/report"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/theme"
)

func seedGrouped(t *testing.T, h harness) {
	t.Helper()
	h.fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})
	h.fx.Category(t, app.AddCategoryRequest{Name: "Groceries", Kind: "expense"})
	h.fx.Category(t, app.AddCategoryRequest{Name: "Transport", Kind: "expense"})
	h.fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-150", Description: "Shop", Category: "Groceries", Payee: "Market", Date: "2026-06-10"})
	h.fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-40", Description: "Bus", Category: "Transport", Payee: "Metro", Date: "2026-06-11"})
	h.fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "300", Description: "Pay", Date: "2026-06-12"})
}

// kindScreen builds a screen of the given size over seeded data, cycling left from the first kind
// n times (the last kinds are cash flow, then spending).
func kindScreen(t *testing.T, h harness, th theme.Theme, left int) scr.Screen {
	t.Helper()
	m := report.New(h.uc, domain.LocaleENUS, 1, th)
	m.SetCompare(h.uc.analytics)
	m.SetSize(80, 24)
	s := run(t, m, m.Init())
	for range left {
		var cmd tea.Cmd
		s, cmd = sendKey(s, tea.KeyLeft)
		s = run(t, s, cmd)
	}
	return s
}

func switchMsg(t *testing.T, cmd tea.Cmd) scr.SwitchScreenMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a drill-down command")
	}
	msg, ok := cmd().(scr.SwitchScreenMsg)
	if !ok {
		t.Fatalf("command did not return a SwitchScreenMsg")
	}
	if msg.Target != scr.TabTransactions || msg.Filter == nil {
		t.Fatalf("SwitchScreenMsg = %+v, want the transactions tab with a filter", msg)
	}
	return msg
}

func TestDrillSpendingRows(t *testing.T) {
	h := newHarness(t)
	seedGrouped(t, h)
	s := kindScreen(t, h, theme.Default(), 2)
	if !strings.Contains(s.View(), "Spending by category") {
		t.Fatalf("expected spending report:\n%s", s.View())
	}
	_, cmd := sendKey(s, tea.KeyEnter)
	got := switchMsg(t, cmd).Filter
	want := &app.ListTxRequest{From: "2026-06-01", To: "2026-06-30", Categories: []string{"Groceries"}, Sign: domain.TxSignExpense}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("filter mismatch (-want +got):\n%s", diff)
	}

	s, _ = sendKey(s, tea.KeyDown)
	_, cmd = sendKey(s, tea.KeyEnter)
	if got := switchMsg(t, cmd).Filter.Categories; len(got) != 1 || got[0] != "Transport" {
		t.Fatalf("second row categories = %v", got)
	}
	// The TOTAL row opens the whole period.
	s, _ = sendKey(s, tea.KeyDown)
	_, cmd = sendKey(s, tea.KeyEnter)
	if f := switchMsg(t, cmd).Filter; len(f.Categories) != 0 || f.From != "2026-06-01" {
		t.Fatalf("TOTAL filter = %+v", f)
	}
}

func TestDrillCashFlowAndPayees(t *testing.T) {
	h := newHarness(t)
	seedGrouped(t, h)
	s := kindScreen(t, h, theme.Default(), 1)
	if !strings.Contains(s.View(), "Cash flow") {
		t.Fatalf("expected cash flow report:\n%s", s.View())
	}
	_, cmd := sendKey(s, tea.KeyEnter)
	want := &app.ListTxRequest{From: "2026-06-01", To: "2026-06-30"}
	if diff := cmp.Diff(want, switchMsg(t, cmd).Filter); diff != "" {
		t.Fatalf("cash flow filter (-want +got):\n%s", diff)
	}

	s = kindScreen(t, h, theme.Default(), 0)
	for range 2 {
		var c tea.Cmd
		s, c = sendKey(s, tea.KeyRight)
		s = run(t, s, c)
	}
	_, cmd = sendKey(s, tea.KeyEnter)
	f := switchMsg(t, cmd).Filter
	if len(f.Payees) != 1 || f.Payees[0] != "Market" || f.From != "2026-06-01" {
		t.Fatalf("payee filter = %+v", f)
	}
}

func TestDrillNotOfferedWithoutRows(t *testing.T) {
	h := newHarness(t)
	seedGrouped(t, h)
	drill := func(s scr.Screen) bool {
		for _, b := range s.KeyMap() {
			if b.Help().Desc == "open transactions" {
				return true
			}
		}
		return false
	}
	s := kindScreen(t, h, theme.Default(), 0) // net worth
	if !drill(s) {
		t.Fatal("net worth should advertise enter")
	}
	s, cmd := sendKey(s, tea.KeyRight) // budget vs actual
	s = run(t, s, cmd)
	if drill(s) {
		t.Fatal("budget must not advertise enter")
	}
	if _, cmd := sendKey(s, tea.KeyEnter); cmd != nil {
		t.Fatalf("enter on budget must do nothing")
	}
	s, cmd = sendKey(s, tea.KeyRight)
	s = run(t, s, cmd)
	s, cmd = sendKey(s, tea.KeyRight) // forecast
	s = run(t, s, cmd)
	if _, cmd := sendKey(s, tea.KeyEnter); cmd != nil || drill(s) {
		t.Fatal("forecast must not drill")
	}
}

func TestDrillNetWorthPoint(t *testing.T) {
	h := newHarness(t)
	seedGrouped(t, h)
	s := kindScreen(t, h, theme.Default(), 0)
	_, cmd := sendKey(s, tea.KeyEnter)
	f := switchMsg(t, cmd).Filter
	if f.From != "2026-06-01" || f.To == "" {
		t.Fatalf("net worth point filter = %+v", f)
	}
}

func TestDrillCompareRow(t *testing.T) {
	s := compareScreen(t, theme.Default())
	s, cmd := sendRunes(s, "c")
	s = run(t, s, cmd)
	_, cmd = sendKey(s, tea.KeyEnter)
	f := switchMsg(t, cmd).Filter
	if f.From != "2026-06-01" || f.To != "2026-06-30" || len(f.Categories) != 1 || f.Categories[0] != "Groceries" {
		t.Fatalf("compare filter = %+v, want period B and Groceries", f)
	}
	if _, cmd := sendKey(s, tea.KeyEnter); cmd == nil {
		t.Fatal("enter must keep drilling")
	}
}

// exportVia drives the export form: what is "" for the default (result) or "transactions".
func exportVia(t *testing.T, s scr.Screen, what, path, format string) scr.Screen {
	t.Helper()
	s, cmd := sendRunes(s, "e")
	s = run(t, s, cmd)
	if what != "" {
		for range len("result") {
			s, cmd = sendKey(s, tea.KeyBackspace)
			s = run(t, s, cmd)
		}
		s, cmd = sendRunes(s, what)
		s = run(t, s, cmd)
	}
	s, cmd = sendKey(s, tea.KeyEnter) // accept the suggestion
	s = run(t, s, cmd)
	s, cmd = sendKey(s, tea.KeyEnter) // leave the what field
	s = run(t, s, cmd)
	s, cmd = sendRunes(s, path)
	s = run(t, s, cmd)
	s, cmd = sendKey(s, tea.KeyEnter)
	s = run(t, s, cmd)
	if format != "" {
		s, cmd = sendRunes(s, format)
		s = run(t, s, cmd)
		s, cmd = sendKey(s, tea.KeyEnter) // accept the suggestion
		s = run(t, s, cmd)
	}
	s, cmd = sendKey(s, tea.KeyEnter)
	return run(t, s, cmd)
}

func TestExportResult(t *testing.T) {
	for _, tc := range []struct{ format, want string }{
		{"", "csv:reporting.GroupedResult"},
		{"json", `"group_by":"category"`},
	} {
		t.Run("format="+tc.format, func(t *testing.T) {
			h := newHarness(t)
			seedGrouped(t, h)
			s := kindScreen(t, h, theme.Default(), 2)
			path := filepath.Join(t.TempDir(), "out")
			exportVia(t, s, "", path, tc.format)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			if !strings.Contains(string(data), tc.want) {
				t.Fatalf("export = %q, want it to contain %q", data, tc.want)
			}
			if n := len(h.uc.exporter.Calls); n != 1 {
				t.Fatalf("ExportResult calls = %d, want 1", n)
			}
		})
	}
}

func TestExportResultRejectsFormat(t *testing.T) {
	h := newHarness(t)
	seedGrouped(t, h)
	s := kindScreen(t, h, theme.Default(), 2)
	s, _ = sendRunes(s, "e")
	s, _ = sendKey(s, tea.KeyEnter)
	s, _ = sendKey(s, tea.KeyEnter)
	s, _ = sendRunes(s, filepath.Join(t.TempDir(), "out"))
	s, _ = sendKey(s, tea.KeyEnter)
	s, _ = sendRunes(s, "fui")
	s, _ = sendKey(s, tea.KeyEnter) // accept the suggestion
	_, cmd := sendKey(s, tea.KeyEnter)
	if cmd == nil {
		t.Fatal("expected an error command")
	}
	if _, ok := cmd().(scr.ErrorMsg); !ok {
		t.Fatal("expected screens.ErrorMsg for the fui format")
	}
}

func TestSaveSpendingStoresGroupBy(t *testing.T) {
	h := newHarness(t)
	seedGrouped(t, h)
	s := kindScreen(t, h, theme.Default(), 2)
	s, cmd := sendRunes(s, "s")
	s = run(t, s, cmd)
	s, cmd = sendRunes(s, "by-cat")
	s = run(t, s, cmd)
	_, cmd = sendKey(s, tea.KeyEnter)
	run(t, s, cmd)
	rows, err := h.uc.List(t.Context())
	if err != nil || len(rows) != 1 {
		t.Fatalf("List = %v, %v", rows, err)
	}
	if !strings.Contains(rows[0].ParamsJSON, `"group_by":"category"`) {
		t.Fatalf("saved params = %s, want group_by", rows[0].ParamsJSON)
	}
}

func TestGroupedReportGoldens(t *testing.T) {
	cases := []struct {
		name string
		left int
		keys string
	}{
		{"Spending", 2, ""},
		{"CashFlow", 1, ""},
		{"Help", 0, "i"},
	}
	themes := []string{"latte", "frappe", "macchiato", "mocha", "terminal", "high-contrast"}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			seedGrouped(t, h)
			render := func(th theme.Theme) string {
				s := kindScreen(t, h, th, c.left)
				if c.keys != "" {
					s, _ = sendRunes(s, c.keys)
				}
				return s.View()
			}
			var first string
			for _, name := range themes {
				plain := ansi.Strip(render(theme.Resolve(theme.Config{Theme: name}, nil)))
				if first == "" {
					first = plain
				} else if plain != first {
					t.Errorf("theme %s renders differently once stripped:\n%s\nvs\n%s", name, plain, first)
				}
			}
			golden.RequireEqual(t, []byte(first))

			t.Setenv("NO_COLOR", "1")
			v := screentest.Downsample(render(theme.Default()), colorprofile.Ascii)
			if screentest.HasColor(v) {
				t.Errorf("NO_COLOR view contains colors: %q", v)
			}
			if c.keys != "" {
				// The help overlay falls back to raw markdown without color, so it has its own golden.
				t.Run("NoColor", func(t *testing.T) { golden.RequireEqual(t, []byte(ansi.Strip(v))) })
				return
			}
			if ansi.Strip(v) != first {
				t.Errorf("NO_COLOR view differs from the plain golden:\n%s", v)
			}
		})
	}
}
