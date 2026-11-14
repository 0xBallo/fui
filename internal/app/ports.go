package app

import (
	"context"
	"time"

	"github.com/0xballo/fui/internal/domain"
)

// Clock is the source of the current time. Adapters use internal/clock; tests inject a fixed one.
type Clock interface {
	Now() time.Time
}

// Backuper creates and restores copies of the whole archive. Reasons are the BackupReason* values.
type Backuper interface {
	// Backup writes a consistent copy of the archive and returns its path.
	Backup(ctx context.Context, reason string) (string, error)
	// Restore replaces the archive with the backup at path.
	Restore(ctx context.Context, path string) error
}

// ResetOptions selects what a reset keeps.
type ResetOptions struct {
	// KeepSettings keeps the categories, tags and payees (and any other settings that exist); everything else is deleted.
	KeepSettings bool
}

// Resetter rebuilds the archive as a fresh one. The Backuper given to New must also implement it
// for the reset use case to work (the SQLite store does); it is a separate interface so the
// Backuper fakes of other packages keep compiling.
type Resetter interface {
	// Reset takes a backup with reason "reset" that rotation never deletes, then atomically replaces
	// the archive with a fresh one holding the default categories (or the kept settings).
	Reset(ctx context.Context, opts ResetOptions) error
}

// Backup reasons passed to Backuper.Backup.
const (
	BackupReasonManual  = "manual"
	BackupReasonImport  = "import"
	BackupReasonBulk    = "bulk"
	BackupReasonRestore = "restore"
)

// Store is the Unit of Work port. Repositories obtained from Repos never open transactions:
// the use case decides the boundaries.
type Store interface {
	// Read runs fn with read-only repositories; writes through them fail.
	Read(ctx context.Context, fn func(Repos) error) error
	// WithTx runs fn in a single transaction: if fn returns an error nothing is written.
	WithTx(ctx context.Context, fn func(Repos) error) error
}

// Repos gives access to the repositories bound to one Read or WithTx call.
//
// Contract shared by every repository: a missing row is reported as domain.ErrNotFound,
// a violated unique name or external id as ErrAlreadyExists, and listings return an empty
// slice (not an error) when nothing matches.
type Repos interface {
	Accounts() AccountRepo
	Transactions() TransactionRepo
	Categories() CategoryRepo
	Rules() RuleRepo
	Transfers() TransferRepo
	FXRates() FXRateRepo
	Imports() ImportRepo
	Tags() TagRepo
	Payees() PayeeRepo
	Splits() SplitRepo
	Loans() LoanRepo
	Budgets() BudgetRepo
	Recurrences() RecurrenceRepo
	SavedSimulations() SavedSimulationRepo
	DismissedInsights() DismissedInsightRepo
	SavedReports() SavedReportRepo
	Reconciliations() ReconciliationRepo
	Assets() AssetRepo
	Securities() SecurityRepo
}

// AccountRepo stores accounts. Names are unique case-insensitively.
type AccountRepo interface {
	// Insert stores a and returns it with its ID assigned.
	Insert(ctx context.Context, a domain.Account) (domain.Account, error)
	Update(ctx context.Context, a domain.Account) error
	Get(ctx context.Context, id domain.AccountID) (domain.Account, error)
	// List returns every account, archived ones included, ordered by ID.
	List(ctx context.Context) ([]domain.Account, error)
}

// TxCursor is the keyset position of the last row of a page.
type TxCursor struct {
	Date domain.Date
	ID   domain.TransactionID
}

// TransactionRepo stores transactions. (AccountID, ExternalID) is unique when ExternalID is not empty.
type TransactionRepo interface {
	// Insert stores t and returns it with its ID assigned.
	Insert(ctx context.Context, t domain.Transaction) (domain.Transaction, error)
	Update(ctx context.Context, t domain.Transaction) error
	Delete(ctx context.Context, id domain.TransactionID) error
	Get(ctx context.Context, id domain.TransactionID) (domain.Transaction, error)
	// List returns the rows matching f in the order of f.Less (sort key, then ID ascending).
	// limit <= 0 means no limit beyond f.Limit(); when both are set the smaller wins. The keyset
	// cursor works only for the date sort, where it returns the rows strictly after it in that
	// order; for the amount and payee sorts paging is limit-only and after is ignored.
	List(ctx context.Context, f domain.TxFilter, after *TxCursor, limit int) ([]domain.Transaction, error)
	// SumUpTo returns the sum, in minor units, of every movement of the account dated on or before date.
	SumUpTo(ctx context.Context, id domain.AccountID, date domain.Date) (int64, error)
	// ByExternalID, ByFingerprint, NearAmount and ByTransferGroup are the lookups of the import
	// dedup and of transfer handling. NearAmount matches the same account and exact signed amount
	// dated within [from, to].
	ByExternalID(ctx context.Context, id domain.AccountID, externalID string) ([]domain.Transaction, error)
	ByFingerprint(ctx context.Context, fingerprint string) ([]domain.Transaction, error)
	NearAmount(ctx context.Context, id domain.AccountID, minor int64, from, to domain.Date) ([]domain.Transaction, error)
	ByTransferGroup(ctx context.Context, id domain.TransferGroupID) ([]domain.Transaction, error)
}

// CategoryRepo stores categories. Names are unique case-insensitively.
type CategoryRepo interface {
	Insert(ctx context.Context, c domain.Category) (domain.Category, error)
	Update(ctx context.Context, c domain.Category) error
	// Delete fails with ErrInUse while transactions reference the category.
	Delete(ctx context.Context, id domain.CategoryID) error
	Get(ctx context.Context, id domain.CategoryID) (domain.Category, error)
	// List returns every category ordered by ID.
	List(ctx context.Context) ([]domain.Category, error)
	// Usage counts the transactions and split rows that reference the category.
	Usage(ctx context.Context, id domain.CategoryID) (int, error)
	// Reassign moves every transaction and split row of category from to category to and returns how many moved.
	Reassign(ctx context.Context, from, to domain.CategoryID) (int, error)
}

// RuleRepo stores categorization rules. Names are unique case-insensitively.
type RuleRepo interface {
	Insert(ctx context.Context, rule domain.Rule) (domain.Rule, error)
	Update(ctx context.Context, rule domain.Rule) error
	Delete(ctx context.Context, name string) error
	Get(ctx context.Context, name string) (domain.Rule, error)
	// List returns every rule ordered by name (case-insensitive).
	List(ctx context.Context) ([]domain.Rule, error)
	// ListEnabledByPriority returns enabled rules ordered by priority ascending, then name.
	ListEnabledByPriority(ctx context.Context) ([]domain.Rule, error)
}

// TransferRepo stores the group that links the two legs of a transfer. The legs themselves are
// transactions carrying the group ID.
type TransferRepo interface {
	// Insert stores the group (its Fee) and returns it with ID assigned; leg fields are ignored.
	Insert(ctx context.Context, g domain.TransferGroup) (domain.TransferGroup, error)
	// Get returns the group with Out and In filled from the transactions that reference it.
	Get(ctx context.Context, id domain.TransferGroupID) (domain.TransferGroup, error)
	Delete(ctx context.Context, id domain.TransferGroupID) error
}

// FXRateRepo stores manual exchange rates, unique per (currency, date).
type FXRateRepo interface {
	// Upsert stores r, replacing the rate of the same currency and date.
	Upsert(ctx context.Context, r domain.FXRate) error
	// List returns the rates of currency (every currency when zero) ordered by currency, then date.
	List(ctx context.Context, currency domain.Currency) ([]domain.FXRate, error)
	// OnOrBefore returns the latest rate of currency dated on or before date, or ErrNotFound.
	OnOrBefore(ctx context.Context, currency domain.Currency, date domain.Date) (domain.FXRate, error)
}

// ImportBatch is one undoable import.
type ImportBatch struct {
	ID           domain.ImportBatchID
	Connector    string
	Source       string // file name or description shown to the user
	SourceSHA256 string // hex digest of the source, to recognise a file already imported
	ImportedAt   time.Time
}

// ImportRepo stores import batches and the association of connector account refs to accounts.
type ImportRepo interface {
	InsertBatch(ctx context.Context, b ImportBatch) (ImportBatch, error)
	Batch(ctx context.Context, id domain.ImportBatchID) (ImportBatch, error)
	// BatchBySHA256 returns the most recent batch imported from a source with that digest, or ErrNotFound.
	BatchBySHA256(ctx context.Context, sha string) (ImportBatch, error)
	// Batches returns every batch, most recent first.
	Batches(ctx context.Context) ([]ImportBatch, error)
	DeleteBatch(ctx context.Context, id domain.ImportBatchID) error
	// AccountRef returns the account bound to (connector, ref), or ErrNotFound on first use.
	AccountRef(ctx context.Context, connector, ref string) (domain.AccountID, error)
	SetAccountRef(ctx context.Context, connector, ref string, id domain.AccountID) error
}

// TagRepo stores tags, unique case-insensitively, and their association to transactions.
type TagRepo interface {
	Insert(ctx context.Context, t domain.Tag) (domain.Tag, error)
	Update(ctx context.Context, t domain.Tag) error
	// Delete removes the tag and detaches it from every transaction.
	Delete(ctx context.Context, id domain.TagID) error
	Get(ctx context.Context, id domain.TagID) (domain.Tag, error)
	// List returns every tag ordered by ID.
	List(ctx context.Context) ([]domain.Tag, error)
	// TagsOf returns the tags of a transaction ordered by ID.
	TagsOf(ctx context.Context, tx domain.TransactionID) ([]domain.Tag, error)
	// SetTags replaces the tags of a transaction; duplicates in ids are ignored.
	SetTags(ctx context.Context, tx domain.TransactionID, ids []domain.TagID) error
}

// PayeeRepo stores payees with their aliases. Names and aliases are unique case-insensitively
// across all payees; transactions refer to a payee through Transaction.PayeeID, and that payee's
// name is part of the transaction search text.
type PayeeRepo interface {
	Insert(ctx context.Context, p domain.Payee) (domain.Payee, error)
	// Update replaces the name and the aliases and refreshes the search text of the payee's transactions.
	Update(ctx context.Context, p domain.Payee) error
	// Delete fails with ErrInUse while transactions reference the payee.
	Delete(ctx context.Context, id domain.PayeeID) error
	Get(ctx context.Context, id domain.PayeeID) (domain.Payee, error)
	// List returns every payee ordered by ID.
	List(ctx context.Context) ([]domain.Payee, error)
}

// SplitRepo stores the category rows a transaction is split into.
type SplitRepo interface {
	// SplitsOf returns the rows in insertion order; none for a transaction that is not split.
	SplitsOf(ctx context.Context, tx domain.TransactionID) ([]domain.Split, error)
	// ReplaceSplits stores rows after checking them with domain.ValidateSplits against the
	// transaction amount; no rows removes the split. Rows refer to existing categories.
	ReplaceSplits(ctx context.Context, tx domain.TransactionID, rows []domain.Split) error
}

// InstallmentRef identifies one installment: the schedule version it belongs to and its due date.
type InstallmentRef struct {
	LoanID       domain.LoanID
	ScheduleFrom domain.Date // ValidFrom of the schedule version
	Due          domain.Date
}

// LoanRepo stores financing contracts and their schedule versions.
type LoanRepo interface {
	Insert(ctx context.Context, loan domain.Loan) (domain.Loan, error)
	Update(ctx context.Context, loan domain.Loan) error
	Delete(ctx context.Context, id domain.LoanID) error
	Get(ctx context.Context, id domain.LoanID) (domain.Loan, error)
	// List returns every loan ordered by ID.
	List(ctx context.Context) ([]domain.Loan, error)
	// AddScheduleVersion stores one schedule version and all its installments atomically.
	AddScheduleVersion(ctx context.Context, schedule domain.Schedule) error
	// ScheduleAt returns the newest schedule version whose ValidFrom is on or before at.
	ScheduleAt(ctx context.Context, loanID domain.LoanID, at domain.Date) (domain.Schedule, error)
	InsertEvent(ctx context.Context, event domain.LoanEvent) (domain.LoanEvent, error)
	// ListEvents returns the loan events ordered by date, then by insertion order.
	ListEvents(ctx context.Context, loanID domain.LoanID) ([]domain.LoanEvent, error)
	LinkInstallment(ctx context.Context, loanID domain.LoanID, scheduleFrom, due domain.Date, txID domain.TransactionID) error
	UnlinkInstallment(ctx context.Context, loanID domain.LoanID, scheduleFrom, due domain.Date) error
	// TransactionOfInstallment returns the transaction linked to the installment; ErrNotFound when
	// the installment does not exist or has no transaction.
	TransactionOfInstallment(ctx context.Context, loanID domain.LoanID, scheduleFrom, due domain.Date) (domain.TransactionID, error)
	// InstallmentOfTransaction returns the installment the transaction is linked to; ErrNotFound
	// when it pays none.
	InstallmentOfTransaction(ctx context.Context, txID domain.TransactionID) (InstallmentRef, error)
}

// BudgetRepo stores budgets (caps and envelopes), their monthly amount overrides, and the
// envelope moves ledger. Callers validate a budget with domain.NewBudget and the repository
// set with domain.ValidateBudgets before calling Insert or Update; a budget's targets refer to
// existing categories or tags, and a category can be the target of only one budget.
type BudgetRepo interface {
	// Insert stores b and returns it with its ID assigned.
	Insert(ctx context.Context, b domain.Budget) (domain.Budget, error)
	// Update replaces the name, mode, amount, targets, overrides, carry flag and thresholds of b.
	Update(ctx context.Context, b domain.Budget) error
	// Delete removes the budget, its targets, overrides and the envelope moves that touch it.
	Delete(ctx context.Context, id domain.BudgetID) error
	Get(ctx context.Context, id domain.BudgetID) (domain.Budget, error)
	// List returns every budget ordered by ID, with every override they have.
	List(ctx context.Context) ([]domain.Budget, error)
	// ForMonth returns every budget ordered by ID, with Overrides holding at most the single
	// entry for k (a budget has no start or end date, so every budget is in effect every month).
	ForMonth(ctx context.Context, k domain.MonthKey) ([]domain.Budget, error)
	// Moves returns the envelope moves dated within [from, to], inclusive, ordered by date then
	// insertion order.
	Moves(ctx context.Context, from, to domain.Date) ([]domain.BudgetMove, error)
	// AddMove appends an envelope move to the ledger; a non-zero From or To must be an existing budget.
	AddMove(ctx context.Context, m domain.BudgetMove) error
}

// RecurrenceID identifies a recurrence row.
type RecurrenceID int64

// Recurrence is one recurrence row persisted by RecurrenceRepo.
type Recurrence struct {
	ID          RecurrenceID
	AccountID   domain.AccountID
	CategoryID  domain.CategoryID
	Description string
	Amount      domain.Money
	Rule        domain.RecurrenceRule
	Start       domain.Date
	End         domain.Date // zero means no end
	NextDate    domain.Date
}

// RecurrenceRepo stores recurrence templates and lists pending transactions created from them.
type RecurrenceRepo interface {
	Insert(ctx context.Context, r Recurrence) (Recurrence, error)
	Update(ctx context.Context, r Recurrence) error
	Delete(ctx context.Context, id RecurrenceID) error
	Get(ctx context.Context, id RecurrenceID) (Recurrence, error)
	// List returns every recurrence ordered by ID.
	List(ctx context.Context) ([]Recurrence, error)
	// Due returns recurrences whose next date is on or before until, ordered by next date then ID.
	Due(ctx context.Context, until domain.Date) ([]Recurrence, error)
	// PendingOf returns pending transactions linked to recurrence id ordered by due_date/date then ID.
	PendingOf(ctx context.Context, id RecurrenceID) ([]domain.Transaction, error)
}

// SavedSimulationID identifies a saved simulation row.
type SavedSimulationID int64

// SavedSimulation is one saved simulation definition.
type SavedSimulation struct {
	ID         SavedSimulationID
	Name       string
	Kind       string
	ParamsJSON string
}

// SavedSimulationRepo stores saved simulation definitions. Names are unique case-insensitively.
type SavedSimulationRepo interface {
	Insert(ctx context.Context, s SavedSimulation) (SavedSimulation, error)
	Delete(ctx context.Context, id SavedSimulationID) error
	Get(ctx context.Context, id SavedSimulationID) (SavedSimulation, error)
	GetByName(ctx context.Context, name string) (SavedSimulation, error)
	// List returns every saved simulation ordered by name (case-insensitive), then ID.
	List(ctx context.Context) ([]SavedSimulation, error)
}

// DismissedInsight is one dismissed automatic insight.
type DismissedInsight struct {
	Key         string
	Kind        domain.InsightKind
	DismissedAt time.Time
}

// DismissedInsightRepo stores dismissed insight keys.
type DismissedInsightRepo interface {
	// Dismiss inserts or updates one dismissed key.
	Dismiss(ctx context.Context, in DismissedInsight) error
	// List returns every dismissed insight ordered by key.
	List(ctx context.Context) ([]DismissedInsight, error)
}

// SavedReportID identifies a saved report row.
type SavedReportID int64

// SavedReport is one saved report definition.
type SavedReport struct {
	ID         SavedReportID
	Name       string
	Kind       string
	ParamsJSON string
}

// SavedReportRepo stores saved report definitions. Names are unique case-insensitively.
type SavedReportRepo interface {
	Insert(ctx context.Context, r SavedReport) (SavedReport, error)
	Update(ctx context.Context, r SavedReport) error
	Delete(ctx context.Context, id SavedReportID) error
	Get(ctx context.Context, id SavedReportID) (SavedReport, error)
	GetByName(ctx context.Context, name string) (SavedReport, error)
	// List returns every saved report ordered by name (case-insensitive), then ID.
	List(ctx context.Context) ([]SavedReport, error)
}

// ReconciliationID identifies a reconciliation row.
type ReconciliationID int64

// Reconciliation is one reconciliation checkpoint for an account and statement date.
type Reconciliation struct {
	ID               ReconciliationID
	AccountID        domain.AccountID
	Date             domain.Date
	StatementBalance domain.Money
	ClosedAt         time.Time // zero means open
}

// ReconciliationRepo stores reconciliation checkpoints.
type ReconciliationRepo interface {
	Open(ctx context.Context, r Reconciliation) (Reconciliation, error)
	Close(ctx context.Context, accountID domain.AccountID, date domain.Date, closedAt time.Time) error
	Get(ctx context.Context, accountID domain.AccountID, date domain.Date) (Reconciliation, error)
	// LastClosed returns the most recent closed reconciliation of account.
	LastClosed(ctx context.Context, accountID domain.AccountID) (Reconciliation, error)
}

// SecurityRepo stores securities, their manual prices, and operations.
type SecurityRepo interface {
	Insert(ctx context.Context, s domain.Security) (domain.Security, error)
	Update(ctx context.Context, s domain.Security) error
	Delete(ctx context.Context, id domain.SecurityID) error
	Get(ctx context.Context, id domain.SecurityID) (domain.Security, error)
	// List returns every security ordered by ID.
	List(ctx context.Context) ([]domain.Security, error)
	// UpsertPrice stores p, replacing the price of the same security and date.
	UpsertPrice(ctx context.Context, p domain.SecurityPrice) error
	DeletePrice(ctx context.Context, securityID domain.SecurityID, date domain.Date) error
	// ListPrices returns the prices of security ordered by date.
	ListPrices(ctx context.Context, securityID domain.SecurityID) ([]domain.SecurityPrice, error)
	// PriceOnOrBefore returns the latest price of security dated on or before date, or ErrNotFound.
	PriceOnOrBefore(ctx context.Context, securityID domain.SecurityID, date domain.Date) (domain.SecurityPrice, error)
	InsertOp(ctx context.Context, op domain.SecurityOp) (domain.SecurityOp, error)
	UpdateOp(ctx context.Context, op domain.SecurityOp) error
	DeleteOp(ctx context.Context, id domain.SecurityOpID) error
	GetOp(ctx context.Context, id domain.SecurityOpID) (domain.SecurityOp, error)
	// ListOpsBySecurity returns operations of security in [from,to], ordered by date then ID.
	// A zero from or to means unbounded on that side.
	ListOpsBySecurity(ctx context.Context, securityID domain.SecurityID, from, to domain.Date) ([]domain.SecurityOp, error)
}

// AssetRepo stores manually valued assets and their valuation history. Asset names are unique
// case-insensitively. A linked loan, when set, must exist; deleting the loan unlinks the asset.
// Callers validate with domain.NewAsset and domain.NewValuation.
type AssetRepo interface {
	// Insert stores a and returns it with its ID assigned.
	Insert(ctx context.Context, a domain.Asset) (domain.Asset, error)
	Update(ctx context.Context, a domain.Asset) error
	// Delete removes the asset and its valuations.
	Delete(ctx context.Context, id domain.AssetID) error
	Get(ctx context.Context, id domain.AssetID) (domain.Asset, error)
	// List returns every asset ordered by ID.
	List(ctx context.Context) ([]domain.Asset, error)
	// SetValuation stores v, replacing any valuation of the same asset on the same date.
	SetValuation(ctx context.Context, v domain.Valuation) error
	// ListValuations returns the asset valuations ordered by date.
	ListValuations(ctx context.Context, assetID domain.AssetID) ([]domain.Valuation, error)
	// DeleteValuation removes the valuation on date; ErrNotFound when there is none.
	DeleteValuation(ctx context.Context, assetID domain.AssetID, date domain.Date) error
}
