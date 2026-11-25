package domain

import (
	"errors"
	"testing"
)

func TestParseReportGroup(t *testing.T) {
	tests := []struct {
		in      string
		want    ReportGroup
		gran    Granularity
		isCycle bool
	}{
		{"category", GroupCategory, "", false},
		{" Tag ", GroupTag, "", false},
		{"payee", GroupPayee, "", false},
		{"ACCOUNT", GroupAccount, "", false},
		{"week", GroupWeek, GranWeek, true},
		{"month", GroupMonth, GranMonth, true},
		{"year", GroupYear, GranYear, true},
	}
	for _, tc := range tests {
		got, err := ParseReportGroup(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("ParseReportGroup(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
		if g, ok := got.Granularity(); g != tc.gran || ok != tc.isCycle {
			t.Errorf("%q.Granularity() = %q, %v", got, g, ok)
		}
	}
	_, err := ParseReportGroup("day")
	var de *Error
	if !errors.As(err, &de) || de.Code != CodeInvalidValue || de.Details["field"] != "group-by" {
		t.Errorf("ParseReportGroup(day) err = %v, want invalid_value on group-by", err)
	}
}
