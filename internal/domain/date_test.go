package domain

import (
	"errors"
	"testing"
	"time"

	"pgregory.net/rapid" //nolint:depguard // test-only dependency allowed by the technical spec
)

func date(t tb, y, m, d int) Date {
	t.Helper()
	got, err := NewDate(y, m, d)
	if err != nil {
		t.Fatalf("NewDate(%d,%d,%d): %v", y, m, d, err)
	}
	return got
}

func TestNewDate(t *testing.T) {
	valid := [][3]int{{2026, 11, 11}, {2024, 2, 29}, {2000, 2, 29}, {1, 1, 1}, {9999, 12, 31}}
	for _, v := range valid {
		if _, err := NewDate(v[0], v[1], v[2]); err != nil {
			t.Errorf("NewDate%v: %v", v, err)
		}
	}
	invalid := [][3]int{{2026, 2, 29}, {1900, 2, 29}, {2026, 4, 31}, {2026, 13, 1}, {2026, 0, 1}, {2026, 1, 0}, {0, 1, 1}, {10000, 1, 1}}
	for _, v := range invalid {
		if _, err := NewDate(v[0], v[1], v[2]); !errors.Is(err, ErrInvalidDate) {
			t.Errorf("NewDate%v err = %v", v, err)
		}
	}
}

func TestDateBasics(t *testing.T) {
	d := date(t, 2026, 3, 9)
	if d.String() != "2026-03-09" || d.Format(LocaleITIT) != "09/03/2026" || d.Format(LocaleENUS) != "03/09/2026" {
		t.Errorf("formats: %s %s %s", d, d.Format(LocaleITIT), d.Format(LocaleENUS))
	}
	if d.Year() != 2026 || d.Month() != 3 || d.Day() != 9 || d.IsZero() || !(Date{}).IsZero() {
		t.Error("accessors")
	}
	later := date(t, 2026, 3, 10)
	if !d.Before(later) || !later.After(d) || d.Cmp(d) != 0 || d.Cmp(later) != -1 || later.Cmp(d) != 1 {
		t.Error("comparison")
	}
	if date(t, 2026, 4, 1).Cmp(d) != 1 || date(t, 2025, 12, 31).Cmp(d) != -1 {
		t.Error("comparison across months/years")
	}
}

func TestAddDays(t *testing.T) {
	tests := []struct {
		from Date
		n    int
		want Date
	}{
		{date(t, 2026, 1, 31), 1, date(t, 2026, 2, 1)},
		{date(t, 2026, 3, 1), -1, date(t, 2026, 2, 28)},
		{date(t, 2024, 3, 1), -1, date(t, 2024, 2, 29)},
		{date(t, 2025, 12, 31), 1, date(t, 2026, 1, 1)},
		{date(t, 2026, 11, 11), 0, date(t, 2026, 11, 11)},
		{date(t, 2026, 11, 11), -365, date(t, 2025, 11, 11)},
	}
	for _, tt := range tests {
		if got, err := tt.from.AddDays(tt.n); err != nil || got != tt.want {
			t.Errorf("%s.AddDays(%d) = %s, %v; want %s", tt.from, tt.n, got, err, tt.want)
		}
	}
	for _, bad := range []struct {
		from Date
		n    int
	}{{date(t, 9999, 12, 31), 1}, {date(t, 1, 1, 1), -1}, {date(t, 2026, 1, 1), 1 << 40}, {date(t, 2026, 1, 1), -1 << 40}} {
		if _, err := bad.from.AddDays(bad.n); !errors.Is(err, ErrInvalidDate) {
			t.Errorf("%s.AddDays(%d) err = %v", bad.from, bad.n, err)
		}
	}
}

func TestAddMonths(t *testing.T) {
	tests := []struct {
		from Date
		n    int
		want Date
	}{
		{date(t, 2026, 1, 31), 1, date(t, 2026, 2, 28)},
		{date(t, 2024, 1, 31), 1, date(t, 2024, 2, 29)},
		{date(t, 2026, 3, 31), -1, date(t, 2026, 2, 28)},
		{date(t, 2026, 11, 11), 2, date(t, 2027, 1, 11)},
		{date(t, 2026, 1, 15), -1, date(t, 2025, 12, 15)},
		{date(t, 2026, 5, 31), -3, date(t, 2026, 2, 28)},
		{date(t, 2026, 5, 31), 0, date(t, 2026, 5, 31)},
	}
	for _, tt := range tests {
		if got, err := tt.from.AddMonths(tt.n); err != nil || got != tt.want {
			t.Errorf("%s.AddMonths(%d) = %s, %v; want %s", tt.from, tt.n, got, err, tt.want)
		}
	}
	for _, bad := range []struct {
		from Date
		n    int
	}{{date(t, 9999, 12, 1), 1}, {date(t, 1, 1, 1), -1}, {date(t, 2026, 1, 1), 1 << 40}, {date(t, 2026, 1, 1), -1 << 40}} {
		if _, err := bad.from.AddMonths(bad.n); !errors.Is(err, ErrInvalidDate) {
			t.Errorf("%s.AddMonths(%d) err = %v", bad.from, bad.n, err)
		}
	}
}

func TestParseDate(t *testing.T) {
	if got, err := ParseDate("2026-11-09"); err != nil || got != date(t, 2026, 11, 9) {
		t.Errorf("ParseDate = %v, %v", got, err)
	}
	for _, bad := range []string{"", "2026-1-09", "2026-11-9", "2026/11/09", "2026-02-30", "20261109", "2026-11-09x", "+026-11-09", "2026-11-0a", "0000-01-01"} {
		if _, err := ParseDate(bad); !errors.Is(err, ErrInvalidDate) {
			t.Errorf("ParseDate(%q) err = %v", bad, err)
		}
	}
}

func TestParseLocalDate(t *testing.T) {
	if got, err := ParseLocalDate("09/11/2026", LocaleITIT); err != nil || got != date(t, 2026, 11, 9) {
		t.Errorf("it-IT = %v, %v", got, err)
	}
	if got, err := ParseLocalDate("11/09/2026", LocaleENUS); err != nil || got != date(t, 2026, 11, 9) {
		t.Errorf("en-US = %v, %v", got, err)
	}
	if _, err := ParseLocalDate("13/09/2026", LocaleENUS); !errors.Is(err, ErrInvalidDate) {
		t.Errorf("month 13 err = %v", err)
	}
}

func TestParseRelativeDate(t *testing.T) {
	today := date(t, 2026, 3, 31)
	tests := []struct {
		in   string
		want Date
	}{
		{"today", today},
		{" Today ", today},
		{"yesterday", date(t, 2026, 3, 30)},
		{"-3d", date(t, 2026, 3, 28)},
		{"-0d", today},
		{"-2w", date(t, 2026, 3, 17)},
		{"-1m", date(t, 2026, 2, 28)},
		{"-12m", date(t, 2025, 3, 31)},
		{"-10D", date(t, 2026, 3, 21)},
	}
	for _, tt := range tests {
		if got, err := ParseRelativeDate(tt.in, today); err != nil || got != tt.want {
			t.Errorf("ParseRelativeDate(%q) = %v, %v; want %v", tt.in, got, err, tt.want)
		}
	}
	for _, bad := range []string{"", "tomorrow", "-d", "3d", "+3d", "-3", "-3x", "-3.5d", "--3d", "-9999999d", "-99999m", "-3999999d"} {
		if _, err := ParseRelativeDate(bad, today); !errors.Is(err, ErrInvalidDate) {
			t.Errorf("ParseRelativeDate(%q) err = %v", bad, err)
		}
	}
}

func TestParseDateInput(t *testing.T) {
	today := date(t, 2026, 11, 11)
	tests := []struct {
		in   string
		loc  Locale
		want Date
	}{
		{"2026-11-09", LocaleITIT, date(t, 2026, 11, 9)},
		{"09/11/2026", LocaleITIT, date(t, 2026, 11, 9)},
		{"11/09/2026", LocaleENUS, date(t, 2026, 11, 9)},
		{"yesterday", LocaleENUS, date(t, 2026, 11, 10)},
		{" -1w ", LocaleITIT, date(t, 2026, 11, 4)},
	}
	for _, tt := range tests {
		if got, err := ParseDateInput(tt.in, tt.loc, today); err != nil || got != tt.want {
			t.Errorf("ParseDateInput(%q, %s) = %v, %v; want %v", tt.in, tt.loc, got, err, tt.want)
		}
	}
	if _, err := ParseDateInput("soon", LocaleENUS, today); !errors.Is(err, ErrInvalidDate) {
		t.Errorf("err = %v", err)
	}
}

func TestTimeConversionAtTheEdges(t *testing.T) {
	loc := time.FixedZone("UTC+14", 14*3600) // Pacific/Kiritimati: guards against UTC-only assumptions
	d := date(t, 2026, 12, 31)
	tm := d.In(loc)
	if got, err := FromTime(tm); err != nil || got != d {
		t.Errorf("FromTime(In) = %v, %v", got, err)
	}
	// FromTime must use the instant's own zone: 23:30 on the 31st in UTC-10 is already Jan 1 in UTC.
	late := time.Date(2026, 12, 31, 23, 30, 0, 0, time.FixedZone("UTC-10", -10*3600))
	if got, _ := FromTime(late); got != d {
		t.Errorf("FromTime(late) = %v", got)
	}
	if _, err := FromTime(time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)); !errors.Is(err, ErrInvalidDate) {
		t.Errorf("year 10000 err = %v", err)
	}
}

func genDate(t *rapid.T) Date {
	y := rapid.IntRange(1, 9999).Draw(t, "year")
	m := rapid.IntRange(1, 12).Draw(t, "month")
	d := rapid.IntRange(1, daysIn(y, m)).Draw(t, "day")
	got, err := NewDate(y, m, d)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestDateRoundTripProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		d := genDate(t)
		if got, err := ParseDate(d.String()); err != nil || got != d {
			t.Fatalf("ISO round trip of %v = %v, %v", d, got, err)
		}
		for _, loc := range []Locale{LocaleITIT, LocaleENUS} {
			if got, err := ParseLocalDate(d.Format(loc), loc); err != nil || got != d {
				t.Fatalf("%s round trip of %v = %v, %v", loc, d, got, err)
			}
		}
	})
}

func TestAddDaysInverseProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		d := genDate(t)
		n := rapid.IntRange(-1000, 1000).Draw(t, "n")
		moved, err := d.AddDays(n)
		if err != nil {
			t.Skip("out of range")
		}
		back, err := moved.AddDays(-n)
		if err != nil || back != d {
			t.Fatalf("%v +%d -%d = %v, %v", d, n, n, back, err)
		}
		if (n > 0) != moved.After(d) && n != 0 {
			t.Fatalf("ordering broken: %v +%d = %v", d, n, moved)
		}
	})
}

func FuzzParseDateInput(f *testing.F) {
	for _, s := range []string{"2026-11-09", "09/11/2026", "today", "-3d", "-2w", "-1m", "", "-", "9999-12-31", "-999999m"} {
		f.Add(s, true)
		f.Add(s, false)
	}
	today, _ := NewDate(2026, 11, 11)
	f.Fuzz(func(t *testing.T, s string, italian bool) {
		loc := LocaleENUS
		if italian {
			loc = LocaleITIT
		}
		d, err := ParseDateInput(s, loc, today)
		if err != nil {
			var de *Error
			if !errors.As(err, &de) {
				t.Fatalf("non-domain error %T: %v", err, err)
			}
			return
		}
		if back, err := ParseDate(d.String()); err != nil || back != d {
			t.Fatalf("parse(%q) = %v does not survive ISO round trip: %v, %v", s, d, back, err)
		}
	})
}
