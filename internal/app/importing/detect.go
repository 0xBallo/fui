package importing

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/domain"
)

// Column roles a CSV column can be guessed to play.
const (
	RoleDate        = "date"
	RoleAmount      = "amount"
	RoleDebit       = "debit"
	RoleCredit      = "credit"
	RoleDescription = "description"
	RoleID          = "id"
	RoleBalance     = "balance"
	RoleOther       = "other"
)

// Confidence is how sure a guess is, from 0 to 100. It is an integer so that suggestions are
// exactly comparable and serialize the same everywhere.
type Confidence int

// ColumnGuess is the role guessed for one CSV column.
type ColumnGuess struct {
	Header     string     `json:"header"`
	Role       string     `json:"role"`
	Confidence Confidence `json:"confidence"`
}

// CSVSuggestion is what the detection of a CSV file found: a profile ready to save, the facts
// behind it and the first rows to show the user.
type CSVSuggestion struct {
	// Profile is the suggested import profile; its TargetAccount is left empty.
	Profile    config.CSVProfile `json:"profile"`
	Delimiter  rune              `json:"delimiter"`
	DateFormat string            `json:"date_format"`
	DecimalSep rune              `json:"decimal_sep"`
	// HasHeader is false when the first row looks like data; a profile needs a header row to
	// name the columns, so the Columns are then named "column N".
	HasHeader bool          `json:"has_header"`
	Columns   []ColumnGuess `json:"columns"`
	// Sample is the first data rows, as read with the suggested delimiter.
	Sample [][]string `json:"sample"`
	// Warnings lists what the user should check: missing columns, rows that fail to parse.
	Warnings []string `json:"warnings"`
}

// CSVDetector inspects a CSV file and suggests how to import it.
type CSVDetector interface {
	DetectCSV(ctx context.Context, file string) (CSVSuggestion, error)
}

// WithCSVDetector returns a copy of the service that detects CSV profiles with d.
func (s *Service) WithCSVDetector(d CSVDetector) *Service {
	c := *s
	c.detector = d
	return &c
}

// DetectCSV suggests the import profile of a CSV file. It only reads the file: nothing is
// stored, so the caller decides whether to save the profile.
func (s *Service) DetectCSV(ctx context.Context, file string) (CSVSuggestion, error) {
	if s.detector == nil {
		return CSVSuggestion{}, newError(domain.ErrMissingField, map[string]any{"field": "csv detector"}, "no csv detector configured")
	}
	if file == "" {
		return CSVSuggestion{}, newError(domain.ErrMissingField, map[string]any{"field": "file"}, "no file given")
	}
	sug, err := s.detector.DetectCSV(ctx, file)
	if err != nil {
		return CSVSuggestion{}, fmt.Errorf("detect csv %s: %w", file, err)
	}
	return sug, nil
}
