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
	// KeepSettings keeps the categories (and any other settings that exist); everything else is deleted.
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
	Transfers() TransferRepo
	FXRates() FXRateRepo
	Imports() ImportRepo
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
	// Usage counts the transactions that reference the category.
	Usage(ctx context.Context, id domain.CategoryID) (int, error)
	// Reassign moves every transaction of category from to category to and returns how many moved.
	Reassign(ctx context.Context, from, to domain.CategoryID) (int, error)
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
