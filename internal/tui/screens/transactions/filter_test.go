package transactions

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/domain"
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
