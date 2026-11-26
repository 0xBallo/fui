package report_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/google/go-cmp/cmp"
	"github.com/muesli/termenv"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	scr "github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/theme"
)

const (
	spendingLeft = 2 // key presses left from the first kind to reach spending
	netWorthLeft = 0
	payeesLeft   = 7
	budgetLeft   = 8
)

// fillForm types vals into the open form's fields in order: "" skips a field, a "+" suffix adds a
// second enter to accept the suggestion the typed text opened.
func fillForm(t *testing.T, s scr.Screen, vals ...string) scr.Screen {
	t.Helper()
	for _, v := range vals {
		accept := strings.HasSuffix(v, "+")
		v = strings.TrimSuffix(v, "+")
		var cmd tea.Cmd
		if v != "" {
			s, cmd = sendRunes(s, v)
			s = run(t, s, cmd)
		}
		presses := 1
		if accept {
			presses = 2
		}
		for range presses {
			s, cmd = sendKey(s, tea.KeyEnter)
			s = run(t, s, cmd)
		}
	}
	return s
}

// spendingFilter fills the spending filter form with a payee filter and a group-by.
func spendingFilter(t *testing.T, s scr.Screen, payee, groupBy string) scr.Screen {
	t.Helper()
	s, cmd := sendRunes(s, "f")
	s = run(t, s, cmd)
	// accounts, categories, tags, payees, min, max, status, text, uncategorized, income/expense, group by
	return fillForm(t, s, "", "", "", payee, "", "", "", "", "", "", groupBy+"+")
}

func TestFilterFormBuildsRequest(t *testing.T) {
	h := newHarness(t)
	seedGrouped(t, h)
	s := kindScreen(t, h, theme.Default(), spendingLeft)
	s, cmd := sendRunes(s, "f")
	s = run(t, s, cmd)
	if !s.InputActive() {
		t.Fatal("InputActive() = false with the filter form open")
	}
	s = fillForm(t, s, "Checking+", "Groceries", "", "Market", "10", "500", "booked+", "shop", "", "expense+", "month+")

	want := app.ListTxRequest{
		Accounts: []string{"Checking"}, Categories: []string{"Groceries"},
		Payees: []string{"Market"}, MinAmount: "10", MaxAmount: "500", Statuses: []string{"booked"},
		Text: "shop", Sign: domain.TxSignExpense,
	}
	got := h.uc.rec.spending
	if diff := cmp.Diff(want, got.Filter); diff != "" {
		t.Fatalf("Filter mismatch (-want +got):\n%s", diff)
	}
	if got.GroupBy != domain.GroupMonth {
		t.Fatalf("GroupBy = %q, want month", got.GroupBy)
	}
	view := ansi.Strip(s.View())
	for _, sub := range []string{"by month · Filter: account=Checking, category=Groceries"} {
		if !strings.Contains(view, sub) {
			t.Fatalf("subtitle missing %q:\n%s", sub, view)
		}
	}
}

func TestFilterFormInvalidGroupByKeepsForm(t *testing.T) {
	h := newHarness(t)
	seedGrouped(t, h)
	s := kindScreen(t, h, theme.Default(), spendingLeft)
	s, cmd := sendRunes(s, "f")
	s = run(t, s, cmd)
	for range 10 {
		s, _ = sendKey(s, tea.KeyEnter)
	}
	s, _ = sendRunes(s, "bogus")
	_, cmd = sendKey(s, tea.KeyEnter)
	if cmd == nil {
		t.Fatal("expected an error command")
	}
	if _, ok := cmd().(scr.ErrorMsg); !ok {
		t.Fatal("expected screens.ErrorMsg for the unknown group-by")
	}
}

func TestFilterFormFieldsPerKind(t *testing.T) {
	cases := []struct {
		name    string
		left    int
		want    []string
		missing []string
	}{
		{"spending", spendingLeft, []string{"Accounts:", "Group by:"}, []string{"Granularity:"}},
		{"payees", payeesLeft, []string{"Accounts:", "Payees:"}, []string{"Group by:", "Granularity:"}},
		{"net worth", netWorthLeft, []string{"Granularity:"}, []string{"Accounts:", "Group by:"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			seedGrouped(t, h)
			s := kindScreen(t, h, theme.Default(), c.left)
			s, _ = sendRunes(s, "f")
			view := s.View()
			for _, w := range c.want {
				if !strings.Contains(view, w) {
					t.Errorf("form lacks %q:\n%s", w, view)
				}
			}
			for _, w := range c.missing {
				if strings.Contains(view, w) {
					t.Errorf("form has %q, which this report ignores:\n%s", w, view)
				}
			}
		})
	}
}

func TestFilterFormNotOpenedWithoutOptions(t *testing.T) {
	h := newHarness(t)
	seedGrouped(t, h)
	s := kindScreen(t, h, theme.Default(), budgetLeft)
	s, _ = sendRunes(s, "f")
	if s.InputActive() {
		t.Fatal("filter form opened on a report with no filter options")
	}
}

func TestFilterGranularityNetWorth(t *testing.T) {
	h := newHarness(t)
	seedGrouped(t, h)
	s := kindScreen(t, h, theme.Default(), netWorthLeft)
	s, cmd := sendRunes(s, "f")
	s = run(t, s, cmd)
	s = fillForm(t, s, "week+")
	if got := h.uc.rec.netWorth.Granularity; got != domain.GranWeek {
		t.Fatalf("NetWorth granularity = %q, want week", got)
	}
	if !strings.Contains(s.View(), "per week") {
		t.Fatalf("subtitle lacks granularity:\n%s", s.View())
	}
}

func TestDrillMergesActiveFilter(t *testing.T) {
	h := newHarness(t)
	seedGrouped(t, h)
	s := kindScreen(t, h, theme.Default(), spendingLeft)
	s = spendingFilter(t, s, "Market", "category")
	_, cmd := sendKey(s, tea.KeyEnter)
	f := switchMsg(t, cmd).Filter
	if len(f.Payees) != 1 || f.Payees[0] != "Market" {
		t.Fatalf("drill lost the active payee filter: %+v", f)
	}
	if len(f.Categories) != 1 || f.Categories[0] != "Groceries" || f.Sign != domain.TxSignExpense {
		t.Fatalf("drill lacks the row criteria: %+v", f)
	}
}

func TestSaveStoresFilterSpecAndRunsIt(t *testing.T) {
	h := newHarness(t)
	seedGrouped(t, h)
	s := kindScreen(t, h, theme.Default(), spendingLeft)
	s = spendingFilter(t, s, "Market", "month")

	s, cmd := sendRunes(s, "s")
	s = run(t, s, cmd)
	s, cmd = sendRunes(s, "market-months")
	s = run(t, s, cmd)
	s, cmd = sendKey(s, tea.KeyEnter)
	s = run(t, s, cmd)
	rows, err := h.uc.List(t.Context())
	if err != nil || len(rows) != 1 {
		t.Fatalf("List = %v, %v", rows, err)
	}
	for _, want := range []string{`"version":2`, `"Payees":["Market"]`, `"group_by":"month"`} {
		if !strings.Contains(rows[0].ParamsJSON, want) {
			t.Fatalf("saved params = %s, want %s", rows[0].ParamsJSON, want)
		}
	}
	for _, bad := range []string{`"From":"2`, `"To":"2`} {
		if strings.Contains(rows[0].ParamsJSON, bad) {
			t.Fatalf("saved params = %s must not hold absolute dates", rows[0].ParamsJSON)
		}
	}

	s, cmd = sendRunes(s, "v")
	s = run(t, s, cmd)
	s, cmd = sendKey(s, tea.KeyEnter)
	s = run(t, s, cmd)
	view := ansi.Strip(s.View())
	if !strings.Contains(view, "saved: market-months") || !strings.Contains(view, "by month · Filter: payee=Market") {
		t.Fatalf("saved run lacks the stored filter in its subtitle:\n%s", view)
	}
	_, cmd = sendKey(s, tea.KeyEnter)
	if f := switchMsg(t, cmd).Filter; len(f.Payees) != 1 || f.Payees[0] != "Market" {
		t.Fatalf("drill of the saved report ignores its filter: %+v", f)
	}
}

func TestExportBothChoices(t *testing.T) {
	cases := []struct {
		what, format string
		want, absent string
	}{
		{"", "json", `"group_by":"category"`, ""},
		{"transactions", "csv", "Shop", "Bus"},
		{"transactions", "fui", "Shop", "Bus"},
	}
	for _, c := range cases {
		t.Run(c.what+"/"+c.format, func(t *testing.T) {
			h := newHarness(t)
			seedGrouped(t, h)
			s := kindScreen(t, h, theme.Default(), spendingLeft)
			s = spendingFilter(t, s, "Market", "category")
			path := filepath.Join(t.TempDir(), "out")
			exportVia(t, s, c.what, path, c.format)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			if !strings.Contains(string(data), c.want) {
				t.Fatalf("export = %q, want %q", data, c.want)
			}
			if c.absent != "" && strings.Contains(string(data), c.absent) {
				t.Fatalf("export = %q must be narrowed by the filter (no %q)", data, c.absent)
			}
		})
	}
}

func TestExportRejectsUnknownTarget(t *testing.T) {
	h := newHarness(t)
	seedGrouped(t, h)
	s := kindScreen(t, h, theme.Default(), spendingLeft)
	s, _ = sendRunes(s, "e")
	for range 6 {
		s, _ = sendKey(s, tea.KeyBackspace)
	}
	s, _ = sendRunes(s, "nothing")
	s, _ = sendKey(s, tea.KeyEnter)
	s, _ = sendRunes(s, filepath.Join(t.TempDir(), "out"))
	s, _ = sendKey(s, tea.KeyEnter)
	_, cmd := sendKey(s, tea.KeyEnter)
	if cmd == nil {
		t.Fatal("expected an error command")
	}
	if _, ok := cmd().(scr.ErrorMsg); !ok {
		t.Fatal("expected screens.ErrorMsg for the unknown target")
	}
}

func TestFilterGoldens(t *testing.T) {
	prev := lipgloss.ColorProfile()
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })

	cases := []struct {
		name  string
		build func(t *testing.T, s scr.Screen) scr.Screen
	}{
		{"Form", func(t *testing.T, s scr.Screen) scr.Screen {
			s, cmd := sendRunes(s, "f")
			return run(t, s, cmd)
		}},
		{"Filtered", func(t *testing.T, s scr.Screen) scr.Screen { return spendingFilter(t, s, "Market", "category") }},
	}
	themes := []string{"latte", "frappe", "macchiato", "mocha", "terminal", "high-contrast"}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			seedGrouped(t, h)
			render := func(th theme.Theme) string {
				return c.build(t, kindScreen(t, h, th, spendingLeft)).View()
			}
			lipgloss.SetColorProfile(termenv.TrueColor)
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

			lipgloss.SetColorProfile(termenv.Ascii)
			v := render(theme.Default())
			if strings.Contains(v, "\x1b") {
				t.Errorf("NO_COLOR view contains escape codes: %q", v)
			}
			if ansi.Strip(v) != first {
				t.Errorf("NO_COLOR view differs from the plain golden:\n%s", v)
			}
		})
	}
}
