package domain

import (
	"errors"
	"slices"
	"testing"

	"pgregory.net/rapid"
)

func TestForecast(t *testing.T) {
	start := date(t, 2026, 11, 15)
	got, err := Forecast(
		start,
		3,
		[]ForecastOpening{
			{AccountID: 10, Balance: money(t, 1_000, "EUR")},
			{AccountID: 11, Balance: money(t, 500, "EUR")},
		},
		[]ForecastFlow{
			{AccountID: 10, Date: date(t, 2026, 11, 16), Amount: money(t, -200, "EUR")},
			{AccountID: 10, Date: date(t, 2026, 11, 18), Amount: money(t, -900, "EUR")},
			{AccountID: 11, Date: date(t, 2026, 11, 17), Amount: money(t, 50, "EUR")},
		},
		[]ForecastThreshold{
			{AccountID: 10, Minimum: money(t, 100, "EUR")},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("accounts = %d, want 2", len(got))
	}
	if got[0].AccountID != 10 || got[1].AccountID != 11 {
		t.Fatalf("account order = %v", []AccountID{got[0].AccountID, got[1].AccountID})
	}
	want10 := []int64{1_000, 800, 800, -100}
	if !seriesEquals(got[0].Series, want10) {
		t.Fatalf("account 10 series = %v, want %v", seriesMinors(got[0].Series), want10)
	}
	if !got[0].HasFirstBelowZero || got[0].FirstBelowZero != date(t, 2026, 11, 18) {
		t.Fatalf("first below zero = %v/%v", got[0].HasFirstBelowZero, got[0].FirstBelowZero)
	}
	if !got[0].HasFirstBelowMinimum || got[0].FirstBelowThreshold != date(t, 2026, 11, 18) {
		t.Fatalf("first below threshold = %v/%v", got[0].HasFirstBelowMinimum, got[0].FirstBelowThreshold)
	}
	want11 := []int64{500, 500, 550, 550}
	if !seriesEquals(got[1].Series, want11) {
		t.Fatalf("account 11 series = %v, want %v", seriesMinors(got[1].Series), want11)
	}
	if got[1].HasFirstBelowZero || got[1].HasFirstBelowMinimum {
		t.Fatalf("account 11 should not have threshold crossings")
	}
}

func TestForecastErrors(t *testing.T) {
	start := date(t, 2026, 11, 15)
	_, err := Forecast(Date{}, 0, nil, nil, nil)
	if !errors.Is(err, ErrMissingField) {
		t.Fatalf("missing date err = %v", err)
	}
	_, err = Forecast(start, -1, nil, nil, nil)
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("negative horizon err = %v", err)
	}
	_, err = Forecast(start, 1, []ForecastOpening{
		{AccountID: 1, Balance: money(t, 0, "EUR")},
	}, []ForecastFlow{
		{AccountID: 2, Date: date(t, 2026, 11, 16), Amount: money(t, 1, "EUR")},
	}, nil)
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("unknown account flow err = %v", err)
	}
	_, err = Forecast(start, 1, []ForecastOpening{
		{AccountID: 1, Balance: money(t, 0, "EUR")},
	}, []ForecastFlow{
		{AccountID: 1, Date: date(t, 2026, 11, 16), Amount: money(t, 1, "USD")},
	}, nil)
	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("flow currency mismatch err = %v", err)
	}
	_, err = Forecast(start, 1, []ForecastOpening{
		{AccountID: 1, Balance: money(t, 0, "EUR")},
	}, nil, []ForecastThreshold{
		{AccountID: 1, Minimum: money(t, 1, "USD")},
	})
	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("threshold currency mismatch err = %v", err)
	}
}

func TestForecastHorizonZeroProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		count := rapid.IntRange(1, 10).Draw(t, "count")
		openings := make([]ForecastOpening, 0, count)
		for i := range count {
			openings = append(openings, ForecastOpening{
				AccountID: AccountID(i + 1),
				Balance:   money(t, rapid.Int64Range(-1<<32, 1<<32).Draw(t, "balance"), "EUR"),
			})
		}
		start := genDate(t)
		got, err := Forecast(start, 0, openings, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(openings) {
			t.Fatalf("len = %d, want %d", len(got), len(openings))
		}
		for i, account := range got {
			if len(account.Series) != 1 {
				t.Fatalf("series len = %d, want 1", len(account.Series))
			}
			if account.Series[0].Date != start {
				t.Fatalf("series date = %v, want %v", account.Series[0].Date, start)
			}
			if account.Series[0].Balance != openings[i].Balance {
				t.Fatalf("balance = %v, want %v", account.Series[0].Balance, openings[i].Balance)
			}
		}
	})
}

func TestForecastZeroFlowNoChangeProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		count := rapid.IntRange(1, 8).Draw(t, "count")
		openings := make([]ForecastOpening, 0, count)
		for i := range count {
			openings = append(openings, ForecastOpening{
				AccountID: AccountID(i + 1),
				Balance:   money(t, rapid.Int64Range(-1<<32, 1<<32).Draw(t, "balance"), "EUR"),
			})
		}
		start := date(t, 2026, 1, 1)
		horizon := rapid.IntRange(1, 20).Draw(t, "horizon")
		base, err := Forecast(start, horizon, openings, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		zeroFlows := make([]ForecastFlow, 0, count)
		for _, opening := range openings {
			dayOffset := rapid.IntRange(1, horizon).Draw(t, "offset")
			day, err := start.AddDays(dayOffset)
			if err != nil {
				t.Fatal(err)
			}
			zeroFlows = append(zeroFlows, ForecastFlow{
				AccountID: opening.AccountID,
				Date:      day,
				Amount:    money(t, 0, "EUR"),
			})
		}
		withZero, err := Forecast(start, horizon, openings, zeroFlows, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !equalAccountForecasts(base, withZero) {
			t.Fatalf("adding zero flows changed forecast")
		}
	})
}

func equalAccountForecasts(left, right []AccountForecast) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].AccountID != right[i].AccountID ||
			left[i].FirstBelowZero != right[i].FirstBelowZero ||
			left[i].FirstBelowThreshold != right[i].FirstBelowThreshold ||
			left[i].HasFirstBelowZero != right[i].HasFirstBelowZero ||
			left[i].HasFirstBelowMinimum != right[i].HasFirstBelowMinimum {
			return false
		}
		if len(left[i].Series) != len(right[i].Series) {
			return false
		}
		for j := range left[i].Series {
			if left[i].Series[j] != right[i].Series[j] {
				return false
			}
		}
	}
	return true
}

func seriesEquals(points []ForecastPoint, minors []int64) bool {
	return slices.Equal(seriesMinors(points), minors)
}

func seriesMinors(points []ForecastPoint) []int64 {
	out := make([]int64, 0, len(points))
	for _, point := range points {
		out = append(out, point.Balance.Minor())
	}
	return out
}
