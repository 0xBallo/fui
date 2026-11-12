package importing

import (
	"fmt"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// EntityImportBatch is the Change.Entity of the batch an import creates.
const EntityImportBatch = "import_batch"

// NewAccount is an account the import creates from an account record.
type NewAccount struct {
	Ref     string
	Account domain.Account // ID is zero until applied
}

// UnmappedRef is a connector account ref bound to no account: the user must map it to an
// existing account (PlanRequest.Mappings) or let the import create it.
type UnmappedRef struct {
	Ref        string
	Name       string
	Currency   string
	Kind       string
	Suggestion string // an existing account with the same name, when creating would clash with it
}

// Binding stores the association of a connector ref to an account, saved on apply.
type Binding struct {
	Ref       string
	AccountID domain.AccountID
}

// Row is one transaction of the stream with its dedup outcome. The UI may flip Include before
// applying; Tx.AccountID is zero while the account is still to be created.
type Row struct {
	Line       int // line of the record in the stream
	Class      domain.DedupClass
	Include    bool
	AccountRef string
	Tx         domain.Transaction
	Category   string // category name to apply, created when missing
	Splits     []SplitRow
	Payee      string   // payee name to apply, created when missing
	Tags       []string // tag names to apply, created when missing
	Transfer   string   // transfer label shared by the two legs, as written in the stream
	Matched    []domain.TransactionID
	Rules      RuleEffect // what the categorization rules set on this row
	// ConfirmsPending is set when the row matches a pending transaction a recurrence created
	// (recurring module only): applying it books that transaction instead of inserting a new one.
	ConfirmsPending domain.TransactionID
}

// SplitRow is one split row attached to an imported transaction row.
type SplitRow struct {
	Category string
	Amount   int64
	Note     string
}

// RowError is a row that could not be converted or that the source reported as invalid.
type RowError struct {
	Line    int
	Message string
}

// TransferProposal suggests linking an incoming leg to another movement as a transfer. It is
// applied only when Accept is set. The partner is another row (Other >= 0) or a transaction
// already in the archive (Other == -1).
type TransferProposal struct {
	Row      int
	Other    int
	Existing domain.TransactionID
	Reason   domain.TransferMatchReason
	Accept   bool
}

// BalanceDiscrepancy is a movement whose balance_after does not follow from the previous one.
type BalanceDiscrepancy struct {
	AccountRef string
	Line       int
	Date       domain.Date
	Expected   int64
	Actual     int64
}

// ImportPlan is the plain-data result of PlanImport: what ApplyImport will do.
type ImportPlan struct {
	app.Plan
	Connector    string
	Source       string
	SourceSHA256 string
	// AlreadyImported is set when a batch with the same source digest exists.
	AlreadyImported *app.ImportBatch
	NewAccounts     []NewAccount
	Bindings        []Binding
	Unmapped        []UnmappedRef
	NewCategories   []domain.Category
	NewPayees       []domain.Payee
	NewTags         []domain.Tag
	Rows            []Row
	Errors          []RowError
	Transfers       []TransferProposal
	Discrepancies   []BalanceDiscrepancy
	// Stderr is what the connector wrote on stderr, for --verbose.
	Stderr string
}

// Counts summarises the rows of a plan.
type Counts struct {
	New, Duplicate, Doubtful, Errors, Included int
}

// Counts tallies the rows by class and the included ones.
func (p ImportPlan) Counts() Counts {
	c := Counts{Errors: len(p.Errors)}
	for _, r := range p.Rows {
		switch r.Class {
		case domain.DedupNew:
			c.New++
		case domain.DedupDuplicate:
			c.Duplicate++
		case domain.DedupDoubtful:
			c.Doubtful++
		}
		if r.Include {
			c.Included++
		}
	}
	return c
}

// SetInclude includes or excludes row i and refreshes the changes shown to the user.
func (p *ImportPlan) SetInclude(i int, include bool) error {
	if i < 0 || i >= len(p.Rows) {
		return fmt.Errorf("row %d: %w", i, domain.ErrNotFound)
	}
	p.Rows[i].Include = include
	p.refresh()
	return nil
}

// refresh recomputes Changes from the other fields. Importing always asks for confirmation:
// Bulk is set, and ApplyImport takes the backup itself.
func (p *ImportPlan) refresh() {
	changes := make([]app.Change, 0, len(p.NewAccounts)+len(p.NewCategories)+len(p.NewPayees)+len(p.NewTags)+2)
	for _, a := range p.NewAccounts {
		f := map[string]any{"name": a.Account.Name, "kind": string(a.Account.Kind)}
		moneyFields(f, "opening_balance", a.Account.OpeningBalance)
		changes = append(changes, app.Change{
			Op: app.OpCreate, Entity: app.EntityAccount, Fields: f,
			Summary: fmt.Sprintf("create account %q (%s)", a.Account.Name, a.Account.Currency),
		})
	}
	for _, c := range p.NewCategories {
		changes = append(changes, app.Change{
			Op: app.OpCreate, Entity: app.EntityCategory, Fields: map[string]any{"name": c.Name, "kind": string(c.Kind)},
			Summary: fmt.Sprintf("create %s category %q", c.Kind, c.Name),
		})
	}
	for _, pe := range p.NewPayees {
		changes = append(changes, app.Change{
			Op: app.OpCreate, Entity: app.EntityPayee, Fields: map[string]any{"name": pe.Name},
			Summary: fmt.Sprintf("create payee %q", pe.Name),
		})
	}
	for _, t := range p.NewTags {
		changes = append(changes, app.Change{
			Op: app.OpCreate, Entity: app.EntityTag, Fields: map[string]any{"name": t.Name},
			Summary: fmt.Sprintf("create tag %q", t.Name),
		})
	}
	n := p.Counts()
	changes = append(changes,
		app.Change{
			Op: app.OpCreate, Entity: EntityImportBatch,
			Fields:  map[string]any{"connector": p.Connector, "source": p.Source, "source_sha256": p.SourceSHA256},
			Summary: fmt.Sprintf("create import batch from %q", p.Source),
		},
		app.Change{
			Op: app.OpCreate, Entity: app.EntityTransaction,
			Fields: map[string]any{
				"count": n.Included, "new": n.New, "duplicate": n.Duplicate, "doubtful": n.Doubtful, "errors": n.Errors,
			},
			Summary: fmt.Sprintf("create %d transactions (%d new, %d duplicate, %d doubtful, %d errors)",
				n.Included, n.New, n.Duplicate, n.Doubtful, n.Errors),
		})
	p.Changes, p.Bulk = changes, true
}

func moneyFields(fields map[string]any, prefix string, m domain.Money) {
	fields[prefix] = m.Minor()
	fields[prefix+"_currency"] = m.Currency().Code()
}
