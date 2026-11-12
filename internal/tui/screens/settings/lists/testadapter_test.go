package lists_test

import (
	"context"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/recurring"
	"github.com/0xballo/fui/internal/app/rules"
	"github.com/0xballo/fui/internal/domain"
)

// lookups adapts app.Service's account/category/payee/tag listing to the plain slices the rules
// and recurrences sub-views use for suggestions and name resolution.
type lookups struct{ svc *app.Service }

func (l lookups) ListAccounts(ctx context.Context) ([]domain.Account, error) {
	balances, err := l.svc.ListAccounts(ctx, app.ListAccountsRequest{IncludeArchived: true})
	if err != nil {
		return nil, err
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

// rulesAdapter satisfies lists.RulesUseCases by combining rules.Service with the shared lookups.
type rulesAdapter struct {
	*rules.Service
	lookups
}

// recurringAdapter satisfies lists.RecurrencesUseCases by combining recurring.Service with the
// shared lookups.
type recurringAdapter struct {
	*recurring.Service
	lookups
}

// stubBackup is a no-op app.Backuper for tests.
type stubBackup struct{}

func (stubBackup) Backup(context.Context, string) (string, error) { return "", nil }

func (stubBackup) Restore(context.Context, string) error { return nil }

// fixedClock is a fixed app.Clock for tests.
type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }
