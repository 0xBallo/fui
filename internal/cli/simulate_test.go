package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/simulating"
	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/testutil/memstore"
)

func newSimulateTest(t *testing.T) (*Deps, *simulating.Service, *memstore.Store) {
	t.Helper()
	store := memstore.New()
	clock := internalFixedClock{time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)}
	eur, err := domain.ParseCurrency("EUR")
	if err != nil {
		t.Fatalf("ParseCurrency: %v", err)
	}
	set, err := module.Validate([]module.ID{module.Core, module.Recurring, module.Simulations})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	deps := &Deps{
		Service: app.New(store, clock, internalFakeBackup{}, app.Config{Locale: domain.LocaleENUS, Location: time.UTC, BaseCurrency: eur}),
		Config: &config.Config{
			Currency: "EUR", Locale: config.LocaleUS, MonthStartDay: 1,
			Modules: config.ModulesConfig{Enabled: set.IDs()},
		},
		Stdin: &bytes.Buffer{}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Version: "test",
	}
	svc := simulating.New(store, clock, simulating.Config{
		Locale: domain.LocaleENUS, Location: time.UTC, BaseCurrency: eur, MonthStartDay: 1,
	})
	return deps, svc, store
}

func TestSimulatePACTable(t *testing.T) {
	deps, svc, _ := newSimulateTest(t)
	buf := outBuf(deps)
	mustRunCmd(t, deps, NewSimulateCmd(deps, svc), []string{"pac", "--monthly", "200", "--years", "3", "--rate", "4,5", "--chart"})
	out := buf.String()
	for _, want := range []string{"PESSIMISTIC 2.50%", "BASE 4.50%", "OPTIMISTIC 6.50%", "Nominal value", "CHART", "█"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestSimulateInvestmentJSONExplicitRates(t *testing.T) {
	deps, svc, _ := newSimulateTest(t)
	buf := outBuf(deps)
	mustRunCmd(t, deps, NewSimulateCmd(deps, svc), []string{
		"investment", "--initial", "1000", "--years", "2", "--rate", "1", "--rate", "3", "--rate", "5", "--json",
	})
	out := buf.String()
	for _, want := range []string{`"pessimistic_bp":100`, `"base_bp":300`, `"optimistic_bp":500`, `"kind":"investment"`} {
		if !strings.Contains(out, want) {
			t.Errorf("json missing %q:\n%s", want, out)
		}
	}
}

func TestSimulateSaveListCompareRm(t *testing.T) {
	deps, svc, store := newSimulateTest(t)
	mustRunCmd(t, deps, NewSimulateCmd(deps, svc), []string{"save", "plan-a", "pac", "--monthly", "100", "--years", "5", "--rate", "4"})
	mustRunCmd(t, deps, NewSimulateCmd(deps, svc), []string{"save", "plan-b", "investment", "--initial", "5000", "--years", "5", "--rate", "4"})
	mustRunCmd(t, deps, NewSimulateCmd(deps, svc), []string{"save", "plan-c", "cashflow", "--months", "3", "--one-off", "2026-12-01:-100"})

	buf := outBuf(deps)
	mustRunCmd(t, deps, NewSimulateCmd(deps, svc), []string{"list"})
	if out := buf.String(); !strings.Contains(out, "plan-a") || !strings.Contains(out, "cashflow") {
		t.Errorf("list = %q", out)
	}

	buf = outBuf(deps)
	mustRunCmd(t, deps, NewSimulateCmd(deps, svc), []string{"compare", "plan-a", "plan-b", "plan-c"})
	out := buf.String()
	for _, want := range []string{"plan-a", "plan-b", "plan-c", "Nominal value", "Difference at horizon"} {
		if !strings.Contains(out, want) {
			t.Errorf("compare missing %q:\n%s", want, out)
		}
	}

	if err := runCmd(deps, NewSimulateCmd(deps, svc), []string{"compare", "plan-a"}); err == nil {
		t.Error("compare with one name should fail")
	}

	mustRunCmd(t, deps, NewSimulateCmd(deps, svc), []string{"rm", "plan-b"})
	buf = outBuf(deps)
	mustRunCmd(t, deps, NewSimulateCmd(deps, svc), []string{"list", "--json"})
	if strings.Contains(buf.String(), "plan-b") {
		t.Errorf("plan-b still listed: %s", buf.String())
	}

	// Simulations never write transactions.
	if n := countTx(t, store); n != 0 {
		t.Errorf("transactions = %d, want 0", n)
	}
}

func countTx(t *testing.T, store *memstore.Store) int {
	t.Helper()
	var n int
	err := store.Read(context.Background(), func(r app.Repos) error {
		txs, err := r.Transactions().List(context.Background(), domain.TxFilter{}, nil, 0)
		n = len(txs)
		return err
	})
	if err != nil {
		t.Fatalf("list transactions: %v", err)
	}
	return n
}

func TestSimulateCashFlowEvents(t *testing.T) {
	deps, svc, _ := newSimulateTest(t)
	seedAccount(t, deps, "Main")
	buf := outBuf(deps)
	mustRunCmd(t, deps, NewSimulateCmd(deps, svc), []string{
		"cashflow", "--months", "4", "--one-off", "2026-12-01:-50", "--change", "2027-01-01:10",
		"--stop-income", "2027-02-01", "--loan", "2026-12-15:1000:4,5:1", "--csv",
	})
	if out := buf.String(); !strings.HasPrefix(out, "MONTH END,BASE,SCENARIO,DELTA") {
		t.Errorf("csv = %q", out)
	}
}

func TestSimulateErrors(t *testing.T) {
	deps, svc, _ := newSimulateTest(t)
	tests := []struct {
		name string
		svc  *simulating.Service
		args []string
	}{
		{"no rate", svc, []string{"investment", "--initial", "1", "--years", "1"}},
		{"two rates", svc, []string{"investment", "--initial", "1", "--years", "1", "--rate", "1", "--rate", "2"}},
		{"bad rate", svc, []string{"investment", "--years", "1", "--rate", "x"}},
		{"pac no frequency", svc, []string{"pac", "--years", "1", "--rate", "1"}},
		{"bad event", svc, []string{"cashflow", "--one-off", "2026-12-01"}},
		{"bad kind", svc, []string{"save", "n", "weird"}},
		{"nil service", nil, []string{"list"}},
	}
	for _, tc := range tests {
		if err := runCmd(deps, NewSimulateCmd(deps, tc.svc), tc.args); err == nil {
			t.Errorf("%s: expected error", tc.name)
		}
	}
}

func TestSimulateModuleDisabled(t *testing.T) {
	deps, svc, _ := newSimulateTest(t)
	deps.Config.Modules.Enabled = module.Default().IDs()
	err := runCmd(deps, NewSimulateCmd(deps, svc), []string{"list"})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != app.CodeModuleDisabled {
		t.Fatalf("err = %v, want module_disabled", err)
	}
}

func TestParsePercentBP(t *testing.T) {
	tests := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"4,5", 450, false},
		{"4.5", 450, false},
		{"-1.25%", -125, false},
		{"0", 0, false},
		{".5", 50, false},
		{"1.234", 0, true},
		{"x", 0, true},
		{"", 0, true},
		{"--1", 0, true},
	}
	for _, tc := range tests {
		got, err := parsePercentBP(tc.in)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("parsePercentBP(%q) = %d, %v; want %d, err=%v", tc.in, got, err, tc.want, tc.wantErr)
		}
	}
}
