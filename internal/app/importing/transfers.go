package importing

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/0xballo/fui/internal/domain"
)

// legPair is two rows that form one transfer; out is the negative leg.
type legPair struct{ out, in int }

// labelledPairs pairs the rows that share a transfer label, among the rows keep accepts. A
// label that is not on exactly two rows, with opposite signs on different accounts, pairs nothing.
func labelledPairs(rows []Row, keep func(i int) bool) []legPair {
	var labels []string
	byLabel := map[string][]int{}
	for i, r := range rows {
		if r.Transfer == "" || !keep(i) {
			continue
		}
		if _, seen := byLabel[r.Transfer]; !seen {
			labels = append(labels, r.Transfer)
		}
		byLabel[r.Transfer] = append(byLabel[r.Transfer], i)
	}
	var out []legPair
	for _, l := range labels {
		idx := byLabel[l]
		if len(idx) != 2 {
			continue
		}
		a, b := idx[0], idx[1]
		if rows[a].AccountRef == rows[b].AccountRef || rows[a].Tx.Amount.Sign()*rows[b].Tx.Amount.Sign() >= 0 {
			continue
		}
		if rows[a].Tx.Amount.Sign() > 0 {
			a, b = b, a
		}
		out = append(out, legPair{out: a, in: b})
	}
	return out
}

// newGroup builds the transfer group of two legs. Within one currency the legs sum to minus
// the fee, so a fee is inferred from them.
func newGroup(out, in domain.Transaction) (domain.TransferGroup, error) {
	g := domain.TransferGroup{
		Out: domain.TransferLeg{AccountID: out.AccountID, Amount: out.Amount},
		In:  domain.TransferLeg{AccountID: in.AccountID, Amount: in.Amount},
	}
	if out.Amount.Currency() == in.Amount.Currency() {
		sum, err := out.Amount.Add(in.Amount)
		if err != nil {
			return domain.TransferGroup{}, fmt.Errorf("transfer legs: %w", err)
		}
		if sum.Sign() < 0 {
			if g.Fee, err = sum.Neg(); err != nil {
				return domain.TransferGroup{}, fmt.Errorf("transfer fee: %w", err)
			}
		}
	}
	g, err := domain.NewTransferGroup(g)
	if err != nil {
		return domain.TransferGroup{}, fmt.Errorf("transfer: %w", err)
	}
	return g, nil
}

// checkTransferLabels reports the labelled legs that cannot be linked; they are imported unlinked.
func (b *builder) checkTransferLabels() {
	rows := b.plan.Rows
	for _, p := range labelledPairs(rows, func(i int) bool { return rows[i].Include }) {
		if _, err := newGroup(rows[p.out].Tx, rows[p.in].Tx); err != nil {
			b.plan.Errors = append(b.plan.Errors, RowError{
				Line: rows[p.out].Line, Message: fmt.Sprintf("transfer %q left unlinked: %v", rows[p.out].Transfer, err),
			})
		}
	}
}

func isTransferKind(k domain.TxKind) bool {
	return k == domain.KindTransfer || k == domain.KindExchange
}

// proposeTransfers suggests links between unlabelled transfer rows, and with transfer
// movements already in the archive: counterparty IBAN of the user's accounts first (both the
// stream's own accounts and every other active account already in the archive), then opposite
// amount within 2 days.
func (b *builder) proposeTransfers(ctx context.Context) error {
	var cands []domain.TransferCandidate
	for i, row := range b.plan.Rows {
		if !row.Include || row.Transfer != "" || !isTransferKind(row.Tx.Kind) {
			continue
		}
		cands = append(cands, domain.TransferCandidate{
			Reference: "row:" + strconv.Itoa(i), AccountID: row.Tx.AccountID, Date: row.Tx.Date,
			Amount: row.Tx.Amount, CounterpartyIBAN: b.ibans[i],
		})
	}
	if len(cands) == 0 {
		return nil
	}
	archived, err := b.archivedCandidates(ctx)
	if err != nil {
		return err
	}
	own := map[string]domain.AccountID{}
	for _, a := range b.active {
		if a.IBAN != "" {
			own[a.IBAN] = a.ID
		}
	}
	for _, ref := range b.order {
		if a := b.accts[ref]; a.rec.IBAN != "" && a.resolved() {
			own[a.rec.IBAN] = a.id
		}
	}
	matches, err := domain.ProposeTransferMatches(append(cands, archived...), own)
	if err != nil {
		return fmt.Errorf("propose transfers: %w", err)
	}
	for _, m := range matches {
		if p, ok := proposal(m); ok {
			b.plan.Transfers = append(b.plan.Transfers, p)
		}
	}
	return nil
}

// archivedCandidates lists the unlinked, unreconciled transfer movements near the dates of the
// rows, other than the ones a row was found to duplicate.
func (b *builder) archivedCandidates(ctx context.Context) ([]domain.TransferCandidate, error) {
	var from, to domain.Date
	matched := map[domain.TransactionID]bool{}
	for _, row := range b.plan.Rows {
		for _, id := range row.Matched {
			matched[id] = true
		}
		if d := row.Tx.Date; from.IsZero() || d.Before(from) {
			from = d
		}
		if d := row.Tx.Date; to.IsZero() || d.After(to) {
			to = d
		}
	}
	from, err := from.AddDays(-2)
	if err != nil {
		return nil, fmt.Errorf("transfer window: %w", err)
	}
	if to, err = to.AddDays(2); err != nil {
		return nil, fmt.Errorf("transfer window: %w", err)
	}
	window, err := domain.NewTxFilter(domain.TxFilterSpec{From: from, To: to, Desc: true})
	if err != nil {
		return nil, fmt.Errorf("transfer window: %w", err)
	}
	near, err := b.r.Transactions().List(ctx, window, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	var out []domain.TransferCandidate
	for _, t := range near {
		if !isTransferKind(t.Kind) || t.TransferGroupID != 0 || t.Status == domain.StatusReconciled || matched[t.ID] {
			continue
		}
		out = append(out, domain.TransferCandidate{
			Reference: "tx:" + strconv.FormatInt(int64(t.ID), 10), AccountID: t.AccountID, Date: t.Date, Amount: t.Amount,
		})
	}
	return out, nil
}

// proposal converts a match into a plan proposal; two archived movements are not our business.
func proposal(m domain.TransferMatch) (TransferProposal, bool) {
	l, r := m.Left, m.Right
	lRow, lIsRow := strings.CutPrefix(l, "row:")
	rRow, rIsRow := strings.CutPrefix(r, "row:")
	switch {
	case lIsRow && rIsRow:
		return TransferProposal{Row: atoi(lRow), Other: atoi(rRow), Reason: m.Reason}, true
	case lIsRow:
		return TransferProposal{Row: atoi(lRow), Other: -1, Existing: archivedID(r), Reason: m.Reason}, true
	case rIsRow:
		return TransferProposal{Row: atoi(rRow), Other: -1, Existing: archivedID(l), Reason: m.Reason}, true
	}
	return TransferProposal{}, false
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s) // built by proposeTransfers from an int
	return n
}

func archivedID(ref string) domain.TransactionID {
	n, _ := strconv.ParseInt(strings.TrimPrefix(ref, "tx:"), 10, 64) // built by archivedCandidates
	return domain.TransactionID(n)
}
