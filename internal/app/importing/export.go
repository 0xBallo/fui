package importing

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/pkg/fuiproto"
)

// ExportFormat is the format of a transaction export.
type ExportFormat string

// Export formats.
const (
	FormatCSV  ExportFormat = "csv"
	FormatJSON ExportFormat = "json"
	FormatFUI  ExportFormat = "fui"
)

// exportVersion is written in the meta record of a FUI export.
const exportVersion = "0.1.0"

// noDescription replaces an empty description in a FUI export, where it is mandatory.
const noDescription = "-"

// ExportRequest selects the transactions to export with the same filter as `tx list`
// (already resolved to IDs by the caller).
type ExportRequest struct {
	Filter domain.TxFilter
	Format ExportFormat
}

// Export writes the matching transactions, oldest first, to w and returns how many it wrote.
// CSV is RFC 4180 with a header and dot decimals; JSON is an array; FUI is the fui/1 stream
// that `import` reads back.
func (s *Service) Export(ctx context.Context, req ExportRequest, w io.Writer) (int, error) {
	var data exportData
	err := s.store.Read(ctx, func(r app.Repos) error {
		var err error
		data, err = s.loadExport(ctx, r, req.Filter)
		return err
	})
	if err != nil {
		return 0, err
	}
	switch req.Format {
	case FormatCSV:
		err = data.writeCSV(w)
	case FormatJSON:
		err = data.writeJSON(w)
	case FormatFUI:
		err = data.writeFUI(w)
	default:
		return 0, newError(domain.ErrInvalidValue, map[string]any{"field": "format"}, "unknown export format %q", req.Format)
	}
	if err != nil {
		return 0, err
	}
	return len(data.txs), nil
}

type exportData struct {
	txs      []domain.Transaction // oldest first
	accounts map[domain.AccountID]domain.Account
	cats     map[domain.CategoryID]domain.Category
	balances map[domain.AccountID]domain.Money // final balance of the accounts involved
	balDate  domain.Date
}

func (s *Service) loadExport(ctx context.Context, r app.Repos, f domain.TxFilter) (exportData, error) {
	d := exportData{
		accounts: map[domain.AccountID]domain.Account{}, cats: map[domain.CategoryID]domain.Category{},
		balances: map[domain.AccountID]domain.Money{}, balDate: f.To(),
	}
	accts, err := r.Accounts().List(ctx)
	if err != nil {
		return d, fmt.Errorf("list accounts: %w", err)
	}
	for _, a := range accts {
		d.accounts[a.ID] = a
	}
	cats, err := r.Categories().List(ctx)
	if err != nil {
		return d, fmt.Errorf("list categories: %w", err)
	}
	for _, c := range cats {
		d.cats[c.ID] = c
	}
	if d.txs, err = r.Transactions().List(ctx, f, nil, 0); err != nil {
		return d, fmt.Errorf("list transactions: %w", err)
	}
	if d.balDate.IsZero() {
		if d.balDate, err = domain.FromTime(s.clock.Now()); err != nil {
			return d, fmt.Errorf("today: %w", err)
		}
	}
	for _, t := range d.txs {
		if _, done := d.balances[t.AccountID]; done {
			continue
		}
		a := d.accounts[t.AccountID]
		sum, err := r.Transactions().SumUpTo(ctx, a.ID, d.balDate)
		if err != nil {
			return d, fmt.Errorf("balance of %q: %w", a.Name, err)
		}
		if d.balances[a.ID], err = domain.NewMoney(a.OpeningBalance.Minor()+sum, a.Currency); err != nil {
			return d, fmt.Errorf("balance of %q: %w", a.Name, err)
		}
	}
	return d, nil
}

func (d exportData) category(t domain.Transaction) string { return d.cats[t.CategoryID].Name }

func (d exportData) writeCSV(w io.Writer) error {
	rows := [][]string{{
		"id", "date", "account", "amount", "currency", "description", "category", "status", "kind", "notes", "external_id",
	}}
	for _, t := range d.txs {
		rows = append(rows, []string{
			strconv.FormatInt(int64(t.ID), 10), t.Date.String(), d.accounts[t.AccountID].Name,
			domain.FormatMachine(t.Amount), t.Amount.Currency().Code(), t.Description, d.category(t),
			string(t.Status), string(t.Kind), t.Notes, t.ExternalID,
		})
	}
	return writeCSVRows(w, rows)
}

// writeCSVRows writes RFC 4180 records (CRLF line ends, quoting where needed).
func writeCSVRows(w io.Writer, rows [][]string) error {
	cw := csv.NewWriter(w)
	cw.UseCRLF = true
	if err := cw.WriteAll(rows); err != nil {
		return fmt.Errorf("write csv: %w", err)
	}
	return nil
}

type jsonTx struct {
	ID          int64  `json:"id"`
	Date        string `json:"date"`
	Account     string `json:"account"`
	Amount      string `json:"amount"`
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
	Description string `json:"description"`
	Category    string `json:"category,omitempty"`
	Status      string `json:"status"`
	Kind        string `json:"kind"`
	Notes       string `json:"notes,omitempty"`
	ExternalID  string `json:"external_id,omitempty"`
	Transfer    int64  `json:"transfer_group,omitempty"`
}

func (d exportData) writeJSON(w io.Writer) error {
	out := make([]jsonTx, len(d.txs))
	for i, t := range d.txs {
		out[i] = jsonTx{
			ID: int64(t.ID), Date: t.Date.String(), Account: d.accounts[t.AccountID].Name,
			Amount: domain.FormatMachine(t.Amount), AmountMinor: t.Amount.Minor(), Currency: t.Amount.Currency().Code(),
			Description: t.Description, Category: d.category(t), Status: string(t.Status), Kind: string(t.Kind),
			Notes: t.Notes, ExternalID: t.ExternalID, Transfer: int64(t.TransferGroupID),
		}
	}
	return encodeJSON(w, out)
}

func encodeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("write json: %w", err)
	}
	return nil
}

// accountRef is the ref of an account in a FUI export: its name, which is unique, so that
// importing the file back into the same archive finds the account without asking.
func accountRef(a domain.Account) string { return a.Name }

func protoKind(k domain.AccountKind) fuiproto.AccountKind {
	if k == domain.AccountCreditCard {
		return fuiproto.AccountKindCard
	}
	return fuiproto.AccountKind(k)
}

func (d exportData) writeFUI(w io.Writer) error {
	recs := []fuiproto.Record{{
		Type: fuiproto.RecordTypeMeta,
		Meta: &fuiproto.MetaRecord{Protocol: fuiproto.ProtocolFUI1, Connector: "fui", Version: exportVersion},
	}}
	var involved []domain.Account
	for _, t := range d.txs {
		if _, ok := d.balances[t.AccountID]; ok && !slices.ContainsFunc(involved, func(a domain.Account) bool { return a.ID == t.AccountID }) {
			involved = append(involved, d.accounts[t.AccountID])
		}
	}
	for _, a := range involved {
		recs = append(recs, fuiproto.Record{Type: fuiproto.RecordTypeAccount, Account: &fuiproto.AccountRecord{
			Ref: accountRef(a), Name: a.Name, Currency: a.Currency.Code(), Kind: protoKind(a.Kind),
		}})
	}
	for _, t := range d.txs {
		recs = append(recs, fuiproto.Record{Type: fuiproto.RecordTypeTransaction, Transaction: d.fuiTx(t)})
	}
	for _, a := range involved {
		recs = append(recs, fuiproto.Record{Type: fuiproto.RecordTypeBalance, Balance: &fuiproto.BalanceRecord{
			Account: accountRef(a), Date: d.balDate.String(), Amount: d.balances[a.ID].Minor(),
		}})
	}
	recs = append(recs, fuiproto.Record{Type: fuiproto.RecordTypeEnd, End: &fuiproto.EndRecord{Transactions: len(d.txs)}})
	if err := fuiproto.EncodeSliceJSONL(w, recs); err != nil {
		return fmt.Errorf("write fui stream: %w", err)
	}
	return nil
}

// fuiTx converts a transaction keeping its external id, or giving it fui-<id>; reconciled
// movements are exported as booked.
func (d exportData) fuiTx(t domain.Transaction) *fuiproto.TransactionRecord {
	rec := &fuiproto.TransactionRecord{
		Account: accountRef(d.accounts[t.AccountID]), ID: t.ExternalID, Date: t.Date.String(),
		Amount: t.Amount.Minor(), Currency: t.Amount.Currency().Code(), Description: orDefault(t.Description, noDescription),
		Status: fuiproto.TransactionStatus(t.Status), Kind: fuiproto.TransactionKind(t.Kind),
		Category: d.category(t), Notes: t.Notes,
	}
	if rec.ID == "" {
		rec.ID = "fui-" + strconv.FormatInt(int64(t.ID), 10)
	}
	if t.Status == domain.StatusReconciled {
		rec.Status = fuiproto.TransactionStatusBooked
	}
	if !t.OperationDate.IsZero() {
		rec.OperationDate = t.OperationDate.String()
	}
	if !t.ValueDate.IsZero() {
		rec.ValueDate = t.ValueDate.String()
	}
	if t.TransferGroupID != 0 {
		rec.Transfer = "fui-t" + strconv.FormatInt(int64(t.TransferGroupID), 10)
	}
	return rec
}
