package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
)

func bannerRoot(t *testing.T, version string, w, h int) tea.Model {
	t.Helper()
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
			if got := plainView(bannerRoot(t, tc.version, 120, 40)); !strings.Contains(got, tc.want) {
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
			golden.RequireEqual(t, []byte(plainView(m)))
		})
	}
}

func TestBannerRows(t *testing.T) {
	banner := plainView(bannerRoot(t, "v0.3.0", 120, 40))
	if lines := strings.Split(banner, "\n"); len(lines) != 40 || !strings.Contains(lines[6], "by 0xBallo") {
		t.Fatalf("banner must keep the height and put the byline on row 7, got %d lines:\n%s", len(lines), banner)
	}
	compact := plainView(bannerRoot(t, "v0.3.0", 140, 29))
	if !strings.Contains(strings.Split(compact, "\n")[1], "FinanceUI v0.3 by 0xBallo") {
		t.Fatalf("compact title must sit in the tab bar:\n%s", compact)
	}
}

func TestBannerTabClick(t *testing.T) {
	m := bannerRoot(t, "v0.3.0", 120, 40)
	// Rows 0-6 are the banner: a click there does nothing; rows 7-9 are the tab bar.
	m, _ = click(m, tea.MouseLeft, 20, 1)
	if !strings.Contains(plainView(m), "1 Dashboard") || strings.Contains(plainView(m), "Description") {
		t.Fatalf("a click on the banner must not switch tabs:\n%s", plainView(m))
	}
	m, cmd := click(m, tea.MouseLeft, 20, 8)
	m = run(t, m, cmd)
	if !strings.Contains(plainView(m), "2 Transactions") || !strings.Contains(plainView(m), "Description") {
		t.Fatalf("a click on the tab bar must open Transactions:\n%s", plainView(m))
	}
}

func TestHyperlinkWidth(t *testing.T) {
	if w := lipgloss.Width(ansi.SetHyperlink("https://github.com/0xBallo") + "0xBallo" + ansi.ResetHyperlink()); w != 7 {
		t.Fatalf("hyperlink width = %d, want 7", w)
	}
}
