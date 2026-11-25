package reporting_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/clock"
	"github.com/0xballo/fui/internal/domain"
)

func TestParseHorizon(t *testing.T) {
	tests := []struct {
		in      string
		want    reporting.Horizon
		wantErr bool
	}{
		{in: "30d", want: reporting.Horizon{Count: 30, Unit: 'd'}},
		{in: "8w", want: reporting.Horizon{Count: 8, Unit: 'w'}},
		{in: "6m", want: reporting.Horizon{Count: 6, Unit: 'm'}},
		{in: "90", want: reporting.Horizon{Count: 90, Unit: 'd'}},
		{in: " 2w ", want: reporting.Horizon{Count: 2, Unit: 'w'}},
		{in: "0", wantErr: true},
		{in: "0m", wantErr: true},
		{in: "-5d", wantErr: true},
		{in: "+5", wantErr: true},
		{in: "", wantErr: true},
		{in: "m", wantErr: true},
		{in: "3y", wantErr: true},
		{in: "1.5m", wantErr: true},
		{in: "abc", wantErr: true},
	}
	for _, tt := range tests {
		got, err := reporting.ParseHorizon(tt.in)
		if tt.wantErr {
			if !errors.Is(err, domain.ErrInvalidValue) {
				t.Errorf("ParseHorizon(%q) err = %v, want invalid_value", tt.in, err)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("ParseHorizon(%q) = %+v, %v; want %+v", tt.in, got, err, tt.want)
		}
	}
}

func TestHorizonDays(t *testing.T) {
	tests := []struct {
		h    reporting.Horizon
		from string
		want int
	}{
		{reporting.Horizon{Count: 10, Unit: 'd'}, "2026-03-15", 10},
		{reporting.Horizon{Count: 2, Unit: 'w'}, "2026-03-15", 14},
		{reporting.Horizon{Count: 6, Unit: 'm'}, "2026-03-15", 184},
		{reporting.Horizon{Count: 1, Unit: 'm'}, "2026-01-31", 28},
		{reporting.Horizon{Count: 12, Unit: 'm'}, "2024-02-29", 365},
	}
	for _, tt := range tests {
		got, err := tt.h.Days(date(tt.from))
		if err != nil || got != tt.want {
			t.Errorf("%+v.Days(%s) = %d, %v; want %d", tt.h, tt.from, got, err, tt.want)
		}
	}
	if _, err := (reporting.Horizon{Count: 1, Unit: 'x'}).Days(date("2026-03-15")); !errors.Is(err, domain.ErrInvalidValue) {
		t.Errorf("unknown unit err = %v", err)
	}
}

func TestForecastDefaultsToSixMonths(t *testing.T) {
	f := newFx(t)
	seedHistory(t, f)
	res, err := f.svc.Forecast(context.Background(), reporting.ForecastRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Days != 184 || len(res.Total) != 185 {
		t.Fatalf("days = %d, points = %d; want 184, 185", res.Days, len(res.Total))
	}
	for req, want := range map[reporting.ForecastRequest]int{
		{Horizon: "2w"}:                     14,
		{Horizon: "10", HorizonDays: 3}:     10,
		{HorizonDays: 3}:                    3,
		{Horizon: "1m", Granularity: "day"}: 31,
	} {
		got, err := f.svc.Forecast(context.Background(), req)
		if err != nil || got.Days != want {
			t.Errorf("%+v: days = %d, %v; want %d", req, got.Days, err, want)
		}
	}
	if _, err := f.svc.Forecast(context.Background(), reporting.ForecastRequest{Horizon: "0"}); !errors.Is(err, domain.ErrInvalidValue) {
		t.Errorf("zero horizon err = %v", err)
	}
	if _, err := f.svc.Forecast(context.Background(), reporting.ForecastRequest{Granularity: "year"}); !errors.Is(err, domain.ErrInvalidValue) {
		t.Errorf("bad granularity err = %v", err)
	}
}

func TestForecastConfigHorizonAndMinBalance(t *testing.T) {
	f := newFx(t)
	seedHistory(t, f)
	cfg := f.cfg
	cfg.Horizon, cfg.MinBalance = "3w", "4000"
	svc := reporting.NewAnalytics(f.store, clock.NewFixed(date("2026-03-15").In(time.UTC)), memSource{f.store}, cfg)
	res, err := svc.Forecast(context.Background(), reporting.ForecastRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Days != 21 || res.FirstBelowMin != "2026-03-15" {
		t.Fatalf("days = %d, below min = %q; want 21, 2026-03-15", res.Days, res.FirstBelowMin)
	}
	// The request wins over the config.
	res, err = svc.Forecast(context.Background(), reporting.ForecastRequest{Horizon: "1w", MinBalance: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Days != 7 || res.Total[0].Amount.Minor == 0 || res.FirstBelowMin != "" {
		t.Fatalf("override: days = %d, below min = %q", res.Days, res.FirstBelowMin)
	}
}

func TestForecastWeekGranularity(t *testing.T) {
	f := newFx(t)
	seedHistory(t, f)
	ctx := context.Background()
	// 2026-03-15 is a Sunday: it closes its own week, then 22 and 29 March, and the clipped last week.
	week, err := f.svc.Forecast(ctx, reporting.ForecastRequest{HorizonDays: 20, Granularity: reporting.GranularityWeek, MinBalance: "4000"})
	if err != nil {
		t.Fatal(err)
	}
	day, err := f.svc.Forecast(ctx, reporting.ForecastRequest{HorizonDays: 20, MinBalance: "4000"})
	if err != nil {
		t.Fatal(err)
	}
	wantDates := []string{"2026-03-15", "2026-03-22", "2026-03-29", "2026-04-04"}
	var got []string
	for _, p := range week.Total {
		got = append(got, p.Date)
	}
	if diff := cmp.Diff(wantDates, got); diff != "" {
		t.Fatalf("total week dates (-want +got):\n%s", diff)
	}
	byDate := map[string]reporting.Amount{}
	for _, p := range day.Total {
		byDate[p.Date] = p.Amount
	}
	for _, p := range week.Total {
		if p.Amount != byDate[p.Date] {
			t.Errorf("total %s = %v, daily %v", p.Date, p.Amount, byDate[p.Date])
		}
	}
	for i, ac := range week.Accounts {
		var dates []string
		for _, p := range ac.Series {
			dates = append(dates, p.Date)
		}
		if diff := cmp.Diff(wantDates, dates); diff != "" {
			t.Errorf("account %s week dates (-want +got):\n%s", ac.Account, diff)
		}
		if ac.FirstBelowMin != day.Accounts[i].FirstBelowMin || ac.FirstBelowZero != day.Accounts[i].FirstBelowZero {
			t.Errorf("account %s first-below changed by granularity", ac.Account)
		}
	}
	if week.FirstBelowMin != day.FirstBelowMin || week.FirstBelowZero != day.FirstBelowZero || week.FirstBelowMin == "" {
		t.Errorf("total first-below week = %q/%q, day = %q/%q", week.FirstBelowMin, week.FirstBelowZero, day.FirstBelowMin, day.FirstBelowZero)
	}
}
