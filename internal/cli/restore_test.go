package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// dumpWithConfig exports a JSON dump whose embedded config is cfgText and returns its path.
func dumpWithConfig(t *testing.T, h *harness, cfgText string) string {
	t.Helper()
	if err := os.WriteFile(h.cfg.ConfigFile, []byte(cfgText), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "dump")
	h.mustRun("export", "--all", "--format", "json", "--output", dir)
	return filepath.Join(dir, "dump.json")
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestRestoreRoutesByExtension covers .json going through the dump loader and anything else
// through the .db restore.
func TestRestoreRoutesByExtension(t *testing.T) {
	h := newHarness(t)
	path := dumpWithConfig(t, h, "locale = \"it-IT\"\n")

	out, _ := h.mustRun("restore", path, "--yes", "--json")
	if !strings.Contains(out, `"restored"`) {
		t.Errorf("output = %q, want restored", out)
	}
	if len(h.backup.loaded) != 1 || len(h.backup.restored) != 0 {
		t.Fatalf("loaded=%d restored=%v, want one dump load and no .db restore", len(h.backup.loaded), h.backup.restored)
	}

	h.mustRun("restore", "/backups/manual.db", "--yes")
	if len(h.backup.loaded) != 1 || len(h.backup.restored) != 1 {
		t.Errorf("loaded=%d restored=%v, want the .db path to use Restore", len(h.backup.loaded), h.backup.restored)
	}
}

// TestRestoreDumpWithConfig covers --with-config: the dump's config replaces the current one,
// which is kept as config.toml.bak; without the flag the config is left alone.
func TestRestoreDumpWithConfig(t *testing.T) {
	h := newHarness(t)
	path := dumpWithConfig(t, h, "from-dump = 1\n")
	if err := os.WriteFile(h.cfg.ConfigFile, []byte("current = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	h.mustRun("restore", path, "--yes")
	if got := readFile(t, h.cfg.ConfigFile); got != "current = 1\n" {
		t.Errorf("config without --with-config = %q, want untouched", got)
	}

	h.mustRun("restore", path, "--yes", "--with-config")
	if got := readFile(t, h.cfg.ConfigFile); got != "from-dump = 1\n" {
		t.Errorf("config = %q, want the dump's", got)
	}
	if got := readFile(t, h.cfg.ConfigFile+".bak"); got != "current = 1\n" {
		t.Errorf("config.toml.bak = %q, want the previous config", got)
	}
}

// TestRestoreDumpErrors covers an invalid dump and --with-config on a .db restore.
func TestRestoreDumpErrors(t *testing.T) {
	h := newHarness(t)
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := h.run("restore", bad, "--yes"); code != 1 {
		t.Errorf("invalid dump: exit = %d, want 1; stderr=%s", code, stderr)
	}
	if _, stderr, code := h.run("restore", "/backups/x.db", "--yes", "--with-config"); code != 1 {
		t.Errorf("--with-config on .db: exit = %d, want 1; stderr=%s", code, stderr)
	}
	if _, _, code := h.run("restore", filepath.Join(t.TempDir(), "missing.json"), "--yes"); code == 0 {
		t.Error("missing dump file: want a failure")
	}
	if len(h.backup.loaded) != 0 {
		t.Errorf("loaded = %d, want nothing loaded", len(h.backup.loaded))
	}
}

// TestRestoreDumpDryRun covers --dry-run writing nothing and confirmation being required.
func TestRestoreDumpDryRun(t *testing.T) {
	h := newHarness(t)
	path := dumpWithConfig(t, h, "")
	h.mustRun("restore", path, "--dry-run")
	if _, stderr, code := h.run("restore", path, "--json"); code != 1 || !strings.Contains(stderr, "confirmation_required") {
		t.Errorf("without --yes: exit = %d stderr=%s, want confirmation_required", code, stderr)
	}
	if len(h.backup.loaded) != 0 {
		t.Errorf("loaded = %d, want nothing loaded", len(h.backup.loaded))
	}
}

// TestExportAllEmbedsConfig covers dump.json carrying the config.toml text.
func TestExportAllEmbedsConfig(t *testing.T) {
	h := newHarness(t)
	path := dumpWithConfig(t, h, "marker-xyz = 1\n")
	if !strings.Contains(readFile(t, path), "marker-xyz") {
		t.Error("dump.json does not embed the config")
	}
}
