// Package main is the fui composition root: the only place that knows every port
// implementation (SQLite store, connector registry, system clock, file logging) and wires them
// into internal/cli.Deps. Nothing else in the module is allowed to import internal/store or
// internal/connector alongside internal/cli; see the depguard rules in .golangci.yml.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/mattn/go-isatty"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/budgeting"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/app/rules"
	"github.com/0xballo/fui/internal/cli"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/clock"
	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/connector"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/logging"
	"github.com/0xballo/fui/internal/store/sqlite"
	"github.com/0xballo/fui/internal/tui"
)

// version is the fui release version, printed by --version and embedded in the agent skill.
// The release build sets it with `-ldflags -X main.version=vX.Y.Z` (see the Makefile); it stays
// "dev" for `go run`/`go test`/`go build` without that flag.
var version = "dev"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	os.Exit(run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run builds the dependencies for args and executes the command tree, returning the process
// exit code. It is a function, not inlined in main, so a test can drive it without os.Exit.
func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, logger, closeLog, err := bootstrap(args)
	if err != nil {
		return bail(stderr, earlyJSON(args), err)
	}
	defer func() { _ = closeLog.Close() }()

	// A failure before cli.Execute parses flags has no plan/report command context yet, so the
	// error format falls back to agent mode or an explicit --json/--agent token on the command
	// line (the same detection bootstrap's own failure path uses).
	asJSON := cfg.AgentMode || earlyJSON(args)

	appClock, err := resolveClock(os.Getenv)
	if err != nil {
		return bail(stderr, asJSON, err)
	}

	deps := &cli.Deps{
		Config: cfg, Stdin: stdin, Stdout: stdout, Stderr: stderr,
		IsTerminal: isTerminal(stdout), Version: version,
		RunTUI: func(context.Context) error {
			return errors.New("fui: the TUI needs the archive open; this path should be unreachable")
		},
	}

	if requiresStore(args) {
		closeStore, err := wireStore(ctx, cfg, appClock, deps)
		if err != nil {
			return bail(stderr, asJSON, err)
		}
		defer func() { _ = closeStore() }()
	}

	logger.Debug("executing command", "args", args)
	return cli.Execute(ctx, deps, args)
}

// bootstrap resolves the configuration and sets up file logging, in that order: logging needs
// the resolved log path and level. Config warnings (unknown TOML keys) are logged, never printed
// to stdout or stderr, so they cannot corrupt script-consumed output.
func bootstrap(args []string) (*config.Config, *slog.Logger, io.Closer, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("resolve home directory: %w", err)
	}
	dbFlag, verboseFlag := prescanGlobalFlags(args)
	cfg, warnings, err := config.Resolve(config.ResolveOptions{
		HomeDir: homeDir,
		Env:     os.Getenv,
		Flags:   config.Flags{DBPath: dbFlag, Verbose: verboseFlag},
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("resolve configuration: %w", err)
	}
	logger, closeLog, err := logging.Setup(cfg.LogPath, logging.Options{
		Level:        logging.ParseLevel(cfg.LogLevel),
		LogSensitive: cfg.LogSensitive,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("set up logging: %w", err)
	}
	for _, w := range warnings {
		logger.Warn(w)
	}
	return cfg, logger, closeLog, nil
}

// wireStore opens the SQLite archive and builds the app, reporting and importing use cases over
// it, filling deps.Service, deps.Reporting, deps.Importing and deps.RunTUI. It returns a closer
// for the store, left to the caller so the store stays open for the whole command.
func wireStore(ctx context.Context, cfg *config.Config, appClock app.Clock, deps *cli.Deps) (func() error, error) {
	store, err := sqlite.Open(ctx, sqlite.Options{
		Path: cfg.DBPath, BackupDir: cfg.BackupDir, BackupKeep: cfg.Backup.Keep, Clock: appClock,
	})
	if err != nil {
		return nil, fmt.Errorf("open archive %q: %w", cfg.DBPath, err)
	}
	baseCurrency, err := domain.ParseCurrency(cfg.Currency)
	if err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("parse base currency %q: %w", cfg.Currency, err)
	}
	deps.Service = app.New(store, appClock, store, app.Config{
		Locale: domain.Locale(cfg.Locale), BaseCurrency: baseCurrency,
	})
	rep, err := newReporting(store, store, appClock, cfg, baseCurrency)
	if err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("build reporting use cases: %w", err)
	}
	deps.Reporting = rep
	deps.Importing = importing.New(store, appClock, store, connector.Registry{
		Dir:      filepath.Join(cfg.ConfigDir, "connectors"),
		Timeout:  cfg.Connectors.Timeout.Duration(),
		Profiles: cfg.Import.Profiles,
		Currency: cfg.Currency,
	})
	deps.Rules = rules.New(store, store)
	modules, err := cfg.ModulesEnabled()
	if err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("resolve modules.enabled: %w", err)
	}
	deps.Budgeting = budgeting.New(store, appClock, budgeting.Config{
		Locale: domain.Locale(cfg.Locale), BaseCurrency: baseCurrency, MonthStartDay: cfg.MonthStartDay,
		Mode: domain.BudgetMode(cfg.Budget.Mode), Modules: modules,
	})
	deps.RunTUI = func(ctx context.Context) error {
		return tui.Run(ctx, newTUIUseCases(deps.Service, rep), tui.Config{
			Locale: domain.Locale(cfg.Locale), Theme: cfg.Theme,
			ThemeLight: cfg.ThemeLight, ThemeDark: cfg.ThemeDark, MonthStartDay: cfg.MonthStartDay,
		})
	}
	return store.Close, nil
}

// tuiUseCases adapts app.Service and the reporting use cases to internal/tui.UseCases: Service
// already implements every account/transaction method the TUI screens need, so only the two
// report-running methods (spending, runway) need a thin forwarding method.
type tuiUseCases struct {
	*app.Service
	reporting cli.Reporting
}

func newTUIUseCases(svc *app.Service, rep cli.Reporting) tuiUseCases {
	return tuiUseCases{Service: svc, reporting: rep}
}

// Spending runs the dashboard's spending-by-category report.
func (u tuiUseCases) Spending(ctx context.Context, params reporting.SpendingParams) (reporting.SpendingResult, error) {
	return u.reporting.Spending.Run(ctx, params)
}

// Runway runs the dashboard's runway KPI.
func (u tuiUseCases) Runway(ctx context.Context, params reporting.RunwayParams) (reporting.RunwayResult, error) {
	return u.reporting.Runway.Run(ctx, params)
}

// newReporting builds the read-only reporting and KPI use cases over source.
func newReporting(
	store app.Store,
	source reporting.Source,
	clk reporting.Clock,
	cfg *config.Config,
	baseCurrency domain.Currency,
) (cli.Reporting, error) {
	spending, err := reporting.NewSpending(source, clk)
	if err != nil {
		return cli.Reporting{}, fmt.Errorf("new spending report: %w", err)
	}
	cashFlow, err := reporting.NewCashFlow(source, clk)
	if err != nil {
		return cli.Reporting{}, fmt.Errorf("new cash-flow report: %w", err)
	}
	runway, err := reporting.NewRunway(source, clk)
	if err != nil {
		return cli.Reporting{}, fmt.Errorf("new runway kpi: %w", err)
	}
	modules, err := cfg.ModulesEnabled()
	if err != nil {
		return cli.Reporting{}, fmt.Errorf("resolve modules.enabled: %w", err)
	}
	analytics := reporting.NewAnalytics(store, clk, source, reporting.Config{
		Locale:        domain.Locale(cfg.Locale),
		BaseCurrency:  baseCurrency,
		MonthStartDay: cfg.MonthStartDay,
		HorizonDays:   cfg.Forecast.Horizon,
		Modules:       modules,
	})
	return cli.Reporting{Spending: spending, CashFlow: cashFlow, Runway: runway, Analytics: analytics}, nil
}

// clockTestOverrideEnv freezes "now" to a fixed instant for the e2e testscripts: it is read only
// here, documented as test-only, and never by application code (which always goes through
// internal/clock). Production users have no reason to set it.
const clockTestOverrideEnv = "FUI_TEST_TODAY"

// resolveClock returns the system clock, unless the test-only FUI_TEST_TODAY environment
// variable names a fixed instant (RFC 3339, or a bare YYYY-MM-DD date at midnight UTC).
func resolveClock(env func(string) string) (app.Clock, error) {
	v := env(clockTestOverrideEnv)
	if v == "" {
		return clock.NewSystem(), nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, v); err == nil {
			return clock.NewFixed(t), nil
		}
	}
	return nil, fmt.Errorf("%s=%q: want RFC3339 or YYYY-MM-DD (test-only override)", clockTestOverrideEnv, v)
}

// storeFreeVerbs lists the top-level commands that run without opening the archive: they need
// no data (help, completion, config) or only read the embedded skill (skill show/install).
var storeFreeVerbs = map[string]bool{"help": true, "completion": true, "config": true, "skill": true}

// requiresStore reports whether args need the archive open. --help/-h/--version never do,
// wherever they appear; otherwise it is decided by the first non-flag argument (the verb), so a
// bare invocation (which starts the TUI) and every data command default to opening it.
func requiresStore(args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "--help" || a == "--version" {
			return false
		}
	}
	return !storeFreeVerbs[firstVerb(args)]
}

// firstVerb returns the first non-flag argument of args, skipping --db and its value.
func firstVerb(args []string) string {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--db" {
			i++
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		return a
	}
	return ""
}

// prescanGlobalFlags extracts --db and --verbose/-v from args without the rest of cobra's flag
// parsing, which cannot run yet: the configuration it would depend on (base currency, locale)
// is what --db indirectly selects by naming a different archive.
func prescanGlobalFlags(args []string) (dbPath string, verbose bool) {
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--db" && i+1 < len(args):
			dbPath = args[i+1]
			i++
		case strings.HasPrefix(a, "--db="):
			dbPath = strings.TrimPrefix(a, "--db=")
		case a == "--verbose" || a == "-v":
			verbose = true
		}
	}
	return dbPath, verbose
}

// earlyJSON decides the error format for a failure before the configuration is resolved (so
// deps.Config.AgentMode is not known yet): --json on the command line, or FUI_AGENT already
// truthy in the environment.
func earlyJSON(args []string) bool {
	if truthyEnv(os.Getenv("FUI_AGENT")) {
		return true
	}
	for _, a := range args {
		if a == "--json" {
			return true
		}
	}
	return false
}

// truthyEnv mirrors internal/config's FUI_BOOL parsing (1/true/yes/on, case-insensitive) for the
// handful of environment variables main.go must read before a *config.Config exists.
func truthyEnv(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// isTerminal reports whether w is a terminal, including a Windows Cygwin/MSYS one.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fd := f.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

// bail renders err to stderr in asJSON's format and returns the matching process exit code; it
// is used for failures before cli.Execute runs the command tree, which otherwise does this
// itself.
func bail(stderr io.Writer, asJSON bool, err error) int {
	if werr := render.WriteError(stderr, asJSON, err); werr != nil {
		fmt.Fprintln(stderr, err) //nolint:errcheck // last-resort fallback, stderr itself is failing
	}
	return cli.ExitCode(err)
}
