package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"
)

func encodeIDSlice[T ~int64](ids []T) (string, error) {
	raw := make([]int64, 0, len(ids))
	for _, id := range ids {
		raw = append(raw, int64(id))
	}
	return encodeJSON(raw)
}

func encodeStatusSlice(statuses []domain.TxStatus) (string, error) {
	raw := make([]string, 0, len(statuses))
	for _, status := range statuses {
		raw = append(raw, string(status))
	}
	return encodeJSON(raw)
}

func encodeJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("encode json: %w", err)
	}
	return string(b), nil
}

func toAccount(row gen.Account) (domain.Account, error) {
	kind, err := domain.ParseAccountKind(row.Kind)
	if err != nil {
		return domain.Account{}, fmt.Errorf("parse account kind %q: %w", row.Kind, err)
	}
	cur, err := domain.ParseCurrency(row.Currency)
	if err != nil {
		return domain.Account{}, fmt.Errorf("parse account currency %q: %w", row.Currency, err)
	}
	opened, err := domain.ParseDate(row.OpenedOn)
	if err != nil {
		return domain.Account{}, fmt.Errorf("parse account opened_on %q: %w", row.OpenedOn, err)
	}
	opening, err := domain.NewMoney(row.OpeningBalanceMinor, cur)
	if err != nil {
		return domain.Account{}, fmt.Errorf("parse account opening balance: %w", err)
	}

	return domain.Account{
		ID:             domain.AccountID(row.ID),
		Name:           row.Name,
		Description:    row.Description,
		Kind:           kind,
		Currency:       cur,
		OpeningBalance: opening,
		OpenedOn:       opened,
		Archived:       row.Archived != 0,
		IBAN:           nullStringOrEmpty(row.Iban),
	}, nil
}

func toCategory(row gen.Category) (domain.Category, error) {
	kind, err := domain.ParseCategoryKind(row.Kind)
	if err != nil {
		return domain.Category{}, fmt.Errorf("parse category kind %q: %w", row.Kind, err)
	}
	return domain.Category{
		ID:   domain.CategoryID(row.ID),
		Name: row.Name,
		Kind: kind,
	}, nil
}

func toTransaction(row gen.Transaction) (domain.Transaction, error) {
	cur, err := domain.ParseCurrency(row.Currency)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("parse transaction currency %q: %w", row.Currency, err)
	}
	amount, err := domain.NewMoney(row.AmountMinor, cur)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("parse transaction amount: %w", err)
	}
	date, err := domain.ParseDate(row.Date)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("parse transaction date %q: %w", row.Date, err)
	}
	status, err := domain.ParseTxStatus(row.Status)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("parse transaction status %q: %w", row.Status, err)
	}
	kind, err := domain.ParseTxKind(row.Kind)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("parse transaction kind %q: %w", row.Kind, err)
	}

	tx := domain.Transaction{
		ID:            domain.TransactionID(row.ID),
		AccountID:     domain.AccountID(row.AccountID),
		Date:          date,
		Amount:        amount,
		PayeeID:       domain.PayeeID(nullInt64OrZero(row.PayeeID)),
		Description:   row.Description,
		CategoryID:    domain.CategoryID(nullInt64OrZero(row.CategoryID)),
		Notes:         row.Notes,
		Status:        status,
		Kind:          kind,
		ExternalID:    nullStringOrEmpty(row.ExternalID),
		Fingerprint:   row.Fingerprint,
		ImportBatchID: domain.ImportBatchID(nullInt64OrZero(row.ImportBatchID)),
		TransferGroupID: domain.TransferGroupID(
			nullInt64OrZero(row.TransferGroupID),
		),
		RecurrenceID: domain.RecurrenceID(nullInt64OrZero(row.RecurrenceID)),
	}

	if row.OperationDate.Valid {
		tx.OperationDate, err = domain.ParseDate(row.OperationDate.String)
		if err != nil {
			return domain.Transaction{}, fmt.Errorf("parse operation_date %q: %w", row.OperationDate.String, err)
		}
	}
	if row.ValueDate.Valid {
		tx.ValueDate, err = domain.ParseDate(row.ValueDate.String)
		if err != nil {
			return domain.Transaction{}, fmt.Errorf("parse value_date %q: %w", row.ValueDate.String, err)
		}
	}
	if row.DueDate.Valid {
		tx.DueDate, err = domain.ParseDate(row.DueDate.String)
		if err != nil {
			return domain.Transaction{}, fmt.Errorf("parse due_date %q: %w", row.DueDate.String, err)
		}
	}
	return tx, nil
}

func toFXRate(row gen.FxRate) (domain.FXRate, error) {
	cur, err := domain.ParseCurrency(row.Currency)
	if err != nil {
		return domain.FXRate{}, fmt.Errorf("parse fx currency %q: %w", row.Currency, err)
	}
	date, err := domain.ParseDate(row.Date)
	if err != nil {
		return domain.FXRate{}, fmt.Errorf("parse fx date %q: %w", row.Date, err)
	}
	rate, err := domain.NewRate(row.RateScaled)
	if err != nil {
		return domain.FXRate{}, fmt.Errorf("parse fx rate: %w", err)
	}
	fx, err := domain.NewFXRate(cur, date, rate)
	if err != nil {
		return domain.FXRate{}, fmt.Errorf("parse fx row: %w", err)
	}
	return fx, nil
}

func toImportBatch(row gen.ImportBatch) (app.ImportBatch, error) {
	importedAt, err := time.Parse(time.RFC3339Nano, row.ImportedAt)
	if err != nil {
		return app.ImportBatch{}, fmt.Errorf("parse imported_at %q: %w", row.ImportedAt, err)
	}
	return app.ImportBatch{
		ID:           domain.ImportBatchID(row.ID),
		Connector:    row.Connector,
		Source:       row.Source,
		SourceSHA256: row.SourceSha256,
		ImportedAt:   importedAt.UTC(),
	}, nil
}

func toNullString(v string) sql.NullString {
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}

func toNullInt64[T ~int64](v T) sql.NullInt64 {
	if v == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(v), Valid: true}
}

func toNullDate(v domain.Date) sql.NullString {
	if v.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: v.String(), Valid: true}
}

func nullInt64OrZero(v sql.NullInt64) int64 {
	if !v.Valid {
		return 0
	}
	return v.Int64
}

func nullStringOrEmpty(v sql.NullString) string {
	if !v.Valid {
		return ""
	}
	return v.String
}
