package domain

import "strings"

// ReportGroup is the dimension a spending or cash-flow report is grouped by.
type ReportGroup string

// Supported report groupings.
const (
	GroupCategory ReportGroup = "category"
	GroupTag      ReportGroup = "tag"
	GroupPayee    ReportGroup = "payee"
	GroupAccount  ReportGroup = "account"
	GroupWeek     ReportGroup = "week"
	GroupMonth    ReportGroup = "month"
	GroupYear     ReportGroup = "year"
)

// ParseReportGroup parses s case-insensitively; an unknown value fails with invalid_value on the
// group-by field.
func ParseReportGroup(s string) (ReportGroup, error) {
	g := ReportGroup(strings.ToLower(strings.TrimSpace(s)))
	switch g {
	case GroupCategory, GroupTag, GroupPayee, GroupAccount, GroupWeek, GroupMonth, GroupYear:
		return g, nil
	}
	return "", invalidValue("group-by", s)
}

// Granularity returns the time bucket of a period grouping, false for the other groupings.
func (g ReportGroup) Granularity() (Granularity, bool) {
	switch g {
	case GroupWeek:
		return GranWeek, true
	case GroupMonth:
		return GranMonth, true
	case GroupYear:
		return GranYear, true
	case GroupCategory, GroupTag, GroupPayee, GroupAccount:
	}
	return "", false
}
