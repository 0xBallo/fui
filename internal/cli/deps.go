// Package cli builds the fui command tree: one cobra command per verb, each a thin adapter that
// parses flags, calls a use case on app.Service, and renders the result. Commands carry no
// business logic; internal/cli/render turns results into table, --json and --csv output.
package cli

import (
	"context"
	"io"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/domain"
)

// Deps are the dependencies every command needs. The composition root (cmd/fui) builds one Deps
// from the resolved configuration and adapters; tests build their own on a memstore-backed
// Service.
type Deps struct {
	// Service runs the account, transaction, transfer, category, fx and backup use cases.
	Service *app.Service
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

// Locale returns the configured number/date locale, defaulting to en-US.
func (d *Deps) Locale() domain.Locale {
	if l := domain.Locale(d.Config.Locale); l.Valid() {
		return l
	}
	return domain.LocaleENUS
}
