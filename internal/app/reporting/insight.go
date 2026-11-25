package reporting

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/recurring"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// InsightView is one insight in JSON-friendly form; Label is the resolved payee or category name.
type InsightView struct {
	Kind    string `json:"kind"`
	Key     string `json:"key"`
	Subject string `json:"subject"`
	Label   string `json:"label"`
	Detail  string `json:"detail"`
	Amount  Amount `json:"amount"`
}

// Insights runs the automatic analyses (anomalies, subscriptions, trends) over the analytics store.
// The recurring service and Analytics must share one Locale: Accept hands typed amounts between them.
type Insights struct {
	a   *Analytics
	rec *recurring.Service
	cfg domain.InsightConfig
}

// NewInsights builds Insights; a zero cfg means domain.DefaultInsightConfig.
func NewInsights(a *Analytics, rec *recurring.Service, cfg domain.InsightConfig) *Insights {
	if cfg == (domain.InsightConfig{}) {
		cfg = domain.DefaultInsightConfig()
	}
	return &Insights{a: a, rec: rec, cfg: cfg}
}

// found is one detected insight with the context Accept needs.
type found struct {
	domain.Insight
	label     string
	lastTx    domain.Transaction // latest transaction of the subject, for subscriptions
	dismissed bool
}

// InsightReport is the result of Report: the current insights plus warnings about data that
// could not be analysed (rows without an exchange rate).
type InsightReport struct {
	Insights []InsightView `json:"insights"`
	Warnings []string      `json:"warnings,omitempty"`
}

// Run returns the current insights that were not dismissed.
func (i *Insights) Run(ctx context.Context) ([]InsightView, error) {
	rep, err := i.Report(ctx)
	return rep.Insights, err
}

// Report is Run plus the warnings of the analysis.
func (i *Insights) Report(ctx context.Context) (InsightReport, error) {
	all, warnings, err := i.detect(ctx, hideCovered)
	if err != nil {
		return InsightReport{}, err
	}
	out := InsightReport{Insights: []InsightView{}, Warnings: warnings}
	for _, f := range all {
		if !f.dismissed {
			out.Insights = append(out.Insights, view(f))
		}
	}
	return out, nil
}

// Dismiss hides the insight with key for good. Dismissing twice is a no-op; an unknown key
// (not currently detected) fails with not_found.
func (i *Insights) Dismiss(ctx context.Context, key string) error {
	all, _, err := i.detect(ctx, keepCovered)
	if err != nil {
		return err
	}
	f, err := pick(all, key)
	if err != nil {
		return err
	}
	return i.a.store.WithTx(ctx, func(r app.Repos) error { return i.dismiss(ctx, r, f.Insight) })
}

func (i *Insights) dismiss(ctx context.Context, r app.Repos, in domain.Insight) error {
	return r.DismissedInsights().Dismiss(ctx, app.DismissedInsight{Key: in.Key, Kind: in.Kind, DismissedAt: i.a.clock.Now()})
}

// AcceptResult reports the recurrence an accepted subscription maps to; Created is false when an
// identical one already existed.
type AcceptResult struct {
	Recurrence app.Recurrence
	Created    bool
}

// AcceptSubscription turns the subscription insight key into a recurrence starting at its next
// expected date and dismisses the insight, in one transaction. It is idempotent: repeating it
// creates no second recurrence.
func (i *Insights) AcceptSubscription(ctx context.Context, key string) (AcceptResult, error) {
	if err := i.a.gate(module.Recurring); err != nil {
		return AcceptResult{}, err
	}
	all, _, err := i.detect(ctx, keepCovered)
	if err != nil {
		return AcceptResult{}, err
	}
	f, err := pick(all, key)
	if err != nil {
		return AcceptResult{}, err
	}
	if f.Kind != domain.InsightSubscript {
		return AcceptResult{}, invalid("key", fmt.Sprintf("insight %s is a %s, not a subscription", key, f.Kind))
	}
	req, err := i.request(ctx, f)
	if err != nil {
		return AcceptResult{}, err
	}
	plan, err := i.rec.PlanAdd(ctx, i.a.cfg.Modules, req)
	if err != nil {
		return AcceptResult{}, err
	}
	var out AcceptResult
	err = i.a.store.WithTx(ctx, func(r app.Repos) error {
		existing, err := r.Recurrences().List(ctx)
		if err != nil {
			return fmt.Errorf("list recurrences: %w", err)
		}
		want := plan.Recurrence
		if j := slices.IndexFunc(existing, func(e app.Recurrence) bool { return sameRecurrence(e, want) }); j >= 0 {
			out.Recurrence = existing[j]
		} else if out.Recurrence, err = r.Recurrences().Insert(ctx, want); err != nil {
			return fmt.Errorf("add recurrence: %w", err)
		} else {
			out.Created = true
		}
		return i.dismiss(ctx, r, f.Insight)
	})
	return out, err
}

func sameRecurrence(a, b app.Recurrence) bool {
	return a.AccountID == b.AccountID && a.Amount == b.Amount && a.Rule == b.Rule && a.Description == b.Description
}

// request builds the recurrence request of a subscription insight; the first occurrence is the
// one after the last seen.
func (i *Insights) request(ctx context.Context, f found) (recurring.AddRequest, error) {
	kv := detailMap(f.Detail)
	every, unit := 1, "months"
	switch kv["interval"] {
	case "weekly":
		unit = "weeks"
	case "yearly":
		every = 12
	}
	last, err := domain.ParseDate(kv["last"])
	if err != nil {
		return recurring.AddRequest{}, fmt.Errorf("insight %s last date: %w", f.Key, err)
	}
	var start domain.Date
	switch unit {
	case "weeks":
		start, err = last.AddDays(7)
	default:
		start, err = last.AddMonths(every)
	}
	if err != nil {
		return recurring.AddRequest{}, err
	}
	// f.Amount is the absolute median; recurrences store the signed value.
	amt, err := domain.NewMoney(-f.Amount.Minor(), f.Amount.Currency())
	if err != nil {
		return recurring.AddRequest{}, err
	}
	var account, category string
	err = i.a.store.Read(ctx, func(r app.Repos) error {
		acct, err := r.Accounts().Get(ctx, f.lastTx.AccountID)
		if err != nil {
			return fmt.Errorf("account of insight %s: %w", f.Key, err)
		}
		account = acct.Name
		if f.lastTx.CategoryID != 0 {
			c, err := r.Categories().Get(ctx, f.lastTx.CategoryID)
			if err != nil {
				return fmt.Errorf("category of insight %s: %w", f.Key, err)
			}
			category = c.Name
		}
		return nil
	})
	return recurring.AddRequest{
		Account: account, Category: category, Description: f.label,
		Amount: domain.FormatAmount(amt, i.a.cfg.Locale), Every: every, Unit: unit,
		Start: start.String(),
	}, err
}

func detailMap(detail string) map[string]string {
	out := map[string]string{}
	for _, p := range strings.Fields(detail) {
		if k, v, ok := strings.Cut(p, "="); ok {
			out[k] = v
		}
	}
	return out
}

func pick(all []found, key string) (found, error) {
	j := slices.IndexFunc(all, func(f found) bool { return f.Key == key })
	if j < 0 {
		return found{}, fmt.Errorf("insight %q: %w", key, domain.ErrNotFound)
	}
	return all[j], nil
}

func view(f found) InsightView {
	return InsightView{
		Kind: string(f.Kind), Key: f.Key, Subject: f.Subject, Label: f.label, Detail: f.Detail, Amount: amountOf(f.Amount),
	}
}

// coverMode tells detect whether subscriptions already covered by a recurrence are dropped. Accept and
// Dismiss keep them so that repeating an accept still resolves its key (idempotency).
type coverMode int

const (
	hideCovered coverMode = iota
	keepCovered
)

// detect runs every analysis over the booked history and marks the dismissed ones.
func (i *Insights) detect(ctx context.Context, mode coverMode) ([]found, []string, error) {
	if err := i.a.gate(); err != nil {
		return nil, nil, err
	}
	today, err := i.a.today()
	if err != nil {
		return nil, nil, err
	}
	cur, err := domain.MonthPeriod(today, i.a.cfg.MonthStartDay)
	if err != nil {
		return nil, nil, err
	}
	ld, err := i.load(ctx, today)
	if err != nil {
		return nil, nil, err
	}
	txs, dismissed, payees, cats := ld.txs, ld.dismissed, ld.payees, ld.cats
	known := ld.known
	if mode == keepCovered {
		known = nil
	}
	var ins []domain.Insight
	for _, run := range []func() ([]domain.Insight, error){
		func() ([]domain.Insight, error) { return domain.DetectAnomalies(i.cfg, cur, txs) },
		func() ([]domain.Insight, error) { return domain.DetectSubscriptions(i.cfg, txs, known) },
		func() ([]domain.Insight, error) { return domain.DetectTrends(i.cfg, cur, txs) },
	} {
		got, err := run()
		if err != nil {
			return nil, nil, fmt.Errorf("detect insights: %w", err)
		}
		ins = append(ins, got...)
	}
	out := make([]found, len(ins))
	for j, in := range ins {
		out[j] = found{Insight: in, label: subjectLabel(in.Subject, payees, cats), dismissed: dismissed[in.Key]}
		out[j].lastTx = lastOfSubject(txs, in)
	}
	return out, ld.warnings(), nil
}

// subjectLabel resolves "payee:<id>" / "category:<id>" to a name; descriptions are shown as typed.
func subjectLabel(subject string, payees map[domain.PayeeID]string, cats map[domain.CategoryID]string) string {
	kind, v, _ := strings.Cut(subject, ":")
	n, _ := strconv.ParseInt(v, 10, 64)
	switch kind {
	case "payee":
		if s := payees[domain.PayeeID(n)]; s != "" {
			return s
		}
	case "category":
		if s := cats[domain.CategoryID(n)]; s != "" {
			return s
		}
	}
	return v
}

// lastOfSubject is the latest expense of the insight's subject and currency (zero when none).
func lastOfSubject(txs []domain.Transaction, in domain.Insight) domain.Transaction {
	var last domain.Transaction
	for _, tx := range txs {
		if tx.Amount.Sign() < 0 && tx.Amount.Currency() == in.Amount.Currency() && txSubject(tx) == in.Subject &&
			(last.Date.IsZero() || !tx.Date.Before(last.Date)) {
			last = tx
		}
	}
	return last
}

// txSubject mirrors the subject the domain analyses group by (payee, category, description).
func txSubject(tx domain.Transaction) string {
	switch {
	case tx.PayeeID != 0:
		return fmt.Sprintf("payee:%d", tx.PayeeID)
	case tx.CategoryID != 0:
		return fmt.Sprintf("category:%d", tx.CategoryID)
	case strings.TrimSpace(tx.Description) != "":
		return "description:" + strings.ToLower(strings.TrimSpace(tx.Description))
	}
	return ""
}

// bookedSplit lists booked and reconciled transactions matching spec, one row per split.
func bookedSplit(ctx context.Context, r app.Repos, spec domain.TxFilterSpec) ([]domain.Transaction, error) {
	spec.Statuses = []domain.TxStatus{domain.StatusBooked, domain.StatusReconciled}
	f, err := domain.NewTxFilter(spec)
	if err != nil {
		return nil, err
	}
	txs, err := r.Transactions().List(ctx, f, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	var out []domain.Transaction
	for _, t := range txs {
		splits, err := r.Splits().SplitsOf(ctx, t.ID)
		if err != nil {
			return nil, fmt.Errorf("splits of transaction %d: %w", t.ID, err)
		}
		if len(splits) == 0 {
			out = append(out, t)
			continue
		}
		for _, sp := range splits {
			c := t
			c.CategoryID, c.Amount = sp.Category, sp.Amount
			out = append(out, c)
		}
	}
	return out, nil
}

// loaded is the data the analyses run over.
type loaded struct {
	txs       []domain.Transaction // booked history converted to the base currency
	dismissed map[string]bool
	payees    map[domain.PayeeID]string
	cats      map[domain.CategoryID]string
	known     []domain.KnownRecurrence
	noRate    int
	noRateCur map[string]bool
}

// warnings describes the rows excluded for lack of an exchange rate.
func (l loaded) warnings() []string {
	if l.noRate == 0 {
		return nil
	}
	cur := make([]string, 0, len(l.noRateCur))
	for c := range l.noRateCur {
		cur = append(cur, c)
	}
	slices.Sort(cur)
	return []string{fmt.Sprintf("%d transactions excluded from insights: missing exchange rate for %s", l.noRate, strings.Join(cur, ", "))}
}

// load reads the booked history converted to the base currency at each transaction date (rows
// without a rate are excluded and counted), the dismissed keys, the payee and category names and
// the existing recurrences.
func (i *Insights) load(ctx context.Context, today domain.Date) (loaded, error) {
	l := loaded{dismissed: map[string]bool{}, payees: map[domain.PayeeID]string{}, cats: map[domain.CategoryID]string{}, noRateCur: map[string]bool{}}
	err := i.a.store.Read(ctx, func(r app.Repos) error {
		rows, err := bookedSplit(ctx, r, domain.TxFilterSpec{})
		if err != nil {
			return err
		}
		l.txs = make([]domain.Transaction, 0, len(rows))
		for _, tx := range rows {
			amt, err := i.a.toBase(ctx, r, tx.Amount, tx.Date)
			if errors.Is(err, app.ErrRateMissing) {
				l.noRate++
				l.noRateCur[tx.Amount.Currency().Code()] = true
				continue
			}
			if err != nil {
				return err
			}
			tx.Amount = amt
			l.txs = append(l.txs, tx)
		}
		ds, err := r.DismissedInsights().List(ctx)
		if err != nil {
			return fmt.Errorf("list dismissed insights: %w", err)
		}
		for _, d := range ds {
			l.dismissed[d.Key] = true
		}
		ps, err := r.Payees().List(ctx)
		if err != nil {
			return fmt.Errorf("list payees: %w", err)
		}
		for _, p := range ps {
			l.payees[p.ID] = p.Name
		}
		if l.cats, err = categoryNames(ctx, r); err != nil {
			return err
		}
		recs, err := r.Recurrences().List(ctx)
		if err != nil {
			return fmt.Errorf("list recurrences: %w", err)
		}
		l.known, err = i.knownRecurrences(ctx, r, recs, ps, today)
		return err
	})
	return l, err
}

// knownRecurrences maps recurrences with a weekly, monthly or yearly cadence to the subjects the
// subscription detection groups by; a recurrence whose amount has no rate today is skipped.
func (i *Insights) knownRecurrences(ctx context.Context, r app.Repos, recs []app.Recurrence, payees []domain.Payee, today domain.Date) ([]domain.KnownRecurrence, error) {
	var out []domain.KnownRecurrence
	for _, rec := range recs {
		var interval string
		switch {
		case rec.Rule.Unit == domain.RecurrenceWeeks && rec.Rule.Every == 1:
			interval = "weekly"
		case rec.Rule.Unit == domain.RecurrenceMonths && rec.Rule.Every == 1:
			interval = "monthly"
		case rec.Rule.Unit == domain.RecurrenceMonths && rec.Rule.Every == 12:
			interval = "yearly"
		default:
			continue
		}
		amt, err := i.a.toBase(ctx, r, rec.Amount, today)
		if errors.Is(err, app.ErrRateMissing) {
			continue
		}
		if err != nil {
			return nil, err
		}
		desc := strings.ToLower(strings.TrimSpace(rec.Description))
		subjects := []string{"description:" + desc}
		for _, p := range payees {
			if strings.ToLower(strings.TrimSpace(p.Name)) == desc {
				subjects = append(subjects, fmt.Sprintf("payee:%d", p.ID))
			}
		}
		out = append(out, domain.KnownRecurrence{
			Subjects: subjects, Amount: absMinor(amt), Currency: i.a.cfg.BaseCurrency, Interval: interval,
		})
	}
	return out, nil
}

func absMinor(m domain.Money) int64 {
	if m.Minor() < 0 {
		return -m.Minor()
	}
	return m.Minor()
}
