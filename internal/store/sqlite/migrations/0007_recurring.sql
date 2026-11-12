-- +goose Up
CREATE TABLE recurrences (
    id INTEGER PRIMARY KEY,
    account_id INTEGER NOT NULL REFERENCES accounts (id) ON DELETE RESTRICT,
    category_id INTEGER REFERENCES categories (id) ON DELETE RESTRICT,
    description TEXT NOT NULL DEFAULT '',
    amount_minor INTEGER NOT NULL,
    currency TEXT NOT NULL CHECK (length(currency) = 3),
    every INTEGER NOT NULL CHECK (every > 0),
    unit TEXT NOT NULL CHECK (unit IN ('days', 'weeks', 'months')),
    day_of_month INTEGER NOT NULL DEFAULT 0 CHECK (day_of_month >= 0 AND day_of_month <= 31),
    start_date TEXT NOT NULL CHECK (length(start_date) = 10),
    end_date TEXT CHECK (end_date IS NULL OR length(end_date) = 10),
    next_date TEXT NOT NULL CHECK (length(next_date) = 10),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE INDEX idx_recurrences_next_date_id ON recurrences (next_date, id);
CREATE INDEX idx_recurrences_account_next_date ON recurrences (account_id, next_date, id);

ALTER TABLE transactions ADD COLUMN recurrence_id INTEGER REFERENCES recurrences (id) ON DELETE SET NULL;
ALTER TABLE transactions ADD COLUMN due_date TEXT CHECK (due_date IS NULL OR length(due_date) = 10);

CREATE INDEX idx_transactions_recurrence_status_due_date
    ON transactions (recurrence_id, status, due_date, id);

PRAGMA user_version = 7;

-- +goose Down
SELECT 'forward-only migration';
