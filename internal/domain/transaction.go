package domain

import (
	"strings"
)

// TxStatus is the lifecycle state of a transaction.
type TxStatus string

// Transaction statuses.
const (
	StatusPending    TxStatus = "pending"
	StatusBooked     TxStatus = "booked"
	StatusReconciled TxStatus = "reconciled"
)

// Valid reports whether s is a known status.
func (s TxStatus) Valid() bool {
	switch s {
	case StatusPending, StatusBooked, StatusReconciled:
		return true
	}
	return false
}

// ParseTxStatus parses a transaction status (case-insensitive).
func ParseTxStatus(s string) (TxStatus, error) { return parseEnum("status", s, TxStatus.Valid) }

// TxKind is the nature of a transaction.
type TxKind string

// Transaction kinds.
const (
	KindPayment    TxKind = "payment"
	KindTransfer   TxKind = "transfer"
	KindExchange   TxKind = "exchange"
	KindFee        TxKind = "fee"
	KindInterest   TxKind = "interest"
	KindRefund     TxKind = "refund"
	KindInvestment TxKind = "investment"
	KindOther      TxKind = "other"
)

// Valid reports whether k is a known transaction kind.
func (k TxKind) Valid() bool {
	switch k {
	case KindPayment, KindTransfer, KindExchange, KindFee, KindInterest, KindRefund, KindInvestment, KindOther:
		return true
	}
	return false
}

// ParseTxKind parses a transaction kind (case-insensitive).
func ParseTxKind(s string) (TxKind, error) { return parseEnum("kind", s, TxKind.Valid) }

// Transaction is a single movement on an account. Amount is signed (negative
// = outflow). Optional references (PayeeID, CategoryID, ImportBatchID,
// TransferGroupID) and optional dates (OperationDate, ValueDate) are zero
// when absent.
type Transaction struct {
	ID              TransactionID
	AccountID       AccountID
	Date            Date // accounting date: the only date used in calculations
	OperationDate   Date
	ValueDate       Date
	Amount          Money
	PayeeID         PayeeID
	Description     string
	CategoryID      CategoryID
	Notes           string
	Status          TxStatus
	Kind            TxKind
	ExternalID      string // identifier assigned by the source bank/connector
	Fingerprint     string // dedup hash, see the domain services
	ImportBatchID   ImportBatchID
	TransferGroupID TransferGroupID
}

// NewTransaction validates t and returns it with text fields trimmed.
func NewTransaction(t Transaction) (Transaction, error) {
	if t.AccountID == 0 {
		return Transaction{}, missingField("account_id")
	}
	if t.Date.IsZero() {
		return Transaction{}, missingField("date")
	}
	if t.Amount.Currency().IsZero() {
		return Transaction{}, missingField("amount")
	}
	if !t.Status.Valid() {
		return Transaction{}, enumError("status", string(t.Status))
	}
	if !t.Kind.Valid() {
		return Transaction{}, enumError("kind", string(t.Kind))
	}
	t.Description = strings.TrimSpace(t.Description)
	t.ExternalID = strings.TrimSpace(t.ExternalID)
	return t, nil
}

// TransferLeg is one side of a transfer.
type TransferLeg struct {
	TransactionID TransactionID // zero before the leg is persisted
	AccountID     AccountID
	Amount        Money
}

// TransferGroup links the two legs of a transfer between the user's accounts
// and the optional fee charged on it.
type TransferGroup struct {
	ID  TransferGroupID
	Out TransferLeg // negative: money leaving the source account
	In  TransferLeg // positive: money arriving on the destination account
	Fee Money       // charged in the source currency; the zero Money means no fee
}

// NewTransferGroup validates a transfer. The legs must be on different
// accounts with Out negative and In positive. Between accounts of the same
// currency the legs must sum to minus the fee; across currencies there is no
// such check (the exchange rate is implicit in the two amounts) and no fee is
// allowed on the group.
func NewTransferGroup(g TransferGroup) (TransferGroup, error) {
	if g.Out.AccountID == 0 || g.In.AccountID == 0 {
		return TransferGroup{}, missingField("account_id")
	}
	if g.Out.AccountID == g.In.AccountID {
		return TransferGroup{}, newError(ErrInvalidValue, map[string]any{"field": "account_id"},
			"transfer legs must be on different accounts")
	}
	if g.Out.Amount.Sign() >= 0 || g.In.Amount.Sign() <= 0 {
		return TransferGroup{}, newError(ErrInvalidValue, map[string]any{"field": "amount"},
			"transfer must have a negative out leg and a positive in leg")
	}
	if err := g.checkFee(); err != nil {
		return TransferGroup{}, err
	}
	return g, nil
}

func (g TransferGroup) checkFee() error {
	c := g.Out.Amount.Currency()
	if c != g.In.Amount.Currency() {
		if !g.Fee.Currency().IsZero() {
			return newError(ErrInvalidValue, map[string]any{"field": "fee"}, "a cross-currency transfer cannot carry a fee")
		}
		return nil
	}
	fee := g.Fee
	if fee.Currency().IsZero() {
		fee = Money{cur: c} // no fee
	}
	if fee.Sign() < 0 {
		return newError(ErrInvalidValue, map[string]any{"field": "fee"}, "fee must not be negative")
	}
	sum, err := g.Out.Amount.Add(g.In.Amount)
	if err != nil {
		return err
	}
	negFee, err := fee.Neg()
	if err != nil {
		return err
	}
	// Cmp also rejects a fee in another currency.
	if cmp, err := sum.Cmp(negFee); err != nil || cmp != 0 {
		if err != nil {
			return err
		}
		return newError(ErrInvalidValue, map[string]any{"field": "amount", "sum": sum.Minor(), "fee": fee.Minor()},
			"transfer legs sum to %s, expected minus the fee (%s)", sum, negFee)
	}
	return nil
}
