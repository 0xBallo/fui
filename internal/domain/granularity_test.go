package domain

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"pgregory.net/rapid"
)

func gd(t testing.TB, s string) Date {
	t.Helper()
	d, err := ParseDate(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestBucketEnds(t *testing.T) {
	tests := []struct {
		name     string
		g        Granularity
		from, to string
		start    int
		want     []string
	}{
		{"day", GranDay, "2026-02-27", "2026-03-01", 1, []string{"2026-02-27", "2026-02-28", "2026-03-01"}},
		{"week mid-bucket", GranWeek, "2026-03-04", "2026-03-18", 1, []string{"2026-03-08", "2026-03-15", "2026-03-18"}},
		{"week starts on sunday", GranWeek, "2026-03-08", "2026-03-09", 1, []string{"2026-03-08", "2026-03-09"}},
		{"week exact end", GranWeek, "2026-03-02", "2026-03-08", 1, []string{"2026-03-08"}},
		{"month calendar", GranMonth, "2026-01-15", "2026-03-10", 1, []string{"2026-01-31", "2026-02-28", "2026-03-10"}},
		{"month start 27", GranMonth, "2026-01-10", "2026-03-30", 27, []string{"2026-01-26", "2026-02-26", "2026-03-26", "2026-03-30"}},
		{"month leap year", GranMonth, "2028-02-01", "2028-03-31", 1, []string{"2028-02-29", "2028-03-31"}},
		{"year", GranYear, "2025-06-01", "2027-02-01", 1, []string{"2025-12-31", "2026-12-31", "2027-02-01"}},
		{"single day", GranYear, "2026-05-05", "2026-05-05", 1, []string{"2026-05-05"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewPeriod(gd(t, tt.from), gd(t, tt.to))
			if err != nil {
				t.Fatal(err)
			}
			got, err := tt.g.BucketEnds(p, tt.start)
			if err != nil {
				t.Fatal(err)
			}
			var gs []string
			for _, d := range got {
				gs = append(gs, d.String())
			}
			if diff := cmp.Diff(tt.want, gs); diff != "" {
				t.Fatalf("(-want +got):\n%s", diff)
			}
		})
	}
}

func TestBucketEndsErrors(t *testing.T) {
	p, _ := NewPeriod(gd(t, "2026-01-01"), gd(t, "2026-01-31"))
	if _, err := Granularity("decade").BucketEnds(p, 1); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("bad granularity err = %v", err)
	}
	if _, err := GranMonth.BucketEnds(p, 0); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("bad month start err = %v", err)
	}
}

func TestParseGranularity(t *testing.T) {
	for in, want := range map[string]Granularity{"day": GranDay, " Week ": GranWeek, "MONTH": GranMonth, "year": GranYear} {
		got, err := ParseGranularity(in)
		if err != nil || got != want {
			t.Fatalf("ParseGranularity(%q) = %q, %v", in, got, err)
		}
	}
	_, err := ParseGranularity("hour")
	var de *Error
	if !errors.As(err, &de) || de.Code != CodeInvalidValue || de.Details["field"] != "granularity" {
		t.Fatalf("err = %v", err)
	}
}

func TestBucketEndsProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		g := rapid.SampledFrom([]Granularity{GranDay, GranWeek, GranMonth, GranYear}).Draw(rt, "g")
		start := rapid.IntRange(1, 31).Draw(rt, "start")
		from, _ := NewDate(2024, 1, 1)
		from, _ = from.AddDays(rapid.IntRange(0, 1500).Draw(rt, "off"))
		to, _ := from.AddDays(rapid.IntRange(0, 800).Draw(rt, "len"))
		p, _ := NewPeriod(from, to)
		ends, err := g.BucketEnds(p, start)
		if err != nil {
			rt.Fatal(err)
		}
		if ends[len(ends)-1] != to {
			rt.Fatalf("last %v != %v", ends[len(ends)-1], to)
		}
		for i := 1; i < len(ends); i++ {
			if !ends[i].After(ends[i-1]) {
				rt.Fatalf("not increasing: %v", ends)
			}
		}
		if ends[0].Before(from) {
			rt.Fatalf("first end before from")
		}
	})
}
