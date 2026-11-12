package importing_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/pkg/fuiproto"
)

// seedExport inserts two accounts, a category, a manual transaction without external id or
// description, a reconciled one and a transfer with a fee.
func seedExport(t *testing.T) env {
	t.Helper()
	e := newEnv(nil)
	a := e.seedAccount(t, "Main, the \"first\"", "", "")
	b := e.seedAccount(t, "Savings", "", "")
	var cat domain.Category
	err := e.store.WithTx(context.Background(), func(r app.Repos) (err error) {
		cat, err = r.Categories().Insert(context.Background(), domain.Category{Name: "Food", Kind: domain.CategoryExpense})
		if err != nil {
			return err
		}
		g, err := r.Transfers().Insert(context.Background(), domain.TransferGroup{Fee: money(100)})
		if err != nil {
			return err
		}
		for _, tx := range []domain.Transaction{
			{
				AccountID: a.ID, Date: date("2026-10-01"), Amount: money(-1250), Description: "Lunch, \"pizza\"", CategoryID: cat.ID,
				Notes: "with Bob", Status: domain.StatusReconciled, Kind: domain.KindPayment, ExternalID: "bank-1",
			},
			{
				AccountID: a.ID, Date: date("2026-10-02"), Amount: money(-1100), Description: "to savings",
				Status: domain.StatusBooked, Kind: domain.KindTransfer, TransferGroupID: g.ID,
			},
			{
				AccountID: b.ID, Date: date("2026-10-02"), Amount: money(1000), Description: "from main",
				Status: domain.StatusBooked, Kind: domain.KindTransfer, TransferGroupID: g.ID,
			},
			{AccountID: a.ID, Date: date("2026-10-03"), Amount: money(500), Status: domain.StatusPending, Kind: domain.KindOther},
		} {
			if _, err := r.Transactions().Insert(context.Background(), tx); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func export(t *testing.T, e env, f importing.ExportFormat, spec domain.TxFilterSpec) (string, int) {
	t.Helper()
	filter, err := domain.NewTxFilter(spec)
	if err != nil {
		t.Fatalf("NewTxFilter: %v", err)
	}
	var buf bytes.Buffer
	n, err := e.svc.Export(context.Background(), importing.ExportRequest{Format: f, Filter: filter}, &buf)
	if err != nil {
		t.Fatalf("export %s: %v", f, err)
	}
	return buf.String(), n
}

func TestExportCSV(t *testing.T) {
	e := seedExport(t)
	out, n := export(t, e, importing.FormatCSV, domain.TxFilterSpec{})
	if n != 4 || !strings.Contains(out, "\r\n") {
		t.Fatalf("n=%d, want 4 records with CRLF line ends:\n%q", n, out)
	}
	rows, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	if err != nil {
		t.Fatalf("output is not valid CSV: %v", err)
	}
	if len(rows) != 5 || rows[0][3] != "amount" {
		t.Fatalf("rows = %q", rows)
	}
	first := rows[1] // oldest first
	if first[1] != "2026-10-01" || first[2] != "Main, the \"first\"" || first[3] != "-12.50" || first[5] != "Lunch, \"pizza\"" || first[6] != "Food" {
		t.Fatalf("first row = %q", first)
	}
}

func TestExportFiltered(t *testing.T) {
	e := seedExport(t)
	_, n := export(t, e, importing.FormatCSV, domain.TxFilterSpec{From: date("2026-10-02"), To: date("2026-10-02")})
	if n != 2 {
		t.Fatalf("filtered export wrote %d transactions, want 2", n)
	}
}

func TestExportJSON(t *testing.T) {
	e := seedExport(t)
	out, _ := export(t, e, importing.FormatJSON, domain.TxFilterSpec{})
	var got []struct {
		Amount      string `json:"amount"`
		AmountMinor int64  `json:"amount_minor"`
		Category    string `json:"category"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(got) != 4 || got[0].Amount != "-12.50" || got[0].AmountMinor != -1250 || got[0].Category != "Food" {
		t.Fatalf("got = %+v", got)
	}
}

func TestExportFUIIsAValidStream(t *testing.T) {
	e := seedExport(t)
	out, n := export(t, e, importing.FormatFUI, domain.TxFilterSpec{})
	res, err := fuiproto.ValidateStream(fuiproto.DecodeJSONL(strings.NewReader(out)))
	if err != nil || len(res.RowErrors) != 0 {
		t.Fatalf("export is not a valid stream: %v %+v", err, res.RowErrors)
	}
	var txs []*fuiproto.TransactionRecord
	balances := map[string]int64{}
	for _, r := range res.Records {
		switch {
		case r.Meta != nil && r.Meta.Connector != "fui":
			t.Fatalf("meta = %+v", r.Meta)
		case r.Transaction != nil:
			txs = append(txs, r.Transaction)
		case r.Balance != nil:
			balances[r.Balance.Account] = r.Balance.Amount
		}
	}
	if n != 4 || len(txs) != 4 || res.End.Transactions != 4 {
		t.Fatalf("n=%d txs=%d end=%+v", n, len(txs), res.End)
	}
	if txs[0].ID != "bank-1" || txs[0].Status != fuiproto.TransactionStatusBooked || txs[0].Category != "Food" || txs[0].Notes != "with Bob" {
		t.Fatalf("reconciled transaction exported as %+v", txs[0])
	}
	if !strings.HasPrefix(txs[1].ID, "fui-") || txs[1].Transfer == "" || txs[1].Transfer != txs[2].Transfer {
		t.Fatalf("transfer legs = %+v %+v", txs[1], txs[2])
	}
	if txs[3].Description != "-" || txs[3].Status != fuiproto.TransactionStatusPending {
		t.Fatalf("empty description / pending exported as %+v", txs[3])
	}
	if balances["Main, the \"first\""] != -1250-1100+500 || balances["Savings"] != 1000 {
		t.Fatalf("balances = %v", balances)
	}
}

func TestExportFUIReimportIntoSameArchiveCreatesNothing(t *testing.T) {
	e := seedExport(t)
	out, _ := export(t, e, importing.FormatFUI, domain.TxFilterSpec{})
	plan := e.plan(t, fuiSource(t, []byte(out)))
	// The transaction without a description is exported as "-", so its fingerprint no longer
	// matches: it is flagged as doubtful rather than created silently.
	if c := plan.Counts(); c.Duplicate != 3 || c.Doubtful != 1 || c.New != 0 {
		t.Fatalf("counts = %+v", c)
	}
}

func TestExportUnknownFormat(t *testing.T) {
	e := newEnv(nil)
	if _, err := e.svc.Export(context.Background(), importing.ExportRequest{Format: "xml"}, io.Discard); !errors.Is(err, domain.ErrInvalidValue) {
		t.Fatalf("err = %v", err)
	}
}

type memFiles map[string]*bytes.Buffer

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

func (m memFiles) create(name string) (io.WriteCloser, error) {
	m[name] = &bytes.Buffer{}
	return nopCloser{m[name]}, nil
}

func TestDump(t *testing.T) {
	e := seedExport(t)
	e.importAll(t, stream("bank", "abc", account("r", "Imported", "EUR"), txRec("r", "a", "2026-10-01", -100, "x")))
	wantFiles := []string{"accounts", "categories", "transactions", "transfers", "fx_rates", "import_batches"}
	t.Run("json", func(t *testing.T) {
		files := memFiles{}
		if err := e.svc.Dump(context.Background(), importing.DumpJSON, files.create); err != nil {
			t.Fatal(err)
		}
		for _, name := range wantFiles {
			buf, ok := files[name+".json"]
			if !ok {
				t.Fatalf("no %s.json in %v", name, files)
			}
			var rows []map[string]any
			if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
				t.Fatalf("%s.json: %v", name, err)
			}
		}
		var txs []map[string]any
		_ = json.Unmarshal(files["transactions.json"].Bytes(), &txs)
		if len(txs) != 5 {
			t.Fatalf("transactions.json has %d rows, want 5", len(txs))
		}
	})
	t.Run("csv", func(t *testing.T) {
		files := memFiles{}
		if err := e.svc.Dump(context.Background(), importing.DumpCSV, files.create); err != nil {
			t.Fatal(err)
		}
		rows, err := csv.NewReader(files["transactions.csv"]).ReadAll()
		if err != nil || len(rows) != 6 {
			t.Fatalf("transactions.csv rows=%d err=%v", len(rows), err)
		}
		batches, _ := csv.NewReader(files["import_batches.csv"]).ReadAll()
		if len(batches) != 2 || batches[1][3] != "abc" {
			t.Fatalf("import_batches.csv = %q", batches)
		}
	})
	t.Run("unknown format", func(t *testing.T) {
		if err := e.svc.Dump(context.Background(), "xml", memFiles{}.create); !errors.Is(err, domain.ErrInvalidValue) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("create failure", func(t *testing.T) {
		boom := errors.New("disk full")
		err := e.svc.Dump(context.Background(), importing.DumpJSON, func(string) (io.WriteCloser, error) { return nil, boom })
		if !errors.Is(err, boom) {
			t.Fatalf("err = %v", err)
		}
	})
}
