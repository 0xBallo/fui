package domain

import (
	"errors"
	"fmt"
	"testing"

	"pgregory.net/rapid"
)

func TestProposeTransferMatches(t *testing.T) {
	candidates := []TransferCandidate{
		{
			Reference:        "a-out",
			AccountID:        1,
			Date:             date(t, 2026, 11, 10),
			Amount:           money(t, -10_000, "EUR"),
			CounterpartyIBAN: "it60x0542811101000000123456",
		},
		{
			Reference: "a-in",
			AccountID: 2,
			Date:      date(t, 2026, 11, 11),
			Amount:    money(t, 10_000, "EUR"),
		},
		{
			Reference: "b-out",
			AccountID: 3,
			Date:      date(t, 2026, 11, 15),
			Amount:    money(t, -5_000, "EUR"),
		},
		{
			Reference: "b-in",
			AccountID: 4,
			Date:      date(t, 2026, 11, 16),
			Amount:    money(t, 5_000, "EUR"),
		},
	}
	matches, err := ProposeTransferMatches(candidates, map[string]AccountID{
		"IT60X0542811101000000123456": 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Fatalf("matches = %d", len(matches))
	}
	if matches[0].Reason != TransferMatchByCounterpartyIBAN || matches[1].Reason != TransferMatchByAmountAndDate {
		t.Fatalf("reasons = %+v", matches)
	}
}

func TestProposeTransferMatchesErrors(t *testing.T) {
	_, err := ProposeTransferMatches([]TransferCandidate{{}}, nil)
	if !errors.Is(err, ErrMissingField) {
		t.Fatalf("missing field err = %v", err)
	}
	_, err = ProposeTransferMatches([]TransferCandidate{{
		Reference: "x",
		AccountID: 1,
		Date:      date(t, 2026, 1, 1),
		Amount:    money(t, 1, "EUR"),
	}, {
		Reference: "y",
		AccountID: 2,
		Date:      date(t, 2026, 1, 1),
		Amount:    money(t, -1, "USD"),
	}}, nil)
	if err != nil {
		t.Fatalf("cross-currency should be ignored, got err=%v", err)
	}
}

func TestTransferWithoutFeeNetsToZeroProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		count := rapid.IntRange(1, 20).Draw(t, "count")
		candidates := make([]TransferCandidate, 0, count*2)
		for i := 0; i < count; i++ {
			amount := rapid.Int64Range(1, 1_000_000).Draw(t, "amount")
			day := rapid.IntRange(1, 28).Draw(t, "day")
			refOut := fmt.Sprintf("out-%d-%d", i, day)
			refIn := fmt.Sprintf("in-%d-%d", i, day)
			candidates = append(candidates,
				TransferCandidate{
					Reference: refOut + "-out",
					AccountID: AccountID(i*2 + 1),
					Date:      date(t, 2026, 11, day),
					Amount:    money(t, -amount, "EUR"),
				},
				TransferCandidate{
					Reference: refIn + "-in",
					AccountID: AccountID(i*2 + 2),
					Date:      date(t, 2026, 11, day),
					Amount:    money(t, amount, "EUR"),
				},
			)
		}
		matches, err := ProposeTransferMatches(candidates, nil)
		if err != nil {
			t.Fatal(err)
		}
		index := make(map[string]TransferCandidate, len(candidates))
		for _, candidate := range candidates {
			index[candidate.Reference] = candidate
		}
		for _, match := range matches {
			left := index[match.Left]
			right := index[match.Right]
			sum, err := left.Amount.Add(right.Amount)
			if err != nil {
				t.Fatal(err)
			}
			if !sum.IsZero() {
				t.Fatalf("match %v does not net to zero", match)
			}
		}
	})
}
