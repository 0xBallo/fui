package connector

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"iter"
	"path/filepath"
	"strings"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/pkg/fuiproto"
)

const (
	camtConnector = "camt"
	camtVersion   = "0.1.0"
	camtNamespace = "camt.053.001."
)

// CAMTOptions tune the CAMT.053 connector; the zero value is valid.
type CAMTOptions struct {
	// Currency is the ISO 4217 code used when a statement names none, nor its first entry.
	Currency string
	// Kind is the account kind, checking when empty.
	Kind fuiproto.AccountKind
	// Since, as YYYY-MM-DD, skips the entries booked before it.
	Since string
}

// CAMTSource is the built-in ISO 20022 camt.053 connector. It reads every statement of the file
// (any camt.053.001.xx with the 02/08 element layout) and emits the fui/1 stream: one account per
// distinct IBAN, one transaction per entry, or per transaction detail when an entry is a batch.
type CAMTSource struct {
	File string
	CAMTOptions
}

var _ importing.Source = CAMTSource{}

// NewCAMT returns the connector reading the CAMT.053 file.
func NewCAMT(file string, opts CAMTOptions) CAMTSource {
	return CAMTSource{File: file, CAMTOptions: opts}
}

// Records implements importing.Source.
func (c CAMTSource) Records(ctx context.Context) iter.Seq2[fuiproto.Record, error] {
	return func(yield func(fuiproto.Record, error) bool) {
		data, err := readCapped(c.File)
		if err != nil {
			yield(fuiproto.Record{}, err)
			return
		}
		sum := sha256.Sum256(data)
		for rec, err := range camtRecords(ctx, data, c.CAMTOptions, filepath.Base(c.File), hex.EncodeToString(sum[:])) {
			if !yield(rec, err) {
				return
			}
		}
	}
}

type camtDoc struct {
	XMLName xml.Name   `xml:"Document"`
	Stmts   []camtStmt `xml:"BkToCstmrStmt>Stmt"`
}

type camtStmt struct {
	Acct    camtAcct    `xml:"Acct"`
	Bals    []camtBal   `xml:"Bal"`
	Entries []camtEntry `xml:"Ntry"`
}

type camtAcct struct {
	ID struct {
		IBAN string `xml:"IBAN"`
		Othr struct {
			ID string `xml:"Id"`
		} `xml:"Othr"`
	} `xml:"Id"`
	Ccy string `xml:"Ccy"`
	Nm  string `xml:"Nm"`
}

type camtAmt struct {
	Ccy   string `xml:"Ccy,attr"`
	Value string `xml:",chardata"`
}

type camtDate struct {
	Dt   string `xml:"Dt"`
	DtTm string `xml:"DtTm"`
}

type camtBal struct {
	Code string   `xml:"Tp>CdOrPrtry>Cd"`
	Amt  camtAmt  `xml:"Amt"`
	Ind  string   `xml:"CdtDbtInd"`
	Date camtDate `xml:"Dt"`
}

type camtEntry struct {
	NtryRef string  `xml:"NtryRef"`
	Amt     camtAmt `xml:"Amt"`
	Ind     string  `xml:"CdtDbtInd"`
	Rvsl    string  `xml:"RvslInd"`
	Sts     struct {
		Value string `xml:",chardata"`
		Cd    string `xml:"Cd"`
	} `xml:"Sts"`
	Booking   camtDate `xml:"BookgDt"`
	Value     camtDate `xml:"ValDt"`
	SvcrRef   string   `xml:"AcctSvcrRef"`
	AddtlInfo string   `xml:"AddtlNtryInf"`
	Details   []camtTx `xml:"NtryDtls>TxDtls"`
}

type camtTx struct {
	Refs struct {
		SvcrRef  string `xml:"AcctSvcrRef"`
		EndToEnd string `xml:"EndToEndId"`
	} `xml:"Refs"`
	Amt     camtAmt `xml:"Amt"`
	Parties struct {
		Dbtr     camtParty `xml:"Dbtr"`
		DbtrAcct camtAcct  `xml:"DbtrAcct"`
		Cdtr     camtParty `xml:"Cdtr"`
		CdtrAcct camtAcct  `xml:"CdtrAcct"`
	} `xml:"RltdPties"`
	Ustrd     []string `xml:"RmtInf>Ustrd"`
	AddtlInfo string   `xml:"AddtlTxInf"`
}

// camtParty reads the name of a party: Nm in camt.053.001.02, Pty/Nm in .08.
type camtParty struct {
	Nm  string `xml:"Nm"`
	Pty struct {
		Nm string `xml:"Nm"`
	} `xml:"Pty"`
}

func (p camtParty) name() string { return clean(orDefault(p.Nm, p.Pty.Nm)) }

func clean(s string) string { return strings.Join(strings.Fields(s), " ") }

// parseCAMT decodes and sanity-checks the document.
func parseCAMT(data []byte) (camtDoc, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = func(label string, in io.Reader) (io.Reader, error) {
		raw, err := io.ReadAll(in)
		if err != nil {
			return nil, fmt.Errorf("read charset %s: %w", label, err)
		}
		text, err := toUTF8(raw, label)
		if err != nil {
			return nil, fmt.Errorf("decode charset %s: %w", label, err)
		}
		return strings.NewReader(text), nil
	}
	var doc camtDoc
	if err := dec.Decode(&doc); err != nil {
		return camtDoc{}, fmt.Errorf("read camt.053: %w", err)
	}
	if !strings.Contains(doc.XMLName.Space, camtNamespace) {
		return camtDoc{}, fmt.Errorf("not a camt.053 document: namespace %q", doc.XMLName.Space)
	}
	if len(doc.Stmts) == 0 {
		return camtDoc{}, errors.New("camt.053 has no statement")
	}
	return doc, nil
}

type camtConv struct {
	since    domain.Date
	kind     fuiproto.AccountKind
	fallback string
	accounts map[string]string // ref -> currency
	seen     map[string]int
	txs      int
	errs     int
}

func camtRecords(ctx context.Context, data []byte, o CAMTOptions, file, sha string) iter.Seq2[fuiproto.Record, error] {
	return func(yield func(fuiproto.Record, error) bool) {
		doc, err := parseCAMT(data)
		if err != nil {
			yield(fuiproto.Record{}, err)
			return
		}
		conv := &camtConv{kind: o.Kind, fallback: o.Currency, accounts: map[string]string{}, seen: map[string]int{}}
		if conv.kind == "" {
			conv.kind = fuiproto.AccountKindChecking
		}
		if o.Since != "" {
			if conv.since, err = domain.ParseDate(o.Since); err != nil {
				yield(fuiproto.Record{}, fmt.Errorf("since: %w", err))
				return
			}
		}
		if !yield(fuiproto.Record{Type: fuiproto.RecordTypeMeta, Meta: &fuiproto.MetaRecord{
			Protocol: fuiproto.ProtocolFUI1, Connector: camtConnector, Version: camtVersion,
			Source: fuiproto.SourceMeta{File: file, SHA256: sha},
		}}, nil) {
			return
		}
		for i := range doc.Stmts {
			if err := ctx.Err(); err != nil {
				yield(fuiproto.Record{}, fmt.Errorf("read camt.053: %w", err))
				return
			}
			recs, err := conv.statement(i+1, &doc.Stmts[i])
			if err != nil {
				yield(fuiproto.Record{}, err)
				return
			}
			for _, r := range recs {
				if !yield(r, nil) {
					return
				}
			}
		}
		yield(fuiproto.Record{Type: fuiproto.RecordTypeEnd, End: &fuiproto.EndRecord{Transactions: conv.txs, Errors: conv.errs}}, nil)
	}
}

// statement converts one Stmt; a statement without an account or currency is a fatal error.
func (c *camtConv) statement(n int, st *camtStmt) ([]fuiproto.Record, error) {
	ref := strings.ToUpper(strings.Join(strings.Fields(st.Acct.ID.IBAN), ""))
	iban := ref
	if ref == "" {
		ref = clean(st.Acct.ID.Othr.ID)
	}
	if ref == "" {
		return nil, fmt.Errorf("statement %d has no account IBAN or id", n)
	}
	cur := strings.ToUpper(strings.TrimSpace(st.Acct.Ccy))
	if cur == "" && len(st.Entries) > 0 {
		cur = strings.ToUpper(strings.TrimSpace(st.Entries[0].Amt.Ccy))
	}
	if cur == "" {
		cur = c.fallback
	}
	currency, err := domain.ParseCurrency(cur)
	if err != nil {
		return nil, fmt.Errorf("statement %d currency: %w", n, err)
	}
	var out []fuiproto.Record
	if have, ok := c.accounts[ref]; !ok {
		c.accounts[ref] = currency.Code()
		out = append(out, fuiproto.Record{Type: fuiproto.RecordTypeAccount, Account: &fuiproto.AccountRecord{
			Ref: ref, Name: orDefault(clean(st.Acct.Nm), ref), Currency: currency.Code(), Kind: c.kind, IBAN: iban,
		}})
	} else if have != currency.Code() {
		return nil, fmt.Errorf("statement %d: account %s is in %s, earlier statements in %s", n, ref, currency.Code(), have)
	}
	for _, b := range st.Bals {
		if rec, ok := c.balance(ref, currency, b); ok {
			out = append(out, rec)
		}
	}
	for i := range st.Entries {
		out = append(out, c.entry(ref, currency, n, i+1, &st.Entries[i])...)
	}
	return out, nil
}

// balance emits the closing booked balance as a reconciliation point.
func (c *camtConv) balance(ref string, cur domain.Currency, b camtBal) (fuiproto.Record, bool) {
	if b.Code != "CLBD" || !strings.EqualFold(strings.TrimSpace(b.Amt.Ccy), cur.Code()) {
		return fuiproto.Record{}, false
	}
	date, err := camtDay(b.Date)
	if err != nil {
		return fuiproto.Record{}, false
	}
	amount, err := camtMoney(b.Amt, b.Ind, cur)
	if err != nil {
		return fuiproto.Record{}, false
	}
	return fuiproto.Record{Type: fuiproto.RecordTypeBalance, Balance: &fuiproto.BalanceRecord{
		Account: ref, Date: date.String(), Amount: amount,
	}}, true
}

// camtDay reads the date part of a Dt or DtTm element.
func camtDay(d camtDate) (domain.Date, error) {
	s := strings.TrimSpace(orDefault(d.Dt, d.DtTm))
	if len(s) < 10 {
		return domain.Date{}, fmt.Errorf("invalid date %q", s)
	}
	return domain.ParseDate(s[:10])
}

// camtMoney converts a positive decimal amount and the credit/debit indicator to signed minor units.
func camtMoney(a camtAmt, ind string, cur domain.Currency) (int64, error) {
	m, err := domain.ParseAmount(strings.TrimSpace(a.Value), domain.LocaleENUS, cur)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q", strings.TrimSpace(a.Value))
	}
	v := m.Minor()
	if v < 0 {
		return 0, fmt.Errorf("negative amount %q", strings.TrimSpace(a.Value))
	}
	switch strings.ToUpper(strings.TrimSpace(ind)) {
	case "CRDT":
		return v, nil
	case "DBIT":
		return -v, nil
	}
	return 0, fmt.Errorf("invalid credit/debit indicator %q", ind)
}

// leg is one transaction extracted from an entry.
type leg struct {
	amt  camtAmt
	tx   *camtTx
	part int
}

func (c *camtConv) entry(ref string, cur domain.Currency, stmt, n int, e *camtEntry) []fuiproto.Record {
	fail := func(format string, args ...any) []fuiproto.Record {
		c.errs++
		return []fuiproto.Record{{Type: fuiproto.RecordTypeError, Error: &fuiproto.ErrorRecord{
			Line: n, Message: fmt.Sprintf(format, args...),
			Raw: map[string]any{"statement": stmt, "entry": n, "amount": strings.TrimSpace(e.Amt.Value), "ref": orDefault(e.SvcrRef, e.NtryRef)},
		}}}
	}
	date, err := camtDay(e.Booking)
	if err != nil {
		if date, err = camtDay(e.Value); err != nil {
			return fail("invalid booking date")
		}
	}
	if !c.since.IsZero() && date.Before(c.since) {
		return nil
	}
	if ccy := strings.ToUpper(strings.TrimSpace(e.Amt.Ccy)); ccy != cur.Code() {
		return fail("entry currency %q differs from account currency %q", ccy, cur.Code())
	}
	total, err := camtMoney(e.Amt, e.Ind, cur)
	if err != nil {
		return fail("%v", err)
	}
	var out []fuiproto.Record
	for _, l := range c.legs(e, cur, total) {
		amount := total
		if l.tx != nil && l.part > 0 {
			amount, _ = camtMoney(l.amt, e.Ind, cur) // validated by legs
		}
		rec, ok := c.transaction(ref, cur, date, e, l, amount)
		if !ok {
			return fail("empty description")
		}
		out = append(out, rec)
	}
	c.txs += len(out)
	return out
}

// legs splits a batch entry into its transaction details when their amounts add up to the entry
// amount; otherwise the entry is a single transaction described by all its details.
func (c *camtConv) legs(e *camtEntry, cur domain.Currency, total int64) []leg {
	if len(e.Details) > 1 {
		var sum int64
		split := true
		for i := range e.Details {
			a, err := camtMoney(e.Details[i].Amt, e.Ind, cur)
			if err != nil || !strings.EqualFold(strings.TrimSpace(e.Details[i].Amt.Ccy), cur.Code()) {
				split = false
				break
			}
			sum += a
		}
		if split && sum == total {
			out := make([]leg, len(e.Details))
			for i := range e.Details {
				out[i] = leg{amt: e.Details[i].Amt, tx: &e.Details[i], part: i + 1}
			}
			return out
		}
	}
	if len(e.Details) > 0 {
		return []leg{{amt: e.Amt, tx: &e.Details[0]}}
	}
	return []leg{{amt: e.Amt}}
}

func (c *camtConv) transaction(ref string, cur domain.Currency, date domain.Date, e *camtEntry, l leg, amount int64) (fuiproto.Record, bool) {
	var payee, cpIBAN string
	var texts []string
	raw := map[string]any{}
	details := e.Details
	if l.part > 0 {
		details = []camtTx{*l.tx}
	}
	for i := range details {
		d := &details[i]
		party, acct := d.Parties.Cdtr, d.Parties.CdtrAcct
		if amount > 0 {
			party, acct = d.Parties.Dbtr, d.Parties.DbtrAcct
		}
		if payee == "" {
			payee = party.name()
			cpIBAN = strings.ToUpper(strings.Join(strings.Fields(acct.ID.IBAN), ""))
		}
		for _, u := range d.Ustrd {
			texts = append(texts, clean(u))
		}
		if len(d.Ustrd) == 0 {
			texts = append(texts, clean(d.AddtlInfo))
		}
		if d.Refs.EndToEnd != "" && d.Refs.EndToEnd != "NOTPROVIDED" {
			raw["end_to_end_id"] = d.Refs.EndToEnd
		}
	}
	desc := clean(strings.Join(texts, " "))
	desc = orDefault(desc, orDefault(clean(e.AddtlInfo), payee))
	if desc == "" {
		return fuiproto.Record{}, false
	}
	tx := &fuiproto.TransactionRecord{
		Account: ref, ID: c.id(ref, date, amount, desc, e, l), Date: date.String(), Amount: amount,
		Currency: cur.Code(), Description: desc, Payee: payee, CounterpartyIBAN: cpIBAN,
		Status: fuiproto.TransactionStatusBooked,
	}
	if vd, err := camtDay(e.Value); err == nil {
		tx.ValueDate = vd.String()
	}
	if strings.EqualFold(strings.TrimSpace(orDefault(e.Sts.Cd, e.Sts.Value)), "PDNG") {
		tx.Status = fuiproto.TransactionStatusPending
	}
	if strings.EqualFold(strings.TrimSpace(e.Rvsl), "true") {
		tx.Kind = fuiproto.TransactionKindRefund
		raw["reversal"] = true
	}
	if len(raw) > 0 {
		tx.Raw = raw
	}
	return fuiproto.Record{Type: fuiproto.RecordTypeTransaction, Transaction: tx}, true
}

// id is the bank's reference (batch parts get a suffix), or a hash of the entry; identical
// entries get distinct hashes so overlapping exports keep the same ids.
func (c *camtConv) id(ref string, date domain.Date, amount int64, desc string, e *camtEntry, l leg) string {
	base := clean(orDefault(e.SvcrRef, e.NtryRef))
	if l.part > 0 {
		if r := clean(l.tx.Refs.SvcrRef); r != "" {
			base = r
		} else if base != "" {
			base = fmt.Sprintf("%s#%d", base, l.part)
		}
	}
	key := fmt.Sprintf("%s|%s|%d|%s", ref, date, amount, desc)
	if base != "" {
		key = ref + "|" + base
	}
	n := c.seen[key]
	c.seen[key] = n + 1
	if base != "" {
		if n > 0 {
			return fmt.Sprintf("%s~%d", base, n)
		}
		return base
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d", key, n)))
	return "camt-" + hex.EncodeToString(sum[:8])
}
