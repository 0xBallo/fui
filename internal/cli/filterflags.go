package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// registerFilterFlags declares the transaction filter flags shared by tx list, tx search and
// export, defined once per the technical spec's "Ricerca e filtri". Paging
// (--after-date/--after-id) is tx list's own, since tx search and export do not page.
func registerFilterFlags(cmd *cobra.Command) {
	cmd.Flags().String("from", "", "earliest date, inclusive")
	cmd.Flags().String("to", "", "latest date, inclusive")
	cmd.Flags().String("month", "", "accounting month, e.g. 2026-10 (incompatible with --from/--to/--year)")
	cmd.Flags().Int("year", 0, "accounting year (incompatible with --from/--to/--month)")
	cmd.Flags().StringArrayP("account", "a", nil, "filter by account (repeatable)")
	cmd.Flags().StringArrayP("category", "c", nil, "filter by category (repeatable)")
	cmd.Flags().Bool("uncategorized", false, "only transactions without a category")
	cmd.Flags().StringArrayP("tag", "t", nil, "filter by tag (repeatable; OR among themselves)")
	cmd.Flags().StringArray("payee", nil, "filter by payee (repeatable; not supported yet, no payees archive)")
	cmd.Flags().String("min", "", "minimum amount, inclusive, on the absolute value")
	cmd.Flags().String("max", "", "maximum amount, inclusive, on the absolute value")
	cmd.Flags().Bool("income", false, "only inflows (incompatible with --expense)")
	cmd.Flags().Bool("expense", false, "only outflows (incompatible with --income)")
	cmd.Flags().StringArray("status", nil, "pending, booked or reconciled (repeatable)")
	cmd.Flags().StringArray("kind", nil, "payment, transfer, exchange, fee, interest, refund, investment, other (repeatable)")
	cmd.Flags().String("batch", "", "import batch id")
	cmd.Flags().String("sort", "", "date (default), amount or payee")
	cmd.Flags().Bool("desc", false, "sort descending (default: the date sort is newest first, any other sort is ascending)")
	cmd.Flags().Int("limit", 0, "page size (default: 100)")
}

// readFilterFlags reads the flags registerFilterFlags declares into an app.ListTxRequest.
// Incompatible --income/--expense fails with invalid_filter here; period (--from/--to vs
// --month vs --year) is left to domain.NewTxFilter, which rejects the same way.
func readFilterFlags(cmd *cobra.Command) (app.ListTxRequest, error) {
	var req app.ListTxRequest
	req.From, _ = cmd.Flags().GetString("from")
	req.To, _ = cmd.Flags().GetString("to")
	req.Month, _ = cmd.Flags().GetString("month")
	req.Year, _ = cmd.Flags().GetInt("year")
	req.Accounts, _ = cmd.Flags().GetStringArray("account")
	req.Categories, _ = cmd.Flags().GetStringArray("category")
	req.Uncategorized, _ = cmd.Flags().GetBool("uncategorized")
	req.Tags, _ = cmd.Flags().GetStringArray("tag")
	req.Payees, _ = cmd.Flags().GetStringArray("payee")
	req.MinAmount, _ = cmd.Flags().GetString("min")
	req.MaxAmount, _ = cmd.Flags().GetString("max")
	req.Statuses, _ = cmd.Flags().GetStringArray("status")
	req.Kinds, _ = cmd.Flags().GetStringArray("kind")
	sort, _ := cmd.Flags().GetString("sort")
	req.Sort = domain.TxSort(sort)
	req.Desc, _ = cmd.Flags().GetBool("desc")
	req.Limit, _ = cmd.Flags().GetInt("limit")

	income, _ := cmd.Flags().GetBool("income")
	expense, _ := cmd.Flags().GetBool("expense")
	if income && expense {
		return app.ListTxRequest{}, invalidFilterError("--income and --expense are incompatible")
	}
	switch {
	case income:
		req.Sign = domain.TxSignIncome
	case expense:
		req.Sign = domain.TxSignExpense
	}

	batch, _ := cmd.Flags().GetString("batch")
	if batch != "" {
		id, err := strconv.ParseInt(batch, 10, 64)
		if err != nil {
			return app.ListTxRequest{}, invalidFilterError("invalid batch %q", batch)
		}
		req.Batch = id
	}
	return req, nil
}

// invalidFilterError builds the same invalid_filter *domain.Error domain.NewTxFilter returns, so
// a CLI-side incompatibility (--income with --expense) looks identical to a domain-side one
// (--month with --from).
func invalidFilterError(format string, args ...any) error {
	return &domain.Error{Code: domain.ErrInvalidFilter.Code, Message: "invalid_filter: " + fmt.Sprintf(format, args...)}
}
