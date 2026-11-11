package domain

// Distinct ID types make it impossible to pass, say, a CategoryID where an
// AccountID is expected. The zero value means "no ID" (not yet persisted, or
// an optional reference left empty).
type (
	// AccountID identifies an Account.
	AccountID int64
	// TransactionID identifies a Transaction.
	TransactionID int64
	// CategoryID identifies a Category.
	CategoryID int64
	// TagID identifies a tag.
	TagID int64
	// PayeeID identifies a payee.
	PayeeID int64
	// TransferGroupID identifies a TransferGroup.
	TransferGroupID int64
	// ImportBatchID identifies an import batch.
	ImportBatchID int64
)
