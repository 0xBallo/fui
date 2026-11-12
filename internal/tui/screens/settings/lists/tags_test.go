package lists_test

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/screens/settings/lists"
	"github.com/0xballo/fui/internal/tui/theme"
)

func newTagsFixture(t *testing.T) (screentest.Fixture, screens.Screen) {
	t.Helper()
	fx := screentest.New(t)
	scr := screens.Screen(lists.NewTags(fx.Service, theme.Default()))
	scr.SetSize(100, 30)
	return fx, scr
}

func addTag(t *testing.T, fx screentest.Fixture, name string) {
	t.Helper()
	ctx := context.Background()
	plan, err := fx.Service.PlanAddTag(ctx, app.AddTagRequest{Name: name})
	if err != nil {
		t.Fatalf("PlanAddTag(%q): %v", name, err)
	}
	if _, err := fx.Service.ApplyAddTag(ctx, plan); err != nil {
		t.Fatalf("ApplyAddTag(%q): %v", name, err)
	}
}

func TestTagsLoadAddRenameRemove(t *testing.T) {
	fx, scr := newTagsFixture(t)
	addTag(t, fx, "business")
	scr = run(t, scr, scr.Init())

	if !strings.Contains(scr.View(), "business") {
		t.Fatalf("expected business in view, got:\n%s", scr.View())
	}

	// Add a tag.
	scr, _ = sendRunes(scr, "a")
	if !strings.Contains(scr.View(), "Add tag") {
		t.Fatalf("expected the add form to open, got:\n%s", scr.View())
	}
	scr, _ = sendRunes(scr, "personal")
	scr, cmd := sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)
	if !strings.Contains(scr.View(), "personal") {
		t.Fatalf("expected personal after add, got:\n%s", scr.View())
	}

	// Rename business.
	scr, _ = sendKey(scr, tea.KeyUp)
	scr, _ = sendRunes(scr, "e")
	if !strings.Contains(scr.View(), "Rename tag") {
		t.Fatalf("expected the rename form to open, got:\n%s", scr.View())
	}
	for range "business" {
		scr, _ = sendKey(scr, tea.KeyBackspace)
	}
	scr, _ = sendRunes(scr, "work")
	scr, cmd = sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)
	if !strings.Contains(scr.View(), "work") {
		t.Fatalf("expected work after rename, got:\n%s", scr.View())
	}

	// Remove work.
	scr, cmd = sendRunes(scr, "d")
	scr = run(t, scr, cmd)
	if !strings.Contains(scr.View(), "Delete tag") {
		t.Fatalf("expected a delete confirmation, got:\n%s", scr.View())
	}
	scr, cmd = sendRunes(scr, "y")
	scr = run(t, scr, cmd)
	if strings.Contains(scr.View(), "work") {
		t.Fatalf("expected work removed, got:\n%s", scr.View())
	}
}
