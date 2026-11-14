// Package memstore is an in-memory implementation of app.Store for tests of the app, CLI and
// TUI layers. It honours the Unit of Work semantics (a failing WithTx leaves no change, Read is
// read-only); RunContract checks the repository contract so SQLite stores can run the same tests.
package memstore

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

var errReadOnly = errors.New("memstore: write in a read-only transaction")

type fxKey struct {
	cur  domain.Currency
	date domain.Date
}

type (
	refKey            struct{ connector, ref string }
	reconciliationKey struct {
		accountID domain.AccountID
		date      domain.Date
	}
)

type state struct {
	Accounts         map[domain.AccountID]domain.Account
	Txs              map[domain.TransactionID]domain.Transaction
	Categories       map[domain.CategoryID]domain.Category
	Rules            map[string]domain.Rule
	Groups           map[domain.TransferGroupID]domain.Money
	FX               map[fxKey]domain.FXRate
	Batches          map[domain.ImportBatchID]app.ImportBatch
	Refs             map[refKey]domain.AccountID
	Tags             map[domain.TagID]domain.Tag
	TxTags           map[domain.TransactionID][]domain.TagID
	Payees           map[domain.PayeeID]domain.Payee
	Splits           map[domain.TransactionID][]domain.Split
	Loans            map[domain.LoanID]domain.Loan
	LoanSchedules    map[loanScheduleKey]domain.Schedule
	LoanEvents       map[domain.LoanID][]domain.LoanEvent
	InstallmentLinks map[loanInstallmentKey]domain.TransactionID
	Budgets          map[domain.BudgetID]domain.Budget
	Moves            []domain.BudgetMove
	Recurs           map[app.RecurrenceID]app.Recurrence
	SavedSimulations map[app.SavedSimulationID]app.SavedSimulation
	Dismissed        map[string]app.DismissedInsight
	SavedReports     map[app.SavedReportID]app.SavedReport
	Reconciliations  map[reconciliationKey]app.Reconciliation
	Assets           map[domain.AssetID]domain.Asset
	Valuations       map[valuationKey]domain.Valuation
	LastID           map[string]int64
}

func newState() *state {
	return &state{
		Accounts:         map[domain.AccountID]domain.Account{},
		Txs:              map[domain.TransactionID]domain.Transaction{},
		Categories:       map[domain.CategoryID]domain.Category{},
		Rules:            map[string]domain.Rule{},
		Groups:           map[domain.TransferGroupID]domain.Money{},
		FX:               map[fxKey]domain.FXRate{},
		Batches:          map[domain.ImportBatchID]app.ImportBatch{},
		Refs:             map[refKey]domain.AccountID{},
		Tags:             map[domain.TagID]domain.Tag{},
		TxTags:           map[domain.TransactionID][]domain.TagID{},
		Payees:           map[domain.PayeeID]domain.Payee{},
		Splits:           map[domain.TransactionID][]domain.Split{},
		Loans:            map[domain.LoanID]domain.Loan{},
		LoanSchedules:    map[loanScheduleKey]domain.Schedule{},
		LoanEvents:       map[domain.LoanID][]domain.LoanEvent{},
		InstallmentLinks: map[loanInstallmentKey]domain.TransactionID{},
		Budgets:          map[domain.BudgetID]domain.Budget{},
		Recurs:           map[app.RecurrenceID]app.Recurrence{},
		SavedSimulations: map[app.SavedSimulationID]app.SavedSimulation{},
		Dismissed:        map[string]app.DismissedInsight{},
		SavedReports:     map[app.SavedReportID]app.SavedReport{},
		Reconciliations:  map[reconciliationKey]app.Reconciliation{},
		Assets:           map[domain.AssetID]domain.Asset{},
		Valuations:       map[valuationKey]domain.Valuation{},
		LastID:           map[string]int64{},
	}
}

func (s *state) clone() *state {
	return &state{
		Accounts:         maps.Clone(s.Accounts),
		Txs:              maps.Clone(s.Txs),
		Categories:       maps.Clone(s.Categories),
		Rules:            maps.Clone(s.Rules),
		Groups:           maps.Clone(s.Groups),
		FX:               maps.Clone(s.FX),
		Batches:          maps.Clone(s.Batches),
		Refs:             maps.Clone(s.Refs),
		LastID:           maps.Clone(s.LastID),
		Tags:             maps.Clone(s.Tags),
		TxTags:           maps.Clone(s.TxTags),
		Payees:           maps.Clone(s.Payees),
		Splits:           maps.Clone(s.Splits),
		Loans:            maps.Clone(s.Loans),
		LoanSchedules:    cloneSchedules(s.LoanSchedules),
		LoanEvents:       cloneEvents(s.LoanEvents),
		InstallmentLinks: maps.Clone(s.InstallmentLinks),
		Budgets:          maps.Clone(s.Budgets),
		Moves:            slices.Clone(s.Moves),
		Recurs:           maps.Clone(s.Recurs),
		SavedSimulations: maps.Clone(s.SavedSimulations),
		Dismissed:        maps.Clone(s.Dismissed),
		SavedReports:     maps.Clone(s.SavedReports),
		Reconciliations:  maps.Clone(s.Reconciliations),
		Assets:           maps.Clone(s.Assets),
		Valuations:       maps.Clone(s.Valuations),
	}
}

func (s *state) nextID(table string) int64 {
	s.LastID[table]++
	return s.LastID[table]
}

// Store implements app.Store in memory.
type Store struct {
	mu sync.RWMutex
	st *state
}

var _ app.Store = (*Store)(nil)

// New returns an empty store.
func New() *Store { return &Store{st: newState()} }

// Read implements app.Store; the repositories passed to fn reject writes.
func (s *Store) Read(ctx context.Context, fn func(app.Repos) error) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("memstore: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return fn(&repos{st: s.st, readOnly: true})
}

// WithTx implements app.Store: fn works on a copy that replaces the data only if fn succeeds.
func (s *Store) WithTx(ctx context.Context, fn func(app.Repos) error) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("memstore: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	work := s.st.clone()
	if err := fn(&repos{st: work}); err != nil {
		return err
	}
	s.st = work
	return nil
}

// Snapshot returns a deterministic text dump of the whole content, to assert that a use case
// did not write.
func (s *Store) Snapshot() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return fmt.Sprintf("%+v", *s.st)
}

type repos struct {
	st       *state
	readOnly bool
}

func (r *repos) Accounts() app.AccountRepo         { return accounts{r} }
func (r *repos) Transactions() app.TransactionRepo { return transactions{r} }
func (r *repos) Categories() app.CategoryRepo      { return categories{r} }
func (r *repos) Rules() app.RuleRepo               { return rules{r} }
func (r *repos) Transfers() app.TransferRepo       { return transfers{r} }
func (r *repos) FXRates() app.FXRateRepo           { return fxRates{r} }
func (r *repos) Imports() app.ImportRepo           { return imports{r} }
func (r *repos) Tags() app.TagRepo                 { return tags{r} }
func (r *repos) Payees() app.PayeeRepo             { return payees{r} }
func (r *repos) Splits() app.SplitRepo             { return splits{r} }
func (r *repos) Loans() app.LoanRepo               { return loans{r} }
func (r *repos) Budgets() app.BudgetRepo           { return budgets{r} }
func (r *repos) Recurrences() app.RecurrenceRepo   { return recurrences{r} }
func (r *repos) SavedSimulations() app.SavedSimulationRepo {
	return savedSimulations{r}
}

func (r *repos) DismissedInsights() app.DismissedInsightRepo {
	return dismissedInsights{r}
}
func (r *repos) SavedReports() app.SavedReportRepo { return savedReports{r} }
func (r *repos) Reconciliations() app.ReconciliationRepo {
	return reconciliations{r}
}

func (r *repos) Assets() app.AssetRepo { return assets{r} }

func (r *repos) writable() error {
	if r.readOnly {
		return errReadOnly
	}
	return nil
}

func notFound(what string, id any) error {
	return fmt.Errorf("%s %v: %w", what, id, domain.ErrNotFound)
}

func exists(what, name string) error {
	return fmt.Errorf("%s %q: %w", what, name, app.ErrAlreadyExists)
}

// sortedValues returns the values of m ordered by key.
func sortedValues[K cmp.Ordered, V any](m map[K]V) []V {
	return slices.Collect(func(yield func(V) bool) {
		for _, k := range slices.Sorted(maps.Keys(m)) {
			if !yield(m[k]) {
				return
			}
		}
	})
}

type accounts struct{ *repos }

func (a accounts) Insert(_ context.Context, acct domain.Account) (domain.Account, error) {
	if err := a.writable(); err != nil {
		return domain.Account{}, err
	}
	if err := a.checkName(acct); err != nil {
		return domain.Account{}, err
	}
	acct.ID = domain.AccountID(a.st.nextID("accounts"))
	a.st.Accounts[acct.ID] = acct
	return acct, nil
}

func (a accounts) checkName(acct domain.Account) error {
	for _, o := range a.st.Accounts {
		if o.ID != acct.ID && strings.EqualFold(o.Name, acct.Name) {
			return exists("account", acct.Name)
		}
	}
	return nil
}

func (a accounts) Update(_ context.Context, acct domain.Account) error {
	if err := a.writable(); err != nil {
		return err
	}
	if _, ok := a.st.Accounts[acct.ID]; !ok {
		return notFound("account", acct.ID)
	}
	if err := a.checkName(acct); err != nil {
		return err
	}
	a.st.Accounts[acct.ID] = acct
	return nil
}

func (a accounts) Get(_ context.Context, id domain.AccountID) (domain.Account, error) {
	acct, ok := a.st.Accounts[id]
	if !ok {
		return domain.Account{}, notFound("account", id)
	}
	return acct, nil
}

func (a accounts) List(context.Context) ([]domain.Account, error) {
	return sortedValues(a.st.Accounts), nil
}

type categories struct{ *repos }

func (c categories) checkName(cat domain.Category) error {
	for _, o := range c.st.Categories {
		if o.ID != cat.ID && strings.EqualFold(o.Name, cat.Name) {
			return exists("category", cat.Name)
		}
	}
	return nil
}

func (c categories) Insert(_ context.Context, cat domain.Category) (domain.Category, error) {
	if err := c.writable(); err != nil {
		return domain.Category{}, err
	}
	if err := c.checkName(cat); err != nil {
		return domain.Category{}, err
	}
	cat.ID = domain.CategoryID(c.st.nextID("categories"))
	c.st.Categories[cat.ID] = cat
	return cat, nil
}

func (c categories) Update(_ context.Context, cat domain.Category) error {
	if err := c.writable(); err != nil {
		return err
	}
	if _, ok := c.st.Categories[cat.ID]; !ok {
		return notFound("category", cat.ID)
	}
	if err := c.checkName(cat); err != nil {
		return err
	}
	c.st.Categories[cat.ID] = cat
	return nil
}

func (c categories) Delete(ctx context.Context, id domain.CategoryID) error {
	if err := c.writable(); err != nil {
		return err
	}
	if _, ok := c.st.Categories[id]; !ok {
		return notFound("category", id)
	}
	if n, _ := c.Usage(ctx, id); n > 0 {
		return fmt.Errorf("category %d has %d transactions: %w", id, n, app.ErrInUse)
	}
	for _, rows := range c.st.Splits {
		if slices.ContainsFunc(rows, func(r domain.Split) bool { return r.Category == id }) {
			return fmt.Errorf("category %d is used by split rows: %w", id, app.ErrInUse)
		}
	}
	delete(c.st.Categories, id)
	return nil
}

func (c categories) Get(_ context.Context, id domain.CategoryID) (domain.Category, error) {
	cat, ok := c.st.Categories[id]
	if !ok {
		return domain.Category{}, notFound("category", id)
	}
	return cat, nil
}

func (c categories) List(context.Context) ([]domain.Category, error) {
	return sortedValues(c.st.Categories), nil
}

func (c categories) Usage(_ context.Context, id domain.CategoryID) (int, error) {
	n := 0
	for _, t := range c.st.Txs {
		if t.CategoryID == id {
			n++
		}
	}
	for _, rows := range c.st.Splits {
		for _, r := range rows {
			if r.Category == id {
				n++
			}
		}
	}
	return n, nil
}

func (c categories) Reassign(_ context.Context, from, to domain.CategoryID) (int, error) {
	if err := c.writable(); err != nil {
		return 0, err
	}
	if _, ok := c.st.Categories[to]; !ok {
		return 0, notFound("category", to)
	}
	n := 0
	for id, t := range c.st.Txs {
		if t.CategoryID == from {
			t.CategoryID = to
			c.st.Txs[id] = t
			n++
		}
	}
	for id, rows := range c.st.Splits {
		moved := slices.Clone(rows)
		for i := range moved {
			if moved[i].Category == from {
				moved[i].Category = to
				n++
			}
		}
		c.st.Splits[id] = moved
	}
	return n, nil
}

type transfers struct{ *repos }

func (t transfers) Insert(_ context.Context, g domain.TransferGroup) (domain.TransferGroup, error) {
	if err := t.writable(); err != nil {
		return domain.TransferGroup{}, err
	}
	g.ID = domain.TransferGroupID(t.st.nextID("transfer_groups"))
	g.Out, g.In = domain.TransferLeg{}, domain.TransferLeg{}
	t.st.Groups[g.ID] = g.Fee
	return g, nil
}

func (t transfers) Get(_ context.Context, id domain.TransferGroupID) (domain.TransferGroup, error) {
	fee, ok := t.st.Groups[id]
	if !ok {
		return domain.TransferGroup{}, notFound("transfer", id)
	}
	g := domain.TransferGroup{ID: id, Fee: fee}
	for _, tx := range sortedValues(t.st.Txs) {
		if tx.TransferGroupID != id {
			continue
		}
		leg := domain.TransferLeg{TransactionID: tx.ID, AccountID: tx.AccountID, Amount: tx.Amount}
		if tx.Amount.Sign() < 0 {
			g.Out = leg
		} else {
			g.In = leg
		}
	}
	return g, nil
}

func (t transfers) Delete(_ context.Context, id domain.TransferGroupID) error {
	if err := t.writable(); err != nil {
		return err
	}
	if _, ok := t.st.Groups[id]; !ok {
		return notFound("transfer", id)
	}
	delete(t.st.Groups, id)
	return nil
}

type fxRates struct{ *repos }

func (f fxRates) Upsert(_ context.Context, r domain.FXRate) error {
	if err := f.writable(); err != nil {
		return err
	}
	f.st.FX[fxKey{r.Currency, r.Date}] = r
	return nil
}

func (f fxRates) List(_ context.Context, c domain.Currency) ([]domain.FXRate, error) {
	var out []domain.FXRate
	for _, r := range f.st.FX {
		if c.IsZero() || r.Currency == c {
			out = append(out, r)
		}
	}
	slices.SortFunc(out, func(a, b domain.FXRate) int {
		return cmp.Or(cmp.Compare(a.Currency.Code(), b.Currency.Code()), a.Date.Cmp(b.Date))
	})
	return out, nil
}

func (f fxRates) OnOrBefore(_ context.Context, c domain.Currency, d domain.Date) (domain.FXRate, error) {
	var best domain.FXRate
	for _, r := range f.st.FX {
		if r.Currency == c && !r.Date.After(d) && (best.Date.IsZero() || r.Date.After(best.Date)) {
			best = r
		}
	}
	if best.Date.IsZero() {
		return domain.FXRate{}, notFound("fx rate", c)
	}
	return best, nil
}

type imports struct{ *repos }

func (i imports) InsertBatch(_ context.Context, b app.ImportBatch) (app.ImportBatch, error) {
	if err := i.writable(); err != nil {
		return app.ImportBatch{}, err
	}
	b.ID = domain.ImportBatchID(i.st.nextID("import_batches"))
	i.st.Batches[b.ID] = b
	return b, nil
}

func (i imports) Batch(_ context.Context, id domain.ImportBatchID) (app.ImportBatch, error) {
	b, ok := i.st.Batches[id]
	if !ok {
		return app.ImportBatch{}, notFound("import batch", id)
	}
	return b, nil
}

func (i imports) Batches(context.Context) ([]app.ImportBatch, error) {
	out := sortedValues(i.st.Batches)
	slices.Reverse(out)
	return out, nil
}

func (i imports) BatchBySHA256(ctx context.Context, sha string) (app.ImportBatch, error) {
	all, _ := i.Batches(ctx)
	for _, b := range all {
		if b.SourceSHA256 == sha {
			return b, nil
		}
	}
	return app.ImportBatch{}, notFound("import batch with digest", sha)
}

func (i imports) DeleteBatch(_ context.Context, id domain.ImportBatchID) error {
	if err := i.writable(); err != nil {
		return err
	}
	if _, ok := i.st.Batches[id]; !ok {
		return notFound("import batch", id)
	}
	delete(i.st.Batches, id)
	return nil
}

func (i imports) AccountRef(_ context.Context, connector, ref string) (domain.AccountID, error) {
	id, ok := i.st.Refs[refKey{connector, ref}]
	if !ok {
		return 0, notFound("account ref", connector+"/"+ref)
	}
	return id, nil
}

func (i imports) SetAccountRef(_ context.Context, connector, ref string, id domain.AccountID) error {
	if err := i.writable(); err != nil {
		return err
	}
	if _, ok := i.st.Accounts[id]; !ok {
		return notFound("account", id)
	}
	i.st.Refs[refKey{connector, ref}] = id
	return nil
}
