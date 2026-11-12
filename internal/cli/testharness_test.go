package cli_test

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/cli"
	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/testutil/memstore"
	"github.com/0xballo/fui/pkg/fuiproto"
)

// fixedClock is a deterministic app.Clock: every test runs as if "now" were 2026-11-11 10:00 UTC.
type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

// fakeBackup is a no-op app.Backuper that records its calls, for restore/backup command tests.
type fakeBackup struct {
	reasons  []string
	restored []string
}

func (b *fakeBackup) Backup(_ context.Context, reason string) (string, error) {
	b.reasons = append(b.reasons, reason)
	return "/backups/" + reason + ".db", nil
}

func (b *fakeBackup) Restore(_ context.Context, path string) error {
	b.restored = append(b.restored, path)
	return nil
}

// harness builds a cli.Deps over an empty memstore and runs command lines against it, capturing
// output and the exit code. Each run gets a fresh Deps so stdout/stderr never leak between cases.
type harness struct {
	t         *testing.T
	store     *memstore.Store
	svc       *app.Service
	importing *importing.Service
	connect   *fakeConnectors
	backup    *fakeBackup
	cfg       *config.Config
	term      bool
	stdin     *bytes.Buffer
	reporting cli.Reporting
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	store := memstore.New()
	backup := &fakeBackup{}
	clock := fixedClock{time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)}
	svcCfg := app.Config{Locale: domain.LocaleENUS, Location: time.UTC, BaseCurrency: currency(t, "EUR")}
	svc := app.New(store, clock, backup, svcCfg)
	connect := &fakeConnectors{csvFixtures: map[string][]fuiproto.Record{}}
	cfg := &config.Config{
		Currency: "EUR", Locale: config.LocaleUS, MonthStartDay: 1,
		Modules:    config.ModulesConfig{Enabled: module.Default().IDs()},
		ConfigFile: filepath.Join(t.TempDir(), "config.toml"),
	}
	return &harness{
		t: t, store: store, svc: svc, importing: importing.New(store, clock, backup, connect), connect: connect,
		backup: backup, cfg: cfg, stdin: &bytes.Buffer{},
		reporting: mustReporting(t, store, clock),
	}
}

// mustReporting builds the reporting use cases over store's reporting.Source test adapter.
func mustReporting(t *testing.T, store *memstore.Store, clock reporting.Clock) cli.Reporting {
	t.Helper()
	src := reportingSource{store: store}
	spending, err := reporting.NewSpending(src, clock)
	if err != nil {
		t.Fatalf("NewSpending: %v", err)
	}
	cashFlow, err := reporting.NewCashFlow(src, clock)
	if err != nil {
		t.Fatalf("NewCashFlow: %v", err)
	}
	runway, err := reporting.NewRunway(src, clock)
	if err != nil {
		t.Fatalf("NewRunway: %v", err)
	}
	analytics := reporting.NewAnalytics(store, clock, src, reporting.Config{
		Locale:        domain.LocaleENUS,
		BaseCurrency:  currency(t, "EUR"),
		MonthStartDay: 1,
		HorizonDays:   config.DefaultForecastDays,
		Modules:       module.Default(),
	})
	return cli.Reporting{Spending: spending, CashFlow: cashFlow, Runway: runway, Analytics: analytics}
}

// useItalianLocale switches the harness to it-IT (comma decimal separator, dd/mm/yyyy dates),
// for tests that exercise the spec's Italian-locale examples; it rebuilds the Service over the
// same store so any data seeded so far is kept.
func (h *harness) useItalianLocale() {
	svcCfg := app.Config{Locale: domain.LocaleITIT, Location: time.UTC, BaseCurrency: currency(h.t, "EUR")}
	h.svc = app.New(h.store, fixedClock{time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)}, h.backup, svcCfg)
	h.cfg.Locale = config.LocaleIT
}

func currency(t *testing.T, code string) domain.Currency {
	t.Helper()
	c, err := domain.ParseCurrency(code)
	if err != nil {
		t.Fatalf("ParseCurrency(%q): %v", code, err)
	}
	return c
}

// run executes args against the harness's Deps and returns stdout, stderr and the exit code.
func (h *harness) run(args ...string) (stdout, stderr string, code int) {
	h.t.Helper()
	var out, errw bytes.Buffer
	deps := h.deps(&out, &errw)
	code = cli.Execute(context.Background(), deps, args)
	return out.String(), errw.String(), code
}

func (h *harness) deps(stdout, stderr *bytes.Buffer) *cli.Deps {
	out := io.Writer(stdout)
	if stdout == nil {
		out = io.Discard
	}
	errw := io.Writer(stderr)
	if stderr == nil {
		errw = io.Discard
	}
	return &cli.Deps{
		Service: h.svc, Importing: h.importing, Reporting: h.reporting, Config: h.cfg,
		Stdin: h.stdin, Stdout: out, Stderr: errw, IsTerminal: h.term, Version: "test",
	}
}

// mustRun runs args and fails the test unless it exits 0.
func (h *harness) mustRun(args ...string) (stdout, stderr string) {
	h.t.Helper()
	out, errw, code := h.run(args...)
	if code != 0 {
		h.t.Fatalf("run(%v) = exit %d, stderr %q", args, code, errw)
	}
	return out, errw
}

// account seeds an account directly through the service, bypassing the CLI, so command tests can
// focus on the command under test.
func (h *harness) account(name, kind, cur, opening string) domain.Account {
	h.t.Helper()
	ctx := context.Background()
	p, err := h.svc.PlanAddAccount(ctx, app.AddAccountRequest{
		Name: name, Kind: kind, Currency: cur, OpeningBalance: opening, OpenedOn: "2026-01-01",
	})
	if err != nil {
		h.t.Fatalf("PlanAddAccount(%s): %v", name, err)
	}
	a, err := h.svc.ApplyAddAccount(ctx, p)
	if err != nil {
		h.t.Fatalf("ApplyAddAccount(%s): %v", name, err)
	}
	return a
}

// category seeds a category directly through the service.
func (h *harness) category(name, kind string) domain.Category {
	h.t.Helper()
	ctx := context.Background()
	p, err := h.svc.PlanAddCategory(ctx, app.AddCategoryRequest{Name: name, Kind: kind})
	if err != nil {
		h.t.Fatalf("PlanAddCategory(%s): %v", name, err)
	}
	c, err := h.svc.ApplyAddCategory(ctx, p)
	if err != nil {
		h.t.Fatalf("ApplyAddCategory(%s): %v", name, err)
	}
	return c
}
