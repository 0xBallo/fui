package domain

import "testing"

func TestMatchPending(t *testing.T) {
	booked, err := NewTransaction(Transaction{
		ID:        100,
		AccountID: 1,
		Date:      date(t, 2026, 11, 10),
		Amount:    money(t, -10_000, "EUR"),
		Status:    StatusBooked,
		Kind:      KindPayment,
	})
	if err != nil {
		t.Fatal(err)
	}
	pending := []Transaction{
		{
			ID:        20,
			AccountID: 1,
			Date:      date(t, 2026, 11, 8),
			Amount:    money(t, -10_000, "EUR"),
			Status:    StatusPending,
			Kind:      KindPayment,
		},
		{
			ID:        10,
			AccountID: 1,
			Date:      date(t, 2026, 11, 9),
			Amount:    money(t, -10_000, "EUR"),
			Status:    StatusPending,
			Kind:      KindPayment,
		},
		{
			ID:        30,
			AccountID: 1,
			Date:      date(t, 2026, 11, 9),
			Amount:    money(t, -10_002, "EUR"),
			Status:    StatusPending,
			Kind:      KindPayment,
		},
	}
	match, ok := MatchPending(pending, booked, 0)
	if !ok {
		t.Fatal("no match")
	}
	if match.ID != 10 {
		t.Fatalf("match id = %d, want 10", match.ID)
	}
	match, ok = MatchPending(pending, booked, 2)
	if !ok {
		t.Fatal("no match with tolerance")
	}
	if match.ID != 10 {
		t.Fatalf("match id with tolerance = %d, want 10", match.ID)
	}
}

func TestMatchPendingFilters(t *testing.T) {
	booked, err := NewTransaction(Transaction{
		ID:        100,
		AccountID: 1,
		Date:      date(t, 2026, 11, 10),
		Amount:    money(t, -10_000, "EUR"),
		Status:    StatusBooked,
		Kind:      KindPayment,
	})
	if err != nil {
		t.Fatal(err)
	}
	pending := []Transaction{
		{
			ID:        1,
			AccountID: 2,
			Date:      date(t, 2026, 11, 10),
			Amount:    money(t, -10_000, "EUR"),
			Status:    StatusPending,
			Kind:      KindPayment,
		},
		{
			ID:        2,
			AccountID: 1,
			Date:      date(t, 2026, 11, 14),
			Amount:    money(t, -10_000, "EUR"),
			Status:    StatusPending,
			Kind:      KindPayment,
		},
		{
			ID:        3,
			AccountID: 1,
			Date:      date(t, 2026, 11, 10),
			Amount:    money(t, -10_000, "USD"),
			Status:    StatusPending,
			Kind:      KindPayment,
		},
		{
			ID:        4,
			AccountID: 1,
			Date:      date(t, 2026, 11, 10),
			Amount:    money(t, -10_000, "EUR"),
			Status:    StatusBooked,
			Kind:      KindPayment,
		},
	}
	if _, ok := MatchPending(pending, booked, 0); ok {
		t.Fatal("unexpected match")
	}
}

func TestMatchPendingDeterministicTieBreakByID(t *testing.T) {
	booked := Transaction{
		AccountID: 1,
		Date:      date(t, 2026, 11, 10),
		Amount:    money(t, -10_000, "EUR"),
		Status:    StatusBooked,
		Kind:      KindPayment,
	}
	pending := []Transaction{
		{
			ID:        21,
			AccountID: 1,
			Date:      date(t, 2026, 11, 11),
			Amount:    money(t, -10_000, "EUR"),
			Status:    StatusPending,
			Kind:      KindPayment,
		},
		{
			ID:        11,
			AccountID: 1,
			Date:      date(t, 2026, 11, 9),
			Amount:    money(t, -10_000, "EUR"),
			Status:    StatusPending,
			Kind:      KindPayment,
		},
	}
	match, ok := MatchPending(pending, booked, 0)
	if !ok {
		t.Fatal("no match")
	}
	if match.ID != 11 {
		t.Fatalf("match id = %d, want 11", match.ID)
	}
}
