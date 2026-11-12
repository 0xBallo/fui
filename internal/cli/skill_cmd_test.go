package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillShowPrintsDocument(t *testing.T) {
	h := newHarness(t)
	out, _ := h.mustRun("skill", "show")
	if !strings.Contains(out, "fui tx split") || !strings.Contains(out, "version: test") {
		t.Fatalf("skill show output missing v0.2 commands or version:\n%s", out)
	}
}

func TestSkillInstallWritesUnderTarget(t *testing.T) {
	h := newHarness(t)
	target := t.TempDir()

	out, _ := h.mustRun("skill", "install", "--target", target, "--dry-run")
	if !strings.Contains(out, "Would install") {
		t.Fatalf("dry-run output = %q", out)
	}
	if _, err := os.Stat(filepath.Join(target, "fui", "SKILL.md")); err == nil {
		t.Fatal("dry-run must not write the skill")
	}

	out, _ = h.mustRun("skill", "install", "--target", target, "--json")
	if !strings.Contains(out, `"written":true`) {
		t.Fatalf("json output = %q", out)
	}
	if _, err := os.Stat(filepath.Join(target, "fui", "SKILL.md")); err != nil {
		t.Fatalf("skill not installed: %v", err)
	}
}

func TestSkillInstallRejectsArgs(t *testing.T) {
	h := newHarness(t)
	if _, _, code := h.run("skill", "install", "extra"); code == 0 {
		t.Fatal("skill install with a positional argument should fail")
	}
}
