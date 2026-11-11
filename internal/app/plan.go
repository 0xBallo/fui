package app

import "github.com/0xballo/fui/internal/domain"

// Op is the kind of a planned change.
type Op string

// Change operations.
const (
	OpCreate Op = "create"
	OpUpdate Op = "update"
	OpDelete Op = "delete"
)

// Entity names used in Change.Entity.
const (
	EntityAccount     = "account"
	EntityTransaction = "transaction"
	EntityCategory    = "category"
	EntityTransfer    = "transfer"
	EntityFXRate      = "fx_rate"
)

// Change is one planned modification, ready to render for --dry-run.
// Fields are the structured values (snake_case keys; money as minor units + currency).
type Change struct {
	Op      Op
	Entity  string
	Summary string
	Fields  map[string]any
}

// Plan is the plain-data result of a Plan* use case: what Apply* will do. Bulk marks mass
// changes: the UI asks for confirmation (or --yes) before applying, and Apply takes a backup first.
type Plan struct {
	Changes []Change
	Bulk    bool
}

// moneyFields flattens m into the keys prefix and prefix_currency of fields.
func moneyFields(fields map[string]any, prefix string, m domain.Money) {
	fields[prefix] = m.Minor()
	fields[prefix+"_currency"] = m.Currency().Code()
}
