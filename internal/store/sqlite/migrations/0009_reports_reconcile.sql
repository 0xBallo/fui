-- +goose Up
CREATE TABLE saved_reports (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    kind TEXT NOT NULL,
    params TEXT NOT NULL CHECK (json_valid(params)),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE TABLE reconciliations (
    id INTEGER PRIMARY KEY,
    account_id INTEGER NOT NULL REFERENCES accounts (id) ON DELETE RESTRICT,
    date TEXT NOT NULL CHECK (length(date) = 10),
    statement_balance_minor INTEGER NOT NULL,
    currency TEXT NOT NULL CHECK (length(currency) = 3),
    closed_at TEXT CHECK (closed_at IS NULL OR length(closed_at) >= 20),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (account_id, date)
) STRICT;

CREATE INDEX idx_reconciliations_account_closed_date
    ON reconciliations (account_id, closed_at, date DESC, id DESC);

PRAGMA user_version = 9;

-- +goose Down
SELECT 'forward-only migration';
