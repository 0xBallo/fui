package render

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/domain"
)

func TestImportDetailLines(t *testing.T) {
	day, err := domain.ParseDate("2026-10-05")
	if err != nil {
		t.Fatal(err)
	}
	p := importing.ImportPlan{
		Warnings:      []importing.RowWarning{{Line: 3, Message: "odd"}},
		NewCategories: []domain.Category{{Name: "Groceries"}},
		NewTags:       []domain.Tag{{Name: "weekly"}},
		NewPayees:     []domain.Payee{{Name: "Shop"}},
		Errors:        []importing.RowError{{Line: 4, Message: "bad"}},
		Discrepancies: []importing.BalanceDiscrepancy{{AccountRef: "a", Line: 5, Date: day, Expected: 1, Actual: 2}},
		Transfers: []importing.TransferProposal{
			{Row: 0, Other: 1, Reason: domain.TransferMatchByAmountAndDate},
			{Row: 2, Other: -1, Existing: 7, Reason: domain.TransferMatchByCounterpartyIBAN, Accept: true},
		},
		BalanceMismatches: []importing.BalanceMismatch{{AccountRef: "a", Date: day, Expected: 9, Computed: 8, Suggest: "fui reconcile A 9"}},
		ModuleRecords:     []importing.ModuleRecords{{Module: "investments", Count: 2, Kinds: []string{"security_op"}}},
	}
	want := []string{
		"warning: line 3: odd",
		"new category: Groceries",
		"new tag: weekly",
		"new payee: Shop",
		"error: line 4: bad",
		"discrepancy: line 5: a 2026-10-05 balance 2, expected 1",
		"transfer proposal: #0 row 0 <-> row 1 (amount_date, not accepted)",
		"transfer proposal: #1 row 2 <-> existing transaction 7 (counterparty_iban, accepted)",
		"balance mismatch: a on 2026-10-05: statement 9, computed 8; record it with: fui reconcile A 9",
		"module investments is disabled: 2 record(s) (security_op) skipped; use --enable-modules investments",
	}
	if diff := cmp.Diff(want, importDetailLines(p)); diff != "" {
		t.Errorf("detail lines mismatch (-want +got):\n%s", diff)
	}
	var b strings.Builder
	if err := WriteImportPlan(&b, JSON, p); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{`"new_payees":["Shop"]`, `"balance_mismatches":[{"account_ref":"a"`, `"module_records":[{"module":"investments","count":2`} {
		if !strings.Contains(b.String(), s) {
			t.Errorf("json = %s, want %s", b.String(), s)
		}
	}
}

func TestWriteImportResultWarnings(t *testing.T) {
	res := importing.ImportResult{Warnings: []importing.RowWarning{{Line: 6, Message: "skipped"}}}
	var tbl, js strings.Builder
	if err := WriteImportResult(&tbl, Table, res); err != nil {
		t.Fatal(err)
	}
	if err := WriteImportResult(&js, JSON, res); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tbl.String(), "warning: line 6: skipped") || !strings.Contains(js.String(), `"warnings":[{"line":6,"message":"skipped"}]`) {
		t.Errorf("table=%q json=%q", tbl.String(), js.String())
	}
}

func TestWriteTxShowsOriginalAndRaw(t *testing.T) {
	eur, _ := domain.ParseCurrency("EUR")
	usd, _ := domain.ParseCurrency("USD")
	amt, _ := domain.NewMoney(-1200, eur)
	orig, _ := domain.NewMoney(-1400, usd)
	v := app.TxView{Account: "A", Transaction: domain.Transaction{Amount: amt, Original: &orig, Raw: `{"memo":"x"}`}}
	var tbl, js strings.Builder
	if err := WriteTx(&tbl, Table, domain.LocaleENUS, false, v); err != nil {
		t.Fatal(err)
	}
	if err := WriteTx(&js, JSON, domain.LocaleENUS, false, v); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"original: USD -14.00", `raw: {"memo":"x"}`} {
		if !strings.Contains(tbl.String(), s) {
			t.Errorf("table = %q, want %q", tbl.String(), s)
		}
	}
	if !strings.Contains(js.String(), `"original":{"amount":-1400,"currency":"USD"}`) {
		t.Errorf("json = %q", js.String())
	}
}
