package importing

import (
	"context"
	"fmt"
	"strings"

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/pkg/fuiproto"
)

// addRow converts one transaction record into a plan row, or into a row error.
func (b *builder) addRow(ctx context.Context, rec *fuiproto.TransactionRecord) error {
	line := b.st.lines[rec]
	info := b.accts[rec.Account]
	tx, err := convert(rec, info)
	if err != nil {
		b.plan.Errors = append(b.plan.Errors, RowError{Line: line, Message: err.Error()})
		return nil
	}
	row := Row{
		Line: line, Class: domain.DedupNew, Include: true, AccountRef: rec.Account, Tx: tx,
		Category: strings.TrimSpace(rec.Category), Transfer: strings.TrimSpace(rec.Transfer),
	}
	if row.Category != "" {
		if _, err := domain.NewCategory(row.Category, domain.CategoryExpense); err != nil {
			b.plan.Errors = append(b.plan.Errors, RowError{Line: line, Message: err.Error()})
			return nil
		}
	}
	b.notePoint(rec, line, tx)
	if info.resolved() {
		if err := b.classify(ctx, &row, info); err != nil {
			return err
		}
	}
	b.noteCategory(&row)
	b.ibans[len(b.plan.Rows)] = rec.CounterpartyIBAN
	b.plan.Rows = append(b.plan.Rows, row)
	return nil
}

func convert(rec *fuiproto.TransactionRecord, info *acctInfo) (domain.Transaction, error) {
	cur, err := domain.ParseCurrency(rec.Currency)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("currency: %w", err)
	}
	if cur != info.cur {
		return domain.Transaction{}, fmt.Errorf("transaction is in %s but account %q is in %s", cur, rec.Account, info.cur)
	}
	amount, err := domain.NewMoney(rec.Amount, cur)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("amount: %w", err)
	}
	t := domain.Transaction{
		AccountID: info.id, Amount: amount, Description: strings.Join(strings.Fields(rec.Description), " "),
		Notes: strings.TrimSpace(rec.Notes), ExternalID: rec.ID, Status: domain.TxStatus(rec.Status),
	}
	for _, d := range []struct {
		in  string
		out *domain.Date
	}{{rec.Date, &t.Date}, {rec.OperationDate, &t.OperationDate}, {rec.ValueDate, &t.ValueDate}} {
		if d.in == "" {
			continue
		}
		if *d.out, err = domain.ParseDate(d.in); err != nil {
			return domain.Transaction{}, fmt.Errorf("date: %w", err)
		}
	}
	if t.Kind, err = domain.ParseTxKind(orDefault(string(rec.Kind), string(domain.KindOther))); err != nil {
		return domain.Transaction{}, fmt.Errorf("kind: %w", err)
	}
	t, err = domain.NewTransaction(t)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("transaction: %w", err)
	}
	return t, nil
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// notePoint remembers the balance_after of the row for the coherence check. A pending row is
// skipped: banks typically report a running balance that excludes still-pending movements, so a
// pending row's balance_after neither advances the chain nor is checked against it.
func (b *builder) notePoint(rec *fuiproto.TransactionRecord, line int, tx domain.Transaction) {
	if rec.BalanceAfter == nil || tx.Status == domain.StatusPending {
		return
	}
	b.bal[rec.Account] = append(b.bal[rec.Account],
		balPoint{line: line, date: tx.Date, amount: tx.Amount.Minor(), after: *rec.BalanceAfter})
}

// classify deduplicates the row against the archive (external id, then fingerprint, then the
// ±2 days window) and against the rows already seen in the same stream by external id.
func (b *builder) classify(ctx context.Context, row *Row, info *acctInfo) error {
	if ext := row.Tx.ExternalID; ext != "" {
		key := row.AccountRef + "\x00" + ext
		if b.seenExt[key] {
			row.Class, row.Include = domain.DedupDuplicate, false
			return nil
		}
		b.seenExt[key] = true
	}
	if info.id < 0 {
		return nil // account not created yet: nothing to compare with
	}
	existing, err := b.existing(ctx, row.Tx)
	if err != nil {
		return err
	}
	res, err := domain.ClassifyDedup(row.Tx, existing)
	if err != nil {
		return fmt.Errorf("deduplicate line %d: %w", row.Line, err)
	}
	row.Class, row.Matched = res.Class, res.Matched
	row.Include = res.Class != domain.DedupDuplicate
	return nil
}

// existing gathers the archive transactions that may duplicate tx.
func (b *builder) existing(ctx context.Context, tx domain.Transaction) ([]domain.Transaction, error) {
	repo := b.r.Transactions()
	var out []domain.Transaction
	seen := map[domain.TransactionID]bool{}
	add := func(ts []domain.Transaction, err error) error {
		if err != nil {
			return fmt.Errorf("look up existing transactions: %w", err)
		}
		for _, t := range ts {
			if !seen[t.ID] {
				seen[t.ID] = true
				out = append(out, t)
			}
		}
		return nil
	}
	if tx.ExternalID != "" {
		if err := add(repo.ByExternalID(ctx, tx.AccountID, tx.ExternalID)); err != nil {
			return nil, err
		}
	}
	fp, err := domain.Fingerprint(tx.AccountID, tx.Date, tx.Amount, tx.Description)
	if err != nil {
		return nil, fmt.Errorf("fingerprint: %w", err)
	}
	if err := add(repo.ByFingerprint(ctx, fp)); err != nil {
		return nil, err
	}
	from, err := tx.Date.AddDays(-2)
	if err != nil {
		return nil, fmt.Errorf("dedup window: %w", err)
	}
	to, err := tx.Date.AddDays(2)
	if err != nil {
		return nil, fmt.Errorf("dedup window: %w", err)
	}
	return out, add(repo.NearAmount(ctx, tx.AccountID, tx.Amount.Minor(), from, to))
}

// noteCategory canonicalises the category name and lists the ones that do not exist yet.
func (b *builder) noteCategory(row *Row) {
	if row.Category == "" {
		return
	}
	key := strings.ToLower(row.Category)
	if c, ok := b.cats[key]; ok {
		row.Category = c.Name
		return
	}
	if row.Class == domain.DedupDuplicate || b.newCats[key] {
		return
	}
	c, _ := domain.NewCategory(row.Category, kindFor(row.Tx)) // name validated in addRow
	b.newCats[key] = true
	b.plan.NewCategories = append(b.plan.NewCategories, c)
}

// kindFor guesses the kind of a category created by an import from the sign of its first use.
func kindFor(t domain.Transaction) domain.CategoryKind {
	if t.Amount.Sign() > 0 {
		return domain.CategoryIncome
	}
	return domain.CategoryExpense
}

// finishAccounts sets the opening date and balance of the accounts the import creates: the
// opening balance is what makes the last balance record of the stream come out right.
func (b *builder) finishAccounts(balances []fuiproto.BalanceRecord) error {
	for i := range b.plan.NewAccounts {
		na := &b.plan.NewAccounts[i]
		opened, opening := b.today, int64(0)
		first := true
		for _, row := range b.plan.Rows {
			if row.AccountRef == na.Ref && (first || row.Tx.Date.Before(opened)) {
				opened, first = row.Tx.Date, false
			}
		}
		if bal, ok := lastBalance(balances, na.Ref); ok {
			date, err := domain.ParseDate(bal.Date)
			if err != nil {
				return fmt.Errorf("balance of %q: %w", na.Ref, err)
			}
			if first || date.Before(opened) {
				opened = date
			}
			opening = bal.Amount
			for _, row := range b.plan.Rows {
				if row.AccountRef == na.Ref && !row.Tx.Date.After(date) {
					opening -= row.Tx.Amount.Minor()
				}
			}
		}
		money, err := domain.NewMoney(opening, na.Account.Currency)
		if err != nil {
			return fmt.Errorf("opening balance of %q: %w", na.Ref, err)
		}
		na.Account.OpeningBalance, na.Account.OpenedOn = money, opened
	}
	return nil
}

// lastBalance returns the balance record of ref with the latest date (the last one on a tie).
func lastBalance(all []fuiproto.BalanceRecord, ref string) (fuiproto.BalanceRecord, bool) {
	var best fuiproto.BalanceRecord
	found := false
	for _, b := range all {
		if b.Account == ref && (!found || b.Date >= best.Date) {
			best, found = b, true
		}
	}
	return best, found
}

// checkBalances verifies that every balance_after follows from the previous one, in date order
// (the stream may list movements newest first).
func (b *builder) checkBalances() {
	for _, ref := range b.order {
		pts := b.bal[ref]
		if len(pts) < 2 {
			continue
		}
		if pts[0].date.After(pts[len(pts)-1].date) {
			rev := make([]balPoint, len(pts))
			for i, p := range pts {
				rev[len(pts)-1-i] = p
			}
			pts = rev
		}
		for i := 1; i < len(pts); i++ {
			if want := pts[i-1].after + pts[i].amount; want != pts[i].after {
				b.plan.Discrepancies = append(b.plan.Discrepancies, BalanceDiscrepancy{
					AccountRef: ref, Line: pts[i].line, Date: pts[i].date, Expected: want, Actual: pts[i].after,
				})
			}
		}
	}
}
