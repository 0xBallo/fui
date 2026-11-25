package transactions

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/theme"
)

type todayStub struct {
	UseCases
	today domain.Date
}

func (s todayStub) Today() (domain.Date, error) { return s.today, nil }

func press(m *Model, k tea.KeyType) { m.handleFilterKey(tea.KeyMsg{Type: k}) }

func newFilterModel(t *testing.T, from, to string) *Model {
	t.Helper()
	today, err := domain.NewDate(2026, 3, 15)
	if err != nil {
		t.Fatal(err)
	}
	m := New(todayStub{today: today}, domain.LocaleENUS, theme.Default())
	m.req.From, m.req.To = from, to
	m.openFilterForm()
	return m
}

func TestDatePicker(t *testing.T) {
	tests := []struct {
		name     string
		from, to string
		toField  bool
		keys     []tea.KeyType
		wantFrom string
		wantTo   string
	}{
		{"pick from today", "", "", false, []tea.KeyType{tea.KeyCtrlD, tea.KeyEnter}, "2026-03-15", ""},
		{"pick from invalid value opens on today", "garbage", "", false, []tea.KeyType{tea.KeyCtrlD, tea.KeyEnter}, "2026-03-15", ""},
		{"pick moves from current value", "2026-01-10", "", false, []tea.KeyType{tea.KeyCtrlD, tea.KeyRight, tea.KeyEnter}, "2026-01-11", ""},
		{"pick to", "", "2026-02-01", true, []tea.KeyType{tea.KeyCtrlD, tea.KeyEnter}, "", "2026-02-01"},
		{"cancel from", "2026-01-10", "", false, []tea.KeyType{tea.KeyCtrlD, tea.KeyRight, tea.KeyEsc}, "2026-01-10", ""},
		{"cancel to", "", "", true, []tea.KeyType{tea.KeyCtrlD, tea.KeyEsc}, "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := newFilterModel(t, tc.from, tc.to)
			for range 6 { // skip to From
				m.filterForm.Next()
			}
			if tc.toField {
				m.filterForm.Next()
			}
			for i, k := range tc.keys {
				press(m, k)
				if i == 0 && (!m.picker.IsOpen() || !m.InputActive() || m.viewFilter() == "") {
					t.Fatal("expected the picker open with input active")
				}
			}
			if m.picker.IsOpen() || m.mode != modeFilter {
				t.Fatal("expected the picker closed and the filter panel still open")
			}
			if got := m.filterFrom.Value(); got != tc.wantFrom {
				t.Errorf("From = %q, want %q", got, tc.wantFrom)
			}
			if got := m.filterTo.Value(); got != tc.wantTo {
				t.Errorf("To = %q, want %q", got, tc.wantTo)
			}
		})
	}
}

func TestDatePickerIgnoredOnOtherFields(t *testing.T) {
	m := newFilterModel(t, "", "")
	press(m, tea.KeyCtrlD)
	if m.picker.IsOpen() {
		t.Fatal("picker must open only on From/To")
	}
}

func TestFilterViewShowsPickHint(t *testing.T) {
	m := newFilterModel(t, "", "")
	if strings.Contains(m.viewFilter(), "ctrl+d pick date") {
		t.Fatal("hint must be hidden off the date fields")
	}
	for range 6 {
		m.filterForm.Next()
	}
	if !strings.Contains(m.viewFilter(), "ctrl+d pick date") {
		t.Fatal("expected the pick date hint on From")
	}
}

// recordingUC captures every ListTx request and answers with an empty page, so filters naming
// entities absent from the archive are not rejected.
type recordingUC struct {
	UseCases
	reqs []app.ListTxRequest
}

func (r *recordingUC) ListTx(_ context.Context, req app.ListTxRequest) (app.TxPage, error) {
	r.reqs = append(r.reqs, req)
	return app.TxPage{}, nil
}

func TestApplyFilterMapsEveryField(t *testing.T) {
	fx := screentest.New(t)
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})
	rec := &recordingUC{UseCases: fx.Service}
	m := New(rec, domain.LocaleENUS, theme.Default())
	m.SetSize(100, 30)

	want := app.ListTxRequest{
		From: "2026-06-01", To: "2026-06-30", Accounts: []string{"Checking"}, Categories: []string{"Food"},
		Tags: []string{"trip"}, Payees: []string{"Shop"}, Uncategorized: true,
		MinAmount: "1", MaxAmount: "99", Statuses: []string{"booked"}, Text: "milk",
		After: &app.TxCursor{}, Limit: 7, // paging is owned by the screen and must be discarded
	}
	m.table.SetCursor(5)
	scr := run(t, screens.Screen(m), m.ApplyFilter(want))

	got := rec.reqs[0]
	want.After, want.Limit = nil, pageSize
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("ListTx request mismatch (-want +got):\n%s", diff)
	}
	if c := scr.(*Model).table.Cursor(); c != 0 {
		t.Fatalf("cursor = %d, want 0", c)
	}
	chips := scr.(*Model).chipsView()
	for _, s := range []string{"From: 2026-06-01", "Tag: trip", "Payee: Shop", "Uncategorized: yes", "Search: milk"} {
		if !strings.Contains(chips, s) {
			t.Errorf("chips %q lack %q", chips, s)
		}
	}
}

func TestApplyFilterMonthYearAndClear(t *testing.T) {
	fx := screentest.New(t)
	rec := &recordingUC{UseCases: fx.Service}
	m := New(rec, domain.LocaleENUS, theme.Default())
	m.SetSize(100, 30)

	scr := run(t, screens.Screen(m), m.ApplyFilter(app.ListTxRequest{Month: "2026-06"}))
	if got := rec.reqs[0].Month; got != "2026-06" {
		t.Fatalf("Month = %q", got)
	}
	scr = run(t, scr, m.ApplyFilter(app.ListTxRequest{Year: 2026}))
	if r := rec.reqs[1]; r.Year != 2026 || r.Month != "" {
		t.Fatalf("second filter must replace the first, got %+v", r)
	}
	if !strings.Contains(m.chipsView(), "Year: 2026") {
		t.Fatalf("missing year chip: %q", m.chipsView())
	}

	scr, cmd := scr.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	run(t, scr, cmd)
	if r := rec.reqs[2]; r.Year != 0 || r.Month != "" {
		t.Fatalf("reset must clear the filter, got %+v", r)
	}
}

func TestRemoveChipClearsTagPayeeMonth(t *testing.T) {
	fx := screentest.New(t)
	m := New(fx.Service, domain.LocaleENUS, theme.Default())
	m.SetSize(100, 30)
	m.req = app.ListTxRequest{Month: "2026-06", Tags: []string{"a", "b"}, Payees: []string{"p"}}
	for range 4 {
		m.removeFocusedChip()
		m.chipFocus = 0
	}
	if len(m.activeChips()) != 0 {
		t.Fatalf("chips left: %+v", m.activeChips())
	}
}
