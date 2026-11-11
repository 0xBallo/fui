package domain

// AccountKind classifies an account.
type AccountKind string

// Account kinds.
const (
	AccountCash       AccountKind = "cash"
	AccountChecking   AccountKind = "checking"
	AccountCreditCard AccountKind = "credit_card"
	AccountSavings    AccountKind = "savings"
	AccountInvestment AccountKind = "investment"
	AccountLoan       AccountKind = "loan"
)

// Valid reports whether k is a known account kind.
func (k AccountKind) Valid() bool {
	switch k {
	case AccountCash, AccountChecking, AccountCreditCard, AccountSavings, AccountInvestment, AccountLoan:
		return true
	}
	return false
}

// ParseAccountKind parses an account kind (case-insensitive).
func ParseAccountKind(s string) (AccountKind, error) {
	return parseEnum("account kind", s, AccountKind.Valid)
}

// Account is a container of money with a balance. The balance itself is never
// stored: it is the opening balance plus the movements up to a date.
type Account struct {
	ID             AccountID
	Name           string
	Description    string
	Kind           AccountKind
	Currency       Currency
	OpeningBalance Money
	OpenedOn       Date
	Archived       bool
	IBAN           string // optional; normalized (ParseIBAN) when set
}

// NewAccount validates a and returns it with the name trimmed. The opening
// balance must be in the account currency. IBAN is optional but, when set,
// must be a syntactically valid IBAN (see ParseIBAN).
func NewAccount(a Account) (Account, error) {
	name, err := requireName("name", a.Name)
	if err != nil {
		return Account{}, err
	}
	a.Name = name
	if !a.Kind.Valid() {
		return Account{}, enumError("kind", string(a.Kind))
	}
	if a.Currency.IsZero() {
		return Account{}, missingField("currency")
	}
	if a.OpeningBalance.Currency() != a.Currency {
		return Account{}, newError(ErrCurrencyMismatch,
			map[string]any{"left": a.Currency.Code(), "right": a.OpeningBalance.Currency().Code()},
			"opening balance is in %s, account in %s", a.OpeningBalance.Currency(), a.Currency)
	}
	if a.OpenedOn.IsZero() {
		return Account{}, missingField("opened_on")
	}
	if a.IBAN != "" {
		iban, err := ParseIBAN(a.IBAN)
		if err != nil {
			return Account{}, err
		}
		a.IBAN = iban
	}
	return a, nil
}
