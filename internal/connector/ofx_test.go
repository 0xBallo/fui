package connector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/pkg/fuiproto"
)

func loadOFXFixture(t *testing.T, name string) []byte {
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
	if seed, err := os.ReadFile(filepath.Join("testdata", "ofx-bank-v1.ofx")); err == nil {
		f.Add(seed)
	}
	if seed, err := os.ReadFile(filepath.Join("testdata", "ofx-card-v2.xml")); err == nil {
		f.Add(seed)
	}
	f.Fuzz(func(_ *testing.T, data []byte) {
		_ = parseOFXForFuzz(data)
	})
}

// parseOFXForFuzz parses arbitrary bytes and validates the generated stream.
func parseOFXForFuzz(data []byte) error {
	stmts, err := parseOFX(data)
	if err != nil {
		return err
	}
	recs := make([]fuiproto.Record, 0, 4)
	recs = append(recs, fuiproto.Record{Type: fuiproto.RecordTypeMeta, Meta: &fuiproto.MetaRecord{
		Protocol: fuiproto.ProtocolFUI1, Connector: ofxConnector, Version: ofxVersion,
		Source: fuiproto.SourceMeta{File: "fuzz.ofx", SHA256: strings.Repeat("0", 64)},
	}})
	txs := 0
	for _, stmt := range stmts {
		recs = append(recs, fuiproto.Record{Type: fuiproto.RecordTypeAccount, Account: &stmt.account})
		for _, tx := range stmt.transactions {
			txs++
			recs = append(recs, fuiproto.Record{Type: fuiproto.RecordTypeTransaction, Transaction: &fuiproto.TransactionRecord{
				Account: stmt.account.Ref, ID: tx.ID, Date: tx.Date.String(), Amount: tx.Amount, Currency: stmt.account.Currency,
				Description: tx.Description, Status: fuiproto.TransactionStatusBooked, Kind: tx.Kind,
			}})
		}
	}
	recs = append(recs, fuiproto.Record{Type: fuiproto.RecordTypeEnd, End: &fuiproto.EndRecord{Transactions: txs, Errors: 0}})
	_, err = fuiproto.ValidateStream(func(yield func(fuiproto.Record, error) bool) {
		for _, rec := range recs {
			if !yield(rec, nil) {
				return
			}
		}
	})
	return err
}

// decodeOFXBodyForTest returns the normalized OFX body for white-box tests.
func decodeOFXBodyForTest(data []byte) (string, error) {
	text, err := decodeOFXText(data)
	if err != nil {
		return "", err
	}
	return normalizeOFX(text)
}
