package importing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
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
		Splits: parseSplits(rec.Splits), Payee: strings.TrimSpace(rec.Payee), Tags: sanitizeTags(rec.Tags),
		Hint: strings.TrimSpace(rec.CategoryHint),
	}
	if err := validateCategoryName(row.Category); err != nil {
		b.plan.Errors = append(b.plan.Errors, RowError{Line: line, Message: err.Error()})
		return nil
	}
	for i := range row.Splits {
		if err := validateCategoryName(row.Splits[i].Category); err != nil {
			b.plan.Errors = append(b.plan.Errors, RowError{Line: line, Message: err.Error()})
			return nil
		}
	}
	if row.Payee != "" {
		if _, err := domain.NewPayee(row.Payee); err != nil {
			b.plan.Errors = append(b.plan.Errors, RowError{Line: line, Message: err.Error()})
			return nil
		}
	}
	for _, t := range row.Tags {
		if _, err := domain.NewTag(t); err != nil {
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
	b.noteSplitCategories(&row)
	b.notePayee(&row)
	b.noteTags(&row)
	b.ibans[len(b.plan.Rows)] = rec.CounterpartyIBAN
	b.plan.Rows = append(b.plan.Rows, row)
	if rec.ID != "" {
		if _, seen := b.txIndex[rec.ID]; !seen {
			b.txIndex[rec.ID] = len(b.plan.Rows) - 1
		}
	}
	return nil
}

func (b *builder) addSecurityOp(_ context.Context, rec *fuiproto.SecurityOpRecord) error {
	line := b.st.sec[rec]
	if !b.req.Modules.Has(module.Investments) {
		b.warn(line, "security_op ignored because module investments is disabled")
		return nil
	}

	info, ok := b.securityOpAccount(rec.Account, line)
	if !ok {
		return nil
	}

	sec, err := parseSecurity(rec.Security)
	if err != nil {
		b.err(line, "security: %v", err)
		return nil
	}
	sec = b.resolveSecurity(sec)

	op, err := parseSecurityOp(rec, info.id, sec)
	if err != nil {
		b.err(line, "security_op: %v", err)
		return nil
	}

	b.plan.SecurityOps = append(b.plan.SecurityOps, SecurityOpPlanRow{
		Line:       line,
		AccountRef: rec.Account,
		Security:   sec,
		Op:         op,
		CashTx:     rec.CashTx,
		Include:    b.includeSecurityOp(line, rec.CashTx, op),
	})
	return nil
}

func (b *builder) includeSecurityOp(line int, cashTx string, op domain.SecurityOp) bool {
	if cashTx != "" {
		i, ok := b.txIndex[cashTx]
		if !ok || !b.plan.Rows[i].Include {
			b.warn(line, "security_op skipped because cash_tx %q is not imported", cashTx)
			return false
		}
		return true
	}
	key := securityOpKey(op, "")
	if b.existingOps[key] {
		b.warn(line, "security_op skipped because it already exists")
		return false
	}
	b.existingOps[key] = true
	return true
}

func (b *builder) securityOpAccount(ref string, line int) (*acctInfo, bool) {
	info := b.accts[ref]
	if info == nil {
		b.err(line, "unknown account ref %q", ref)
		return nil, false
	}
	if !info.resolved() {
		b.warn(line, "security_op ignored because account %q is unresolved", ref)
		return nil, false
	}
	return info, true
}

func (b *builder) warn(line int, format string, args ...any) {
	b.plan.Warnings = append(b.plan.Warnings, RowWarning{Line: line, Message: fmt.Sprintf(format, args...)})
}

func (b *builder) err(line int, format string, args ...any) {
	b.plan.Errors = append(b.plan.Errors, RowError{Line: line, Message: fmt.Sprintf(format, args...)})
}

func securityKeys(sec domain.Security) []string {
	keys := []string{
		"n:" + strings.ToLower(sec.Name) + "|" + string(sec.Kind) + "|" + sec.Currency.Code() + "|" + fmt.Sprintf("%d", sec.QuantityScale),
	}
	if sec.ISIN != "" {
		keys = append(keys, "i:"+sec.ISIN)
	}
	if sec.Ticker != "" {
		keys = append(keys, "t:"+sec.Ticker)
	}
	return keys
}

func (b *builder) noteSecurity(sec domain.Security) {
	for _, key := range securityKeys(sec) {
		if _, ok := b.secs[key]; !ok {
			b.secs[key] = sec
		}
	}
}

func (b *builder) resolveSecurity(sec domain.Security) domain.Security {
	for _, key := range securityKeys(sec) {
		if existing, ok := b.secs[key]; ok {
			return existing
		}
	}
	b.pseudo--
	sec.ID = domain.SecurityID(b.pseudo)
	b.plan.NewSecurities = append(b.plan.NewSecurities, sec)
	b.noteSecurity(sec)
	return sec
}

func parseSecurity(meta fuiproto.SecurityMeta) (domain.Security, error) {
	cur, err := domain.ParseCurrency(meta.Currency)
	if err != nil {
		return domain.Security{}, fmt.Errorf("currency: %w", err)
	}
	kind, err := domain.ParseSecurityKind(string(meta.Kind))
	if err != nil {
		return domain.Security{}, fmt.Errorf("kind: %w", err)
	}
	sec, err := domain.NewSecurity(domain.Security{
		Name:          meta.Name,
		Ticker:        meta.Ticker,
		ISIN:          meta.ISIN,
		Kind:          kind,
		Currency:      cur,
		QuantityScale: meta.QuantityScale,
	})
	if err != nil {
		return domain.Security{}, err
	}
	return sec, nil
}

func parseSecurityOp(rec *fuiproto.SecurityOpRecord, accountID domain.AccountID, sec domain.Security) (domain.SecurityOp, error) {
	date, err := domain.ParseDate(rec.Date)
	if err != nil {
		return domain.SecurityOp{}, fmt.Errorf("date: %w", err)
	}
	kind, err := domain.ParseSecurityOpKind(string(rec.Kind))
	if err != nil {
		return domain.SecurityOp{}, fmt.Errorf("kind: %w", err)
	}
	qty, err := domain.NewQuantity(rec.Quantity, sec.QuantityScale)
	if err != nil {
		return domain.SecurityOp{}, fmt.Errorf("quantity: %w", err)
	}
	price, err := domain.NewMoney(rec.UnitPrice, sec.Currency)
	if err != nil {
		return domain.SecurityOp{}, fmt.Errorf("unit price: %w", err)
	}
	fees, err := domain.NewMoney(rec.Fees, sec.Currency)
	if err != nil {
		return domain.SecurityOp{}, fmt.Errorf("fees: %w", err)
	}
	op, err := domain.NewSecurityOp(domain.SecurityOp{
		Date:      date,
		Account:   accountID,
		Security:  sec.ID,
		Kind:      kind,
		Quantity:  qty,
		UnitPrice: price,
		Fees:      fees,
	})
	if err != nil {
		return domain.SecurityOp{}, err
	}
	return op, nil
}

func securityOpKey(op domain.SecurityOp, cashRef string) string {
	return fmt.Sprintf("%d|%d|%s|%s|%d|%d|%d|%s",
		op.Account, op.Security, op.Date, op.Kind, op.Quantity.Units(), op.UnitPrice.Minor(), op.Fees.Minor(), cashRef)
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
	if rec.Original != nil {
		ocur, err := domain.ParseCurrency(rec.Original.Currency)
		if err != nil {
			return domain.Transaction{}, fmt.Errorf("original currency: %w", err)
		}
		om, err := domain.NewMoney(rec.Original.Amount, ocur)
		if err != nil {
			return domain.Transaction{}, fmt.Errorf("original amount: %w", err)
		}
		t.Original = &om
	}
	if t.Raw, err = compactRaw(rec.Raw); err != nil {
		return domain.Transaction{}, err
	}
	t, err = domain.NewTransaction(t)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("transaction: %w", err)
	}
	return t, nil
}

// compactRaw encodes the source record as compact JSON; encoding/json sorts map keys, so the
// text is deterministic.
func compactRaw(raw map[string]any) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return "", fmt.Errorf("raw record: %w", err)
	}
	return string(data), nil
}

// parseRaw is the inverse of compactRaw; numbers keep their exact text.
func parseRaw(raw string) map[string]any {
	if raw == "" {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.UseNumber()
	var out map[string]any
	if err := dec.Decode(&out); err != nil {
		return nil
	}
	return out
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// sanitizeTags trims the tag names of a record and drops empty or case-insensitively duplicate
// ones, keeping the first occurrence's spelling.
func sanitizeTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(tags))
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		key := strings.ToLower(t)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, t)
	}
	return out
}

func parseSplits(rows []fuiproto.TransactionSplit) []SplitRow {
	if len(rows) == 0 {
		return nil
	}
	out := make([]SplitRow, len(rows))
	for i, row := range rows {
		out[i] = SplitRow{
			Category: strings.TrimSpace(row.Category),
			Amount:   row.Amount,
			Note:     strings.TrimSpace(row.Note),
		}
	}
	return out
}

func validateCategoryName(name string) error {
	if name == "" {
		return nil
	}
	_, err := domain.NewCategory(name, domain.CategoryExpense)
	return err
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
	row.Category = b.noteCategoryName(row.Category, row.Class, kindFor(row.Tx))
}

func (b *builder) noteSplitCategories(row *Row) {
	for i := range row.Splits {
		row.Splits[i].Category = b.noteCategoryName(row.Splits[i].Category, row.Class, kindFor(row.Tx))
	}
}

func (b *builder) noteCategoryName(name string, class domain.DedupClass, kind domain.CategoryKind) string {
	if name == "" {
		return ""
	}
	key := strings.ToLower(name)
	if c, ok := b.cats[key]; ok {
		return c.Name
	}
	if class == domain.DedupDuplicate || b.newCats[key] {
		return name
	}
	c, _ := domain.NewCategory(name, kind) // name validated in addRow
	b.newCats[key] = true
	b.plan.NewCategories = append(b.plan.NewCategories, c)
	return name
}

// notePayee canonicalises the payee name (matched by name or alias) and lists it if it is not
// in the archive yet.
func (b *builder) notePayee(row *Row) {
	if row.Payee == "" {
		return
	}
	key := domain.NormalizePayee(row.Payee)
	if p, ok := b.payees[key]; ok {
		row.Payee = p.Name
		return
	}
	if row.Class == domain.DedupDuplicate || b.newPayees[key] {
		return
	}
	p, _ := domain.NewPayee(row.Payee) // name validated in addRow
	b.newPayees[key] = true
	b.plan.NewPayees = append(b.plan.NewPayees, p)
}

// noteTags canonicalises the row's tag names and lists the ones that do not exist yet.
func (b *builder) noteTags(row *Row) {
	for i, name := range row.Tags {
		key := strings.ToLower(name)
		if t, ok := b.tags[key]; ok {
			row.Tags[i] = t.Name
			continue
		}
		if row.Class == domain.DedupDuplicate || b.newTags[key] {
			continue
		}
		t, _ := domain.NewTag(name) // name validated in addRow
		b.newTags[key] = true
		b.plan.NewTags = append(b.plan.NewTags, t)
	}
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

// checkStatementBalances compares each balance record of an account that already exists with the
// balance the archive would have at that date once the included rows of the plan are applied,
// and reports the mismatches with the reconcile command that settles them.
func (b *builder) checkStatementBalances(ctx context.Context, balances []fuiproto.BalanceRecord) error {
	for _, bal := range balances {
		info := b.accts[bal.Account]
		if info == nil || !info.mapped || info.id <= 0 {
			continue
		}
		if err := b.checkStatementBalance(ctx, bal, info); err != nil {
			return err
		}
	}
	return nil
}

func (b *builder) checkStatementBalance(ctx context.Context, bal fuiproto.BalanceRecord, info *acctInfo) error {
	date, err := domain.ParseDate(bal.Date)
	if err != nil {
		return fmt.Errorf("balance of %q: %w", bal.Account, err)
	}
	acct, err := b.r.Accounts().Get(ctx, info.id)
	if err != nil {
		return fmt.Errorf("get account %q: %w", bal.Account, err)
	}
	sum, err := b.r.Transactions().SumUpTo(ctx, info.id, date)
	if err != nil {
		return fmt.Errorf("balance of %q: %w", bal.Account, err)
	}
	b.plan.BalanceChecks = append(b.plan.BalanceChecks, BalanceCheck{
		AccountRef: bal.Account, AccountID: info.id, AccountName: acct.Name, Currency: info.cur,
		Date: date, Expected: bal.Amount, Base: acct.OpeningBalance.Minor() + sum,
	})
	return nil
}

// refreshBalanceMismatches recomputes BalanceMismatches from BalanceChecks and the rows that are
// currently included, so toggling a row keeps the preview coherent.
func (p *ImportPlan) refreshBalanceMismatches() {
	p.BalanceMismatches = nil
	for _, c := range p.BalanceChecks {
		computed := c.Base
		for _, row := range p.Rows {
			if row.AccountRef == c.AccountRef && row.Include && row.Class != domain.DedupDuplicate &&
				row.ConfirmsPending == 0 && !row.Tx.Date.After(c.Date) {
				computed += row.Tx.Amount.Minor()
			}
		}
		if computed == c.Expected {
			continue
		}
		want, err := domain.NewMoney(c.Expected, c.Currency)
		if err != nil {
			continue // the currency was validated when the account was resolved
		}
		p.BalanceMismatches = append(p.BalanceMismatches, BalanceMismatch{
			AccountRef: c.AccountRef, AccountID: c.AccountID, Date: c.Date, Expected: c.Expected, Computed: computed,
			Suggest: fmt.Sprintf("fui reconcile %s --date %s --balance %s", shellQuote(c.AccountName), c.Date, domain.FormatMachine(want)),
		})
	}
}

// shellQuote quotes s for a POSIX shell only when it needs it.
func shellQuote(s string) string {
	if s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-.") == "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
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
