package cli_test

import (
	"strings"
	"testing"
)

// TestCSVIsLocaleIndependent covers the technical spec's "Output" rule for --csv: RFC 4180 with a
// comma separator and a dot decimal, independent of locale. Every CSV-capable command is run under
// it-IT (comma decimal, dd/mm/yyyy dates) and the CSV output must still show dot-decimal amounts
// and ISO YYYY-MM-DD dates, never the it-IT display format.
func TestCSVIsLocaleIndependent(t *testing.T) {
	t.Run("account list", func(t *testing.T) {
		h := newHarness(t)
		h.useItalianLocale()
		h.account("Checking", "checking", "EUR", "1000")

		csv, _ := h.mustRun("account", "list", "--csv")
		if !strings.Contains(csv, "Checking,checking,EUR,1000.00,no") {
			t.Errorf("account list --csv under it-IT = %q, want machine-format balance 1000.00", csv)
		}
		if strings.Contains(csv, "1.000,00") {
			t.Errorf("account list --csv under it-IT = %q, leaked locale-formatted balance", csv)
		}
	})

	t.Run("tx list", func(t *testing.T) {
		h := newHarness(t)
		h.useItalianLocale()
		h.account("Checking", "checking", "EUR", "0")
		h.category("Groceries", "expense")
		h.mustRun("add", "-12,50", "Esselunga", "-c", "Groceries", "-a", "Checking", "--date", "2026-01-05")

		csv, _ := h.mustRun("tx", "list", "--csv")
		if !strings.Contains(csv, "2026-01-05") {
			t.Errorf("tx list --csv under it-IT = %q, want ISO date 2026-01-05", csv)
		}
		if !strings.Contains(csv, "-12.50") {
			t.Errorf("tx list --csv under it-IT = %q, want machine-format amount -12.50", csv)
		}
		if strings.Contains(csv, "05/01/2026") || strings.Contains(csv, "-12,50") {
			t.Errorf("tx list --csv under it-IT = %q, leaked locale-formatted date/amount", csv)
		}
	})

	t.Run("category list", func(t *testing.T) {
		h := newHarness(t)
		h.useItalianLocale()
		h.category("Groceries", "expense")

		csv, _ := h.mustRun("category", "list", "--csv")
		if !strings.Contains(csv, "Groceries") {
			t.Errorf("category list --csv under it-IT = %q, want Groceries", csv)
		}
	})

	t.Run("fx list", func(t *testing.T) {
		h := newHarness(t)
		h.useItalianLocale()
		h.mustRun("fx", "set", "USD", "1.0842", "--date", "2026-01-01")

		csv, _ := h.mustRun("fx", "list", "--csv")
		if !strings.Contains(csv, "2026-01-01") || !strings.Contains(csv, "1.0842") {
			t.Errorf("fx list --csv under it-IT = %q, want ISO date and plain rate", csv)
		}
	})

	t.Run("report networth", func(t *testing.T) {
		h := newHarness(t)
		h.useItalianLocale()
		seedSpendingHistory(h)

		csv, _ := h.mustRun("report", "networth", "--period", "2026-11", "--csv")
		if !strings.Contains(csv, "2026-11-30") {
			t.Errorf("report networth --csv under it-IT = %q, want ISO date 2026-11-30", csv)
		}
		if strings.Contains(csv, "30/11/2026") {
			t.Errorf("report networth --csv under it-IT = %q, leaked locale-formatted date", csv)
		}
	})

	t.Run("forecast", func(t *testing.T) {
		h := newHarness(t)
		h.useItalianLocale()
		seedSpendingHistory(h)

		csv, _ := h.mustRun("forecast", "--horizon", "3", "--csv")
		if !strings.Contains(csv, "DATE,TOTAL") {
			t.Errorf("forecast --csv under it-IT = %q, want csv header", csv)
		}
		if strings.Contains(csv, "1.000,00") || strings.Contains(csv, "950,00") {
			t.Errorf("forecast --csv under it-IT = %q, leaked locale-formatted amounts", csv)
		}
	})

	t.Run("kpi runway", func(t *testing.T) {
		h := newHarness(t)
		h.useItalianLocale()
		seedSpendingHistory(h)

		csv, _ := h.mustRun("kpi", "runway", "--csv", "--at", "2026-11-10")
		if !strings.Contains(csv, "2026-11-10") {
			t.Errorf("kpi runway --csv under it-IT = %q, want ISO as-of date 2026-11-10", csv)
		}
		if strings.Contains(csv, "10/11/2026") {
			t.Errorf("kpi runway --csv under it-IT = %q, leaked locale-formatted date", csv)
		}
	})

	t.Run("import list", func(t *testing.T) {
		h := newHarness(t)
		h.useItalianLocale()
		file := writeFUIFile(t, t.TempDir(), "statement.fui.jsonl",
			fuiStream("fui", "statement.fui.jsonl", "sha-1",
				fuiAccount("acc-1", "Checking", "EUR"),
				fuiTx("acc-1", "tx-1", "2026-11-01", -1000, "Pizza"),
			)...)
		h.mustRun("import", file, "--create-accounts", "--yes", "--json")

		csv, _ := h.mustRun("import", "list", "--csv")
		if !strings.Contains(csv, "fui") || !strings.Contains(csv, "statement.fui.jsonl") {
			t.Errorf("import list --csv under it-IT = %q, want the batch just created", csv)
		}
	})
}
