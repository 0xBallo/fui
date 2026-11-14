package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app/investing"
	"github.com/0xballo/fui/internal/cli"
	"github.com/0xballo/fui/internal/module"
)

func TestSecurityInvestPositionCommands(t *testing.T) {
	h := newHarness(t)
	h.account("Broker", "investment", "EUR", "0")
	h.account("Cash", "checking", "EUR", "10000")
	mods := investingModules(t, true)

	out, _, err := runWithInvestingCommands(h, mods, "security", "add", "VWCE ETF", "--kind", "etf", "--ticker", "VWCE", "--currency", "EUR", "--json")
	if err != nil {
		t.Fatalf("security add: %v", err)
	}
	if !strings.Contains(out, `"kind":"etf"`) || !strings.Contains(out, `"ticker":"VWCE"`) {
		t.Fatalf("security add output = %q", out)
	}

	out, _, err = runWithInvestingCommands(h, mods, "security", "price", "VWCE", "100", "--date", "2026-03-01", "--json")
	if err != nil {
		t.Fatalf("security price: %v", err)
	}
	if !strings.Contains(out, `"price":10000`) || !strings.Contains(out, `"date":"2026-03-01"`) {
		t.Fatalf("security price output = %q", out)
	}

	out, _, err = runWithInvestingCommands(h, mods, "invest", "buy", "VWCE", "--account", "Broker", "--cash-account", "Cash", "--quantity", "10", "--price", "100", "--fees", "1", "--date", "2026-03-01", "--json")
	if err != nil {
		t.Fatalf("invest buy: %v", err)
	}
	if !strings.Contains(out, `"kind":"buy"`) || !strings.Contains(out, `"quantity":"10"`) {
		t.Fatalf("invest buy output = %q", out)
	}

	out, _, err = runWithInvestingCommands(h, mods, "position", "list", "--json")
	if err != nil {
		t.Fatalf("position list: %v", err)
	}
	if !strings.Contains(out, `"security":"VWCE ETF"`) || !strings.Contains(out, `"market_value":100000`) {
		t.Fatalf("position list output = %q", out)
	}
}

func TestInvestingDryRun(t *testing.T) {
	h := newHarness(t)
	h.account("Broker", "investment", "EUR", "0")
	mods := investingModules(t, true)

	if _, _, err := runWithInvestingCommands(h, mods, "security", "add", "VWCE ETF", "--kind", "etf", "--ticker", "VWCE", "--currency", "EUR", "--json"); err != nil {
		t.Fatalf("security add: %v", err)
	}

	out, _, err := runWithInvestingCommands(h, mods, "invest", "buy", "VWCE", "--account", "Broker", "--quantity", "1", "--price", "100", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("invest buy dry-run: %v", err)
	}
	if !strings.Contains(out, `"entity":"security_op"`) {
		t.Fatalf("invest buy dry-run output = %q", out)
	}

	out, _, err = runWithInvestingCommands(h, mods, "position", "list", "--json")
	if err != nil {
		t.Fatalf("position list: %v", err)
	}
	if strings.Contains(out, "VWCE ETF") {
		t.Fatalf("position list after dry-run = %q, want no position rows", out)
	}
}

func TestSecurityCommandModuleAnnotation(t *testing.T) {
	h := newHarness(t)
	deps := h.deps(nil, nil)
	inv := newInvestingService(t, h, investingModules(t, true))
	cmd := cli.NewSecurityCmd(deps, inv)
	if got := cmd.Annotations["module"]; got != "investments" {
		t.Fatalf("security module annotation = %q, want investments", got)
	}
}

func TestSecurityCryptoRequiresCryptoModule(t *testing.T) {
	h := newHarness(t)
	mods := investingModules(t, false)
	_, _, err := runWithInvestingCommands(h, mods, "security", "add", "Bitcoin", "--kind", "crypto", "--ticker", "BTC", "--currency", "EUR", "--json")
	if err == nil {
		t.Fatal("expected crypto command error, got nil")
	}
	if !strings.Contains(err.Error(), "module") || !strings.Contains(err.Error(), "crypto") {
		t.Fatalf("error = %v, want crypto module-disabled error", err)
	}
}

func TestGoldenInvestingOutputs(t *testing.T) {
	h := newHarness(t)
	h.account("Broker", "investment", "EUR", "0")
	h.account("Cash", "checking", "EUR", "10000")
	mods := investingModules(t, true)

	if _, _, err := runWithInvestingCommands(h, mods, "security", "add", "VWCE ETF", "--kind", "etf", "--ticker", "VWCE", "--currency", "EUR", "--json"); err != nil {
		t.Fatalf("security add: %v", err)
	}
	if _, _, err := runWithInvestingCommands(h, mods, "security", "price", "VWCE", "100", "--date", "2026-03-01", "--json"); err != nil {
		t.Fatalf("security price: %v", err)
	}
	if _, _, err := runWithInvestingCommands(h, mods, "invest", "buy", "VWCE", "--account", "Broker", "--cash-account", "Cash", "--quantity", "10", "--price", "100", "--fees", "1", "--date", "2026-03-01", "--json"); err != nil {
		t.Fatalf("invest buy: %v", err)
	}

	out, _, err := runWithInvestingCommands(h, mods, "position", "list", "--json")
	if err != nil {
		t.Fatalf("position list --json: %v", err)
	}
	checkGolden(t, "position_list.json", []byte(out))

	out, _, err = runWithInvestingCommands(h, mods, "position", "list", "--csv")
	if err != nil {
		t.Fatalf("position list --csv: %v", err)
	}
	checkGolden(t, "position_list.csv", []byte(out))
}

func TestGoldenInvestingHelpJSON(t *testing.T) {
	h := newHarness(t)
	mods := investingModules(t, true)
	out, _, err := runWithInvestingCommands(h, mods, "help", "security", "--json")
	if err != nil {
		t.Fatalf("help security --json: %v", err)
	}
	checkGolden(t, "help_security.json", []byte(out))

	out, _, err = runWithInvestingCommands(h, mods, "help", "invest", "--json")
	if err != nil {
		t.Fatalf("help invest --json: %v", err)
	}
	checkGolden(t, "help_invest.json", []byte(out))

	out, _, err = runWithInvestingCommands(h, mods, "help", "position", "--json")
	if err != nil {
		t.Fatalf("help position --json: %v", err)
	}
	checkGolden(t, "help_position.json", []byte(out))
}

func investingModules(t *testing.T, withCrypto bool) module.Set {
	t.Helper()
	ids := append(module.Default().IDs(), module.Investments)
	if withCrypto {
		ids = append(ids, module.Crypto)
	}
	set, err := module.Validate(ids)
	if err != nil {
		t.Fatalf("module.Validate: %v", err)
	}
	return set
}

func newInvestingService(t *testing.T, h *harness, mods module.Set) *investing.Service {
	t.Helper()
	base := currency(t, h.cfg.Currency)
	return investing.New(h.store, fixedClock{time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)}, investing.Config{
		Locale: "en-US", BaseCurrency: base, Location: time.UTC, Modules: mods,
	})
}

func runWithInvestingCommands(h *harness, mods module.Set, args ...string) (stdout, stderr string, runErr error) {
	var out bytes.Buffer
	var errw bytes.Buffer
	h.cfg.Modules.Enabled = mods.IDs()
	deps := h.deps(&out, &errw)
	inv := newInvestingService(h.t, h, mods)
	root := cli.NewRoot(deps)
	root.AddCommand(cli.NewSecurityCmd(deps, inv), cli.NewInvestCmd(deps, inv), cli.NewPositionCmd(deps, inv))
	root.SetArgs(args)
	root.SetIn(h.stdin)
	root.SetOut(&out)
	root.SetErr(&errw)
	runErr = root.ExecuteContext(context.Background())
	return out.String(), errw.String(), runErr
}
