-- +goose Up
ALTER TABLE accounts ADD COLUMN iban TEXT CHECK (iban IS NULL OR length(iban) BETWEEN 15 AND 34);

PRAGMA user_version = 2;

-- +goose Down
SELECT 'forward-only migration';
