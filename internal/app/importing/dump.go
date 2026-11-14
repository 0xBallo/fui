package importing

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strconv"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// DumpFormat is the format of a full dump.
type DumpFormat string

// Dump formats.
const (
	DumpJSON DumpFormat = "json"
	DumpCSV  DumpFormat = "csv"
)

// DumpFile creates the output file of one entity, named like "accounts.json".
type DumpFile func(name string) (io.WriteCloser, error)

type accountDump struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	Kind           string `json:"kind"`
	Currency       string `json:"currency"`
	OpeningBalance int64  `json:"opening_balance"`
	OpenedOn       string `json:"opened_on"`
	Archived       bool   `json:"archived"`
}

type categoryDump struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type transactionDump struct {
	ID              int64  `json:"id"`
	AccountID       int64  `json:"account_id"`
	Date            string `json:"date"`
	OperationDate   string `json:"operation_date,omitempty"`
	ValueDate       string `json:"value_date,omitempty"`
	Amount          int64  `json:"amount"`
	Currency        string `json:"currency"`
	Description     string `json:"description"`
	CategoryID      int64  `json:"category_id,omitempty"`
	Notes           string `json:"notes,omitempty"`
	Status          string `json:"status"`
	Kind            string `json:"kind"`
	ExternalID      string `json:"external_id,omitempty"`
	Fingerprint     string `json:"fingerprint,omitempty"`
	ImportBatchID   int64  `json:"import_batch_id,omitempty"`
	TransferGroupID int64  `json:"transfer_group_id,omitempty"`
}

type transferDump struct {
	ID          int64  `json:"id"`
	Fee         int64  `json:"fee"`
	FeeCurrency string `json:"fee_currency,omitempty"`
}

type fxRateDump struct {
	Currency string `json:"currency"`
	Date     string `json:"date"`
	Rate     int64  `json:"rate"` // scaled by domain.RateScale
}

type batchDump struct {
	ID           int64  `json:"id"`
	Connector    string `json:"connector"`
	Source       string `json:"source"`
	SourceSHA256 string `json:"source_sha256,omitempty"`
	ImportedAt   string `json:"imported_at"`
}

type savedSimulationDump struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	ParamsJSON string `json:"params"`
}

type dismissedInsightDump struct {
	Key         string `json:"key"`
	Kind        string `json:"kind"`
	DismissedAt string `json:"dismissed_at"`
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// Dump writes every entity of the archive, one file per entity, in the given format: accounts,
// categories, transactions, transfers, fx_rates, import_batches, saved_simulations and
// dismissed_insights. Amounts are minor units.
func (s *Service) Dump(ctx context.Context, format DumpFormat, create DumpFile) error {
	if format != DumpJSON && format != DumpCSV {
		return newError(domain.ErrInvalidValue, map[string]any{"field": "format"}, "unknown dump format %q", format)
	}
	return s.store.Read(ctx, func(r app.Repos) error {
		steps := []func() error{
			func() error { return dumpAccounts(ctx, r, format, create) },
			func() error { return dumpCategories(ctx, r, format, create) },
			func() error { return dumpTransactions(ctx, r, format, create) },
			func() error { return dumpFX(ctx, r, format, create) },
			func() error { return dumpBatches(ctx, r, format, create) },
			func() error { return dumpSavedSimulations(ctx, r, format, create) },
			func() error { return dumpDismissedInsights(ctx, r, format, create) },
		}
		for _, step := range steps {
			if err := step(); err != nil {
				return err
			}
		}
		return nil
	})
}

// writeEntity writes items as a JSON array or as CSV rows under header.
func writeEntity[T any](create DumpFile, format DumpFormat, name string, header []string, items []T, row func(T) []string) (err error) {
	f, err := create(name + "." + string(format))
	if err != nil {
		return fmt.Errorf("create %s: %w", name, err)
	}
	defer func() {
		if cerr := f.Close(); err == nil && cerr != nil {
			err = fmt.Errorf("close %s: %w", name, cerr)
		}
	}()
	if format == DumpJSON {
		if items == nil {
			items = []T{}
		}
		return encodeJSON(f, items)
	}
	rows := make([][]string, 0, len(items)+1)
	rows = append(rows, header)
	for _, it := range items {
		rows = append(rows, row(it))
	}
	return writeCSVRows(f, rows)
}

func dumpAccounts(ctx context.Context, r app.Repos, format DumpFormat, create DumpFile) error {
	all, err := r.Accounts().List(ctx)
	if err != nil {
		return fmt.Errorf("list accounts: %w", err)
	}
	items := make([]accountDump, len(all))
	for i, a := range all {
		items[i] = accountDump{
			ID: int64(a.ID), Name: a.Name, Description: a.Description, Kind: string(a.Kind), Currency: a.Currency.Code(),
			OpeningBalance: a.OpeningBalance.Minor(), OpenedOn: a.OpenedOn.String(), Archived: a.Archived,
		}
	}
	return writeEntity(create, format, "accounts",
		[]string{"id", "name", "description", "kind", "currency", "opening_balance", "opened_on", "archived"}, items,
		func(a accountDump) []string {
			return []string{itoa(a.ID), a.Name, a.Description, a.Kind, a.Currency, itoa(a.OpeningBalance), a.OpenedOn, strconv.FormatBool(a.Archived)}
		})
}

func dumpCategories(ctx context.Context, r app.Repos, format DumpFormat, create DumpFile) error {
	all, err := r.Categories().List(ctx)
	if err != nil {
		return fmt.Errorf("list categories: %w", err)
	}
	items := make([]categoryDump, len(all))
	for i, c := range all {
		items[i] = categoryDump{ID: int64(c.ID), Name: c.Name, Kind: string(c.Kind)}
	}
	return writeEntity(create, format, "categories", []string{"id", "name", "kind"}, items,
		func(c categoryDump) []string { return []string{itoa(c.ID), c.Name, c.Kind} })
}

// dumpTransactions writes transactions and, from their group IDs, the transfers.
func dumpTransactions(ctx context.Context, r app.Repos, format DumpFormat, create DumpFile) error {
	all, err := r.Transactions().List(ctx, domain.TxFilter{}, nil, 0) // zero value: everything, oldest first
	if err != nil {
		return fmt.Errorf("list transactions: %w", err)
	}
	items := make([]transactionDump, len(all))
	var groups []domain.TransferGroupID
	for i, t := range all {
		items[i] = transactionDump{
			ID: int64(t.ID), AccountID: int64(t.AccountID), Date: t.Date.String(), Amount: t.Amount.Minor(),
			Currency: t.Amount.Currency().Code(), Description: t.Description, CategoryID: int64(t.CategoryID), Notes: t.Notes,
			Status: string(t.Status), Kind: string(t.Kind), ExternalID: t.ExternalID, Fingerprint: t.Fingerprint,
			ImportBatchID: int64(t.ImportBatchID), TransferGroupID: int64(t.TransferGroupID),
		}
		if !t.OperationDate.IsZero() {
			items[i].OperationDate = t.OperationDate.String()
		}
		if !t.ValueDate.IsZero() {
			items[i].ValueDate = t.ValueDate.String()
		}
		if t.TransferGroupID != 0 && !slices.Contains(groups, t.TransferGroupID) {
			groups = append(groups, t.TransferGroupID)
		}
	}
	err = writeEntity(create, format, "transactions",
		[]string{
			"id", "account_id", "date", "operation_date", "value_date", "amount", "currency", "description", "category_id",
			"notes", "status", "kind", "external_id", "fingerprint", "import_batch_id", "transfer_group_id",
		}, items,
		func(t transactionDump) []string {
			return []string{
				itoa(t.ID), itoa(t.AccountID), t.Date, t.OperationDate, t.ValueDate, itoa(t.Amount), t.Currency, t.Description,
				itoa(t.CategoryID), t.Notes, t.Status, t.Kind, t.ExternalID, t.Fingerprint, itoa(t.ImportBatchID), itoa(t.TransferGroupID),
			}
		})
	if err != nil {
		return err
	}
	return dumpTransfers(ctx, r, format, create, groups)
}

func dumpTransfers(ctx context.Context, r app.Repos, format DumpFormat, create DumpFile, ids []domain.TransferGroupID) error {
	items := make([]transferDump, len(ids))
	for i, id := range ids {
		g, err := r.Transfers().Get(ctx, id)
		if err != nil {
			return fmt.Errorf("get transfer %d: %w", id, err)
		}
		items[i] = transferDump{ID: int64(id), Fee: g.Fee.Minor(), FeeCurrency: g.Fee.Currency().Code()}
	}
	return writeEntity(create, format, "transfers", []string{"id", "fee", "fee_currency"}, items,
		func(t transferDump) []string { return []string{itoa(t.ID), itoa(t.Fee), t.FeeCurrency} })
}

func dumpFX(ctx context.Context, r app.Repos, format DumpFormat, create DumpFile) error {
	all, err := r.FXRates().List(ctx, domain.Currency{})
	if err != nil {
		return fmt.Errorf("list fx rates: %w", err)
	}
	items := make([]fxRateDump, len(all))
	for i, x := range all {
		items[i] = fxRateDump{Currency: x.Currency.Code(), Date: x.Date.String(), Rate: x.Rate.Scaled()}
	}
	return writeEntity(create, format, "fx_rates", []string{"currency", "date", "rate"}, items,
		func(x fxRateDump) []string { return []string{x.Currency, x.Date, itoa(x.Rate)} })
}

func dumpBatches(ctx context.Context, r app.Repos, format DumpFormat, create DumpFile) error {
	all, err := r.Imports().Batches(ctx)
	if err != nil {
		return fmt.Errorf("list import batches: %w", err)
	}
	slices.Reverse(all)
	items := make([]batchDump, len(all))
	for i, b := range all {
		items[i] = batchDump{
			ID: int64(b.ID), Connector: b.Connector, Source: b.Source, SourceSHA256: b.SourceSHA256,
			ImportedAt: b.ImportedAt.UTC().Format("2006-01-02T15:04:05Z"),
		}
	}
	return writeEntity(create, format, "import_batches", []string{"id", "connector", "source", "source_sha256", "imported_at"}, items,
		func(b batchDump) []string {
			return []string{itoa(b.ID), b.Connector, b.Source, b.SourceSHA256, b.ImportedAt}
		})
}

func dumpSavedSimulations(ctx context.Context, r app.Repos, format DumpFormat, create DumpFile) error {
	all, err := r.SavedSimulations().List(ctx)
	if err != nil {
		return fmt.Errorf("list saved simulations: %w", err)
	}
	items := make([]savedSimulationDump, len(all))
	for i, sim := range all {
		items[i] = savedSimulationDump{
			ID:         int64(sim.ID),
			Name:       sim.Name,
			Kind:       sim.Kind,
			ParamsJSON: sim.ParamsJSON,
		}
	}
	return writeEntity(create, format, "saved_simulations", []string{"id", "name", "kind", "params"}, items,
		func(sim savedSimulationDump) []string {
			return []string{itoa(sim.ID), sim.Name, sim.Kind, sim.ParamsJSON}
		})
}

func dumpDismissedInsights(ctx context.Context, r app.Repos, format DumpFormat, create DumpFile) error {
	all, err := r.DismissedInsights().List(ctx)
	if err != nil {
		return fmt.Errorf("list dismissed insights: %w", err)
	}
	items := make([]dismissedInsightDump, len(all))
	for i, insight := range all {
		items[i] = dismissedInsightDump{
			Key:         insight.Key,
			Kind:        string(insight.Kind),
			DismissedAt: insight.DismissedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		}
	}
	return writeEntity(create, format, "dismissed_insights", []string{"key", "kind", "dismissed_at"}, items,
		func(insight dismissedInsightDump) []string {
			return []string{insight.Key, insight.Kind, insight.DismissedAt}
		})
}
