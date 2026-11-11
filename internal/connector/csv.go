package connector

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"iter"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/pkg/fuiproto"
)

const (
	csvConnector = "csv"
	csvVersion   = "0.1.0"
)

// CSVSource is the built-in CSV connector: it reads a bank statement as the profile describes
// it and emits the fui/1 stream of one account. When the profile names no external id column,
// ids are a hash of the row, so a row gets the same id in overlapping exports.
type CSVSource struct {
	Profile config.CSVProfile
	File    string
	// Account is the ref and name of the account the rows belong to; Profile.TargetAccount when empty.
	Account string
	// Currency is the ISO 4217 code of the account; CSV files carry none.
	Currency string
	// Kind is the account kind, checking when empty.
	Kind fuiproto.AccountKind
	// Since, as YYYY-MM-DD, skips the rows dated before it.
	Since string
}

var _ importing.Source = CSVSource{}

// Records implements importing.Source.
func (c CSVSource) Records(ctx context.Context) iter.Seq2[fuiproto.Record, error] {
	return func(yield func(fuiproto.Record, error) bool) {
		data, err := readCapped(c.File)
		if err != nil {
			yield(fuiproto.Record{}, err)
			return
		}
		sum := sha256.Sum256(data)
		for rec, err := range csvRecords(ctx, data, csvOptions{
			profile: c.Profile, account: c.Account, currency: c.Currency, kind: c.Kind, since: c.Since,
			file: filepath.Base(c.File), sha: hex.EncodeToString(sum[:]),
		}) {
			if !yield(rec, err) {
				return
			}
		}
	}
}

type csvOptions struct {
	profile  config.CSVProfile
	account  string
	currency string
	kind     fuiproto.AccountKind
	since    string
	file     string
	sha      string
}

// columns holds the header positions of the profile columns; -1 when the column is not used.
type columns struct {
	date, amount, debit, credit, description, id int
}

// csvConv converts the rows of one file.
type csvConv struct {
	cols   columns
	cur    domain.Currency
	loc    domain.Locale
	layout string
	since  domain.Date
	ref    string
	seen   map[string]int // occurrences of each row hash, to keep ids of identical rows distinct
	header []string
	skip   int
	txs    int
	errs   int
}

func csvRecords(ctx context.Context, data []byte, o csvOptions) iter.Seq2[fuiproto.Record, error] {
	return func(yield func(fuiproto.Record, error) bool) {
		conv, rows, lines, err := prepareCSV(data, o)
		if err != nil {
			yield(fuiproto.Record{}, err)
			return
		}
		kind := o.kind
		if kind == "" {
			kind = fuiproto.AccountKindChecking
		}
		head := []fuiproto.Record{
			{Type: fuiproto.RecordTypeMeta, Meta: &fuiproto.MetaRecord{
				Protocol: fuiproto.ProtocolFUI1, Connector: csvConnector, Version: csvVersion,
				Source: fuiproto.SourceMeta{File: o.file, SHA256: o.sha},
			}},
			{Type: fuiproto.RecordTypeAccount, Account: &fuiproto.AccountRecord{
				Ref: conv.ref, Name: conv.ref, Currency: conv.cur.Code(), Kind: kind,
			}},
		}
		for _, r := range head {
			if !yield(r, nil) {
				return
			}
		}
		for i, row := range rows {
			if err := ctx.Err(); err != nil {
				yield(fuiproto.Record{}, fmt.Errorf("read csv: %w", err))
				return
			}
			rec, ok := conv.row(row, lines[i])
			if ok && !yield(rec, nil) {
				return
			}
		}
		yield(fuiproto.Record{Type: fuiproto.RecordTypeEnd, End: &fuiproto.EndRecord{Transactions: conv.txs, Errors: conv.errs}}, nil)
	}
}

// prepareCSV validates the options and splits the data into the header and the data rows.
func prepareCSV(data []byte, o csvOptions) (*csvConv, [][]string, []int, error) {
	conv := &csvConv{seen: map[string]int{}, skip: o.profile.SkipRows, ref: strings.TrimSpace(o.account)}
	if conv.ref == "" {
		conv.ref = strings.TrimSpace(o.profile.TargetAccount)
	}
	if conv.ref == "" {
		return nil, nil, nil, errors.New("no target account: set target_account in the profile or name the account")
	}
	var err error
	if conv.cur, err = domain.ParseCurrency(o.currency); err != nil {
		return nil, nil, nil, fmt.Errorf("currency: %w", err)
	}
	if o.since != "" {
		if conv.since, err = domain.ParseDate(o.since); err != nil {
			return nil, nil, nil, fmt.Errorf("since: %w", err)
		}
	}
	conv.layout = goLayout(o.profile.DateFormat)
	conv.loc = domain.LocaleENUS
	if strings.TrimSpace(o.profile.DecimalSeparator) == "," {
		conv.loc = domain.LocaleITIT
	}
	text, err := toUTF8(data, o.profile.Encoding)
	if err != nil {
		return nil, nil, nil, err
	}
	rows, lines, err := readRows(text, o.profile)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(rows) == 0 {
		return nil, nil, nil, errors.New("csv has no header row")
	}
	conv.header = rows[0]
	if conv.cols, err = findColumns(rows[0], o.profile); err != nil {
		return nil, nil, nil, err
	}
	return conv, rows[1:], lines[1:], nil
}

// readRows skips the profile's leading lines and reads every record with its line number.
func readRows(text string, p config.CSVProfile) ([][]string, []int, error) {
	skip := p.SkipRows
	for ; skip > 0; skip-- {
		i := strings.IndexByte(text, '\n')
		if i < 0 {
			return nil, nil, errors.New("csv has fewer lines than skip_rows")
		}
		text = text[i+1:]
	}
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = delimiter(p.Delimiter)
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	r.TrimLeadingSpace = true
	var rows [][]string
	var lines []int
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			return rows, lines, nil
		}
		if err != nil {
			return nil, nil, fmt.Errorf("read csv: %w", err)
		}
		line, _ := r.FieldPos(0)
		rows, lines = append(rows, rec), append(lines, line+p.SkipRows)
	}
}

func delimiter(s string) rune {
	switch strings.ToLower(s) {
	case "", ",":
		return ','
	case `\t`, "tab":
		return '\t'
	}
	if r := []rune(s); len(r) == 1 {
		return r[0]
	}
	return ','
}

func findColumns(header []string, p config.CSVProfile) (columns, error) {
	find := func(name string, required bool) (int, error) {
		name = strings.TrimSpace(name)
		if name == "" {
			if required {
				return -1, errors.New("profile names an empty column")
			}
			return -1, nil
		}
		for i, h := range header {
			if strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(h, "\uFEFF")), name) {
				return i, nil
			}
		}
		return -1, fmt.Errorf("column %q not found in the header %q", name, header)
	}
	var c columns
	var err error
	for _, f := range []struct {
		dst      *int
		name     string
		required bool
	}{
		{&c.date, p.DateColumn, true},
		{&c.description, p.DescriptionColumn, true},
		{&c.id, p.ExternalIDColumn, false},
		{&c.amount, p.AmountColumn, false},
		{&c.debit, p.DebitColumn, false},
		{&c.credit, p.CreditColumn, false},
	} {
		if *f.dst, err = find(f.name, f.required); err != nil {
			return columns{}, err
		}
	}
	if c.amount < 0 && (c.debit < 0 || c.credit < 0) {
		return columns{}, errors.New("profile needs amount_column, or both debit_column and credit_column")
	}
	return c, nil
}

// goLayout turns a profile date format into a time.Parse layout. Go layouts ("02/01/2006") are
// used as they are; otherwise the tokens YYYY, YY, MM and DD (any case) are replaced.
func goLayout(format string) string {
	if strings.Contains(format, "2006") {
		return format
	}
	return strings.NewReplacer("YYYY", "2006", "YY", "06", "MM", "01", "DD", "02").Replace(strings.ToUpper(format))
}

func cell(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

// row converts one data row into a transaction record or an error record; ok is false for blank
// rows and rows before Since.
func (c *csvConv) row(row []string, line int) (rec fuiproto.Record, ok bool) {
	if blank(row) {
		return fuiproto.Record{}, false
	}
	fail := func(format string, args ...any) (fuiproto.Record, bool) {
		c.errs++
		raw := make(map[string]any, len(row))
		for i, v := range row {
			key := fmt.Sprintf("column %d", i+1)
			if i < len(c.header) {
				key = c.header[i]
			}
			raw[key] = v
		}
		return fuiproto.Record{Type: fuiproto.RecordTypeError, Error: &fuiproto.ErrorRecord{
			Line: line, Message: fmt.Sprintf(format, args...), Raw: raw,
		}}, true
	}
	t, err := time.Parse(c.layout, cell(row, c.cols.date))
	if err != nil {
		return fail("invalid date %q", cell(row, c.cols.date))
	}
	date, err := domain.NewDate(t.Year(), int(t.Month()), t.Day())
	if err != nil {
		return fail("invalid date %q", cell(row, c.cols.date))
	}
	if !c.since.IsZero() && date.Before(c.since) {
		return fuiproto.Record{}, false
	}
	amount, err := c.amount(row)
	if err != nil {
		return fail("%v", err)
	}
	desc := strings.Join(strings.Fields(cell(row, c.cols.description)), " ")
	if desc == "" {
		return fail("empty description")
	}
	c.txs++
	return fuiproto.Record{Type: fuiproto.RecordTypeTransaction, Transaction: &fuiproto.TransactionRecord{
		Account: c.ref, ID: c.id(row, date, amount, desc), Date: date.String(), Amount: amount,
		Currency: c.cur.Code(), Description: desc, Status: fuiproto.TransactionStatusBooked,
	}}, true
}

func blank(row []string) bool {
	for _, v := range row {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

// amount reads the signed amount in minor units: the amount column, or credit minus debit.
func (c *csvConv) amount(row []string) (int64, error) {
	if c.cols.amount >= 0 {
		m, err := c.money(cell(row, c.cols.amount))
		if err != nil {
			return 0, err
		}
		return m, nil
	}
	debit, err := c.money(cell(row, c.cols.debit))
	if err != nil {
		return 0, err
	}
	credit, err := c.money(cell(row, c.cols.credit))
	if err != nil {
		return 0, err
	}
	if debit != 0 && credit != 0 {
		return 0, errors.New("both debit and credit are set")
	}
	if debit < 0 {
		debit = -debit
	}
	if credit < 0 {
		credit = -credit
	}
	return credit - debit, nil
}

// money parses one amount cell; an empty cell is zero (a debit/credit column left blank).
func (c *csvConv) money(s string) (int64, error) {
	s = strings.NewReplacer(" ", "", " ", "").Replace(s)
	if s == "" {
		return 0, nil
	}
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") { // accounting negative
		s = "-" + s[1:len(s)-1]
	}
	m, err := domain.ParseAmount(s, c.loc, c.cur)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q", s)
	}
	return m.Minor(), nil
}

// id is the external id column, or a hash of the row; identical rows get distinct hashes.
func (c *csvConv) id(row []string, date domain.Date, amount int64, desc string) string {
	if id := cell(row, c.cols.id); id != "" {
		return id
	}
	key := fmt.Sprintf("%s|%s|%d|%s", c.ref, date, amount, desc)
	n := c.seen[key]
	c.seen[key] = n + 1
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d", key, n)))
	return "csv-" + hex.EncodeToString(sum[:8])
}
