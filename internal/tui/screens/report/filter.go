package report

import (
	"strings"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
)

// filterFields holds the Autocomplete fields of the filter / group-by form.
type filterFields struct {
	accounts, categories, tags, payees components.Autocomplete
	status, text, uncategorized, sign  components.Autocomplete
	minAmount, maxAmount               components.Autocomplete
	groupBy, granularity               components.Autocomplete
}

// joinList renders a filter list as the comma-separated text the form edits.
func joinList(items []string) string { return strings.Join(items, ", ") }

// splitList parses comma-separated form text into a trimmed list without empty entries.
func splitList(text string) []string {
	var out []string
	for _, part := range strings.Split(text, ",") {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// openFilterForm opens the transaction filter fields plus the group-by and granularity the
// active report honours; a report that honours none reports so instead.
func (m *Model) openFilterForm() {
	k := predefinedKinds()[m.kindIdx]
	var fields []*components.Autocomplete
	f := &m.ff
	if k.Filter {
		fields = append(fields, m.filterFieldSet()...)
	}
	if k.GroupBy {
		f.groupBy = components.NewAutocomplete("Group by:", "default", m.th)
		f.groupBy.SetSuggestions([]string{"category", "tag", "payee", "account", "week", "month", "year"})
		f.groupBy.SetValue(m.groupByText)
		fields = append(fields, &f.groupBy)
	}
	if k.Granularity {
		f.granularity = components.NewAutocomplete("Granularity:", "default", m.th)
		f.granularity.SetSuggestions(granularities(k))
		f.granularity.SetValue(m.granText)
		fields = append(fields, &f.granularity)
	}
	if len(fields) == 0 {
		return
	}
	m.form = components.NewForm(fields...)
	m.mode = modeFilterForm
	m.form.Focus()
}

// filterFieldSet builds the transaction filter fields, prefilled from the active filter.
func (m *Model) filterFieldSet() []*components.Autocomplete {
	f := &m.ff
	f.accounts = components.NewAutocomplete("Accounts:", "any (comma-separated)", m.th)
	f.accounts.SetValue(joinList(m.filter.Accounts))
	f.accounts.SetSuggestions(m.accountNames())
	f.categories = components.NewAutocomplete("Categories:", "any (comma-separated)", m.th)
	f.categories.SetValue(joinList(m.filter.Categories))
	f.tags = components.NewAutocomplete("Tags:", "any (comma-separated)", m.th)
	f.tags.SetValue(joinList(m.filter.Tags))
	f.payees = components.NewAutocomplete("Payees:", "any (comma-separated)", m.th)
	f.payees.SetValue(joinList(m.filter.Payees))
	f.minAmount = components.NewAutocomplete("Min amount:", "any", m.th)
	f.minAmount.SetValue(m.filter.MinAmount)
	f.maxAmount = components.NewAutocomplete("Max amount:", "any", m.th)
	f.maxAmount.SetValue(m.filter.MaxAmount)
	f.status = components.NewAutocomplete("Status:", "any", m.th)
	f.status.SetSuggestions([]string{string(domain.StatusPending), string(domain.StatusBooked), string(domain.StatusReconciled)})
	f.status.SetValue(joinList(m.filter.Statuses))
	f.text = components.NewAutocomplete("Text:", "any", m.th)
	f.text.SetValue(m.filter.Text)
	f.uncategorized = components.NewAutocomplete("Uncategorized:", "no", m.th)
	f.uncategorized.SetSuggestions([]string{"yes", "no"})
	if m.filter.Uncategorized {
		f.uncategorized.SetValue("yes")
	}
	f.sign = components.NewAutocomplete("Income/Expense:", "any", m.th)
	f.sign.SetSuggestions([]string{string(domain.TxSignIncome), string(domain.TxSignExpense)})
	if m.filter.Sign != "" && m.filter.Sign != domain.TxSignAny {
		f.sign.SetValue(string(m.filter.Sign))
	}
	return []*components.Autocomplete{
		&f.accounts, &f.categories, &f.tags, &f.payees, &f.minAmount, &f.maxAmount,
		&f.status, &f.text, &f.uncategorized, &f.sign,
	}
}

// granularities are the buckets a kind accepts: the forecast projects by day or week only.
func granularities(k kindInfo) []string {
	if k.IsForecast {
		return []string{"day", "week"}
	}
	return []string{"day", "week", "month", "year"}
}

// submitFilter validates the form, stores the filter, group-by and granularity on the screen
// and re-runs the active report with them; the period keeps its own field.
func (m *Model) submitFilter() error {
	k := predefinedKinds()[m.kindIdx]
	f := &m.ff
	if k.Filter {
		m.filter = app.ListTxRequest{
			Accounts: splitList(f.accounts.Value()), Categories: splitList(f.categories.Value()),
			Tags: splitList(f.tags.Value()), Payees: splitList(f.payees.Value()),
			Statuses:  splitList(f.status.Value()),
			MinAmount: strings.TrimSpace(f.minAmount.Value()), MaxAmount: strings.TrimSpace(f.maxAmount.Value()),
			Text: strings.TrimSpace(f.text.Value()), Uncategorized: f.uncategorized.Value() == "yes",
		}
		switch f.sign.Value() {
		case string(domain.TxSignIncome):
			m.filter.Sign = domain.TxSignIncome
		case string(domain.TxSignExpense):
			m.filter.Sign = domain.TxSignExpense
		}
	}
	if k.GroupBy {
		v := strings.TrimSpace(f.groupBy.Value())
		if _, err := parseGroupBy(v); err != nil {
			return err
		}
		m.groupByText = v
	}
	if k.Granularity {
		v := strings.TrimSpace(f.granularity.Value())
		if _, err := parseGranularity(v, k); err != nil {
			return err
		}
		m.granText = v
	}
	return nil
}

// parseGroupBy parses the group-by text; empty keeps the report's default dimension.
func parseGroupBy(text string) (domain.ReportGroup, error) {
	if text == "" {
		return "", nil
	}
	return domain.ParseReportGroup(text)
}

// parseGranularity parses the granularity text; empty keeps the report's default bucket.
func parseGranularity(text string, k kindInfo) (domain.Granularity, error) {
	if text == "" {
		return "", nil
	}
	g, err := domain.ParseGranularity(text)
	if err != nil {
		return "", err
	}
	if k.IsForecast && g != domain.GranDay && g != domain.GranWeek {
		return "", &domain.Error{
			Code: domain.CodeInvalidValue, Message: "forecast granularity must be day or week",
			Details: map[string]any{"field": "granularity", "value": text},
		}
	}
	return g, nil
}

// filterActive reports whether f narrows the transactions in any way.
func filterActive(f app.ListTxRequest) bool {
	return len(f.Accounts) > 0 || len(f.Categories) > 0 || len(f.Tags) > 0 || len(f.Payees) > 0 ||
		len(f.Statuses) > 0 || len(f.Kinds) > 0 || f.Uncategorized || f.Text != "" ||
		f.MinAmount != "" || f.MaxAmount != "" || (f.Sign != "" && f.Sign != domain.TxSignAny)
}

// mergeFilter narrows base with the criteria of row: a criterion the row sets replaces the one
// of base, one it leaves empty is inherited, and the row's dates always win.
func mergeFilter(base, row app.ListTxRequest) app.ListTxRequest {
	out := base
	out.From, out.To, out.Month, out.Year = row.From, row.To, row.Month, row.Year
	if len(row.Accounts) > 0 {
		out.Accounts = row.Accounts
	}
	if len(row.Categories) > 0 {
		out.Categories, out.Uncategorized = row.Categories, false
	}
	if row.Uncategorized {
		out.Categories, out.Uncategorized = nil, true
	}
	if len(row.Payees) > 0 {
		out.Payees = row.Payees
	}
	if len(row.Tags) > 0 {
		out.Tags = row.Tags
	}
	if row.Sign != "" && row.Sign != domain.TxSignAny {
		out.Sign = row.Sign
	}
	return out
}

// filterSummary renders the active filter, group-by and granularity of the displayed report for
// the subtitle line; empty when none is set.
func filterSummary(f app.ListTxRequest, groupBy, granularity string) string {
	var parts []string
	add := func(label string, values ...string) {
		if len(values) > 0 && values[0] != "" {
			parts = append(parts, label+"="+strings.Join(values, "|"))
		}
	}
	add("account", f.Accounts...)
	add("category", f.Categories...)
	add("tag", f.Tags...)
	add("payee", f.Payees...)
	add("status", f.Statuses...)
	add("min", f.MinAmount)
	add("max", f.MaxAmount)
	add("text", f.Text)
	if f.Uncategorized {
		parts = append(parts, "uncategorized")
	}
	if f.Sign != "" && f.Sign != domain.TxSignAny {
		parts = append(parts, string(f.Sign))
	}
	// Group-by and granularity come first so truncation on a narrow screen cuts the filter list.
	var out []string
	if groupBy != "" {
		out = append(out, "by "+groupBy)
	}
	if granularity != "" {
		out = append(out, "per "+granularity)
	}
	if len(parts) > 0 {
		out = append(out, "Filter: "+strings.Join(parts, ", "))
	}
	return strings.Join(out, " · ")
}
