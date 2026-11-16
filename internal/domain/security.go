package domain

import (
	"strings"
)

type (
	// SecurityID identifies a Security.
	SecurityID int64
	// SecurityOpID identifies a SecurityOp.
	SecurityOpID int64
)

// SecurityKind classifies a security.
type SecurityKind string

// Security kinds.
const (
	SecurityStock  SecurityKind = "stock"
	SecurityETF    SecurityKind = "etf"
	SecurityBond   SecurityKind = "bond"
	SecurityFund   SecurityKind = "fund"
	SecurityCrypto SecurityKind = "crypto"
)

// Valid reports whether k is a known security kind.
func (k SecurityKind) Valid() bool {
	switch k {
	case SecurityStock, SecurityETF, SecurityBond, SecurityFund, SecurityCrypto:
		return true
	}
	return false
}

// ParseSecurityKind parses a security kind (case-insensitive).
func ParseSecurityKind(s string) (SecurityKind, error) {
	return parseEnum("security kind", s, SecurityKind.Valid)
}

// Security is a financial instrument held in an investment account. Its
// quantities carry QuantityScale decimals (0 for shares, up to 8 for crypto).
type Security struct {
	ID            SecurityID
	Name          string
	Ticker        string // optional if ISIN is set; upper-cased
	ISIN          string // optional if Ticker is set; upper-cased, 12 chars
	Kind          SecurityKind
	Currency      Currency
	QuantityScale int
}

// NewSecurity validates s and returns it normalized: name trimmed, ticker and
// ISIN upper-cased. At least one of ticker and ISIN is required.
func NewSecurity(s Security) (Security, error) {
	name, err := requireName("name", s.Name)
	if err != nil {
		return Security{}, err
	}
	s.Name = name
	s.Ticker = strings.ToUpper(strings.TrimSpace(s.Ticker))
	s.ISIN = strings.ToUpper(strings.TrimSpace(s.ISIN))
	if s.Ticker == "" && s.ISIN == "" {
		return Security{}, missingField("ticker")
	}
	if s.ISIN != "" && !validISIN(s.ISIN) {
		return Security{}, invalidValue("isin", s.ISIN)
	}
	if !s.Kind.Valid() {
		return Security{}, enumError("kind", string(s.Kind))
	}
	if s.Currency.IsZero() {
		return Security{}, missingField("currency")
	}
	if _, err := NewQuantity(0, s.QuantityScale); err != nil {
		return Security{}, err
	}
	return s, nil
}

// validISIN checks the ISO 6166 shape: 2 letters, 9 alphanumerics, 1 digit.
func validISIN(s string) bool {
	if len(s) != 12 {
		return false
	}
	for i, r := range s {
		letter := r >= 'A' && r <= 'Z'
		digit := r >= '0' && r <= '9'
		switch {
		case i < 2 && !letter, i >= 2 && i < 11 && !letter && !digit, i == 11 && !digit:
			return false
		}
	}
	return true
}

// SecurityOpKind is the type of a security operation.
type SecurityOpKind string

// Security operation kinds.
const (
	OpBuy      SecurityOpKind = "buy"
	OpSell     SecurityOpKind = "sell"
	OpDividend SecurityOpKind = "dividend"
	OpCoupon   SecurityOpKind = "coupon"
)

// Valid reports whether k is a known operation kind.
func (k SecurityOpKind) Valid() bool {
	switch k {
	case OpBuy, OpSell, OpDividend, OpCoupon:
		return true
	}
	return false
}

// ParseSecurityOpKind parses an operation kind (case-insensitive).
func ParseSecurityOpKind(s string) (SecurityOpKind, error) {
	return parseEnum("security operation kind", s, SecurityOpKind.Valid)
}

// SecurityOp is a buy, sell, dividend or coupon on a security. For income
// kinds, Quantity is the quantity held and UnitPrice the amount per unit.
type SecurityOp struct {
	ID        SecurityOpID
	Date      Date
	Account   AccountID
	Security  SecurityID
	Kind      SecurityOpKind
	Quantity  Quantity
	UnitPrice Money
	Fees      Money
	CashTx    TransactionID // optional linked liquidity movement
}

// NewSecurityOp validates op: known kind, set date, account and security,
// strictly positive quantity, non-negative price and fees in a single
// currency.
func NewSecurityOp(op SecurityOp) (SecurityOp, error) {
	if !op.Kind.Valid() {
		return SecurityOp{}, enumError("kind", string(op.Kind))
	}
	if op.Date.IsZero() {
		return SecurityOp{}, missingField("date")
	}
	if op.Account == 0 {
		return SecurityOp{}, missingField("account")
	}
	if op.Security == 0 {
		return SecurityOp{}, missingField("security")
	}
	if op.Quantity.Sign() <= 0 {
		return SecurityOp{}, newError(ErrInvalidValue, map[string]any{"field": "quantity"},
			"quantity must be positive, got %s", op.Quantity)
	}
	if op.UnitPrice.Currency().IsZero() {
		return SecurityOp{}, missingField("unit price")
	}
	if op.UnitPrice.Sign() < 0 {
		return SecurityOp{}, newError(ErrInvalidValue, map[string]any{"field": "unit price"}, "unit price must not be negative")
	}
	if op.Fees.Currency().IsZero() {
		fees, err := NewMoney(0, op.UnitPrice.Currency())
		if err != nil {
			return SecurityOp{}, err
		}
		op.Fees = fees
	}
	if op.Fees.Currency() != op.UnitPrice.Currency() {
		return SecurityOp{}, newError(ErrCurrencyMismatch,
			map[string]any{"left": op.UnitPrice.Currency().Code(), "right": op.Fees.Currency().Code()},
			"fees currency %s does not match price currency %s", op.Fees.Currency(), op.UnitPrice.Currency())
	}
	if op.Fees.Sign() < 0 {
		return SecurityOp{}, newError(ErrInvalidValue, map[string]any{"field": "fees"}, "fees must not be negative")
	}
	return op, nil
}

// SecurityPrice is a manual price of a security on a date.
type SecurityPrice struct {
	Security SecurityID
	Date     Date
	Price    Money
}
