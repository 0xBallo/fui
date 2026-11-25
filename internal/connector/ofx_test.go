package connector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/pkg/fuiproto"
)

func loadOFXFixture(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func runOFX(t *testing.T, data []byte, since string) ([]*fuiproto.AccountRecord, []*fuiproto.TransactionRecord) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "statement.ofx")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	recs, err := collect(t, NewOFX(path, since).Records(ctx()))
	if err != nil {
		t.Fatalf("OFX records: %v", err)
	}
	if _, err := fuiproto.ValidateStream(func(yield func(fuiproto.Record, error) bool) {
		for _, rec := range recs {
			if !yield(rec, nil) {
				return
			}
		}
	}); err != nil {
		t.Fatalf("stream is invalid: %v", err)
	}
	var accts []*fuiproto.AccountRecord
	var txs []*fuiproto.TransactionRecord
	for _, rec := range recs {
		if rec.Account != nil {
			accts = append(accts, rec.Account)
		}
		if rec.Transaction != nil {
			txs = append(txs, rec.Transaction)
		}
	}
	return accts, txs
}

func TestOFXSGMLBankStatement(t *testing.T) {
	accts, txs := runOFX(t, loadOFXFixture(t, "ofx-bank-v1.ofx"), "")
	if len(accts) != 1 || len(txs) != 2 {
		t.Fatalf("accounts=%d txs=%d", len(accts), len(txs))
	}
	if got, want := accts[0].Ref, "IT60X0542811101000000123456"; got != want {
		t.Fatalf("account ref = %q, want %q", got, want)
	}
	if got, want := accts[0].IBAN, "IT60X0542811101000000123456"; got != want {
		t.Fatalf("iban = %q, want %q", got, want)
	}
	first := txs[0]
	if first.ID != "TRX-1" || first.Date != "2026-10-03" || first.Amount != -1250 || first.Currency != "EUR" ||
		first.Kind != fuiproto.TransactionKindPayment || first.Description != "Coffee bar - Milan branch" {
		t.Fatalf("first tx = %+v", first)
	}
	second := txs[1]
	if second.Amount != 102025 || second.Kind != fuiproto.TransactionKindRefund {
		t.Fatalf("second tx = %+v", second)
	}
}

func TestOFXXMLCardStatementAndSince(t *testing.T) {
	accts, txs := runOFX(t, loadOFXFixture(t, "ofx-card-v2.xml"), "2026-10-04")
	if len(accts) != 1 || len(txs) != 1 {
		t.Fatalf("accounts=%d txs=%d", len(accts), len(txs))
	}
	if got, want := accts[0].Kind, fuiproto.AccountKindCard; got != want {
		t.Fatalf("kind = %q, want %q", got, want)
	}
	if got, want := txs[0].ID, "ofx-"; !strings.HasPrefix(got, want) {
		t.Fatalf("id = %q, want prefix %q", got, want)
	}
	if got, want := txs[0].Kind, fuiproto.TransactionKindFee; got != want {
		t.Fatalf("kind = %q, want %q", got, want)
	}
}

func TestOFXCP1252(t *testing.T) {
	data := []byte(
		"OFXHEADER:100\n" +
			"DATA:OFXSGML\n" +
			"VERSION:102\n" +
			"CHARSET:1252\n\n" +
			"<OFX><BANKMSGSRSV1><STMTTRNRS><STMTRS><CURDEF>EUR<BANKACCTFROM><BANKID>1<ACCTID>123</BANKACCTFROM>" +
			"<BANKTRANLIST><STMTTRN><TRNTYPE>DEBIT<DTPOSTED>20261003<TRNAMT>-1.00<FITID>1<NAME>Caf\xe9 \x80</STMTTRN></BANKTRANLIST>" +
			"</STMTRS></STMTTRNRS></BANKMSGSRSV1></OFX>",
	)
	_, txs := runOFX(t, data, "")
	if len(txs) != 1 || txs[0].Description != "Café €" {
		t.Fatalf("txs = %+v", txs)
	}
}

func TestOFXMalformedFileReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.ofx")
	if err := os.WriteFile(path, []byte("not an ofx"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := collect(t, NewOFX(path, "").Records(ctx())); err == nil {
		t.Fatal("missing parse error")
	}
}

func TestOFXDateAndAmountParsing(t *testing.T) {
	tests := []struct {
		name   string
		date   string
		amount string
		wantD  string
		wantA  int64
	}{
		{"timezone suffix", "20261003120000[-5:EST]", "-12,50", "2026-10-03", -1250},
		{"dot decimal", "20261004", "1000.25", "2026-10-04", 100025},
		{"thousands comma", "20261005", "1,234.56", "2026-10-05", 123456},
		{"thousands dot", "20261006", "1.234,56", "2026-10-06", 123456},
		{"negative parenthesis", "20261007", "(12.34)", "2026-10-07", -1234},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			date, err := parseOFXDate(tt.date)
			if err != nil {
				t.Fatalf("parseOFXDate: %v", err)
			}
			amount, err := parseOFXAmount(tt.amount)
			if err != nil {
				t.Fatalf("parseOFXAmount: %v", err)
			}
			if diff := cmp.Diff(tt.wantD, date.String()); diff != "" {
				t.Fatalf("date mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantA, amount); diff != "" {
				t.Fatalf("amount mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDecodeOFXBodyForSGML(t *testing.T) {
	body, err := decodeOFXBodyForTest(loadOFXFixture(t, "ofx-bank-v1.ofx"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "<TRNTYPE>DEBIT") || !strings.Contains(body, "</TRNTYPE>") {
		t.Fatalf("normalized body does not close TRNTYPE tag:\n%s", body)
	}
	if !strings.Contains(body, "<DTPOSTED>20261003120000[-5:EST]") || !strings.Contains(body, "</DTPOSTED>") {
		t.Fatalf("normalized body does not close DTPOSTED tag:\n%s", body)
	}
}

func FuzzOFX(f *testing.F) {
	for _, name := range []string{"ofx-bank-v1.ofx", "ofx-card-v2.xml", "ofx-errors.ofx"} {
		f.Add(loadOFXFixture(f, name))
	}
	f.Add([]byte("<OFX><BANKMSGSRSV1><STMTTRNRS><STMTRS><CURDEF>EUR<BANKACCTFROM><ACCTID>1</BANKACCTFROM>" +
		"<BANKTRANLIST><STMTTRN><DTPOSTED>x<TRNAMT>1</STMTTRN></BANKTRANLIST><LEDGERBAL><BALAMT>x<DTASOF>y</LEDGERBAL></STMTRS></STMTTRNRS></BANKMSGSRSV1></OFX>"))
	f.Fuzz(func(t *testing.T, data []byte) {
		var recs []fuiproto.Record
		for r, err := range ofxRecords(ctx(), data, "", "fuzz.ofx", strings.Repeat("0", 64)) {
			if err != nil {
				return
			}
			if r.Error != nil && r.Error.Line < 1 {
				t.Fatalf("error record with line %d", r.Error.Line)
			}
			recs = append(recs, r)
		}
		if _, err := fuiproto.ValidateStream(func(yield func(fuiproto.Record, error) bool) {
			for _, rec := range recs {
				if !yield(rec, nil) {
					return
				}
			}
		}); err != nil {
			t.Fatalf("invalid stream: %v", err)
		}
	})
}

// decodeOFXBodyForTest returns the normalized OFX body for white-box tests.
func decodeOFXBodyForTest(data []byte) (string, error) {
	text, err := decodeOFXText(data)
	if err != nil {
		return "", err
	}
	return normalizeOFX(text)
}

func ofxStream(t *testing.T, data []byte) (bal []fuiproto.BalanceRecord, errs []fuiproto.ErrorRecord, txs []fuiproto.TransactionRecord, end fuiproto.EndRecord) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "s.ofx")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	recs, err := collect(t, NewOFX(path, "").Records(ctx()))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range recs {
		switch {
		case r.Balance != nil:
			bal = append(bal, *r.Balance)
		case r.Error != nil:
			errs = append(errs, *r.Error)
		case r.Transaction != nil:
			txs = append(txs, *r.Transaction)
		case r.End != nil:
			end = *r.End
		}
	}
	return bal, errs, txs, end
}

func TestOFXBadTransactionsAreErrorRecords(t *testing.T) {
	_, errs, txs, end := ofxStream(t, loadOFXFixture(t, "ofx-errors.ofx"))
	if len(txs) != 2 || txs[0].ID != "OK-1" || txs[1].ID != "OK-2" {
		t.Fatalf("txs = %+v", txs)
	}
	if len(errs) != 2 || errs[0].Line != 30 || errs[1].Line != 37 {
		t.Fatalf("errs = %+v, want lines 30 and 37", errs)
	}
	if end.Errors != 2 || end.Transactions != 2 {
		t.Fatalf("end = %+v", end)
	}
}

func TestOFXXMLErrorLine(t *testing.T) {
	doc := "<?xml version=\"1.0\"?>\n<OFX>\n<BANKMSGSRSV1><STMTTRNRS><STMTRS><CURDEF>EUR</CURDEF>\n" +
		"<BANKACCTFROM><ACCTID>A1</ACCTID></BANKACCTFROM><BANKTRANLIST>\n" +
		"<STMTTRN><DTPOSTED>20260101</DTPOSTED><TRNAMT>1.00</TRNAMT><NAME>ok</NAME></STMTTRN>\n" +
		"<STMTTRN><DTPOSTED>20260102</DTPOSTED><TRNAMT>zz</TRNAMT><NAME>bad</NAME></STMTTRN>\n" +
		"</BANKTRANLIST></STMTRS></STMTTRNRS></BANKMSGSRSV1></OFX>\n"
	_, errs, txs, _ := ofxStream(t, []byte(doc))
	if len(txs) != 1 || len(errs) != 1 || errs[0].Line != 6 {
		t.Fatalf("txs=%d errs=%+v, want 1 tx and an error on line 6", len(txs), errs)
	}
}

func TestOFXLedgerBalance(t *testing.T) {
	const head = "<OFX><BANKMSGSRSV1><STMTTRNRS><STMTRS><CURDEF>EUR</CURDEF><BANKACCTFROM><ACCTID>A1</ACCTID></BANKACCTFROM>"
	const tail = "</STMTRS></STMTTRNRS></BANKMSGSRSV1></OFX>"
	tests := []struct {
		name, ledger string
		wantBal      []fuiproto.BalanceRecord
		wantErrs     int
	}{
		{
			"present", "<LEDGERBAL><BALAMT>-10.50</BALAMT><DTASOF>20261031120000</DTASOF></LEDGERBAL>",
			[]fuiproto.BalanceRecord{{Account: "A1", Date: "2026-10-31", Amount: -1050}},
			0,
		},
		{"absent", "", nil, 0},
		{"bad amount", "<LEDGERBAL><BALAMT>x</BALAMT><DTASOF>20261031</DTASOF></LEDGERBAL>", nil, 1},
		{"bad date", "<LEDGERBAL><BALAMT>1.00</BALAMT><DTASOF>nope</DTASOF></LEDGERBAL>", nil, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bal, errs, _, end := ofxStream(t, []byte(head+tt.ledger+tail))
			if diff := cmp.Diff(tt.wantBal, bal); diff != "" {
				t.Fatalf("balances (-want +got):\n%s", diff)
			}
			if len(errs) != tt.wantErrs || end.Errors != tt.wantErrs {
				t.Fatalf("errs=%+v end=%+v", errs, end)
			}
			for _, e := range errs {
				if e.Line != 1 {
					t.Fatalf("line = %d", e.Line)
				}
			}
		})
	}
}
