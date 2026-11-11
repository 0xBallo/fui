package connector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/pkg/fuiproto"
)

func profile(mods ...func(*config.CSVProfile)) config.CSVProfile {
	p := config.CSVProfile{
		DateColumn: "Date", AmountColumn: "Amount", DescriptionColumn: "Description",
		Delimiter: ",", DateFormat: "2006-01-02", DecimalSeparator: ".", TargetAccount: "Checking",
	}
	for _, m := range mods {
		m(&p)
	}
	return p
}

func opts(p config.CSVProfile) csvOptions {
	return csvOptions{profile: p, currency: "EUR", file: "x.csv", sha: "abc"}
}

// run converts data and returns the transactions and the error records of the stream, after
// checking that the stream validates.
func run(t *testing.T, data string, o csvOptions) (txs []*fuiproto.TransactionRecord, errs []*fuiproto.ErrorRecord) {
	t.Helper()
	recs, err := collect(t, csvRecords(ctx(), []byte(data), o))
	if err != nil {
		t.Fatalf("csvRecords: %v", err)
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
	for _, r := range recs {
		if r.Transaction != nil {
			txs = append(txs, r.Transaction)
		}
		if r.Error != nil {
			errs = append(errs, r.Error)
		}
	}
	return txs, errs
}

func TestCSVBasic(t *testing.T) {
	txs, errs := run(t, "Date,Amount,Description\n2026-10-01,-12.50,  Coffee   bar \n2026-10-02,1000.00,Salary\n\n", opts(profile()))
	if len(errs) != 0 || len(txs) != 2 {
		t.Fatalf("txs=%d errs=%+v", len(txs), errs)
	}
	got := txs[0]
	if got.Account != "Checking" || got.Date != "2026-10-01" || got.Amount != -1250 || got.Currency != "EUR" ||
		got.Description != "Coffee bar" || got.Status != fuiproto.TransactionStatusBooked || !strings.HasPrefix(got.ID, "csv-") {
		t.Fatalf("tx = %+v", got)
	}
	if txs[1].Amount != 100000 {
		t.Fatalf("second amount = %d", txs[1].Amount)
	}
}

func TestCSVProfiles(t *testing.T) {
	tests := []struct {
		name string
		data string
		p    config.CSVProfile
		want []int64
	}{
		{
			name: "decimal comma and dd/mm/yyyy tokens", data: "Date;Amount;Description\n03/10/2026;-1.234,56;Rent\n",
			p: profile(func(p *config.CSVProfile) { p.Delimiter, p.DecimalSeparator, p.DateFormat = ";", ",", "DD/MM/YYYY" }), want: []int64{-123456},
		},
		{
			name: "go layout", data: "Date,Amount,Description\n03.10.26,5.00,x\n",
			p: profile(func(p *config.CSVProfile) { p.DateFormat = "02.01.06" }), want: []int64{500},
		},
		{
			name: "debit and credit", data: "Date,Debit,Credit,Description\n2026-10-01,10.00,,Out\n2026-10-02,,20.00,In\n2026-10-03,-5.00,,Negative debit\n",
			p: profile(func(p *config.CSVProfile) {
				p.AmountColumn, p.DebitColumn, p.CreditColumn = "", "Debit", "Credit"
			}), want: []int64{-1000, 2000, -500},
		},
		{
			name: "tab delimiter", data: "Date\tAmount\tDescription\n2026-10-01\t-1.00\tx\n",
			p: profile(func(p *config.CSVProfile) { p.Delimiter = `\t` }), want: []int64{-100},
		},
		{
			name: "skip rows", data: "Bank statement\nAccount 123\nDate,Amount,Description\n2026-10-01,-1.00,x\n",
			p: profile(func(p *config.CSVProfile) { p.SkipRows = 2 }), want: []int64{-100},
		},
		{
			name: "BOM and header case", data: "\xEF\xBB\xBFdate,AMOUNT,description\n2026-10-01,-1.00,x\n", p: profile(), want: []int64{-100},
		},
		{
			name: "accounting negative and spaces", data: "Date,Amount,Description\n2026-10-01,(1 000.00),x\n", p: profile(), want: []int64{-100000},
		},
		{
			name: "missing trailing cells", data: "Date,Amount,Description,Extra\n2026-10-01,-1.00,x\n", p: profile(), want: []int64{-100},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txs, errs := run(t, tt.data, opts(tt.p))
			if len(errs) != 0 {
				t.Fatalf("errors: %+v", errs)
			}
			var got []int64
			for _, tx := range txs {
				got = append(got, tx.Amount)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("amounts (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCSVEncodings(t *testing.T) {
	tests := map[string]struct {
		encoding string
		data     []byte
		want     string
	}{
		"latin-1":      {"latin-1", []byte("Date,Amount,Description\n2026-10-01,-1.00,Caff\xe8 \xe0 \xa3\n"), "Caffè à £"},
		"windows-1252": {"windows-1252", []byte("Date,Amount,Description\n2026-10-01,-1.00,\x80 \x93quoted\x94 \xe9\n"), "€ “quoted” é"},
		"utf-8":        {"", []byte("Date,Amount,Description\n2026-10-01,-1.00,Caffè €\n"), "Caffè €"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			txs, errs := run(t, string(tt.data), opts(profile(func(p *config.CSVProfile) { p.Encoding = tt.encoding })))
			if len(errs) != 0 || len(txs) != 1 || txs[0].Description != tt.want {
				t.Fatalf("txs=%+v errs=%+v, want description %q", txs, errs, tt.want)
			}
		})
	}
}

func TestCSVRowErrors(t *testing.T) {
	data := "Date,Amount,Description\n" +
		"not a date,1.00,a\n" +
		"2026-02-30,1.00,b\n" +
		"2026-10-01,abc,c\n" +
		"2026-10-01,1.00,\n" +
		"2026-10-02,2.00,ok\n"
	txs, errs := run(t, data, opts(profile()))
	if len(txs) != 1 || len(errs) != 4 {
		t.Fatalf("txs=%d errs=%+v", len(txs), errs)
	}
	if errs[0].Line != 2 || errs[3].Line != 5 || errs[0].Raw["Date"] != "not a date" {
		t.Fatalf("error records = %+v", errs)
	}
}

func TestCSVBothDebitAndCreditIsAnError(t *testing.T) {
	p := profile(func(p *config.CSVProfile) { p.AmountColumn, p.DebitColumn, p.CreditColumn = "", "D", "C" })
	_, errs := run(t, "Date,D,C,Description\n2026-10-01,1.00,2.00,x\n", opts(p))
	if len(errs) != 1 {
		t.Fatalf("errs = %+v", errs)
	}
}

func TestCSVLineNumbersCountSkippedRows(t *testing.T) {
	p := profile(func(p *config.CSVProfile) { p.SkipRows = 1 })
	_, errs := run(t, "junk\nDate,Amount,Description\n2026-10-01,1.00,ok\nbad,1.00,x\n", opts(p))
	if len(errs) != 1 || errs[0].Line != 4 {
		t.Fatalf("errs = %+v", errs)
	}
}

func TestCSVIDs(t *testing.T) {
	data := "Date,Amount,Description\n2026-10-01,-1.00,Coffee\n2026-10-01,-1.00,Coffee\n2026-10-01,-2.00,Coffee\n"
	first, _ := run(t, data, opts(profile()))
	second, _ := run(t, data, opts(profile()))
	if first[0].ID == first[1].ID || first[0].ID == first[2].ID {
		t.Fatalf("ids of distinct or identical rows collide: %s %s %s", first[0].ID, first[1].ID, first[2].ID)
	}
	for i := range first {
		if first[i].ID != second[i].ID {
			t.Fatalf("id %d is not deterministic: %s vs %s", i, first[i].ID, second[i].ID)
		}
	}
	withID := profile(func(p *config.CSVProfile) { p.ExternalIDColumn = "Ref" })
	txs, _ := run(t, "Date,Amount,Description,Ref\n2026-10-01,-1.00,x,BANK-7\n2026-10-02,-1.00,y,\n", opts(withID))
	if txs[0].ID != "BANK-7" || !strings.HasPrefix(txs[1].ID, "csv-") {
		t.Fatalf("ids = %s %s", txs[0].ID, txs[1].ID)
	}
}

func TestCSVSince(t *testing.T) {
	o := opts(profile())
	o.since = "2026-10-02"
	txs, _ := run(t, "Date,Amount,Description\n2026-10-01,1.00,old\n2026-10-02,1.00,new\n", o)
	if len(txs) != 1 || txs[0].Description != "new" {
		t.Fatalf("txs = %+v", txs)
	}
}

func TestCSVAccountAndKind(t *testing.T) {
	o := opts(profile())
	o.account, o.kind = "Card", fuiproto.AccountKindCard
	recs, err := collect(t, csvRecords(ctx(), []byte("Date,Amount,Description\n"), o))
	if err != nil {
		t.Fatal(err)
	}
	if a := recs[1].Account; a.Ref != "Card" || a.Name != "Card" || a.Kind != fuiproto.AccountKindCard || a.Currency != "EUR" {
		t.Fatalf("account = %+v", a)
	}
	if m := recs[0].Meta; m.Connector != "csv" || m.Source.SHA256 != "abc" || m.Source.File != "x.csv" {
		t.Fatalf("meta = %+v", m)
	}
}

func TestCSVSetupErrors(t *testing.T) {
	tests := map[string]struct {
		data string
		mod  func(*csvOptions)
	}{
		"no account":        {"Date,Amount,Description\n", func(o *csvOptions) { o.profile.TargetAccount = "" }},
		"bad currency":      {"Date,Amount,Description\n", func(o *csvOptions) { o.currency = "ZZZ" }},
		"bad since":         {"Date,Amount,Description\n", func(o *csvOptions) { o.since = "yesterday" }},
		"missing column":    {"Date,Description\n", func(*csvOptions) {}},
		"no amount columns": {"Date,Amount,Description\n", func(o *csvOptions) { o.profile.AmountColumn = "" }},
		"empty file":        {"", func(*csvOptions) {}},
		"too few lines":     {"x\n", func(o *csvOptions) { o.profile.SkipRows = 5 }},
		"bad utf-8":         {"Date,Amount,Description\n2026-10-01,1,\xff\n", func(*csvOptions) {}},
		"unknown encoding":  {"Date,Amount,Description\n", func(o *csvOptions) { o.profile.Encoding = "ebcdic" }},
		"empty column name": {"Date,Amount,Description\n", func(o *csvOptions) { o.profile.DateColumn = "" }},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			o := opts(profile())
			tt.mod(&o)
			if _, err := collect(t, csvRecords(ctx(), []byte(tt.data), o)); err == nil {
				t.Fatal("no error")
			}
		})
	}
}

func TestCSVSourceReadsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stmt.csv")
	if err := os.WriteFile(path, []byte("Date,Amount,Description\n2026-10-01,-1.00,x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	recs, err := collect(t, CSVSource{Profile: profile(), File: path, Currency: "EUR"}.Records(ctx()))
	if err != nil || len(recs) != 4 {
		t.Fatalf("records=%d err=%v", len(recs), err)
	}
	if m := recs[0].Meta; m.Source.File != "stmt.csv" || len(m.Source.SHA256) != 64 {
		t.Fatalf("meta source = %+v", m.Source)
	}
	if _, err := collect(t, CSVSource{Profile: profile(), File: path + ".missing", Currency: "EUR"}.Records(ctx())); err == nil {
		t.Fatal("missing file: no error")
	}
}

func FuzzCSV(f *testing.F) {
	f.Add([]byte("Date,Amount,Description\n2026-10-01,-12.50,Coffee\n"), uint8(0))
	f.Add([]byte("Date;Amount;Description\n03/10/2026;-1.234,56;Rent\n"), uint8(1))
	f.Add([]byte("\xEF\xBB\xBFDate,Amount,Description\n\"2026-10-01\",\"(1 000.00)\",\"a\nb\"\n"), uint8(2))
	f.Fuzz(func(t *testing.T, data []byte, variant uint8) {
		p := profile()
		switch variant % 3 {
		case 1:
			p.Delimiter, p.DecimalSeparator, p.DateFormat, p.Encoding = ";", ",", "DD/MM/YYYY", "windows-1252"
		case 2:
			p.AmountColumn, p.DebitColumn, p.CreditColumn, p.SkipRows = "", "Amount", "Description", 1
		}
		recs, err := collect(t, csvRecords(ctx(), data, opts(p)))
		if err != nil {
			return // refusing the input is fine; emitting a broken stream is not
		}
		if _, err := fuiproto.ValidateStream(func(yield func(fuiproto.Record, error) bool) {
			for _, r := range recs {
				if !yield(r, nil) {
					return
				}
			}
		}); err != nil {
			t.Fatalf("invalid stream for %q: %v", data, err)
		}
	})
}
