package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app/assetting"
	"github.com/0xballo/fui/internal/cli"
	"github.com/0xballo/fui/internal/module"
)

func TestAssetCommandsAddListValue(t *testing.T) {
	h := newHarness(t)
	mods := assetsModules(t)

	out, _, err := runWithAssetCommands(h, mods, "asset", "add", "House", "--kind", "property", "--value", "300000", "--purchase-date", "2026-01-01", "--json")
	if err != nil {
		t.Fatalf("asset add: %v", err)
	}
	if !strings.Contains(out, `"name":"House"`) || !strings.Contains(out, `"kind":"property"`) {
		t.Fatalf("asset add output = %q", out)
	}

	out, _, err = runWithAssetCommands(h, mods, "asset", "value", "House", "320000", "--date", "2026-12-31", "--json")
	if err != nil {
		t.Fatalf("asset value: %v", err)
	}
	if !strings.Contains(out, `"date":"2026-12-31"`) || !strings.Contains(out, `"value":32000000`) {
		t.Fatalf("asset value output = %q", out)
	}

	out, _, err = runWithAssetCommands(h, mods, "asset", "list", "--at", "2026-12-31", "--json")
	if err != nil {
		t.Fatalf("asset list: %v", err)
	}
	if !strings.Contains(out, `"name":"House"`) || !strings.Contains(out, `"net":32000000`) {
		t.Fatalf("asset list output = %q", out)
	}
}

func TestAssetDryRun(t *testing.T) {
	h := newHarness(t)
	mods := assetsModules(t)

	out, _, err := runWithAssetCommands(h, mods, "asset", "add", "House", "--kind", "property", "--value", "300000", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("asset add dry-run: %v", err)
	}
	if !strings.Contains(out, `"entity":"asset"`) {
		t.Fatalf("asset add dry-run output = %q", out)
	}

	out, _, err = runWithAssetCommands(h, mods, "asset", "list", "--json")
	if err != nil {
		t.Fatalf("asset list: %v", err)
	}
	if strings.Contains(out, "House") {
		t.Fatalf("asset list after dry-run = %q, want no assets", out)
	}
}

func TestAssetManageCommands(t *testing.T) {
	h := newHarness(t)
	mods := assetsModules(t)
	run := func(args ...string) string {
		t.Helper()
		out, _, err := runWithAssetCommands(h, mods, args...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		return out
	}
	run("asset", "add", "House", "--kind", "property", "--value", "300000", "--purchase-date", "2026-01-01")
	run("asset", "value", "House", "320000", "--date", "2026-06-01")

	if out := run("asset", "show", "House", "--json"); !strings.Contains(out, `"history":[{"date":"2026-06-01","value":32000000}]`) {
		t.Fatalf("show json = %q", out)
	}
	if out := run("asset", "show", "House"); !strings.Contains(out, "valuations:") || !strings.Contains(out, "loan: -") {
		t.Fatalf("show table = %q", out)
	}
	if out := run("asset", "show", "House", "--csv"); !strings.Contains(out, "2026-06-01,320000") {
		t.Fatalf("show csv = %q", out)
	}
	if out := run("asset", "edit", "House", "--name", "Home", "--json"); !strings.Contains(out, `"name":"Home"`) {
		t.Fatalf("edit = %q", out)
	}
	if _, _, err := runWithAssetCommands(h, mods, "asset", "edit", "Home"); err == nil {
		t.Fatal("edit without changes: want error")
	}
	if out := run("asset", "value", "rm", "Home", "2026-06-01", "--dry-run", "--json"); !strings.Contains(out, `"entity":"valuation"`) {
		t.Fatalf("value rm dry-run = %q", out)
	}
	if out := run("asset", "value", "rm", "Home", "2026-06-01", "--json"); !strings.Contains(out, `"action":"valuation removed from"`) {
		t.Fatalf("value rm = %q", out)
	}
	if _, _, err := runWithAssetCommands(h, mods, "asset", "unlink", "Home"); err == nil {
		t.Fatal("unlink without link: want error")
	}
	if out := run("asset", "rm", "Home", "--yes", "--csv"); !strings.Contains(out, "removed,Home") {
		t.Fatalf("rm = %q", out)
	}
	if out := run("asset", "list", "--json"); strings.Contains(out, "Home") {
		t.Fatalf("list after rm = %q", out)
	}
}

func TestAssetCommandModuleAnnotation(t *testing.T) {
	h := newHarness(t)
	deps := h.deps(nil, nil)
	cmd := cli.NewAssetCmd(deps, newAssetService(t, h))
	if got := cmd.Annotations["module"]; got != "assets" {
		t.Fatalf("asset module annotation = %q, want assets", got)
	}
}

func newAssetService(t *testing.T, h *harness) *assetting.Service {
	t.Helper()
	return assetting.New(h.store, fixedClock{time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)}, h.backup, assetting.Config{
		Locale: "en-US", Location: time.UTC, BaseCurrency: currency(t, h.cfg.Currency),
	})
}

func assetsModules(t *testing.T) module.Set {
	t.Helper()
	ids := append(module.Default().IDs(), module.Assets)
	set, err := module.Validate(ids)
	if err != nil {
		t.Fatalf("module.Validate: %v", err)
	}
	return set
}

func runWithAssetCommands(h *harness, mods module.Set, args ...string) (stdout, stderr string, runErr error) {
	var out bytes.Buffer
	var errw bytes.Buffer
	h.cfg.Modules.Enabled = mods.IDs()
	deps := h.deps(&out, &errw)
	deps.Assets = newAssetService(h.t, h)
	root := cli.NewRoot(deps)
	root.SetArgs(args)
	root.SetIn(h.stdin)
	root.SetOut(&out)
	root.SetErr(&errw)
	runErr = root.ExecuteContext(context.Background())
	return out.String(), errw.String(), runErr
}
