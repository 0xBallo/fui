package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
)

// InsightKind classifies an automatic insight.
type InsightKind string

// Insight kinds.
const (
	InsightAnomaly     InsightKind = "anomaly"
	InsightSubscript   InsightKind = "subscription"
	InsightPriceChange InsightKind = "price_change"
	InsightTrend       InsightKind = "trend"
)

// InsightConfig holds the thresholds of the automatic analyses. Ratios are
// expressed in permille so that no floating point is needed.
type InsightConfig struct {
	AnomalyFactorPermille int   // an expense is anomalous at >= factor × median (2500 = 2.5×)
	AnomalyWindowMonths   int   // history window preceding the expense
	AnomalyMinHistory     int   // minimum number of past expenses for a baseline
	SubscriptionTolPermil int   // amount tolerance around the median (50 = ±5%)
	SubscriptionMinCount  int   // minimum occurrences
	TrendPeriods          int   // number of previous monthly periods averaged
	TrendPermille         int   // minimum relative change vs the mean (250 = ±25%)
	TrendMinMinor         int64 // minimum amount (minor units) for a trend to be reported
}

// DefaultInsightConfig returns the default thresholds.
func DefaultInsightConfig() InsightConfig {
	return InsightConfig{
		AnomalyFactorPermille: 2500,
		AnomalyWindowMonths:   6,
		AnomalyMinHistory:     3,
		SubscriptionTolPermil: 50,
		SubscriptionMinCount:  3,
		TrendPeriods:          3,
		TrendPermille:         250,
		TrendMinMinor:         5000,
	}
}

// Insight is one finding. Key is a stable hash usable to dismiss it for good;
// Subject is "payee:<id>", "category:<id>" or "description:<text>"; Detail is
// a machine-readable summary ("key=value ...") left to the UI to render.
type Insight struct {
	Kind    InsightKind
	Key     string
	Subject string
	Detail  string
	Amount  Money
}

// Transactions are expected one per category: a split transaction is passed
// as one row per split, each with its own CategoryID and Amount.

func insightKey(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:8])
}

func sortInsights(out []Insight) []Insight {
	slices.SortFunc(out, func(a, b Insight) int {
		return strings.Compare(string(a.Kind)+"\x00"+a.Subject+"\x00"+a.Key, string(b.Kind)+"\x00"+b.Subject+"\x00"+b.Key)
	})
	return out
}

func sortedTxs(txs []Transaction) []Transaction {
	out := slices.Clone(txs)
	slices.SortFunc(out, func(a, b Transaction) int {
		if c := a.Date.Cmp(b.Date); c != 0 {
			return c
		}
		if a.ID != b.ID {
			return int(sign64(int64(a.ID) - int64(b.ID)))
		}
		if a.Amount.Minor() != b.Amount.Minor() {
			return int(sign64(a.Amount.Minor() - b.Amount.Minor()))
		}
		return int(sign64(int64(a.CategoryID) - int64(b.CategoryID)))
	})
	return out
}

func sign64(n int64) int64 {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}

// isExpense reports whether tx counts in the analyses: negative, valid and
// neither a transfer nor an investment.
func isExpense(tx Transaction) bool {
	return tx.Amount.Sign() < 0 && !tx.Date.IsZero() && tx.Kind != KindTransfer && tx.Kind != KindInvestment
}

func absMinor(m Money) int64 {
	if m.Minor() < 0 {
		return -m.Minor()
	}
	return m.Minor()
}

func subjectOf(tx Transaction, allowDescription bool) string {
	switch {
	case tx.PayeeID != 0:
		return fmt.Sprintf("payee:%d", tx.PayeeID)
	case tx.CategoryID != 0:
		return fmt.Sprintf("category:%d", tx.CategoryID)
	case allowDescription && strings.TrimSpace(tx.Description) != "":
		return "description:" + strings.ToLower(strings.TrimSpace(tx.Description))
	}
	return ""
}

func median(vals []int64) int64 {
	s := slices.Clone(vals)
	slices.Sort(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

func absMoney(minor int64, c Currency) (Money, error) { return NewMoney(minor, c) }

func anomalyHistory(all []Transaction, tx Transaction, subj string, start Date) []int64 {
	var hist []int64
	for _, h := range all {
		if h.ID == tx.ID && h.Date == tx.Date && h.Amount == tx.Amount && h.CategoryID == tx.CategoryID {
			continue
		}
		if isExpense(h) && h.Amount.Currency() == tx.Amount.Currency() && subjectOf(h, false) == subj &&
			!h.Date.Before(start) && h.Date.Before(tx.Date) {
			hist = append(hist, absMinor(h.Amount))
		}
	}
	return hist
}

// DetectAnomalies flags the expenses dated in period whose amount is at least
// AnomalyFactorPermille/1000 times the median of the same payee (or category
// when there is no payee) over the preceding AnomalyWindowMonths.
func DetectAnomalies(cfg InsightConfig, period Period, txs []Transaction) ([]Insight, error) {
	all := sortedTxs(txs)
	var out []Insight
	for _, tx := range all {
		if !isExpense(tx) || !period.Contains(tx.Date) {
			continue
		}
		subj := subjectOf(tx, false)
		if subj == "" {
			continue
		}
		start, err := tx.Date.AddMonths(-cfg.AnomalyWindowMonths)
		if err != nil {
			return nil, err
		}
		hist := anomalyHistory(all, tx, subj, start)
		if len(hist) < max(cfg.AnomalyMinHistory, 1) {
			continue
		}
		med := median(hist)
		amt := absMinor(tx.Amount)
		if med <= 0 || amt*1000 < int64(cfg.AnomalyFactorPermille)*med {
			continue
		}
		m, err := absMoney(amt, tx.Amount.Currency())
		if err != nil {
			return nil, err
		}
		out = append(out, Insight{
			Kind:    InsightAnomaly,
			Key:     insightKey(string(InsightAnomaly), fmt.Sprint(int64(tx.ID))),
			Subject: subj,
			Detail:  fmt.Sprintf("date=%s median=%d count=%d", tx.Date, med, len(hist)),
			Amount:  m,
		})
	}
	return sortInsights(out), nil
}

type subKey struct {
	subject string
	cur     Currency
}

func intervalUnit(days int) string {
	switch {
	case days >= 6 && days <= 8:
		return "weekly"
	case days >= 27 && days <= 32:
		return "monthly"
	case days >= 360 && days <= 370:
		return "yearly"
	}
	return ""
}

func daysBetween(a, b Date) (int, error) {
	n := 0
	for cur := a; cur.Before(b); n++ {
		if n > 400 {
			return n, nil
		}
		next, err := cur.AddDays(1)
		if err != nil {
			return 0, err
		}
		cur = next
	}
	return n, nil
}

func withinTol(v, ref int64, permille int) bool {
	d := v - ref
	if d < 0 {
		d = -d
	}
	return d*1000 <= int64(permille)*ref
}

// DetectSubscriptions finds expenses repeating at a regular weekly, monthly
// or yearly interval with a stable amount. Groups already tied to a
// recurrence (RecurrenceID set) are not reported as new subscriptions; for
// them a price change of the latest occurrence is reported instead.
func DetectSubscriptions(cfg InsightConfig, txs []Transaction) ([]Insight, error) {
	groups := map[subKey][]Transaction{}
	for _, tx := range sortedTxs(txs) {
		if !isExpense(tx) {
			continue
		}
		if s := subjectOf(tx, true); s != "" {
			k := subKey{s, tx.Amount.Currency()}
			groups[k] = append(groups[k], tx)
		}
	}
	var out []Insight
	for k, g := range groups {
		ins, ok, err := analyseGroup(cfg, k, g)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, ins)
		}
	}
	return sortInsights(out), nil
}

// groupInterval returns the common interval unit of g ("" when irregular) and
// whether any occurrence is tied to a recurrence.
func groupInterval(g []Transaction) (string, bool, error) {
	known := false
	unit := ""
	for i, tx := range g {
		known = known || tx.RecurrenceID != 0
		if i == 0 {
			continue
		}
		d, err := daysBetween(g[i-1].Date, tx.Date)
		if err != nil {
			return "", false, err
		}
		u := intervalUnit(d)
		if u == "" || (unit != "" && u != unit) {
			return "", false, nil
		}
		unit = u
	}
	return unit, known, nil
}

func analyseGroup(cfg InsightConfig, k subKey, g []Transaction) (Insight, bool, error) {
	if len(g) < max(cfg.SubscriptionMinCount, 2) {
		return Insight{}, false, nil
	}
	unit, known, err := groupInterval(g)
	if err != nil || unit == "" {
		return Insight{}, false, err
	}
	last := g[len(g)-1]
	prev := make([]int64, 0, len(g)-1)
	for _, tx := range g[:len(g)-1] {
		prev = append(prev, absMinor(tx.Amount))
	}
	prevMed := median(prev)
	stable := func(vals []int64, ref int64) bool {
		for _, v := range vals {
			if !withinTol(v, ref, cfg.SubscriptionTolPermil) {
				return false
			}
		}
		return true
	}
	lastAmt := absMinor(last.Amount)
	if known {
		if !stable(prev, prevMed) || withinTol(lastAmt, prevMed, cfg.SubscriptionTolPermil) {
			return Insight{}, false, nil
		}
		m, err := absMoney(lastAmt, k.cur)
		if err != nil {
			return Insight{}, false, err
		}
		return Insight{
			Kind:    InsightPriceChange,
			Key:     insightKey(string(InsightPriceChange), k.subject, k.cur.Code(), last.Date.String(), fmt.Sprint(lastAmt)),
			Subject: k.subject,
			Detail:  fmt.Sprintf("previous=%d current=%d interval=%s", prevMed, lastAmt, unit),
			Amount:  m,
		}, true, nil
	}
	all := append(prev, lastAmt)
	allMed := median(all)
	if !stable(all, allMed) {
		return Insight{}, false, nil
	}
	m, err := absMoney(allMed, k.cur)
	if err != nil {
		return Insight{}, false, err
	}
	return Insight{
		Kind:    InsightSubscript,
		Key:     insightKey(string(InsightSubscript), k.subject, k.cur.Code(), unit),
		Subject: k.subject,
		Detail:  fmt.Sprintf("interval=%s count=%d last=%s", unit, len(g), last.Date),
		Amount:  m,
	}, true, nil
}

// DetectTrends compares, per category, the spending of the monthly period
// current with the mean of the TrendPeriods monthly periods preceding it.
// A category is reported when the relative change reaches TrendPermille and
// either side is at least TrendMinMinor.
func DetectTrends(cfg InsightConfig, current Period, txs []Transaction) ([]Insight, error) {
	n := max(cfg.TrendPeriods, 1)
	prev, err := precedingMonths(current, n)
	if err != nil {
		return nil, err
	}
	type acc struct{ cur, prev int64 }
	totals := map[subKey]*acc{}
	for _, tx := range sortedTxs(txs) {
		if !isExpense(tx) || tx.CategoryID == 0 {
			continue
		}
		k := subKey{fmt.Sprintf("category:%d", tx.CategoryID), tx.Amount.Currency()}
		a := totals[k]
		if a == nil {
			a = &acc{}
			totals[k] = a
		}
		switch {
		case current.Contains(tx.Date):
			a.cur += absMinor(tx.Amount)
		case slices.ContainsFunc(prev, func(p Period) bool { return p.Contains(tx.Date) }):
			a.prev += absMinor(tx.Amount)
		}
	}
	var out []Insight
	for k, a := range totals {
		if max(a.cur, a.prev/int64(n)) < cfg.TrendMinMinor {
			continue
		}
		diff := a.cur*int64(n) - a.prev // change scaled by n
		abs := diff
		if abs < 0 {
			abs = -abs
		}
		if abs*1000 < int64(cfg.TrendPermille)*a.prev || abs == 0 {
			continue
		}
		dir := "up"
		if diff < 0 {
			dir = "down"
		}
		m, err := absMoney(diff/int64(n), k.cur)
		if err != nil {
			return nil, err
		}
		out = append(out, Insight{
			Kind:    InsightTrend,
			Key:     insightKey(string(InsightTrend), k.subject, k.cur.Code(), current.From().String(), dir),
			Subject: k.subject,
			Detail:  fmt.Sprintf("direction=%s current=%d mean=%d", dir, a.cur, a.prev/int64(n)),
			Amount:  m,
		})
	}
	return sortInsights(out), nil
}

// precedingMonths returns the n monthly periods before current, newest first.
func precedingMonths(current Period, n int) ([]Period, error) {
	prev := make([]Period, 0, n)
	to, err := current.From().AddDays(-1)
	if err != nil {
		return nil, err
	}
	for i := 1; i <= n; i++ {
		from, err := current.From().AddMonths(-i)
		if err != nil {
			return nil, err
		}
		p, err := NewPeriod(from, to)
		if err != nil {
			return nil, err
		}
		prev = append(prev, p)
		if to, err = from.AddDays(-1); err != nil {
			return nil, err
		}
	}
	return prev, nil
}
