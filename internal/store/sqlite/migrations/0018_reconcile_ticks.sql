-- +goose Up
-- Transactions ticked against a statement while an account reconciliation is in progress.
CREATE TABLE reconcile_ticks (
    account_id INTEGER NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    transaction_id INTEGER NOT NULL REFERENCES transactions (id) ON DELETE CASCADE,
    PRIMARY KEY (account_id, transaction_id)
) STRICT, WITHOUT ROWID;

PRAGMA user_version = 18;

-- +goose Down
SELECT 'forward-only migration';
