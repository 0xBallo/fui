package render

import (
	"fmt"
	"io"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// RecurrenceLookup resolves ids to names while rendering recurrences.
type RecurrenceLookup struct {
	accounts   map[domain.AccountID]string
	categories map[domain.CategoryID]string
}

// NewRecurrenceLookup builds RecurrenceLookup from app/domain rows.
func NewRecurrenceLookup(accts []app.AccountBalance, cats []domain.Category) RecurrenceLookup {
	out := RecurrenceLookup{
		accounts:   make(map[domain.AccountID]string, len(accts)),
		categories: make(map[domain.CategoryID]string, len(cats)),
	}
	for _, a := range accts {
		out.accounts[a.Account.ID] = a.Account.Name
	}
	for _, c := range cats {
		out.categories[c.ID] = c.Name
	}
	return out
}

// RecurrenceDTO is the --json shape of one recurrence.
type RecurrenceDTO struct {
	ID          int64  `json:"id"`
	Account     string `json:"account"`
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	Every       int    `json:"every"`
	Unit        string `json:"unit"`
	DayOfMonth  int    `json:"day_of_month,omitempty"`
	Start       string `json:"start"`
	End         string `json:"end,omitempty"`
	NextDate    string `json:"next_date,omitempty"`
}

func recurrenceDTO(r app.Recurrence, lookup RecurrenceLookup) RecurrenceDTO {
	out := RecurrenceDTO{
		ID: int64(r.ID), Account: lookup.accounts[r.AccountID], Description: r.Description,
		Amount: r.Amount.Minor(), Currency: r.Amount.Currency().Code(),
		Every: r.Rule.Every, Unit: string(r.Rule.Unit), DayOfMonth: r.Rule.DayOfMonth,
		Start: r.Start.String(),
	}
	if r.CategoryID != 0 {
		out.Category = lookup.categories[r.CategoryID]
	}
	if !r.End.IsZero() {
		out.End = r.End.String()
	}
	if !r.NextDate.IsZero() {
		out.NextDate = r.NextDate.String()
	}
	return out
}

func recurrenceRows(items []app.Recurrence, lookup RecurrenceLookup, loc domain.Locale) Rows {
	t := Rows{Header: []string{"ID", "ACCOUNT", "CATEGORY", "DESCRIPTION", "AMOUNT", "EVERY", "NEXT_DATE"}, MoneyCols: []int{4}}
	for _, r := range items {
		every := fmt.Sprintf("%d %s", r.Rule.Every, r.Rule.Unit)
		t.Body = append(t.Body, []string{
			fmt.Sprint(r.ID), lookup.accounts[r.AccountID], lookup.categories[r.CategoryID], r.Description,
			FormatMoney(r.Amount, loc), every, FormatDate(r.NextDate, loc),
		})
	}
	return t
}

// WriteRecurrenceList renders a list of recurrences.
func WriteRecurrenceList(w io.Writer, f Format, loc domain.Locale, items []app.Recurrence, lookup RecurrenceLookup) error {
	if f == JSON {
		dtos := make([]RecurrenceDTO, len(items))
		for i, r := range items {
			dtos[i] = recurrenceDTO(r, lookup)
		}
		return WriteJSON(w, dtos)
	}
	return WriteTable(w, recurrenceRows(items, lookup, loc), false)
}

// WriteRecurrence renders one recurrence.
func WriteRecurrence(w io.Writer, f Format, loc domain.Locale, r app.Recurrence, lookup RecurrenceLookup) error {
	if f == JSON {
		return WriteJSON(w, recurrenceDTO(r, lookup))
	}
	return WriteRecurrenceList(w, f, loc, []app.Recurrence{r}, lookup)
}

// RunResultDTO is the --json shape of the count `recurring run` or `loan run` created.
type RunResultDTO struct {
	Created int `json:"created"`
}

// WriteRecurringRunResult renders how many pending transactions `recurring run` created.
func WriteRecurringRunResult(w io.Writer, f Format, created int) error {
	if f == JSON {
		return WriteJSON(w, RunResultDTO{Created: created})
	}
	_, err := fmt.Fprintf(w, "Created %d pending transaction(s).\n", created)
	if err != nil {
		return fmt.Errorf("write recurring run result: %w", err)
	}
	return nil
}
