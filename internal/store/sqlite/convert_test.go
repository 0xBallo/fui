package sqlite

import (
	"database/sql"
	"testing"

	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"
)

func TestConvertHelpers(t *testing.T) {
	account, err := toAccount(gen.Account{
		ID:                  1,
		Name:                "A",
		Description:         "desc",
		Kind:                "checking",
		Currency:            "EUR",
		OpeningBalanceMinor: 10,
		OpenedOn:            "2026-01-01",
		Archived:            1,
	})
	if err != nil {
		t.Fatalf("toAccount: %v", err)
	}
	if !account.Archived || account.ID != 1 {
		t.Fatalf("account = %+v", account)
	}

	category, err := toCategory(gen.Category{ID: 2, Name: "Food", Kind: "expense"})
	if err != nil {
		t.Fatalf("toCategory: %v", err)
	}
	if category.Kind != domain.CategoryExpense {
		t.Fatalf("category kind = %s", category.Kind)
	}

	fx, err := toFXRate(gen.FxRate{Currency: "USD", Date: "2026-01-02", RateScaled: 900_000_000})
	if err != nil {
		t.Fatalf("toFXRate: %v", err)
	}
	if fx.Currency.Code() != "USD" {
		t.Fatalf("fx currency = %s", fx.Currency.Code())
	}
}

func TestToTransactionAndImportBatch(t *testing.T) {
	tx, err := toTransaction(gen.Transaction{
		ID:              1,
		AccountID:       2,
		Date:            "2026-01-02",
		OperationDate:   sql.NullString{String: "2026-01-01", Valid: true},
		ValueDate:       sql.NullString{String: "2026-01-03", Valid: true},
		AmountMinor:     -100,
		Currency:        "EUR",
		PayeeID:         sql.NullInt64{Int64: 8, Valid: true},
		Description:     "x",
		CategoryID:      sql.NullInt64{Int64: 9, Valid: true},
		Notes:           "n",
		Status:          "booked",
		Kind:            "payment",
		ExternalID:      sql.NullString{String: "ext", Valid: true},
		Fingerprint:     "fp",
		ImportBatchID:   sql.NullInt64{Int64: 3, Valid: true},
		TransferGroupID: sql.NullInt64{Int64: 4, Valid: true},
	})
	if err != nil {
		t.Fatalf("toTransaction: %v", err)
	}
	if tx.OperationDate.IsZero() || tx.ValueDate.IsZero() || tx.PayeeID != 8 {
		t.Fatalf("transaction = %+v", tx)
	}

	batch, err := toImportBatch(gen.ImportBatch{
		ID:           1,
		Connector:    "csv",
		Source:       "a.csv",
		SourceSha256: "aa",
		ImportedAt:   "2026-01-02T03:04:05Z",
	})
	if err != nil {
		t.Fatalf("toImportBatch: %v", err)
	}
	if batch.ID != 1 || batch.Connector != "csv" {
		t.Fatalf("batch = %+v", batch)
	}
}

func TestEncodeHelpers(t *testing.T) {
	s, err := encodeIDSlice([]domain.AccountID{1, 2, 3})
	if err != nil || s != "[1,2,3]" {
		t.Fatalf("encodeIDSlice = %q, %v", s, err)
	}
	s, err = encodeStatusSlice([]domain.TxStatus{domain.StatusBooked, domain.StatusPending})
	if err != nil || s != "[\"booked\",\"pending\"]" {
		t.Fatalf("encodeStatusSlice = %q, %v", s, err)
	}
	if got := nullInt64OrZero(sql.NullInt64{}); got != 0 {
		t.Fatalf("nullInt64OrZero = %d", got)
	}
	if got := nullStringOrEmpty(sql.NullString{}); got != "" {
		t.Fatalf("nullStringOrEmpty = %q", got)
	}
}
