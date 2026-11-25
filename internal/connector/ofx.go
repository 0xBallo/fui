package connector

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"iter"
	"path/filepath"
	"strings"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/pkg/fuiproto"
)

const (
	ofxConnector = "ofx"
	ofxVersion   = "0.1.0"
)

// OFXSource is the built-in OFX connector: it reads OFX 1.x (SGML) and 2.x (XML) bank/card
// statements and emits the fui/1 stream.
type OFXSource struct {
	File  string
	Since string
}

var _ importing.Source = OFXSource{}

// NewOFX builds an OFX source for the given file and optional since date (YYYY-MM-DD).
func NewOFX(file, since string) OFXSource {
	return OFXSource{File: file, Since: since}
}

// Records implements importing.Source.
func (o OFXSource) Records(ctx context.Context) iter.Seq2[fuiproto.Record, error] {
	return func(yield func(fuiproto.Record, error) bool) {
		data, err := readCapped(o.File)
		if err != nil {
			yield(fuiproto.Record{}, err)
			return
		}
		sum := sha256.Sum256(data)
		for rec, err := range ofxRecords(ctx, data, o.Since, filepath.Base(o.File), hex.EncodeToString(sum[:])) {
			if !yield(rec, err) {
				return
			}
		}
	}
}

func ofxRecords(ctx context.Context, data []byte, sinceStr, file, sha string) iter.Seq2[fuiproto.Record, error] {
	return func(yield func(fuiproto.Record, error) bool) {
		stmts, err := parseOFX(data)
		if err != nil {
			yield(fuiproto.Record{}, err)
			return
		}
		var since domain.Date
		if sinceStr != "" {
			since, err = domain.ParseDate(sinceStr)
			if err != nil {
				yield(fuiproto.Record{}, fmt.Errorf("since: %w", err))
				return
			}
		}
		if !yield(fuiproto.Record{Type: fuiproto.RecordTypeMeta, Meta: &fuiproto.MetaRecord{
			Protocol: fuiproto.ProtocolFUI1, Connector: ofxConnector, Version: ofxVersion,
			Source: fuiproto.SourceMeta{File: file, SHA256: sha},
		}}, nil) {
			return
		}
		transactions, errs := 0, 0
		for _, stmt := range stmts {
			if err := ctx.Err(); err != nil {
				yield(fuiproto.Record{}, fmt.Errorf("read ofx: %w", err))
				return
			}
			recs, txs, es := stmt.records(since)
			transactions += txs
			errs += es
			for _, r := range recs {
				if !yield(r, nil) {
					return
				}
			}
		}
		yield(fuiproto.Record{
			Type: fuiproto.RecordTypeEnd,
			End:  &fuiproto.EndRecord{Transactions: transactions, Errors: errs},
		}, nil)
	}
}

type ofxStatement struct {
	account      fuiproto.AccountRecord
	transactions []ofxTransaction
	balance      *ofxBalance
	errors       []ofxError
}

// records lists the statement as fui/1 records, with the counts of transactions and errors.
func (s ofxStatement) records(since domain.Date) (recs []fuiproto.Record, transactions, errs int) {
	recs = append(recs, fuiproto.Record{Type: fuiproto.RecordTypeAccount, Account: &fuiproto.AccountRecord{
		Ref: s.account.Ref, Name: s.account.Ref, Currency: s.account.Currency, Kind: s.account.Kind, IBAN: s.account.IBAN,
	}})
	if b := s.balance; b != nil {
		recs = append(recs, fuiproto.Record{Type: fuiproto.RecordTypeBalance, Balance: &fuiproto.BalanceRecord{
			Account: s.account.Ref, Date: b.Date.String(), Amount: b.Amount,
		}})
	}
	for _, e := range s.errors {
		recs = append(recs, fuiproto.Record{Type: fuiproto.RecordTypeError, Error: &fuiproto.ErrorRecord{
			Line: e.Line, Message: e.Message, Raw: e.Raw,
		}})
	}
	errs = len(s.errors)
	for _, tx := range s.transactions {
		if !since.IsZero() && tx.Date.Before(since) {
			continue
		}
		transactions++
		recs = append(recs, fuiproto.Record{Type: fuiproto.RecordTypeTransaction, Transaction: &fuiproto.TransactionRecord{
			Account: s.account.Ref, ID: tx.ID, Date: tx.Date.String(), Amount: tx.Amount, Currency: s.account.Currency,
			Description: tx.Description, Status: fuiproto.TransactionStatusBooked, Kind: tx.Kind,
		}})
	}
	return recs, transactions, errs
}

type ofxTransaction struct {
	ID          string
	Date        domain.Date
	Amount      int64
	Description string
	Kind        fuiproto.TransactionKind
}

type ofxBalance struct {
	Date   domain.Date
	Amount int64
}

// ofxError is a recoverable problem of one element, cited by its source line.
type ofxError struct {
	Line    int
	Message string
	Raw     map[string]any
}

func parseOFX(data []byte) ([]ofxStatement, error) {
	text, err := decodeOFXText(data)
	if err != nil {
		return nil, err
	}
	body, lineDelta, err := normalizeOFXBody(text)
	if err != nil {
		return nil, err
	}
	doc := ofxDocument{}
	dec := xml.NewDecoder(strings.NewReader(body))
	dec.Strict = false
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("parse OFX: %w", err)
	}
	lineOf := func(off int64) int { return sourceLine([]byte(body), off) + lineDelta }
	statements := make([]ofxStatement, 0, len(doc.BankStatements)+len(doc.CardStatements))
	for _, s := range doc.BankStatements {
		stmt, err := convertStatement(s.CurDef, s.BankFrom.BankID, s.BankFrom.AccountID, fuiproto.AccountKindChecking, s.Transactions, s.Ledger, lineOf)
		if err != nil {
			return nil, err
		}
		statements = append(statements, stmt)
	}
	for _, s := range doc.CardStatements {
		stmt, err := convertStatement(s.CurDef, "", s.CardFrom.AccountID, fuiproto.AccountKindCard, s.Transactions, s.Ledger, lineOf)
		if err != nil {
			return nil, err
		}
		statements = append(statements, stmt)
	}
	if len(statements) == 0 {
		return nil, errors.New("OFX has no statement")
	}
	return statements, nil
}

// convertStatement turns a statement into records; a malformed transaction or ledger balance
// becomes an error entry instead of failing the whole file.
func convertStatement(currency, bankID, acctID string, kind fuiproto.AccountKind, transactions []ofxTxn, ledger *ofxLedger, lineOf func(int64) int) (ofxStatement, error) {
	cur, err := domain.ParseCurrency(strings.TrimSpace(currency))
	if err != nil {
		return ofxStatement{}, fmt.Errorf("currency: %w", err)
	}
	account := ofxAccount(strings.TrimSpace(bankID), strings.TrimSpace(acctID), cur.Code(), kind)
	if account.Ref == "" {
		return ofxStatement{}, errors.New("statement account is missing")
	}
	out := ofxStatement{account: account, transactions: make([]ofxTransaction, 0, len(transactions))}
	if ledger != nil {
		bal, err := convertLedger(ledger)
		if err != nil {
			out.errors = append(out.errors, ofxError{Line: lineOf(ledger.off), Message: err.Error(), Raw: map[string]any{
				"element": "LEDGERBAL", "amount": strings.TrimSpace(ledger.Amount), "date": strings.TrimSpace(ledger.Date),
			}})
		} else {
			out.balance = &bal
		}
	}
	for i, t := range transactions {
		tx, err := convertTxn(t, account.Ref, i)
		if err != nil {
			out.errors = append(out.errors, ofxError{Line: lineOf(t.off), Message: err.Error(), Raw: map[string]any{
				"element": "STMTTRN", "fitid": strings.TrimSpace(t.FitID), "date": strings.TrimSpace(t.DatePosted), "amount": strings.TrimSpace(t.Amount),
			}})
			continue
		}
		out.transactions = append(out.transactions, tx)
	}
	return out, nil
}

func convertLedger(l *ofxLedger) (ofxBalance, error) {
	date, err := parseOFXDate(strings.TrimSpace(l.Date))
	if err != nil {
		return ofxBalance{}, fmt.Errorf("DTASOF: %w", err)
	}
	amount, err := parseOFXAmount(strings.TrimSpace(l.Amount))
	if err != nil {
		return ofxBalance{}, fmt.Errorf("BALAMT: %w", err)
	}
	return ofxBalance{Date: date, Amount: amount}, nil
}

func convertTxn(t ofxTxn, accountRef string, i int) (ofxTransaction, error) {
	date, err := parseOFXDate(strings.TrimSpace(t.DatePosted))
	if err != nil {
		return ofxTransaction{}, fmt.Errorf("DTPOSTED: %w", err)
	}
	amount, err := parseOFXAmount(strings.TrimSpace(t.Amount))
	if err != nil {
		return ofxTransaction{}, fmt.Errorf("TRNAMT: %w", err)
	}
	description := normalizeText(t.Name)
	if memo := normalizeText(t.Memo); memo != "" && memo != description {
		if description == "" {
			description = memo
		} else {
			description += " - " + memo
		}
	}
	if description == "" {
		description = strings.TrimSpace(t.Type)
	}
	if description == "" {
		return ofxTransaction{}, errors.New("transaction has neither NAME nor MEMO")
	}
	id := strings.TrimSpace(t.FitID)
	if id == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d|%s|%d", accountRef, date, amount, description, i)))
		id = "ofx-" + hex.EncodeToString(sum[:8])
	}
	return ofxTransaction{ID: id, Date: date, Amount: amount, Description: description, Kind: trnTypeKind(t.Type)}, nil
}

func decodeOFXText(data []byte) (string, error) {
	encoding := detectOFXEncoding(data)
	text, err := toUTF8(data, encoding)
	if err == nil {
		return text, nil
	}
	if encoding == "" {
		if fallback, ferr := toUTF8(data, "windows-1252"); ferr == nil {
			return fallback, nil
		}
	}
	return "", err
}

func detectOFXEncoding(data []byte) string {
	peek := data
	if len(peek) > 2048 {
		peek = peek[:2048]
	}
	up := strings.ToUpper(string(peek))
	switch {
	case strings.Contains(up, "CHARSET:1252"), strings.Contains(up, "CHARSET:CP1252"), strings.Contains(up, "WINDOWS-1252"):
		return "windows-1252"
	case strings.Contains(up, "CHARSET:8859-1"), strings.Contains(up, "CHARSET:ISO-8859-1"), strings.Contains(up, "ISO-8859-1"):
		return "latin-1"
	default:
		return ""
	}
}

func normalizeOFX(text string) (string, error) {
	body, _, err := normalizeOFXBody(text)
	return body, err
}

// normalizeOFXBody returns well-formed XML and the delta to add to a line of that XML to get the
// matching line of the original text (the SGML path prepends one line and drops the header).
func normalizeOFXBody(text string) (string, int, error) {
	start := strings.IndexByte(text, '<')
	if start < 0 {
		return "", 0, errors.New("OFX payload has no XML/SGML body")
	}
	body := text[start:]
	skipped := strings.Count(text[:start], "\n")
	if !strings.Contains(strings.ToUpper(text[:start]), "OFXHEADER:") && strings.Contains(body, "</") {
		return body, skipped, nil
	}
	norm, err := normalizeSGMLOFX(body)
	return norm, skipped - 1, err
}

func normalizeSGMLOFX(input string) (string, error) {
	var out strings.Builder
	out.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	var stack []sgmlFrame
	autoClose := func(nextName string, closing bool) {
		for len(stack) > 0 && stack[len(stack)-1].text {
			top := stack[len(stack)-1].name
			if closing && strings.EqualFold(top, nextName) {
				return
			}
			out.WriteString("</")
			out.WriteString(top)
			out.WriteByte('>')
			stack = stack[:len(stack)-1]
		}
	}
	i := 0
	for i < len(input) {
		next, err := scanSGML(input, i)
		if err != nil {
			return "", err
		}
		if next.lt < 0 {
			if err := appendSGMLText(&out, &stack, next.text); err != nil {
				return "", err
			}
			break
		}
		if err := appendSGMLText(&out, &stack, next.text); err != nil {
			return "", err
		}
		handleSGMLTag(next.tag, &out, &stack, autoClose)
		i = next.next
	}
	for len(stack) > 0 {
		out.WriteString("</")
		out.WriteString(stack[len(stack)-1].name)
		out.WriteByte('>')
		stack = stack[:len(stack)-1]
	}
	return out.String(), nil
}

type sgmlScanResult struct {
	lt   int
	text string
	tag  string
	next int
}

type sgmlFrame struct {
	name string
	text bool
}

func scanSGML(input string, start int) (sgmlScanResult, error) {
	lt := strings.IndexByte(input[start:], '<')
	if lt < 0 {
		return sgmlScanResult{lt: -1, text: input[start:]}, nil
	}
	lt += start
	gt := strings.IndexByte(input[lt:], '>')
	if gt < 0 {
		return sgmlScanResult{}, errors.New("unterminated tag in OFX SGML")
	}
	gt += lt
	return sgmlScanResult{
		lt: lt, text: input[start:lt], tag: strings.TrimSpace(input[lt+1 : gt]), next: gt + 1,
	}, nil
}

func appendSGMLText(out *strings.Builder, stack *[]sgmlFrame, text string) error {
	if strings.TrimSpace(text) != "" && len(*stack) > 0 {
		(*stack)[len(*stack)-1].text = true
	}
	var esc bytes.Buffer
	if err := xml.EscapeText(&esc, []byte(text)); err != nil {
		return fmt.Errorf("escape text: %w", err)
	}
	// Keep real newlines so decoder offsets map back to the source lines.
	out.WriteString(strings.ReplaceAll(esc.String(), "&#xA;", "\n"))
	return nil
}

func handleSGMLTag(tag string, out *strings.Builder, stack *[]sgmlFrame, autoClose func(string, bool)) {
	if tag == "" {
		return
	}
	switch {
	case strings.HasPrefix(tag, "!--"), strings.HasPrefix(tag, "?"), strings.HasPrefix(tag, "!"):
		out.WriteString("<")
		out.WriteString(tag)
		out.WriteByte('>')
	case strings.HasPrefix(tag, "/"):
		handleSGMLClosingTag(tag, out, stack, autoClose)
	default:
		autoClose("", false)
		out.WriteByte('<')
		out.WriteString(tag)
		out.WriteByte('>')
		if !strings.HasSuffix(tag, "/") {
			*stack = append(*stack, sgmlFrame{name: tagName(tag)})
		}
	}
}

func handleSGMLClosingTag(tag string, out *strings.Builder, stack *[]sgmlFrame, autoClose func(string, bool)) {
	name := tagName(tag[1:])
	autoClose(name, true)
	for len(*stack) > 0 && !strings.EqualFold((*stack)[len(*stack)-1].name, name) {
		out.WriteString("</")
		out.WriteString((*stack)[len(*stack)-1].name)
		out.WriteByte('>')
		*stack = (*stack)[:len(*stack)-1]
	}
	if len(*stack) == 0 {
		return
	}
	out.WriteString("</")
	out.WriteString(tag[1:])
	out.WriteByte('>')
	*stack = (*stack)[:len(*stack)-1]
}

func tagName(tag string) string {
	tag = strings.TrimSpace(tag)
	tag = strings.TrimPrefix(tag, "/")
	for i := 0; i < len(tag); i++ {
		switch tag[i] {
		case ' ', '\t', '\r', '\n', '/':
			return tag[:i]
		}
	}
	return tag
}

type ofxDocument struct {
	XMLName        xml.Name      `xml:"OFX"`
	BankStatements []ofxBankStmt `xml:"BANKMSGSRSV1>STMTTRNRS>STMTRS"`
	CardStatements []ofxCardStmt `xml:"CREDITCARDMSGSRSV1>CCSTMTTRNRS>CCSTMTRS"`
}

type ofxBankStmt struct {
	CurDef       string      `xml:"CURDEF"`
	BankFrom     ofxBankFrom `xml:"BANKACCTFROM"`
	Transactions []ofxTxn    `xml:"BANKTRANLIST>STMTTRN"`
	Ledger       *ofxLedger  `xml:"LEDGERBAL"`
}

type ofxCardStmt struct {
	CurDef       string      `xml:"CURDEF"`
	CardFrom     ofxCardFrom `xml:"CCACCTFROM"`
	Transactions []ofxTxn    `xml:"BANKTRANLIST>STMTTRN"`
	Ledger       *ofxLedger  `xml:"LEDGERBAL"`
}

type ofxBankFrom struct {
	BankID    string `xml:"BANKID"`
	AccountID string `xml:"ACCTID"`
}

type ofxCardFrom struct {
	AccountID string `xml:"ACCTID"`
}

type ofxTxn struct {
	Type       string `xml:"TRNTYPE"`
	DatePosted string `xml:"DTPOSTED"`
	Amount     string `xml:"TRNAMT"`
	FitID      string `xml:"FITID"`
	Name       string `xml:"NAME"`
	Memo       string `xml:"MEMO"`

	off int64 // decoder offset right after the opening tag
}

// UnmarshalXML decodes the element and remembers where it starts, to cite the source line.
func (t *ofxTxn) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	off := d.InputOffset()
	type plain ofxTxn
	var p plain
	if err := d.DecodeElement(&p, &start); err != nil {
		return fmt.Errorf("decode %s: %w", start.Name.Local, err)
	}
	*t = ofxTxn(p)
	t.off = off
	return nil
}

type ofxLedger struct {
	Amount string `xml:"BALAMT"`
	Date   string `xml:"DTASOF"`

	off int64
}

// UnmarshalXML decodes the element and remembers where it starts, to cite the source line.
func (l *ofxLedger) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	off := d.InputOffset()
	type plain ofxLedger
	var p plain
	if err := d.DecodeElement(&p, &start); err != nil {
		return fmt.Errorf("decode %s: %w", start.Name.Local, err)
	}
	*l = ofxLedger(p)
	l.off = off
	return nil
}

func parseOFXDate(s string) (domain.Date, error) {
	digits := make([]byte, 0, 8)
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			digits = append(digits, s[i])
			if len(digits) == 8 {
				break
			}
		}
	}
	if len(digits) != 8 {
		return domain.Date{}, fmt.Errorf("invalid date %q", s)
	}
	year := atoi4(digits[0:4])
	month := atoi2(digits[4:6])
	day := atoi2(digits[6:8])
	d, err := domain.NewDate(year, month, day)
	if err != nil {
		return domain.Date{}, fmt.Errorf("invalid date %q", s)
	}
	return d, nil
}

func parseOFXAmount(s string) (int64, error) {
	in, neg, err := ofxAmountSign(s)
	if err != nil {
		return 0, err
	}
	intPart, fracPart, err := splitOFXAmount(in, s)
	if err != nil {
		return 0, err
	}
	amount, err := composeOFXMinor(intPart, fracPart, s)
	if err != nil {
		return 0, err
	}
	if neg {
		return -amount, nil
	}
	return amount, nil
}

func ofxAmountSign(s string) (string, bool, error) {
	in := strings.NewReplacer(" ", "", "\u00A0", "").Replace(strings.TrimSpace(s))
	if in == "" {
		return "", false, errors.New("empty amount")
	}
	neg := false
	if strings.HasPrefix(in, "(") && strings.HasSuffix(in, ")") {
		neg = true
		in = in[1 : len(in)-1]
	}
	if strings.HasPrefix(in, "+") {
		in = in[1:]
	} else if strings.HasPrefix(in, "-") {
		neg = true
		in = in[1:]
	}
	return in, neg, nil
}

func splitOFXAmount(in, original string) (string, string, error) {
	if in == "" {
		return "", "", errors.New("empty amount")
	}
	lastDot := strings.LastIndexByte(in, '.')
	lastComma := strings.LastIndexByte(in, ',')
	sep := -1
	if lastDot > lastComma {
		sep = lastDot
	}
	if lastComma > lastDot {
		sep = lastComma
	}
	intPart, fracPart := in, ""
	if sep >= 0 {
		intPart, fracPart = in[:sep], in[sep+1:]
	}
	intPart = strings.NewReplacer(".", "", ",", "").Replace(intPart)
	fracPart = strings.NewReplacer(".", "", ",", "").Replace(fracPart)
	if intPart == "" {
		intPart = "0"
	}
	if !digitsOnly(intPart) {
		return "", "", fmt.Errorf("invalid amount %q", original)
	}
	for len(fracPart) < 2 {
		fracPart += "0"
	}
	if len(fracPart) > 2 {
		fracPart = fracPart[:2]
	}
	if !digitsOnly(fracPart) {
		return "", "", fmt.Errorf("invalid amount %q", original)
	}
	return intPart, fracPart, nil
}

func composeOFXMinor(intPart, fracPart, original string) (int64, error) {
	whole := int64(0)
	for i := 0; i < len(intPart); i++ {
		whole = whole*10 + int64(intPart[i]-'0')
	}
	cents := int64(0)
	for i := 0; i < len(fracPart); i++ {
		cents = cents*10 + int64(fracPart[i]-'0')
	}
	amount := whole*100 + cents
	if amount < 0 {
		return 0, fmt.Errorf("invalid amount %q", original)
	}
	return amount, nil
}

func normalizeText(s string) string { return strings.Join(strings.Fields(strings.TrimSpace(s)), " ") }

func trnTypeKind(s string) fuiproto.TransactionKind {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "POS", "DEBIT", "CHECK", "ATM", "PAYMENT", "DIRECTDEBIT":
		return fuiproto.TransactionKindPayment
	case "XFER", "TRANSFER", "DIRECTDEP", "DEP":
		return fuiproto.TransactionKindTransfer
	case "INT":
		return fuiproto.TransactionKindInterest
	case "FEE", "SRVCHG":
		return fuiproto.TransactionKindFee
	case "CREDIT", "REFUND":
		return fuiproto.TransactionKindRefund
	default:
		return fuiproto.TransactionKindOther
	}
}

func ofxAccount(bankID, acctID, currency string, kind fuiproto.AccountKind) fuiproto.AccountRecord {
	ref := acctID
	iban := ""
	if parsed, err := domain.ParseIBAN(acctID); err == nil {
		iban = parsed
		ref = parsed
	} else if bankID != "" && acctID != "" {
		ref = bankID + "-" + acctID
	} else if ref == "" {
		ref = bankID
	}
	return fuiproto.AccountRecord{Ref: ref, Currency: currency, Kind: kind, IBAN: iban}
}

func atoi2(d []byte) int { return int(d[0]-'0')*10 + int(d[1]-'0') }

func atoi4(d []byte) int { return atoi2(d[:2])*100 + atoi2(d[2:]) }

func digitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
