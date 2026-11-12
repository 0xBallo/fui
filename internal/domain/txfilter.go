package domain

import (
	"slices"
	"strings"
)

// TxSign filters transactions by sign.
type TxSign string

// Sign filter values.
const (
	TxSignAny     TxSign = "any"
	TxSignIncome  TxSign = "income"
	TxSignExpense TxSign = "expense"
)

// TxSort is the transaction ordering key.
type TxSort string

// Transaction sort keys.
const (
	TxSortDate   TxSort = "date"
	TxSortAmount TxSort = "amount"
	TxSortPayee  TxSort = "payee"
)

// TxFilterSpec is the input for NewTxFilter.
//
// Period can be set either as From/To (inclusive), or as a convenience Month
// (any day in the wanted month) or Year. Month and Year are mutually
// exclusive, and both are exclusive with From/To.
type TxFilterSpec struct {
	From, To      Date
	Month         Date
	Year          int
	AccountIDs    []AccountID
	CategoryIDs   []CategoryID
	Uncategorized bool
	PayeeIDs      []PayeeID
	Statuses      []TxStatus
	Kinds         []TxKind
	ImportBatchID ImportBatchID
	MinAbs        *int64
	MaxAbs        *int64
	Sign          TxSign
	Text          string
	Sort          TxSort
	Desc          bool
	Limit         int
}

// TxFilter is an immutable transaction filter and order.
type TxFilter struct {
	from          Date
	to            Date
	accountIDs    []AccountID
	categoryIDs   []CategoryID
	uncategorized bool
	payeeIDs      []PayeeID
	statuses      []TxStatus
	kinds         []TxKind
	importBatchID ImportBatchID
	minAbs        *int64
	maxAbs        *int64
	sign          TxSign
	text          string
	sort          TxSort
	desc          bool
	limit         int
}

// NewTxFilter validates spec and builds an immutable filter.
func NewTxFilter(spec TxFilterSpec) (TxFilter, error) {
	var out TxFilter
	from, to, err := resolveFilterPeriod(spec)
	if err != nil {
		return TxFilter{}, err
	}
	out.from, out.to = from, to
	if err := fillFilterSets(&out, spec); err != nil {
		return TxFilter{}, err
	}
	if err := fillFilterScalars(&out, spec); err != nil {
		return TxFilter{}, err
	}
	return out, nil
}

func resolveFilterPeriod(spec TxFilterSpec) (Date, Date, error) {
	periodFromTo := !spec.From.IsZero() || !spec.To.IsZero()
	periodMonth := !spec.Month.IsZero()
	periodYear := spec.Year != 0
	if periodFromTo && (periodMonth || periodYear) || periodMonth && periodYear {
		return Date{}, Date{}, invalidFilter("period must be either from/to, month, or year")
	}
	switch {
	case periodMonth:
		return monthBounds(spec.Month)
	case periodYear:
		return yearBounds(spec.Year)
	default:
		if !spec.From.IsZero() && !spec.To.IsZero() && spec.To.Before(spec.From) {
			return Date{}, Date{}, invalidFilter("to %s is before from %s", spec.To, spec.From)
		}
		return spec.From, spec.To, nil
	}
}

func monthBounds(month Date) (Date, Date, error) {
	from, err := NewDate(month.Year(), month.Month(), 1)
	if err != nil {
		return Date{}, Date{}, invalidFilter("invalid month %q", month)
	}
	next, err := from.AddMonths(1)
	if err != nil {
		return Date{}, Date{}, invalidFilter("invalid month %q", month)
	}
	to, err := next.AddDays(-1)
	if err != nil {
		return Date{}, Date{}, invalidFilter("invalid month %q", month)
	}
	return from, to, nil
}

func yearBounds(year int) (Date, Date, error) {
	if year < 1 || year > 9999 {
		return Date{}, Date{}, invalidFilter("year must be in 1..9999, got %d", year)
	}
	from, err := NewDate(year, 1, 1)
	if err != nil {
		return Date{}, Date{}, invalidFilter("invalid year %d", year)
	}
	to, err := NewDate(year, 12, 31)
	if err != nil {
		return Date{}, Date{}, invalidFilter("invalid year %d", year)
	}
	return from, to, nil
}

func fillFilterSets(out *TxFilter, spec TxFilterSpec) error {
	var err error
	if out.accountIDs, err = uniqueValid(spec.AccountIDs, func(id AccountID) bool { return id > 0 }, "account id"); err != nil {
		return err
	}
	if out.categoryIDs, err = uniqueValid(spec.CategoryIDs, func(id CategoryID) bool { return id > 0 }, "category id"); err != nil {
		return err
	}
	if out.payeeIDs, err = uniqueValid(spec.PayeeIDs, func(id PayeeID) bool { return id > 0 }, "payee id"); err != nil {
		return err
	}
	if out.statuses, err = uniqueValid(spec.Statuses, TxStatus.Valid, "status"); err != nil {
		return err
	}
	if out.kinds, err = uniqueValid(spec.Kinds, TxKind.Valid, "kind"); err != nil {
		return err
	}
	return nil
}

func fillFilterScalars(out *TxFilter, spec TxFilterSpec) error {
	if spec.ImportBatchID < 0 {
		return invalidFilter("import batch id must be >= 0, got %d", spec.ImportBatchID)
	}
	out.importBatchID = spec.ImportBatchID
	var err error
	if out.minAbs, err = normalizeAbs(spec.MinAbs, "min_abs"); err != nil {
		return err
	}
	if out.maxAbs, err = normalizeAbs(spec.MaxAbs, "max_abs"); err != nil {
		return err
	}
	if out.minAbs != nil && out.maxAbs != nil && *out.maxAbs < *out.minAbs {
		return invalidFilter("max_abs %d is lower than min_abs %d", *out.maxAbs, *out.minAbs)
	}
	if out.sign, err = normalizeSign(spec.Sign); err != nil {
		return err
	}
	if out.sort, err = normalizeSort(spec.Sort); err != nil {
		return err
	}
	if spec.Limit < 0 {
		return invalidFilter("limit must be >= 0, got %d", spec.Limit)
	}
	out.uncategorized = spec.Uncategorized
	out.text = strings.TrimSpace(spec.Text)
	out.desc = spec.Desc
	out.limit = spec.Limit
	return nil
}

func normalizeSign(sign TxSign) (TxSign, error) {
	switch sign {
	case "", TxSignAny:
		return TxSignAny, nil
	case TxSignIncome, TxSignExpense:
		return sign, nil
	default:
		return "", invalidFilter("invalid sign %q", sign)
	}
}

func normalizeSort(sort TxSort) (TxSort, error) {
	switch sort {
	case "", TxSortDate:
		return TxSortDate, nil
	case TxSortAmount, TxSortPayee:
		return sort, nil
	default:
		return "", invalidFilter("invalid sort %q", sort)
	}
}

// Match reports whether tx and its normalized search text match the filter.
func (f TxFilter) Match(tx Transaction, searchText string) bool {
	return f.matchesDate(tx.Date) &&
		f.matchesAccount(tx.AccountID) &&
		f.matchesCategory(tx.CategoryID) &&
		f.matchesPayee(tx.PayeeID) &&
		f.matchesStatus(tx.Status) &&
		f.matchesKind(tx.Kind) &&
		f.matchesBatch(tx.ImportBatchID) &&
		f.matchesSign(tx.Amount) &&
		f.matchesAbs(tx.Amount.Minor()) &&
		f.matchesText(searchText)
}

func (f TxFilter) matchesDate(d Date) bool {
	return (f.from.IsZero() || !d.Before(f.from)) && (f.to.IsZero() || !d.After(f.to))
}

func (f TxFilter) matchesAccount(id AccountID) bool {
	if len(f.accountIDs) == 0 {
		return true
	}
	return slices.Contains(f.accountIDs, id)
}

func (f TxFilter) matchesCategory(id CategoryID) bool {
	if len(f.categoryIDs) == 0 && !f.uncategorized {
		return true
	}
	if id == 0 {
		return f.uncategorized
	}
	return slices.Contains(f.categoryIDs, id)
}

func (f TxFilter) matchesPayee(id PayeeID) bool {
	if len(f.payeeIDs) == 0 {
		return true
	}
	return slices.Contains(f.payeeIDs, id)
}

func (f TxFilter) matchesStatus(status TxStatus) bool {
	if len(f.statuses) == 0 {
		return true
	}
	return slices.Contains(f.statuses, status)
}

func (f TxFilter) matchesKind(kind TxKind) bool {
	if len(f.kinds) == 0 {
		return true
	}
	return slices.Contains(f.kinds, kind)
}

func (f TxFilter) matchesBatch(id ImportBatchID) bool {
	return f.importBatchID == 0 || id == f.importBatchID
}

func (f TxFilter) matchesSign(amount Money) bool {
	switch f.sign {
	case TxSignAny, "": // the zero value means any sign
		return true
	case TxSignIncome:
		return amount.Sign() > 0
	case TxSignExpense:
		return amount.Sign() < 0
	default:
		return false
	}
}

func (f TxFilter) matchesAbs(minor int64) bool {
	if f.minAbs == nil && f.maxAbs == nil {
		return true
	}
	abs, ok := absMinorInt64(minor)
	if !ok {
		return f.maxAbs == nil
	}
	if f.minAbs != nil && abs < *f.minAbs {
		return false
	}
	if f.maxAbs != nil && abs > *f.maxAbs {
		return false
	}
	return true
}

func (f TxFilter) matchesText(searchText string) bool {
	return f.text == "" || strings.Contains(searchText, f.text)
}

// Less reports whether a sorts before b under the filter's sort settings.
// When the chosen sort key ties, ID is the final deterministic tie-breaker.
func (f TxFilter) Less(a, b Transaction, payeeA, payeeB string) bool {
	cmp := 0
	switch f.sort {
	case TxSortDate, "": // the zero value sorts by date
		cmp = a.Date.Cmp(b.Date)
	case TxSortAmount:
		cmp = cmpInt64(a.Amount.Minor(), b.Amount.Minor())
	case TxSortPayee:
		cmp = strings.Compare(normalizedSortText(payeeA), normalizedSortText(payeeB))
	default:
		return false
	}
	if f.desc {
		cmp = -cmp
	}
	if cmp < 0 {
		return true
	}
	if cmp > 0 {
		return false
	}
	return a.ID < b.ID
}

// From returns the inclusive lower date bound.
func (f TxFilter) From() Date { return f.from }

// To returns the inclusive upper date bound.
func (f TxFilter) To() Date { return f.to }

// AccountIDs returns the account ids used by the filter.
func (f TxFilter) AccountIDs() []AccountID { return append([]AccountID(nil), f.accountIDs...) }

// CategoryIDs returns the category ids used by the filter.
func (f TxFilter) CategoryIDs() []CategoryID { return append([]CategoryID(nil), f.categoryIDs...) }

// Uncategorized reports whether uncategorized transactions are included.
func (f TxFilter) Uncategorized() bool { return f.uncategorized }

// PayeeIDs returns the payee ids used by the filter.
func (f TxFilter) PayeeIDs() []PayeeID { return append([]PayeeID(nil), f.payeeIDs...) }

// Statuses returns the accepted transaction statuses.
func (f TxFilter) Statuses() []TxStatus { return append([]TxStatus(nil), f.statuses...) }

// Kinds returns the accepted transaction kinds.
func (f TxFilter) Kinds() []TxKind { return append([]TxKind(nil), f.kinds...) }

// ImportBatchID returns the selected import batch id (zero when unset).
func (f TxFilter) ImportBatchID() ImportBatchID { return f.importBatchID }

// MinAbs returns the inclusive minimum absolute amount in minor units.
func (f TxFilter) MinAbs() *int64 { return cloneInt64(f.minAbs) }

// MaxAbs returns the inclusive maximum absolute amount in minor units.
func (f TxFilter) MaxAbs() *int64 { return cloneInt64(f.maxAbs) }

// Sign returns the sign filter.
func (f TxFilter) Sign() TxSign { return f.sign }

// Text returns the normalized text substring filter.
func (f TxFilter) Text() string { return f.text }

// Sort returns the selected sort key.
func (f TxFilter) Sort() TxSort { return f.sort }

// Desc reports whether sort is descending.
func (f TxFilter) Desc() bool { return f.desc }

// Limit returns the maximum number of rows (zero means "no limit").
func (f TxFilter) Limit() int { return f.limit }

func uniqueValid[T comparable](in []T, valid func(T) bool, name string) ([]T, error) {
	seen := map[T]struct{}{}
	out := make([]T, 0, len(in))
	for _, item := range in {
		if !valid(item) {
			return nil, invalidFilter("invalid %s %v", name, item)
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out, nil
}

func normalizeAbs(v *int64, field string) (*int64, error) {
	if v == nil {
		return nil, nil
	}
	if *v < 0 {
		return nil, invalidFilter("%s must be >= 0, got %d", field, *v)
	}
	out := *v
	return &out, nil
}

func cloneInt64(v *int64) *int64 {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}

func absMinorInt64(v int64) (int64, bool) {
	if v == -v && v != 0 {
		return 0, false
	}
	if v >= 0 {
		return v, true
	}
	return -v, true
}

func cmpInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func normalizedSortText(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func invalidFilter(format string, args ...any) error {
	return newError(ErrInvalidFilter, nil, "invalid_filter: "+format, args...)
}
