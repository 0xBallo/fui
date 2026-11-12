// Package cli builds the fui command tree: one cobra command per verb, each a thin adapter that
// parses flags, calls a use case on app.Service, and renders the result. Commands carry no
// business logic; internal/cli/render turns results into table, --json and --csv output.
package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/budgeting"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/app/lending"
	"github.com/0xballo/fui/internal/app/recurring"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/app/rules"
	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/domain"
)

// Deps are the dependencies every command needs. The composition root (cmd/fui) builds one Deps
// from the resolved configuration and adapters; tests build their own on a memstore-backed
// Service.
type Deps struct {
	// Service runs the account, transaction, transfer, category, fx and backup use cases.
	Service *app.Service
	// Importing runs the import, export and connector use cases; nil in builds that have not
	// wired a connector registry yet (requireImporting fails cleanly instead of panicking).
	Importing *importing.Service
	// Reporting runs the read-only report and KPI use cases.
	Reporting Reporting
	// Rules runs the categorization-rule CRUD/test/apply use cases.
	Rules *rules.Service
	// Budgeting runs the budget module's use cases; nil in builds that have not wired it yet
	// (requireBudgeting fails cleanly instead of panicking).
	Budgeting *budgeting.Service
	// Recurring runs the recurring-transaction CRUD, run, skip and confirm use cases.
	Recurring *recurring.Service
	// Loans runs the loan use cases: add, show, schedule, events, pending installments and
	// their confirmation.
	Loans *lending.Service
	// Config is the resolved configuration; AgentMode reflects FUI_AGENT, Locale and Currency
	// drive parsing and formatting.
	Config *config.Config
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	// IsTerminal reports whether Stdout is attached to a terminal: it gates colors and whether
	// missing-field and confirmation prompts may run at all.
	IsTerminal bool
	// RunTUI starts the TUI; called when fui runs with no subcommand.
	RunTUI func(ctx context.Context) error
	// Version is printed by --version and fui/1 help --json.
	Version string
}

// Reporting are the read-only reporting and KPI use cases backing `report run` and `kpi runway`.
// The composition root builds them over the real store; tests build them over a memstore
// adapter.
type Reporting struct {
	Spending reporting.Spending
	CashFlow reporting.CashFlow
	Runway   reporting.Runway
}

// Locale returns the configured number/date locale, defaulting to en-US.
func (d *Deps) Locale() domain.Locale {
	if l := domain.Locale(d.Config.Locale); l.Valid() {
		return l
	}
	return domain.LocaleENUS
}

// BaseCurrency parses the configured base currency, used by report and KPI commands to convert
// multi-currency amounts.
func (d *Deps) BaseCurrency() (domain.Currency, error) {
	return domain.ParseCurrency(d.Config.Currency)
}

// requireImporting returns d.Importing, or a data error when this build has none configured.
func requireImporting(d *Deps) (*importing.Service, error) {
	if d.Importing == nil {
		return nil, fmt.Errorf("fui: no connector registry configured")
	}
	return d.Importing, nil
}

// requireBudgeting returns d.Budgeting, or a data error when this build has none configured.
func requireBudgeting(d *Deps) (*budgeting.Service, error) {
	if d.Budgeting == nil {
		return nil, fmt.Errorf("fui: no budgeting service configured")
	}
	return d.Budgeting, nil
}
