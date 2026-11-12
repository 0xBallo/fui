package lists_test

import (
	"strings"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/recurring"
	"github.com/0xballo/fui/internal/app/rules"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/screens/settings/lists"
	"github.com/0xballo/fui/internal/tui/theme"
)

// TestScreenChrome checks the small Screen-interface accessors (Title, KeyMap, InputActive)
// across every sub-view: cheap bugs here (e.g. a copy-pasted wrong title) are easy to miss.
func TestScreenChrome(t *testing.T) {
	fx := screentest.New(t)
	th := theme.Default()
	enabled, err := module.Validate([]module.ID{module.Core, module.Recurring})
	if err != nil {
		t.Fatalf("module.Validate: %v", err)
	}

	cats := lists.NewCategories(fx.Service, th)
	if cats.Title() != "Categories" || len(cats.KeyMap()) == 0 || cats.InputActive() {
		t.Fatalf("unexpected Categories chrome: title=%q keys=%d active=%v", cats.Title(), len(cats.KeyMap()), cats.InputActive())
	}

	tags := lists.NewTags(fx.Service, th)
	if tags.Title() != "Tags" || len(tags.KeyMap()) == 0 || tags.InputActive() {
		t.Fatalf("unexpected Tags chrome: title=%q", tags.Title())
	}

	rulesUC := rulesAdapter{Service: rules.New(fx.Store, stubBackup{}), lookups: lookups{svc: fx.Service}}
	rl := lists.NewRules(rulesUC, th)
	if rl.Title() != "Rules" || len(rl.KeyMap()) != 7 || rl.InputActive() {
		t.Fatalf("unexpected Rules chrome: title=%q keys=%d", rl.Title(), len(rl.KeyMap()))
	}

	recCfg := recurring.Config{Locale: domain.LocaleENUS, Location: time.UTC}
	recUC := recurringAdapter{
		Service: recurring.New(fx.Store, fixedClock{now: time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)}, stubBackup{}, recCfg),
		lookups: lookups{svc: fx.Service},
	}
	rec := lists.NewRecurrences(recUC, enabled, domain.LocaleENUS, th)
	if rec.Title() != "Recurrences" || len(rec.KeyMap()) == 0 || rec.InputActive() {
		t.Fatalf("unexpected Recurrences chrome: title=%q", rec.Title())
	}

	profiles := lists.NewProfiles(newStubProfilesConfig(), th)
	if profiles.Title() != "Import profiles" || len(profiles.KeyMap()) == 0 || profiles.InputActive() {
		t.Fatalf("unexpected Profiles chrome: title=%q", profiles.Title())
	}
}

// TestConfirmCancel checks that pressing "n" on the delete confirmation returns to the list
// without deleting.
func TestConfirmCancel(t *testing.T) {
	fx, scr := newCategoriesFixture(t)
	fx.Category(t, app.AddCategoryRequest{Name: "Groceries", Kind: "expense"})
	scr = run(t, scr, scr.Init())
	scr, cmd := sendRunes(scr, "d")
	scr = run(t, scr, cmd)
	scr, _ = sendRunes(scr, "n")
	if !strings.Contains(scr.View(), "Groceries") {
		t.Fatalf("expected Groceries still listed after cancel, got:\n%s", scr.View())
	}
}
