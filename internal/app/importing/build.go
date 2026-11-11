package importing

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/pkg/fuiproto"
)

// PlanRequest asks for the plan of one import. The records come from Source or, when it is
// nil, from the named Connector fetching Fetch.
type PlanRequest struct {
	Source    Source
	Connector string
	Fetch     fuiproto.FetchRequest
	// CreateAccounts creates the accounts of unknown refs from their account records.
	CreateAccounts bool
	// Mappings binds a connector account ref to an existing account typed by the user.
	Mappings map[string]string
}

// PlanImport runs steps 1–7 of the import pipeline: it validates the stream, resolves the
// account refs, normalises descriptions, deduplicates, proposes transfers and builds the plan.
// It never writes.
func (s *Service) PlanImport(ctx context.Context, req PlanRequest) (ImportPlan, error) {
	src, err := s.open(req.Connector, req.Fetch, req.Source)
	if err != nil {
		return ImportPlan{}, err
	}
	st, err := readStream(ctx, src)
	if err != nil {
		return ImportPlan{Stderr: st.stderr}, err
	}
	plan := ImportPlan{
		Connector: st.meta.Connector, Source: st.meta.Source.File,
		SourceSHA256: st.meta.Source.SHA256, Stderr: st.stderr,
	}
	today, err := domain.FromTime(s.clock.Now())
	if err != nil {
		return ImportPlan{}, fmt.Errorf("today: %w", err)
	}
	err = s.store.Read(ctx, func(r app.Repos) error {
		return newBuilder(r, req, st, today).build(ctx, &plan)
	})
	if err != nil {
		return ImportPlan{}, err
	}
	plan.refresh()
	return plan, nil
}

// acctInfo is a connector account and what it resolved to. id is the real ID, or a negative
// placeholder while the account is still to be created or mapped.
type acctInfo struct {
	rec    fuiproto.AccountRecord
	id     domain.AccountID
	cur    domain.Currency
	mapped bool // bound to an existing account
	create bool // the import creates it
}

// resolved reports whether the account is known, or will be once the import is applied.
func (a *acctInfo) resolved() bool { return a.mapped || a.create }

type balPoint struct {
	line          int
	date          domain.Date
	amount, after int64
}

type builder struct {
	r     app.Repos
	req   PlanRequest
	st    stream
	today domain.Date
	plan  *ImportPlan

	accts   map[string]*acctInfo
	order   []string         // refs in stream order
	active  []domain.Account // active accounts, for IBAN-based transfer matching
	cats    map[string]domain.Category
	newCats map[string]bool
	seenExt map[string]bool
	ibans   map[int]string // counterparty IBAN by row index
	bal     map[string][]balPoint
	pseudo  domain.AccountID
}

func newBuilder(r app.Repos, req PlanRequest, st stream, today domain.Date) *builder {
	return &builder{
		r: r, req: req, st: st, today: today,
		accts: map[string]*acctInfo{}, cats: map[string]domain.Category{}, newCats: map[string]bool{},
		seenExt: map[string]bool{}, ibans: map[int]string{}, bal: map[string][]balPoint{},
	}
}

func (b *builder) build(ctx context.Context, plan *ImportPlan) error {
	b.plan = plan
	if err := b.resolveAccounts(ctx); err != nil {
		return err
	}
	cats, err := b.r.Categories().List(ctx)
	if err != nil {
		return fmt.Errorf("list categories: %w", err)
	}
	for _, c := range cats {
		b.cats[strings.ToLower(c.Name)] = c
	}
	var balances []fuiproto.BalanceRecord
	for _, rec := range b.st.result.Records {
		switch {
		case rec.Transaction != nil:
			if err := b.addRow(ctx, rec.Transaction); err != nil {
				return err
			}
		case rec.Error != nil:
			plan.Errors = append(plan.Errors, RowError{Line: rec.Error.Line, Message: "source: " + rec.Error.Message})
		case rec.Balance != nil:
			balances = append(balances, *rec.Balance)
		}
	}
	for _, e := range b.st.result.RowErrors {
		plan.Errors = append(plan.Errors, RowError{Line: e.Line, Message: fmt.Sprintf("%s: %s", e.Type, e.Message)})
	}
	slices.SortStableFunc(plan.Errors, func(x, y RowError) int { return cmp.Compare(x.Line, y.Line) })
	if err := b.finishAccounts(balances); err != nil {
		return err
	}
	b.checkTransferLabels()
	if err := b.proposeTransfers(ctx); err != nil {
		return err
	}
	b.checkBalances()
	return b.checkAlreadyImported(ctx)
}

func (b *builder) checkAlreadyImported(ctx context.Context) error {
	if b.plan.SourceSHA256 == "" {
		return nil
	}
	batch, err := b.r.Imports().BatchBySHA256(ctx, b.plan.SourceSHA256)
	switch {
	case err == nil:
		b.plan.AlreadyImported = &batch
	case !errors.Is(err, domain.ErrNotFound):
		return fmt.Errorf("look up source digest: %w", err)
	}
	return nil
}

func (b *builder) resolveAccounts(ctx context.Context) error {
	all, err := b.r.Accounts().List(ctx)
	if err != nil {
		return fmt.Errorf("list accounts: %w", err)
	}
	var active []domain.Account
	for _, a := range all {
		if !a.Archived {
			active = append(active, a)
		}
	}
	b.active = active
	for _, rec := range b.st.result.Records {
		if rec.Account == nil {
			continue
		}
		info, err := b.resolve(ctx, *rec.Account, active)
		if err != nil {
			return err
		}
		b.accts[rec.Account.Ref] = info
		b.order = append(b.order, rec.Account.Ref)
	}
	return nil
}

// resolve binds one account record: explicit mapping, saved association, an account named like
// the ref, then creation (when asked) or a request for a mapping.
func (b *builder) resolve(ctx context.Context, rec fuiproto.AccountRecord, active []domain.Account) (*acctInfo, error) {
	cur, err := domain.ParseCurrency(rec.Currency)
	if err != nil {
		return nil, fmt.Errorf("account %q: %w", rec.Ref, err)
	}
	info := &acctInfo{rec: rec, cur: cur}
	bind := func(a domain.Account, save bool) *acctInfo {
		info.id, info.cur, info.mapped = a.ID, a.Currency, true
		if save {
			b.plan.Bindings = append(b.plan.Bindings, Binding{Ref: rec.Ref, AccountID: a.ID})
		}
		return info
	}
	if name, ok := b.req.Mappings[rec.Ref]; ok {
		a, err := pickByName(active, name)
		if err != nil {
			return nil, err
		}
		return bind(a, true), nil
	}
	id, err := b.r.Imports().AccountRef(ctx, b.st.meta.Connector, rec.Ref)
	switch {
	case err == nil:
		if a, err := b.r.Accounts().Get(ctx, id); err == nil {
			return bind(a, false), nil
		}
	case !errors.Is(err, domain.ErrNotFound):
		return nil, fmt.Errorf("account ref %q: %w", rec.Ref, err)
	}
	for _, a := range active {
		if strings.EqualFold(a.Name, rec.Ref) && a.Currency == cur {
			return bind(a, true), nil
		}
	}
	b.pseudo--
	info.id = b.pseudo
	return info, b.unresolved(info, active)
}

// unresolved plans the creation of the account or asks for a mapping.
func (b *builder) unresolved(info *acctInfo, active []domain.Account) error {
	rec, clash := info.rec, ""
	for _, a := range active {
		if strings.EqualFold(a.Name, strings.TrimSpace(rec.Name)) {
			clash = a.Name
		}
	}
	if b.req.CreateAccounts && clash == "" {
		a, err := newAccount(rec, info.cur, b.today)
		if err != nil {
			return fmt.Errorf("account %q: %w", rec.Ref, err)
		}
		b.plan.NewAccounts = append(b.plan.NewAccounts, NewAccount{Ref: rec.Ref, Account: a})
		info.create = true
		return nil
	}
	b.plan.Unmapped = append(b.plan.Unmapped, UnmappedRef{
		Ref: rec.Ref, Name: rec.Name, Currency: rec.Currency, Kind: string(rec.Kind), Suggestion: clash,
	})
	return nil
}

func newAccount(rec fuiproto.AccountRecord, cur domain.Currency, openedOn domain.Date) (domain.Account, error) {
	kind := domain.AccountKind(rec.Kind)
	if rec.Kind == fuiproto.AccountKindCard {
		kind = domain.AccountCreditCard
	}
	opening, err := domain.NewMoney(0, cur)
	if err != nil {
		return domain.Account{}, fmt.Errorf("opening balance: %w", err)
	}
	a, err := domain.NewAccount(domain.Account{
		Name: rec.Name, Kind: kind, Currency: cur, OpeningBalance: opening, OpenedOn: openedOn,
	})
	if err != nil {
		return domain.Account{}, fmt.Errorf("account: %w", err)
	}
	return a, nil
}

func pickByName(accts []domain.Account, name string) (domain.Account, error) {
	names := make([]string, len(accts))
	for i, a := range accts {
		names[i] = a.Name
	}
	i, err := domain.Resolve(name, names)
	if err != nil {
		return domain.Account{}, fmt.Errorf("account: %w", err)
	}
	return accts[i], nil
}
