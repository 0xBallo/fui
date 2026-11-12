package main

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/recurring"
	"github.com/0xballo/fui/internal/app/rules"
	"github.com/0xballo/fui/internal/cli"
	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens/settings"
	"github.com/0xballo/fui/internal/tui/screens/settings/lists"
)

// lookups adapts app.Service's account/category/payee/tag listing to the plain slices the rules and
// recurrences sub-views use for suggestions and name resolution.
type lookups struct{ svc *app.Service }

func (l lookups) ListAccounts(ctx context.Context) ([]domain.Account, error) {
	balances, err := l.svc.ListAccounts(ctx, app.ListAccountsRequest{IncludeArchived: true})
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	out := make([]domain.Account, len(balances))
	for i, b := range balances {
		out[i] = b.Account
	}
	return out, nil
}

func (l lookups) ListCategories(ctx context.Context) ([]domain.Category, error) {
	return l.svc.ListCategories(ctx)
}

func (l lookups) ListPayees(ctx context.Context) ([]domain.Payee, error) {
	return l.svc.ListPayees(ctx)
}

func (l lookups) ListTags(ctx context.Context) ([]domain.Tag, error) { return l.svc.ListTags(ctx) }

// rulesUseCases satisfies lists.RulesUseCases.
type rulesUseCases struct {
	*rules.Service
	lookups
}

// recurrenceUseCases satisfies lists.RecurrencesUseCases.
type recurrenceUseCases struct {
	*recurring.Service
	lookups
}

// profilesConfig satisfies lists.ProfilesConfig over the in-memory configuration, persisting each
// change to the config file.
type profilesConfig struct{ cfg *config.Config }

func (p profilesConfig) ListProfiles() ([]lists.Profile, error) {
	out := make([]lists.Profile, 0, len(p.cfg.Import.Profiles))
	for name, c := range p.cfg.Import.Profiles {
		skip := ""
		if c.SkipRows != 0 {
			skip = strconv.Itoa(c.SkipRows)
		}
		out = append(out, lists.Profile{
			Name: name, DateColumn: c.DateColumn, AmountColumn: c.AmountColumn, DebitColumn: c.DebitColumn,
			CreditColumn: c.CreditColumn, DescriptionColumn: c.DescriptionColumn,
			ExternalIDColumn: c.ExternalIDColumn, Delimiter: c.Delimiter, Encoding: c.Encoding,
			DateFormat: c.DateFormat, DecimalSeparator: c.DecimalSeparator, SkipRows: skip,
			TargetAccount: c.TargetAccount,
		})
	}
	slices.SortFunc(out, func(a, b lists.Profile) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

func (p profilesConfig) SaveProfile(pr lists.Profile) error {
	skip := 0
	if s := strings.TrimSpace(pr.SkipRows); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			return fmt.Errorf("skip rows %q: want a non-negative number", pr.SkipRows)
		}
		skip = n
	}
	if p.cfg.Import.Profiles == nil {
		p.cfg.Import.Profiles = map[string]config.CSVProfile{}
	}
	prev, had := p.cfg.Import.Profiles[pr.Name]
	p.cfg.Import.Profiles[pr.Name] = config.CSVProfile{
		DateColumn: pr.DateColumn, AmountColumn: pr.AmountColumn, DebitColumn: pr.DebitColumn,
		CreditColumn: pr.CreditColumn, DescriptionColumn: pr.DescriptionColumn,
		ExternalIDColumn: pr.ExternalIDColumn, Delimiter: pr.Delimiter, Encoding: pr.Encoding,
		DateFormat: pr.DateFormat, DecimalSeparator: pr.DecimalSeparator, SkipRows: skip,
		TargetAccount: pr.TargetAccount,
	}
	if err := p.cfg.Save(); err != nil {
		if had {
			p.cfg.Import.Profiles[pr.Name] = prev
		} else {
			delete(p.cfg.Import.Profiles, pr.Name)
		}
		return fmt.Errorf("save profile %q: %w", pr.Name, err)
	}
	return nil
}

func (p profilesConfig) DeleteProfile(name string) error {
	prev, ok := p.cfg.Import.Profiles[name]
	if !ok {
		return fmt.Errorf("profile %q not found", name)
	}
	delete(p.cfg.Import.Profiles, name)
	if err := p.cfg.Save(); err != nil {
		p.cfg.Import.Profiles[name] = prev
		return fmt.Errorf("delete profile %q: %w", name, err)
	}
	return nil
}

// settingsLists wires the five settings CRUD sub-views over deps; a service that is not wired
// leaves its section out.
func settingsLists(cfg *config.Config, deps *cli.Deps) settings.Lists {
	l := settings.Lists{Locale: domain.Locale(cfg.Locale), Profiles: profilesConfig{cfg: cfg}}
	if deps.Service == nil {
		return l
	}
	lk := lookups{svc: deps.Service}
	l.Categories, l.Tags = deps.Service, deps.Service
	if deps.Rules != nil {
		l.Rules = rulesUseCases{Service: deps.Rules, lookups: lk}
	}
	if deps.Recurring != nil {
		l.Recurrences = recurrenceUseCases{Service: deps.Recurring, lookups: lk}
	}
	return l
}
