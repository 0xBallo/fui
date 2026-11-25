package importing

import (
	"fmt"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
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
	// Hint is the source's category hint: only the rules' hint conditions see it, it never sets
	// the category by itself.
	Hint string
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

// RowWarning is a non-fatal warning tied to one source line.
type RowWarning struct {
	Line    int
	Message string
}

// SecurityOpPlanRow is one security operation to apply.
type SecurityOpPlanRow struct {
	Line       int
	AccountRef string
	Security   domain.Security
	Op         domain.SecurityOp
	CashTx     string // stream transaction id in SecurityOpRecord.cash_tx
	Include    bool
	// Module is set when the record belongs to a module that is not enabled: it is imported
	// only if ApplyOptions.EnableModules names it, and skipped with a warning otherwise.
	Module module.ID
}

// ModuleRecords lists the records of one module that is not enabled, so that the user can
// enable the module or skip them. Schedules and installments are only detected, never imported.
type ModuleRecords struct {
	Module module.ID
	Count  int
	Kinds  []string // record types, in stream order of first appearance
	Lines  []int
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

// BalanceMismatch is a statement balance record of an existing account that differs from the
// balance computed at that date (archive plus the rows the import adds).
type BalanceMismatch struct {
	AccountRef string
	AccountID  domain.AccountID
	Date       domain.Date
	Expected   int64 // balance stated by the stream
	Computed   int64
	Suggest    string // the reconcile command that records the stated balance
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
	NewSecurities   []domain.Security
	Rows            []Row
	SecurityOps     []SecurityOpPlanRow
	Errors          []RowError
	Warnings        []RowWarning
	Transfers       []TransferProposal
	ModuleRecords   []ModuleRecords
	Discrepancies   []BalanceDiscrepancy
	// BalanceMismatches lists balance records of existing accounts the computed balance contradicts.
	BalanceMismatches []BalanceMismatch
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

// SetAcceptTransfer accepts or rejects the transfer proposal i; an accepted one is linked on apply.
func (p *ImportPlan) SetAcceptTransfer(i int, accept bool) error {
	if i < 0 || i >= len(p.Transfers) {
		return fmt.Errorf("transfer proposal %d: %w", i, domain.ErrNotFound)
	}
	p.Transfers[i].Accept = accept
	return nil
}

// refresh recomputes Changes from the other fields. Importing always asks for confirmation:
// Bulk is set, and ApplyImport takes the backup itself.
func (p *ImportPlan) refresh() {
	changes := make([]app.Change, 0, len(p.NewAccounts)+len(p.NewCategories)+len(p.NewPayees)+len(p.NewTags)+len(p.NewSecurities)+3)
	changes = p.appendEntityCreates(changes)
	secOps := p.includedSecurityOps()
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
	if len(p.SecurityOps) > 0 {
		changes = append(changes, app.Change{
			Op: app.OpCreate, Entity: "security_op",
			Fields: map[string]any{
				"count": secOps,
			},
			Summary: fmt.Sprintf("create %d security operations", secOps),
		})
	}
	p.Changes, p.Bulk = changes, true
}

func (p *ImportPlan) appendEntityCreates(changes []app.Change) []app.Change {
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
	for _, s := range p.NewSecurities {
		changes = append(changes, app.Change{
			Op: app.OpCreate, Entity: "security",
			Fields: map[string]any{
				"name": s.Name, "ticker": s.Ticker, "isin": s.ISIN, "kind": string(s.Kind),
				"currency": s.Currency.Code(), "quantity_scale": s.QuantityScale,
			},
			Summary: fmt.Sprintf("create security %q", s.Name),
		})
	}
	return changes
}

func (p *ImportPlan) includedSecurityOps() int {
	n := 0
	for _, row := range p.SecurityOps {
		if row.Include && row.Module == "" {
			n++
		}
	}
	return n
}

func moneyFields(fields map[string]any, prefix string, m domain.Money) {
	fields[prefix] = m.Minor()
	fields[prefix+"_currency"] = m.Currency().Code()
}
