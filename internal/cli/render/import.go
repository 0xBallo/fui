package render

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
)

// NewAccountDTO is the --json shape of an account the import plans to create.
type NewAccountDTO struct {
	Ref      string `json:"ref"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Currency string `json:"currency"`
}

// BindingDTO is the --json shape of a connector ref bound to an existing account.
type BindingDTO struct {
	Ref       string `json:"ref"`
	AccountID int64  `json:"account_id"`
}

// UnmappedRefDTO is the --json shape of a connector ref still bound to no account.
type UnmappedRefDTO struct {
	Ref        string `json:"ref"`
	Name       string `json:"name"`
	Currency   string `json:"currency"`
	Kind       string `json:"kind"`
	Suggestion string `json:"suggestion,omitempty"`
}

// TransferProposalDTO is the --json shape of one proposed transfer link.
type TransferProposalDTO struct {
	Row          int    `json:"row"`
	Other        int    `json:"other,omitempty"`
	ExistingTxID int64  `json:"existing_transaction_id,omitempty"`
	Reason       string `json:"reason"`
	Accept       bool   `json:"accept"`
}

// BalanceDiscrepancyDTO is the --json shape of one movement whose running balance does not match.
type BalanceDiscrepancyDTO struct {
	AccountRef string `json:"account_ref"`
	Line       int    `json:"line"`
	Date       string `json:"date"`
	Expected   int64  `json:"expected"`
	Actual     int64  `json:"actual"`
}

// ImportCountsDTO is the --json shape of the row tally of an import plan.
type ImportCountsDTO struct {
	New       int `json:"new"`
	Duplicate int `json:"duplicate"`
	Doubtful  int `json:"doubtful"`
	Errors    int `json:"errors"`
	Included  int `json:"included"`
}

// ImportPlanDTO is the --json shape of `import`'s dry-run and confirmation preview.
type ImportPlanDTO struct {
	Connector       string                  `json:"connector"`
	Source          string                  `json:"source"`
	SourceSHA256    string                  `json:"source_sha256,omitempty"`
	AlreadyImported *int64                  `json:"already_imported_batch_id,omitempty"`
	Counts          ImportCountsDTO         `json:"counts"`
	NewAccounts     []NewAccountDTO         `json:"new_accounts,omitempty"`
	Bindings        []BindingDTO            `json:"bindings,omitempty"`
	Unmapped        []UnmappedRefDTO        `json:"unmapped,omitempty"`
	NewCategories   []CategoryDTO           `json:"new_categories,omitempty"`
	Errors          []RowErrorDTO           `json:"errors,omitempty"`
	Transfers       []TransferProposalDTO   `json:"transfers,omitempty"`
	Discrepancies   []BalanceDiscrepancyDTO `json:"discrepancies,omitempty"`
	NewPayees       []string                `json:"new_payees,omitempty"`
	NewTags         []string                `json:"new_tags,omitempty"`
	Warnings        []RowWarningDTO         `json:"warnings,omitempty"`
	BalanceMismatch []BalanceMismatchDTO    `json:"balance_mismatches,omitempty"`
	ModuleRecords   []ModuleRecordsDTO      `json:"module_records,omitempty"`
	Bulk            bool                    `json:"bulk"`
}

// RowWarningDTO is the --json shape of a non-fatal warning tied to a source line.
type RowWarningDTO struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// BalanceMismatchDTO is the --json shape of a stated balance the computed one contradicts.
type BalanceMismatchDTO struct {
	AccountRef string `json:"account_ref"`
	AccountID  int64  `json:"account_id"`
	Date       string `json:"date"`
	Expected   int64  `json:"expected"`
	Computed   int64  `json:"computed"`
	Suggest    string `json:"suggest,omitempty"`
}

// ModuleRecordsDTO is the --json shape of the records of one module that is not enabled.
type ModuleRecordsDTO struct {
	Module string   `json:"module"`
	Count  int      `json:"count"`
	Kinds  []string `json:"kinds,omitempty"`
	Lines  []int    `json:"lines,omitempty"`
}

func importPlanDTO(p importing.ImportPlan) ImportPlanDTO {
	n := p.Counts()
	out := ImportPlanDTO{
		Connector: p.Connector, Source: p.Source, SourceSHA256: p.SourceSHA256, Bulk: p.Bulk,
		Counts: ImportCountsDTO{New: n.New, Duplicate: n.Duplicate, Doubtful: n.Doubtful, Errors: n.Errors, Included: n.Included},
	}
	if p.AlreadyImported != nil {
		id := int64(p.AlreadyImported.ID)
		out.AlreadyImported = &id
	}
	for _, a := range p.NewAccounts {
		out.NewAccounts = append(out.NewAccounts,
			NewAccountDTO{Ref: a.Ref, Name: a.Account.Name, Kind: string(a.Account.Kind), Currency: a.Account.Currency.Code()})
	}
	for _, b := range p.Bindings {
		out.Bindings = append(out.Bindings, BindingDTO{Ref: b.Ref, AccountID: int64(b.AccountID)})
	}
	for _, u := range p.Unmapped {
		out.Unmapped = append(out.Unmapped, UnmappedRefDTO{Ref: u.Ref, Name: u.Name, Currency: u.Currency, Kind: u.Kind, Suggestion: u.Suggestion})
	}
	for _, c := range p.NewCategories {
		out.NewCategories = append(out.NewCategories, categoryDTO(c))
	}
	for _, e := range p.Errors {
		out.Errors = append(out.Errors, RowErrorDTO{Line: e.Line, Message: e.Message})
	}
	for _, t := range p.Transfers {
		out.Transfers = append(out.Transfers,
			TransferProposalDTO{Row: t.Row, Other: t.Other, ExistingTxID: int64(t.Existing), Reason: string(t.Reason), Accept: t.Accept})
	}
	for _, d := range p.Discrepancies {
		out.Discrepancies = append(out.Discrepancies,
			BalanceDiscrepancyDTO{AccountRef: d.AccountRef, Line: d.Line, Date: d.Date.String(), Expected: d.Expected, Actual: d.Actual})
	}
	for _, p := range p.NewPayees {
		out.NewPayees = append(out.NewPayees, p.Name)
	}
	for _, t := range p.NewTags {
		out.NewTags = append(out.NewTags, t.Name)
	}
	for _, w := range p.Warnings {
		out.Warnings = append(out.Warnings, RowWarningDTO{Line: w.Line, Message: w.Message})
	}
	for _, m := range p.BalanceMismatches {
		out.BalanceMismatch = append(out.BalanceMismatch, BalanceMismatchDTO{
			AccountRef: m.AccountRef, AccountID: int64(m.AccountID), Date: m.Date.String(),
			Expected: m.Expected, Computed: m.Computed, Suggest: m.Suggest,
		})
	}
	for _, m := range p.ModuleRecords {
		out.ModuleRecords = append(out.ModuleRecords,
			ModuleRecordsDTO{Module: string(m.Module), Count: m.Count, Kinds: m.Kinds, Lines: m.Lines})
	}
	return out
}

func importSummaryRows(p importing.ImportPlan) Rows {
	n := p.Counts()
	t := Rows{Header: []string{"FIELD", "VALUE"}}
	t.Body = append(t.Body,
		[]string{"connector", p.Connector}, []string{"source", p.Source},
		[]string{"new", strconv.Itoa(n.New)}, []string{"duplicate", strconv.Itoa(n.Duplicate)},
		[]string{"doubtful", strconv.Itoa(n.Doubtful)}, []string{"errors", strconv.Itoa(n.Errors)},
		[]string{"included", strconv.Itoa(n.Included)}, []string{"new_accounts", strconv.Itoa(len(p.NewAccounts))},
		[]string{"new_categories", strconv.Itoa(len(p.NewCategories))}, []string{"unmapped", strconv.Itoa(len(p.Unmapped))},
		[]string{"transfer_proposals", strconv.Itoa(len(p.Transfers))}, []string{"discrepancies", strconv.Itoa(len(p.Discrepancies))},
	)
	if p.AlreadyImported != nil {
		t.Body = append(t.Body, []string{"already_imported_batch_id", strconv.FormatInt(int64(p.AlreadyImported.ID), 10)})
	}
	return t
}

// WriteImportPlan renders the preview of `import`: --json is the full ImportPlanDTO; the table
// and CSV formats show the summary counts, then one line per unmapped ref (the detail the user
// most needs to act on before importing again).
func WriteImportPlan(w io.Writer, f Format, p importing.ImportPlan) error {
	if f == JSON {
		return WriteJSON(w, importPlanDTO(p))
	}
	t := importSummaryRows(p)
	var err error
	if f == CSV {
		err = WriteCSV(w, t)
	} else {
		err = WriteTable(w, t, false)
	}
	if err != nil {
		return err
	}
	if err := writeImportDetails(w, p); err != nil {
		return err
	}
	if p.Bulk {
		if _, werr := fmt.Fprintln(w, "This is a bulk change: it needs --yes or a confirmation."); werr != nil {
			return fmt.Errorf("write plan note: %w", werr)
		}
	}
	return nil
}

// importDetailLines lists what the user should review before confirming: one line per item,
// grouped by kind and prefixed by it so that the output stays greppable.
func importDetailLines(p importing.ImportPlan) []string {
	var l []string
	for _, u := range p.Unmapped {
		l = append(l, fmt.Sprintf("unmapped: %s (%s, %s)", u.Ref, u.Name, u.Currency))
	}
	for _, x := range p.Warnings {
		l = append(l, fmt.Sprintf("warning: line %d: %s", x.Line, x.Message))
	}
	for _, c := range p.NewCategories {
		l = append(l, "new category: "+c.Name)
	}
	for _, t := range p.NewTags {
		l = append(l, "new tag: "+t.Name)
	}
	for _, x := range p.NewPayees {
		l = append(l, "new payee: "+x.Name)
	}
	for _, e := range p.Errors {
		l = append(l, fmt.Sprintf("error: line %d: %s", e.Line, e.Message))
	}
	for _, d := range p.Discrepancies {
		l = append(l, fmt.Sprintf("discrepancy: line %d: %s %s balance %d, expected %d",
			d.Line, d.AccountRef, d.Date, d.Actual, d.Expected))
	}
	for i, t := range p.Transfers {
		l = append(l, "transfer proposal: "+TransferProposalText(i, t))
	}
	for _, m := range p.BalanceMismatches {
		line := fmt.Sprintf("balance mismatch: %s on %s: statement %d, computed %d", m.AccountRef, m.Date, m.Expected, m.Computed)
		if m.Suggest != "" {
			line += "; record it with: " + m.Suggest
		}
		l = append(l, line)
	}
	for _, m := range p.ModuleRecords {
		l = append(l, fmt.Sprintf("module %s is disabled: %d record(s) (%s) skipped; use --enable-modules %s",
			m.Module, m.Count, strings.Join(m.Kinds, ", "), m.Module))
	}
	return l
}

// TransferProposalText describes proposal i in one line, shared by the preview and the prompt.
func TransferProposalText(i int, t importing.TransferProposal) string {
	other := fmt.Sprintf("row %d", t.Other)
	if t.Other < 0 {
		other = fmt.Sprintf("existing transaction %d", int64(t.Existing))
	}
	state := "not accepted"
	if t.Accept {
		state = "accepted"
	}
	return fmt.Sprintf("#%d row %d <-> %s (%s, %s)", i, t.Row, other, t.Reason, state)
}

func writeImportDetails(w io.Writer, p importing.ImportPlan) error {
	for _, line := range importDetailLines(p) {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return fmt.Errorf("write import detail: %w", err)
		}
	}
	return nil
}

// ImportResultDTO is the --json shape of what `import` wrote.
type ImportResultDTO struct {
	BatchID      int64  `json:"batch_id,omitempty"`
	Transactions int    `json:"transactions"`
	Accounts     int    `json:"accounts"`
	Categories   int    `json:"categories"`
	Transfers    int    `json:"transfers"`
	BackupPath   string `json:"backup_path,omitempty"`
	// Warnings lists what the apply skipped, e.g. records of modules left disabled.
	Warnings []RowWarningDTO `json:"warnings,omitempty"`
}

func importResultDTO(res importing.ImportResult) ImportResultDTO {
	dto := ImportResultDTO{
		Transactions: res.Transactions, Accounts: res.Accounts, Categories: res.Categories,
		Transfers: res.Transfers, BackupPath: res.BackupPath,
	}
	for _, w := range res.Warnings {
		dto.Warnings = append(dto.Warnings, RowWarningDTO{Line: w.Line, Message: w.Message})
	}
	if res.Batch.ID != 0 {
		dto.BatchID = int64(res.Batch.ID)
	}
	return dto
}

// WriteImportResult renders what ApplyImport wrote.
func WriteImportResult(w io.Writer, f Format, res importing.ImportResult) error {
	dto := importResultDTO(res)
	if f == JSON {
		return WriteJSON(w, dto)
	}
	t := Rows{Header: []string{"FIELD", "VALUE"}}
	t.Body = append(t.Body,
		[]string{"batch_id", strconv.FormatInt(dto.BatchID, 10)}, []string{"transactions", strconv.Itoa(res.Transactions)},
		[]string{"accounts", strconv.Itoa(res.Accounts)}, []string{"categories", strconv.Itoa(res.Categories)},
		[]string{"transfers", strconv.Itoa(res.Transfers)},
	)
	var err error
	if f == CSV {
		err = WriteCSV(w, t)
	} else {
		err = WriteTable(w, t, false)
	}
	if err != nil {
		return err
	}
	for _, x := range res.Warnings {
		if _, werr := fmt.Fprintf(w, "warning: line %d: %s\n", x.Line, x.Message); werr != nil {
			return fmt.Errorf("write import warning: %w", werr)
		}
	}
	return nil
}

// ImportBatchDTO is the --json shape of one import batch.
type ImportBatchDTO struct {
	ID           int64  `json:"id"`
	Connector    string `json:"connector"`
	Source       string `json:"source"`
	SourceSHA256 string `json:"source_sha256,omitempty"`
	ImportedAt   string `json:"imported_at"`
}

func importBatchDTO(b app.ImportBatch) ImportBatchDTO {
	return ImportBatchDTO{
		ID: int64(b.ID), Connector: b.Connector, Source: b.Source, SourceSHA256: b.SourceSHA256,
		ImportedAt: b.ImportedAt.UTC().Format(time.RFC3339),
	}
}

func importBatchRows(bs []app.ImportBatch) Rows {
	t := Rows{Header: []string{"ID", "CONNECTOR", "SOURCE", "IMPORTED_AT"}}
	for _, b := range bs {
		t.Body = append(t.Body, []string{strconv.FormatInt(int64(b.ID), 10), b.Connector, b.Source, b.ImportedAt.UTC().Format(time.RFC3339)})
	}
	return t
}

// WriteImportBatchList renders the batches of `import list`.
func WriteImportBatchList(w io.Writer, f Format, bs []app.ImportBatch) error {
	if f == JSON {
		dtos := make([]ImportBatchDTO, len(bs))
		for i, b := range bs {
			dtos[i] = importBatchDTO(b)
		}
		return WriteJSON(w, dtos)
	}
	if f == CSV {
		return WriteCSV(w, importBatchRows(bs))
	}
	return WriteTable(w, importBatchRows(bs), false)
}
