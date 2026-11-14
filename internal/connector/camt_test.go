package connector

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/pkg/fuiproto"
)

func camtStream(t *testing.T, src CAMTSource) []fuiproto.Record {
	t.Helper()
	recs, err := collect(t, src.Records(ctx()))
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	if _, err := fuiproto.ValidateStream(func(yield func(fuiproto.Record, error) bool) {
		for _, r := range recs {
			if !yield(r, nil) {
				return
			}
		}
	}); err != nil {
		t.Fatalf("stream is invalid: %v", err)
	}
	return recs
}

type camtWant struct {
	accounts []fuiproto.AccountRecord
	balances []fuiproto.BalanceRecord
	txs      []fuiproto.TransactionRecord
	errs     int
}

func camtSplit(recs []fuiproto.Record) (got camtWant) {
	for _, r := range recs {
		switch {
		case r.Account != nil:
			got.accounts = append(got.accounts, *r.Account)
		case r.Balance != nil:
			got.balances = append(got.balances, *r.Balance)
		case r.Transaction != nil:
			tx := *r.Transaction
			got.txs = append(got.txs, tx)
		case r.Error != nil:
			got.errs++
		}
	}
	return got
}

func TestCAMTFixtures(t *testing.T) {
	const de = "DE89370400440532013000"
	const it = "IT60X0542811101000000123456"
	booked := fuiproto.TransactionStatusBooked
	tests := []struct {
		file string
		opts CAMTOptions
		want camtWant
	}{
		{
			file: "v02.xml",
			want: camtWant{
				accounts: []fuiproto.AccountRecord{
					{Ref: de, Name: "Synthetic Current", Currency: "EUR", Kind: "checking", IBAN: de},
					{Ref: "CASH-ACC-7", Name: "CASH-ACC-7", Currency: "USD", Kind: "checking"},
				},
				balances: []fuiproto.BalanceRecord{{Account: de, Date: "2026-01-31", Amount: 123456}},
				txs: []fuiproto.TransactionRecord{
					{
						Account: de, ID: "REF-0001", Date: "2026-01-05", ValueDate: "2026-01-06", Amount: 250000, Currency: "EUR",
						Description: "Salary January", Payee: "Acme Payroll Ltd", CounterpartyIBAN: "FR1420041010050500013M02606", Status: booked,
					},
					{
						Account: de, ID: "REF-0002", Date: "2026-01-12", Amount: -4590, Currency: "EUR",
						Description: "Card payment groceries 12/01", Payee: "Corner Grocery", CounterpartyIBAN: it, Status: booked,
					},
					{
						Account: de, ID: "REF-0003", Date: "2026-01-14", Amount: 4590, Currency: "EUR",
						Description: "Reversal of card payment", Status: booked, Kind: fuiproto.TransactionKindRefund,
						Raw: map[string]any{"reversal": true},
					},
					{
						Account: "CASH-ACC-7", ID: txID("CASH-ACC-7", "2026-01-20", -1000, "Monthly fee"), Date: "2026-01-20", Amount: -1000,
						Currency: "USD", Description: "Monthly fee", Status: booked,
					},
				},
			},
		},
		{
			file: "v08.xml",
			want: camtWant{
				accounts: []fuiproto.AccountRecord{{Ref: it, Name: it, Currency: "EUR", Kind: "checking", IBAN: it}},
				balances: []fuiproto.BalanceRecord{{Account: it, Date: "2026-02-28", Amount: -9050}},
				txs: []fuiproto.TransactionRecord{
					{
						Account: it, ID: "SVC-100#1", Date: "2026-02-03", ValueDate: "2026-02-03", Amount: -1000, Currency: "EUR",
						Description: "Water bill", Payee: "Water Utility", CounterpartyIBAN: de, Status: booked,
					},
					{
						Account: it, ID: "SVC-100#2", Date: "2026-02-03", ValueDate: "2026-02-03", Amount: -2000, Currency: "EUR",
						Description: "Power bill", Payee: "Power Utility", Status: booked,
					},
					{
						Account: it, ID: txID(it, "2026-02-10", -6050, "Membership"), Date: "2026-02-10", Amount: -6050, Currency: "EUR",
						Description: "Membership", Payee: "Gym Club", Status: fuiproto.TransactionStatusPending,
					},
					{
						Account: it, ID: "SVC-101", Date: "2026-02-11", Amount: 500, Currency: "EUR",
						Description: "Lunch", Payee: "Friend", Status: booked,
					},
					{
						Account: it, ID: "SVC-102", Date: "2026-03-01", Amount: 100, Currency: "EUR",
						Description: "Interest", Status: booked,
					},
				},
			},
		},
		{
			file: "v08.xml", opts: CAMTOptions{Since: "2026-02-11", Kind: fuiproto.AccountKindSavings},
			want: camtWant{
				accounts: []fuiproto.AccountRecord{{Ref: it, Name: it, Currency: "EUR", Kind: "savings", IBAN: it}},
				balances: []fuiproto.BalanceRecord{{Account: it, Date: "2026-02-28", Amount: -9050}},
				txs: []fuiproto.TransactionRecord{
					{Account: it, ID: "SVC-101", Date: "2026-02-11", Amount: 500, Currency: "EUR", Description: "Lunch", Payee: "Friend", Status: booked},
					{Account: it, ID: "SVC-102", Date: "2026-03-01", Amount: 100, Currency: "EUR", Description: "Interest", Status: booked},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.file+tt.opts.Since, func(t *testing.T) {
			got := camtSplit(camtStream(t, NewCAMT(filepath.Join("testdata", "camt", tt.file), tt.opts)))
			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(camtWant{})); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// txID mirrors the hash id of an entry without a bank reference.
func txID(ref, date string, amount int64, desc string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d|%s|0", ref, date, amount, desc)))
	return "camt-" + hex.EncodeToString(sum[:8])
}

func TestCAMTMalformed(t *testing.T) {
	const ns = `xmlns="urn:iso:std:iso:20022:tech:xsd:camt.053.001.02"`
	stmt := func(acct, body string) string {
		return `<Document ` + ns + `><BkToCstmrStmt><Stmt>` + acct + body + `</Stmt></BkToCstmrStmt></Document>`
	}
	acct := `<Acct><Id><IBAN>DE89370400440532013000</IBAN></Id><Ccy>EUR</Ccy></Acct>`
	tests := []struct {
		name, doc, want string
	}{
		{"empty", ``, "read camt.053"},
		{"not xml", `hello`, "read camt.053"},
		{"truncated", `<Document ` + ns + `><BkToCstmrStmt><Stmt>`, "read camt.053"},
		{"wrong root", `<Other ` + ns + `/>`, "read camt.053"},
		{"wrong namespace", `<Document xmlns="urn:iso:std:iso:20022:tech:xsd:pain.001.001.03"/>`, "not a camt.053"},
		{"no statements", `<Document ` + ns + `><BkToCstmrStmt/></Document>`, "no statement"},
		{"no account", stmt(``, ``), "no account"},
		{"no currency", stmt(`<Acct><Id><IBAN>DE89370400440532013000</IBAN></Id></Acct>`, ``), "currency"},
		{"bad currency", stmt(`<Acct><Id><IBAN>X1</IBAN></Id><Ccy>ZZZZ</Ccy></Acct>`, ``), "currency"},
		{"unknown charset", `<?xml version="1.0" encoding="klingon"?><Document ` + ns + `/>`, "read camt.053"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := collect(t, camtRecords(ctx(), []byte(tt.doc), CAMTOptions{}, "x", "y"))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}

	t.Run("bad since", func(t *testing.T) {
		_, err := collect(t, camtRecords(ctx(), []byte(stmt(acct, ``)), CAMTOptions{Since: "yesterday"}, "x", "y"))
		if err == nil || !strings.Contains(err.Error(), "since") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("missing file", func(t *testing.T) {
		if _, err := collect(t, NewCAMT(filepath.Join(t.TempDir(), "nope.xml"), CAMTOptions{}).Records(ctx())); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("account currency conflict", func(t *testing.T) {
		doc := `<Document ` + ns + `><BkToCstmrStmt><Stmt>` + acct + `</Stmt><Stmt>` +
			strings.Replace(acct, "EUR", "USD", 1) + `</Stmt></BkToCstmrStmt></Document>`
		if _, err := collect(t, camtRecords(ctx(), []byte(doc), CAMTOptions{}, "x", "y")); err == nil {
			t.Fatal("want error")
		}
	})
}

func TestCAMTEntryErrors(t *testing.T) {
	entry := func(amt, ccy, ind, date, rest string) string {
		return `<Ntry><Amt Ccy="` + ccy + `">` + amt + `</Amt><CdtDbtInd>` + ind + `</CdtDbtInd><BookgDt><Dt>` + date + `</Dt></BookgDt>` + rest + `</Ntry>`
	}
	info := `<AddtlNtryInf>x</AddtlNtryInf>`
	tests := []struct {
		name, entry string
	}{
		{"bad amount", entry("abc", "EUR", "CRDT", "2026-01-01", info)},
		{"negative amount", entry("-1.00", "EUR", "CRDT", "2026-01-01", info)},
		{"bad indicator", entry("1.00", "EUR", "XXXX", "2026-01-01", info)},
		{"bad date", entry("1.00", "EUR", "CRDT", "01/01/2026", info)},
		{"other currency", entry("1.00", "USD", "CRDT", "2026-01-01", info)},
		{"no description", entry("1.00", "EUR", "CRDT", "2026-01-01", "")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := `<Document xmlns="urn:iso:std:iso:20022:tech:xsd:camt.053.001.02"><BkToCstmrStmt><Stmt>` +
				`<Acct><Id><IBAN>DE89370400440532013000</IBAN></Id><Ccy>EUR</Ccy></Acct>` + tt.entry + `</Stmt></BkToCstmrStmt></Document>`
			recs, err := collect(t, camtRecords(ctx(), []byte(doc), CAMTOptions{}, "x", "y"))
			if err != nil {
				t.Fatal(err)
			}
			got := camtSplit(recs)
			if got.errs != 1 || len(got.txs) != 0 {
				t.Fatalf("errors=%d txs=%d, want one error record and no transaction", got.errs, len(got.txs))
			}
			if end := recs[len(recs)-1].End; end == nil || end.Errors != 1 {
				t.Fatalf("end = %+v", end)
			}
		})
	}
}

func TestCAMTDuplicateRefsStayDistinct(t *testing.T) {
	e := func(amt string) string {
		return `<Ntry><Amt Ccy="EUR">` + amt + `</Amt><CdtDbtInd>DBIT</CdtDbtInd><BookgDt><Dt>2026-01-01</Dt></BookgDt><AcctSvcrRef>R</AcctSvcrRef><AddtlNtryInf>x</AddtlNtryInf></Ntry>`
	}
	doc := `<Document xmlns="urn:iso:std:iso:20022:tech:xsd:camt.053.001.08"><BkToCstmrStmt><Stmt><Acct><Id><IBAN>DE89370400440532013000</IBAN></Id><Ccy>EUR</Ccy></Acct>` +
		e("1.00") + e("2.00") + e("2.00") + `</Stmt></BkToCstmrStmt></Document>`
	recs, err := collect(t, camtRecords(ctx(), []byte(doc), CAMTOptions{}, "x", "y"))
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, tx := range camtSplit(recs).txs {
		ids[tx.ID] = true
	}
	if len(ids) != 3 {
		t.Fatalf("ids = %v, want 3 distinct", ids)
	}
}

func TestCAMTLatin1Document(t *testing.T) {
	doc := "<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><Document xmlns=\"urn:iso:std:iso:20022:tech:xsd:camt.053.001.02\"><BkToCstmrStmt><Stmt>" +
		"<Acct><Id><IBAN>DE89370400440532013000</IBAN></Id><Ccy>EUR</Ccy></Acct>" +
		"<Ntry><Amt Ccy=\"EUR\">1.00</Amt><CdtDbtInd>CRDT</CdtDbtInd><BookgDt><Dt>2026-01-01</Dt></BookgDt><AddtlNtryInf>Caf\xe9</AddtlNtryInf></Ntry>" +
		"</Stmt></BkToCstmrStmt></Document>"
	recs, err := collect(t, camtRecords(ctx(), []byte(doc), CAMTOptions{}, "x", "y"))
	if err != nil {
		t.Fatal(err)
	}
	if got := camtSplit(recs).txs[0].Description; got != "Café" {
		t.Fatalf("description = %q", got)
	}
}

func FuzzCAMT(f *testing.F) {
	for _, name := range []string{"v02.xml", "v08.xml"} {
		data, err := os.ReadFile(filepath.Join("testdata", "camt", name))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		var recs []fuiproto.Record
		for r, err := range camtRecords(ctx(), data, CAMTOptions{}, "f.xml", "sha") {
			if err != nil {
				return
			}
			recs = append(recs, r)
		}
		if _, err := fuiproto.ValidateStream(func(yield func(fuiproto.Record, error) bool) {
			for _, r := range recs {
				if !yield(r, nil) {
					return
				}
			}
		}); err != nil {
			t.Fatalf("invalid stream: %v", err)
		}
	})
}
