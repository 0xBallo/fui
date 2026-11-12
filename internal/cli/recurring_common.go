package cli

import (
	"context"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
)

// loadRecurrenceLookup resolves the account and category names a recurrence rendering needs.
func loadRecurrenceLookup(ctx context.Context, deps *Deps) (render.RecurrenceLookup, error) {
	accts, err := deps.Service.ListAccounts(ctx, app.ListAccountsRequest{IncludeArchived: true})
	if err != nil {
		return render.RecurrenceLookup{}, err
	}
	cats, err := deps.Service.ListCategories(ctx)
	if err != nil {
		return render.RecurrenceLookup{}, err
	}
	return render.NewRecurrenceLookup(accts, cats), nil
}
