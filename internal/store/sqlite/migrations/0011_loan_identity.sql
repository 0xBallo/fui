-- +goose Up
-- A loan gets a name and the liability account that carries its remaining debt.
-- Both columns are appended with ALTER TABLE (SQLite cannot add a NOT NULL column
-- without a default), so the domain layer enforces they are set; existing loans are
-- backfilled with a "Loan <id>" liability account so no row is left without one.
ALTER TABLE loans ADD COLUMN name TEXT NOT NULL DEFAULT '';
ALTER TABLE loans ADD COLUMN liability_account_id INTEGER REFERENCES accounts (id) ON DELETE RESTRICT;

INSERT INTO accounts (name, description, kind, currency, opening_balance_minor, opened_on)
SELECT 'Loan ' || id, '', 'loan', currency, -principal_minor, disbursed_on
FROM loans;

UPDATE loans
SET name = 'Loan ' || id,
    liability_account_id = (SELECT a.id FROM accounts AS a WHERE a.name = 'Loan ' || loans.id);

CREATE UNIQUE INDEX idx_loans_name ON loans (name COLLATE NOCASE);

CREATE UNIQUE INDEX idx_loans_liability_account
    ON loans (liability_account_id) WHERE liability_account_id IS NOT NULL;

PRAGMA user_version = 11;

-- +goose Down
SELECT 'forward-only migration';
