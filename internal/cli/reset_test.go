package cli_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// resetter makes the harness Backuper also an app.Resetter over the harness memstore.
type resetter struct {
	*fakeBackup
	h *harness
}

func (r resetter) Reset(ctx context.Context, o app.ResetOptions) error {
	return r.h.store.Reset(ctx, o)
}

// resetHarness returns a harness whose Service can reset, holding one custom category and one account.
func resetHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	cfg := app.Config{Locale: domain.LocaleENUS, Location: time.UTC, BaseCurrency: currency(t, "EUR")}
	h.svc = app.New(h.store, fixedClock{time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)}, resetter{h.backup, h}, cfg)
	h.category("Custom", "expense")
	h.account("Checking", "checking", "EUR", "0")
	return h
}

func (h *harness) accountCount() int {
	h.t.Helper()
	out, _ := h.mustRun("account", "list", "--json")
	return strings.Count(out, `"name"`)
}

func TestResetYes(t *testing.T) {
	h := resetHarness(t)
	out, _ := h.mustRun("reset", "--yes", "--json")
	if !strings.Contains(out, `"reset":true`) && !strings.Contains(out, `"reset": true`) {
		t.Errorf("output = %q, want reset true", out)
	}
	if n := h.accountCount(); n != 0 {
		t.Errorf("accounts after reset = %d, want 0", n)
	}
	cats, _ := h.mustRun("category", "list")
	if strings.Contains(cats, "Custom") || !strings.Contains(cats, "Groceries") {
		t.Errorf("categories after reset = %q, want defaults only", cats)
	}
	if _, err := os.Stat(h.cfg.ConfigFile); err == nil {
		t.Error("config file was created or touched")
	}
}

func TestResetKeepSettings(t *testing.T) {
	h := resetHarness(t)
	h.mustRun("reset", "--keep-settings", "--yes")
	if n := h.accountCount(); n != 0 {
		t.Errorf("accounts after reset = %d, want 0", n)
	}
	cats, _ := h.mustRun("category", "list")
	if !strings.Contains(cats, "Custom") {
		t.Errorf("categories after keep-settings reset = %q, want Custom kept", cats)
	}
}

func TestResetDryRun(t *testing.T) {
	h := resetHarness(t)
	before := h.store.Snapshot()
	out, _ := h.mustRun("reset", "--dry-run")
	if !strings.Contains(out, "delete 1 accounts") {
		t.Errorf("plan = %q, want the account deletion listed", out)
	}
	if h.store.Snapshot() != before {
		t.Error("dry-run changed the store")
	}
}

func TestResetNeedsConfirmationWithoutTerminalOrInAgentMode(t *testing.T) {
	for _, agent := range []bool{false, true} {
		h := resetHarness(t)
		h.cfg.AgentMode = agent
		before := h.store.Snapshot()
		_, stderr, code := h.run("reset", "--json")
		if code != 1 || !strings.Contains(stderr, "confirmation_required") {
			t.Errorf("agent=%v: exit=%d stderr=%q, want confirmation_required", agent, code, stderr)
		}
		if h.store.Snapshot() != before {
			t.Errorf("agent=%v: store changed without confirmation", agent)
		}
	}
}

func TestResetTypedConfirmation(t *testing.T) {
	tests := []struct {
		typed     string
		wantReset bool
	}{
		{"reset", true},
		{"y", false},
		{"", false},
	}
	for _, tt := range tests {
		h := resetHarness(t)
		h.term = true
		h.stdin.WriteString(tt.typed + "\n")
		out, _ := h.mustRun("reset")
		gone := h.accountCount() == 0
		if gone != tt.wantReset {
			t.Errorf("typed %q: archive reset = %v, want %v (out %q)", tt.typed, gone, tt.wantReset, out)
		}
	}
}

func TestResetEmptyArchiveActsAsInit(t *testing.T) {
	h := newHarness(t)
	out, _ := h.mustRun("reset", "--json")
	if !strings.Contains(out, "initialized") || !strings.Contains(out, "true") {
		t.Errorf("output = %q, want initialized", out)
	}
	cats, _ := h.mustRun("category", "list")
	if !strings.Contains(cats, "Groceries") {
		t.Errorf("categories = %q, want defaults", cats)
	}
}
