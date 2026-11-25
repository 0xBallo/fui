package render

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/0xballo/fui/internal/app/importing"
)

// CSVProfileDTO is the --json shape of a suggested CSV import profile.
type CSVProfileDTO struct {
	DateColumn        string `json:"date_column"`
	AmountColumn      string `json:"amount_column,omitempty"`
	DebitColumn       string `json:"debit_column,omitempty"`
	CreditColumn      string `json:"credit_column,omitempty"`
	DescriptionColumn string `json:"description_column"`
	ExternalIDColumn  string `json:"external_id_column,omitempty"`
	Delimiter         string `json:"delimiter"`
	Encoding          string `json:"encoding,omitempty"`
	DateFormat        string `json:"date_format"`
	DecimalSeparator  string `json:"decimal_separator"`
	SkipRows          int    `json:"skip_rows,omitempty"`
}

// ColumnGuessDTO is the --json shape of the role guessed for one column.
type ColumnGuessDTO struct {
	Header     string `json:"header"`
	Role       string `json:"role"`
	Confidence int    `json:"confidence"`
}

// CSVSuggestionDTO is the --json shape of `import detect`.
type CSVSuggestionDTO struct {
	Profile   CSVProfileDTO    `json:"profile"`
	HasHeader bool             `json:"has_header"`
	Columns   []ColumnGuessDTO `json:"columns"`
	Sample    [][]string       `json:"sample"`
	Warnings  []string         `json:"warnings"`
	SavedAs   string           `json:"saved_as,omitempty"`
}

// CSVSuggestionOf converts a detection result to its --json shape; saved is the profile name it
// was stored under, empty when it was not saved.
func CSVSuggestionOf(s importing.CSVSuggestion, saved string) CSVSuggestionDTO {
	p := s.Profile
	dto := CSVSuggestionDTO{
		Profile: CSVProfileDTO{
			DateColumn: p.DateColumn, AmountColumn: p.AmountColumn, DebitColumn: p.DebitColumn,
			CreditColumn: p.CreditColumn, DescriptionColumn: p.DescriptionColumn, ExternalIDColumn: p.ExternalIDColumn,
			Delimiter: p.Delimiter, Encoding: p.Encoding, DateFormat: p.DateFormat,
			DecimalSeparator: p.DecimalSeparator, SkipRows: p.SkipRows,
		},
		HasHeader: s.HasHeader, Sample: s.Sample, Warnings: s.Warnings, SavedAs: saved,
		Columns: make([]ColumnGuessDTO, len(s.Columns)),
	}
	if dto.Sample == nil {
		dto.Sample = [][]string{}
	}
	if dto.Warnings == nil {
		dto.Warnings = []string{}
	}
	for i, c := range s.Columns {
		dto.Columns[i] = ColumnGuessDTO{Header: c.Header, Role: c.Role, Confidence: int(c.Confidence)}
	}
	return dto
}

// WriteCSVSuggestion renders the profile `import detect` suggests: --json is the full
// CSVSuggestionDTO, the other formats the profile fields, the guessed columns and the warnings.
func WriteCSVSuggestion(w io.Writer, f Format, s importing.CSVSuggestion, saved string) error {
	dto := CSVSuggestionOf(s, saved)
	if f == JSON {
		return WriteJSON(w, dto)
	}
	p := dto.Profile
	fields := Rows{Header: []string{"FIELD", "VALUE"}}
	for _, kv := range [][2]string{
		{"date_column", p.DateColumn},
		{"amount_column", p.AmountColumn},
		{"debit_column", p.DebitColumn},
		{"credit_column", p.CreditColumn},
		{"description_column", p.DescriptionColumn},
		{"external_id_column", p.ExternalIDColumn},
		{"delimiter", p.Delimiter},
		{"encoding", p.Encoding},
		{"date_format", p.DateFormat},
		{"decimal_separator", p.DecimalSeparator},
	} {
		if kv[1] != "" {
			fields.Body = append(fields.Body, []string{kv[0], kv[1]})
		}
	}
	if p.SkipRows > 0 {
		fields.Body = append(fields.Body, []string{"skip_rows", strconv.Itoa(p.SkipRows)})
	}
	if f == CSV {
		return WriteCSV(w, fields)
	}
	if err := WriteTable(w, fields, false); err != nil {
		return err
	}
	cols := Rows{Header: []string{"COLUMN", "ROLE", "CONFIDENCE"}}
	for _, c := range dto.Columns {
		cols.Body = append(cols.Body, []string{c.Header, c.Role, strconv.Itoa(c.Confidence)})
	}
	if err := WriteTable(w, cols, false); err != nil {
		return err
	}
	var b strings.Builder
	for _, x := range dto.Warnings {
		fmt.Fprintf(&b, "warning: %s\n", x)
	}
	if saved != "" {
		fmt.Fprintf(&b, "Saved as import.profiles.%s.\n", saved)
	}
	if _, err := io.WriteString(w, b.String()); err != nil {
		return fmt.Errorf("write detection notes: %w", err)
	}
	return nil
}
