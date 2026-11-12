package skill_test

import (
	"strings"
	"testing"

	"github.com/0xballo/fui/internal/skill"
)

func TestRenderSubstitutesVersion(t *testing.T) {
	out := skill.Render("1.2.3")
	if strings.Contains(out, "{{FUI_VERSION}}") {
		t.Errorf("Render left the version placeholder unsubstituted:\n%s", out)
	}
	if !strings.Contains(out, "version: 1.2.3") {
		t.Errorf("Render(%q) front matter missing the rendered version", "1.2.3")
	}
}

// TestRenderCoversRequiredSections guards the functional spec's skill content requirements: a
// command reference, the JSON output contract, the typical flows, and the four safety rules.
func TestRenderCoversRequiredSections(t *testing.T) {
	out := skill.Render("dev")
	for _, want := range []string{
		"## Command reference",
		"## JSON output format",
		"## Typical flows",
		"## Safety rules",
		"fui module list --json",
		"fui tx search TEXT",
		"--month",
		"--uncategorized",
		"--payee",
		"--income",
		"--expense",
		"--batch",
		"--sort",
		"--desc",
		"--after-date",
		"Preview before writing",
		"Confirm, then `--yes`",
		"Never delete, reset, or restore without an explicit request",
		"Prefer aggregates to raw rows",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("skill document missing required content %q", want)
		}
	}
}
