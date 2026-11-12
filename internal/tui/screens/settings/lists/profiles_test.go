package lists_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/settings/lists"
	"github.com/0xballo/fui/internal/tui/theme"
)

// stubProfilesConfig is an in-memory lists.ProfilesConfig for tests; no real internal/config
// adapter exists yet (see the task report's REQ note on listing/deleting profiles).
type stubProfilesConfig struct{ profiles map[string]lists.Profile }

func newStubProfilesConfig() *stubProfilesConfig {
	return &stubProfilesConfig{profiles: map[string]lists.Profile{}}
}

func (c *stubProfilesConfig) ListProfiles() ([]lists.Profile, error) {
	out := make([]lists.Profile, 0, len(c.profiles))
	for _, p := range c.profiles {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *stubProfilesConfig) SaveProfile(p lists.Profile) error {
	c.profiles[p.Name] = p
	return nil
}

func (c *stubProfilesConfig) DeleteProfile(name string) error {
	if _, ok := c.profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}
	delete(c.profiles, name)
	return nil
}

func newProfilesFixture(t *testing.T) (*stubProfilesConfig, screens.Screen) {
	t.Helper()
	cfg := newStubProfilesConfig()
	scr := screens.Screen(lists.NewProfiles(cfg, theme.Default()))
	scr.SetSize(100, 30)
	return cfg, scr
}

func TestProfilesLoadAddEditRemove(t *testing.T) {
	cfg, scr := newProfilesFixture(t)
	if err := cfg.SaveProfile(lists.Profile{Name: "revolut", TargetAccount: "Checking"}); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}
	scr = run(t, scr, scr.Init())

	if !strings.Contains(scr.View(), "revolut") {
		t.Fatalf("expected revolut in view, got:\n%s", scr.View())
	}

	// Add a profile. Thirteen fields: name, date, amount, debit, credit, description,
	// external id, delimiter, encoding, date format, decimal separator, skip rows, target account.
	scr, _ = sendRunes(scr, "a")
	if !strings.Contains(scr.View(), "Add import profile") {
		t.Fatalf("expected the add form to open, got:\n%s", scr.View())
	}
	scr, _ = sendRunes(scr, "widiba")
	for range 12 {
		scr, _ = sendKey(scr, tea.KeyEnter)
	}
	scr, cmd := sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)
	if !strings.Contains(scr.View(), "widiba") {
		t.Fatalf("expected widiba after add, got:\n%s", scr.View())
	}

	// Edit revolut's target account.
	scr, _ = sendKey(scr, tea.KeyUp)
	scr, _ = sendRunes(scr, "e")
	if !strings.Contains(scr.View(), "Edit import profile") {
		t.Fatalf("expected the edit form to open, got:\n%s", scr.View())
	}
	for range 12 {
		scr, _ = sendKey(scr, tea.KeyEnter)
	}
	for range "Checking" {
		scr, _ = sendKey(scr, tea.KeyBackspace)
	}
	scr, _ = sendRunes(scr, "Savings")
	scr, cmd = sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)
	if !strings.Contains(scr.View(), "Savings") {
		t.Fatalf("expected Savings after edit, got:\n%s", scr.View())
	}

	// Remove widiba.
	scr, _ = sendKey(scr, tea.KeyDown)
	scr, cmd = sendRunes(scr, "d")
	scr = run(t, scr, cmd)
	if !strings.Contains(scr.View(), "Delete profile") {
		t.Fatalf("expected a delete confirmation, got:\n%s", scr.View())
	}
	scr, cmd = sendRunes(scr, "y")
	scr = run(t, scr, cmd)
	if strings.Contains(scr.View(), "widiba") {
		t.Fatalf("expected widiba removed, got:\n%s", scr.View())
	}
}
