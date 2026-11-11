package domain

import (
	"cmp"
	"errors"
	"slices"
	"strings"
)

// TransferMatchReason explains why two movements were proposed as a transfer.
type TransferMatchReason string

// Transfer match reasons.
const (
	TransferMatchByCounterpartyIBAN TransferMatchReason = "counterparty_iban"
	TransferMatchByAmountAndDate    TransferMatchReason = "amount_date"
)

// TransferCandidate is an input movement for transfer matching.
type TransferCandidate struct {
	Reference        string
	AccountID        AccountID
	Date             Date
	Amount           Money
	CounterpartyIBAN string
}

// TransferMatch is a proposed link between two movements.
type TransferMatch struct {
	Left   string
	Right  string
	Reason TransferMatchReason
}

// ProposeTransferMatches proposes transfer pairs; it never decides or mutates.
func ProposeTransferMatches(candidates []TransferCandidate, ownIBAN map[string]AccountID) ([]TransferMatch, error) {
	sorted := slices.Clone(candidates)
	slices.SortFunc(sorted, compareCandidate)
	if err := validateCandidates(sorted); err != nil {
		return nil, err
	}
	normalizedIBAN := normalizeIBANMap(ownIBAN)
	used := make([]bool, len(sorted))
	matches, err := matchByIBAN(sorted, normalizedIBAN, used)
	if err != nil {
		return nil, err
	}
	fallback, err := matchByAmountDate(sorted, used)
	if err != nil {
		return nil, err
	}
	return append(matches, fallback...), nil
}

func compareCandidate(a, b TransferCandidate) int {
	if cmpDate := a.Date.Cmp(b.Date); cmpDate != 0 {
		return cmpDate
	}
	return cmp.Compare(a.Reference, b.Reference)
}

func validateCandidates(candidates []TransferCandidate) error {
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.Reference) == "" {
			return missingField("reference")
		}
		if candidate.AccountID == 0 {
			return missingField("account_id")
		}
		if candidate.Date.IsZero() {
			return missingField("date")
		}
		if candidate.Amount.Currency().IsZero() {
			return missingField("amount")
		}
	}
	return nil
}

func normalizeIBANMap(ownIBAN map[string]AccountID) map[string]AccountID {
	normalized := make(map[string]AccountID, len(ownIBAN))
	for iban, accountID := range ownIBAN {
		normalized[normalizeIBAN(iban)] = accountID
	}
	return normalized
}

func normalizeIBAN(iban string) string {
	replacer := strings.NewReplacer(" ", "", "\t", "", "\n", "")
	return strings.ToUpper(replacer.Replace(strings.TrimSpace(iban)))
}

func matchByIBAN(candidates []TransferCandidate, ownIBAN map[string]AccountID, used []bool) ([]TransferMatch, error) {
	var matches []TransferMatch
	for i, candidate := range candidates {
		if used[i] {
			continue
		}
		targetAccount, ok := ownIBAN[normalizeIBAN(candidate.CounterpartyIBAN)]
		if !ok || targetAccount == candidate.AccountID {
			continue
		}
		partner, err := findPartner(candidates, used, i, func(other TransferCandidate) bool {
			return other.AccountID == targetAccount
		})
		if err != nil {
			return nil, err
		}
		if partner == -1 {
			continue
		}
		used[i], used[partner] = true, true
		matches = append(matches, TransferMatch{
			Left:   candidate.Reference,
			Right:  candidates[partner].Reference,
			Reason: TransferMatchByCounterpartyIBAN,
		})
	}
	return matches, nil
}

func matchByAmountDate(candidates []TransferCandidate, used []bool) ([]TransferMatch, error) {
	var matches []TransferMatch
	for i, candidate := range candidates {
		if used[i] {
			continue
		}
		partner, err := findPartner(candidates, used, i, func(other TransferCandidate) bool {
			return other.AccountID != candidate.AccountID
		})
		if err != nil {
			return nil, err
		}
		if partner == -1 {
			continue
		}
		used[i], used[partner] = true, true
		matches = append(matches, TransferMatch{
			Left:   candidate.Reference,
			Right:  candidates[partner].Reference,
			Reason: TransferMatchByAmountAndDate,
		})
	}
	return matches, nil
}

func findPartner(candidates []TransferCandidate, used []bool, source int, accountFilter func(TransferCandidate) bool) (int, error) {
	best := -1
	bestDistance := 3
	for i, candidate := range candidates {
		if i == source || used[i] || !accountFilter(candidate) {
			continue
		}
		distance, ok, err := canMatch(candidates[source], candidate)
		if err != nil {
			return -1, err
		}
		if !ok {
			continue
		}
		if distance < bestDistance || distance == bestDistance && candidate.Reference < candidates[best].Reference {
			best = i
			bestDistance = distance
		}
	}
	return best, nil
}

func canMatch(left, right TransferCandidate) (int, bool, error) {
	if left.AccountID == right.AccountID {
		return 0, false, nil
	}
	sum, err := left.Amount.Add(right.Amount)
	if err != nil {
		if errors.Is(err, ErrCurrencyMismatch) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if !sum.IsZero() {
		return 0, false, nil
	}
	distance := dateDistanceDays(left.Date, right.Date)
	return distance, distance <= 2, nil
}
