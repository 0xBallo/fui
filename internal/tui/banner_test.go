package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/muesli/termenv"

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
)

func bannerRoot(t *testing.T, version string, w, h int) tea.Model {
	t.Helper()
	forceColorProfile(t, termenv.Ascii)
	fx := screentest.New(t)
	uc := useCases{Service: fx.Service, Reporting: fx.NewReporting(t)}
	m, _ := tui.New(uc, tui.Config{Locale: domain.LocaleENUS, Version: version}).
		Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

func TestVersionFormat(t *testing.T) {
	for _, tc := range []struct{ version, want string }{
		{"v0.3.0", "v0.3 by 0xBallo"},
		{"v1.12.4-rc1", "v1.12 by 0xBallo"},
		{"v0.3", "v0.3 by 0xBallo"},
		{"dev", "dev by 0xBallo"},
		{"", "dev by 0xBallo"},
		{"abc1234", "abc1234 by 0xBallo"},
	} {
		t.Run(tc.version, func(t *testing.T) {
			if got := bannerRoot(t, tc.version, 120, 40).View(); !strings.Contains(got, tc.want) {
				t.Fatalf("want %q in:\n%s", tc.want, got)
			}
		})
	}
}

func TestBannerSnapshots(t *testing.T) {
	for _, tc := range []struct {
		name string
		w, h int
	}{{"Compact", 80, 24}, {"Banner", 120, 40}} {
		t.Run(tc.name, func(t *testing.T) {
			m := bannerRoot(t, "v0.3.0", tc.w, tc.h)
			golden.RequireEqual(t, []byte(m.View()))
		})
	}
}

func TestBannerRows(t *testing.T) {
	banner := bannerRoot(t, "v0.3.0", 120, 40).View()
	if lines := strings.Split(banner, "\n"); len(lines) != 40 || !strings.Contains(lines[2], "by 0xBallo") {
		t.Fatalf("banner must keep the height and put the byline on row 3, got %d lines:\n%s", len(lines), banner)
	}
	compact := bannerRoot(t, "v0.3.0", 140, 29).View()
	if !strings.Contains(strings.Split(compact, "\n")[1], "FinanceUI v0.3 by 0xBallo") {
		t.Fatalf("compact title must sit in the tab bar:\n%s", compact)
	}
}

func TestBannerTabClick(t *testing.T) {
	m := bannerRoot(t, "v0.3.0", 120, 40)
	// Rows 0-2 are the banner: a click there does nothing; rows 3-5 are the tab bar.
	m, _ = click(m, tea.MouseButtonLeft, 20, 1)
	if !strings.Contains(m.View(), "1 Dashboard") || strings.Contains(m.View(), "Description") {
		t.Fatalf("a click on the banner must not switch tabs:\n%s", m.View())
	}
	m, cmd := click(m, tea.MouseButtonLeft, 20, 4)
	m = run(t, m, cmd)
	if !strings.Contains(m.View(), "2 Transactions") || !strings.Contains(m.View(), "Description") {
		t.Fatalf("a click on the tab bar must open Transactions:\n%s", m.View())
	}
}

func TestHyperlinkWidth(t *testing.T) {
	if w := lipgloss.Width(termenv.Hyperlink("https://github.com/0xBallo", "0xBallo")); w != 7 {
		t.Fatalf("hyperlink width = %d, want 7", w)
	}
}
