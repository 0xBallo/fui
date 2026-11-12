// Package app holds the use cases of FUI and the ports they depend on. Every write use case
// has a PlanX that computes the changes without writing and an ApplyX that performs them in
// one transaction; UIs only render plans and call Apply.
package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/0xballo/fui/internal/domain"
)

// Config holds the user settings the use cases depend on.
type Config struct {
	Locale       domain.Locale  // number and date input format; default en-US
	Location     *time.Location // zone that defines "today"; default time.Local
	BaseCurrency domain.Currency
}

// Service runs the use cases over a Store.
type Service struct {
	store  Store
	clock  Clock
	backup Backuper
	cfg    Config
}

// New builds a Service, defaulting Locale and Location when unset.
func New(store Store, clock Clock, backup Backuper, cfg Config) *Service {
	if cfg.Locale == "" {
		cfg.Locale = domain.LocaleENUS
	}
	if cfg.Location == nil {
		cfg.Location = time.Local
	}
	return &Service{store: store, clock: clock, backup: backup, cfg: cfg}
}

// Today returns the current date in the configured zone.
func (s *Service) Today() (domain.Date, error) {
	d, err := domain.FromTime(s.clock.Now().In(s.cfg.Location))
	if err != nil {
		return domain.Date{}, fmt.Errorf("today: %w", err)
	}
	return d, nil
}

// date parses a user-typed date; an empty input means today.
func (s *Service) date(in string) (domain.Date, error) {
	today, err := s.Today()
	if err != nil || in == "" {
		return today, err
	}
	d, err := domain.ParseDateInput(in, s.cfg.Locale, today)
	if err != nil {
		return domain.Date{}, fmt.Errorf("date: %w", err)
	}
	return d, nil
}

// amount parses a user-typed amount in currency c.
func (s *Service) amount(in string, c domain.Currency) (domain.Money, error) {
	m, err := domain.ParseAmount(in, s.cfg.Locale, c)
	if err != nil {
		return domain.Money{}, fmt.Errorf("amount: %w", err)
	}
	return m, nil
}

// apply runs fn in one transaction, taking a backup first when the plan is bulk.
func (s *Service) apply(ctx context.Context, bulk bool, fn func(Repos) error) error {
	if bulk {
		if _, err := s.backup.Backup(ctx, BackupReasonBulk); err != nil {
			return fmt.Errorf("backup before bulk change: %w", err)
		}
	}
	if err := s.store.WithTx(ctx, fn); err != nil {
		return fmt.Errorf("apply: %w", err)
	}
	return nil
}

// pickAccount resolves a user-typed account reference among accts.
func pickAccount(accts []domain.Account, ref string) (domain.Account, error) {
	names := make([]string, len(accts))
	for i, a := range accts {
		names[i] = a.Name
	}
	i, err := domain.Resolve(ref, names)
	if err != nil {
		return domain.Account{}, fmt.Errorf("account: %w", err)
	}
	return accts[i], nil
}

// pickCategory resolves a user-typed category reference among cats.
func pickCategory(cats []domain.Category, ref string) (domain.Category, error) {
	names := make([]string, len(cats))
	for i, c := range cats {
		names[i] = c.Name
	}
	i, err := domain.Resolve(ref, names)
	if err != nil {
		return domain.Category{}, fmt.Errorf("category: %w", err)
	}
	return cats[i], nil
}

// pickTag resolves a user-typed tag reference among tags.
func pickTag(tags []domain.Tag, ref string) (domain.Tag, error) {
	names := make([]string, len(tags))
	for i, tg := range tags {
		names[i] = tg.Name
	}
	i, err := domain.Resolve(ref, names)
	if err != nil {
		return domain.Tag{}, fmt.Errorf("tag: %w", err)
	}
	return tags[i], nil
}

// pickPayee resolves a user-typed payee reference among payees.
func pickPayee(payees []domain.Payee, ref string) (domain.Payee, error) {
	names := make([]string, len(payees))
	for i, p := range payees {
		names[i] = p.Name
	}
	i, err := domain.Resolve(ref, names)
	if err != nil {
		return domain.Payee{}, fmt.Errorf("payee: %w", err)
	}
	return payees[i], nil
}

// activeAccounts returns the accounts that are not archived.
func activeAccounts(all []domain.Account) []domain.Account {
	out := make([]domain.Account, 0, len(all))
	for _, a := range all {
		if !a.Archived {
			out = append(out, a)
		}
	}
	return out
}

func equalNames(a, b string) bool { return strings.EqualFold(a, b) }

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
