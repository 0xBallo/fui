package report

import (
	"context"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
)

// Values of the export form's "What" field.
const (
	exportResult       = "result"
	exportTransactions = "transactions"
)

// submitExport writes what the export form asks for to the entered path: the displayed report
// result (csv or json, through the ExportResult port) or the transactions behind it (csv, json
// or fui, through Export).
func (m *Model) submitExport() tea.Cmd {
	m.mode = modeReport
	what := m.fields.what.Value()
	if what == "" {
		what = exportResult
	}
	path := m.fields.path.Value()
	format := m.fields.format.Value()
	if format == "" {
		format = "csv"
	}
	if path == "" {
		return errCmd(&domain.Error{Code: domain.CodeMissingField, Message: "output path is required", Details: map[string]any{"field": "path"}})
	}
	switch what {
	case exportResult:
		return m.exportResultCmd(path, format)
	case exportTransactions:
		return m.exportTransactionsCmd(path, format)
	}
	return errCmd(&domain.Error{
		Code: domain.CodeInvalidValue, Message: "unknown export target " + what + " (use result or transactions)",
		Details: map[string]any{"field": "what", "value": what},
	})
}

func (m *Model) exportResultCmd(path, format string) tea.Cmd {
	if format != "csv" && format != "json" {
		return errCmd(&domain.Error{
			Code: domain.CodeInvalidValue, Message: "unknown export format " + format + " (use csv or json)",
			Details: map[string]any{"field": "format", "value": format},
		})
	}
	data := m.result.data
	if data == nil {
		return errCmd(&domain.Error{Code: domain.CodeInvalidValue, Message: "no report to export"})
	}
	return writeFile(path, func(w io.Writer) (int, error) {
		return -1, m.uc.ExportResult(w, format, data)
	})
}

// exportTransactionsCmd exports the transactions of the displayed report: its period and account
// narrowed by the active filter.
func (m *Model) exportTransactionsCmd(path, format string) tea.Cmd {
	f := importing.ExportFormat(format)
	if f != importing.FormatCSV && f != importing.FormatJSON && f != importing.FormatFUI {
		return errCmd(&domain.Error{
			Code: domain.CodeInvalidValue, Message: "unknown export format " + format + " (use csv, json or fui)",
			Details: map[string]any{"field": "format", "value": format},
		})
	}
	period, err := m.resolvePeriod(m.periodText)
	if err != nil {
		return errCmd(err)
	}
	req := m.result.filter
	if !period.From().IsZero() && !period.To().IsZero() {
		req.From, req.To = period.From().String(), period.To().String()
	}
	if len(req.Accounts) == 0 && m.accountText != "" {
		req.Accounts = []string{m.accountText}
	}
	return func() tea.Msg {
		ctx := context.Background()
		filter, err := m.uc.ResolveTxFilter(ctx, req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return writeFile(path, func(w io.Writer) (int, error) {
			return m.uc.Export(ctx, importing.ExportRequest{Filter: filter, Format: f}, w)
		})()
	}
}

// writeFile creates path, runs write on it and reports the outcome as a message.
func writeFile(path string, write func(io.Writer) (int, error)) tea.Cmd {
	return func() tea.Msg {
		f, err := os.Create(path) // #nosec G304 -- path is provided by the user through the TUI form.
		if err != nil {
			return screens.ErrorMsg{Err: fmt.Errorf("create output file: %w", err)}
		}
		n, err := write(f)
		if err != nil {
			_ = f.Close()
			return screens.ErrorMsg{Err: err}
		}
		if err := f.Close(); err != nil {
			return screens.ErrorMsg{Err: fmt.Errorf("close output file: %w", err)}
		}
		return exportDoneMsg{path: path, count: n}
	}
}
