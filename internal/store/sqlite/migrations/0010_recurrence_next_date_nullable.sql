-- +goose NO TRANSACTION
-- +goose Up
-- SQLite cannot relax a column's NOT NULL/CHECK constraints in place, so
-- recurrences is rebuilt. A NULL next_date now represents an exhausted
-- recurrence (one with an end date that has been fully advanced past).
--
-- transactions.recurrence_id has a foreign key to this table, so it must be
-- built under its final name without ever renaming the live "recurrences"
-- table away: SQLite rewrites other tables' foreign key clauses to follow a
-- RENAME, and would otherwise leave transactions pointing at a scratch name
-- once that scratch table is dropped. Building as recurrences_new, dropping
-- the old recurrences, then renaming recurrences_new into place keeps
-- transactions' REFERENCES clause (which already says "recurrences")
-- correct throughout. Ids are preserved so recurrence_id keeps resolving.
--
-- Foreign keys are disabled for the rebuild: with them on, SQLite applies
-- recurrences' ON DELETE SET NULL action to transactions.recurrence_id when
-- the old recurrences table is dropped, wiping every link. PRAGMA
-- foreign_keys is a no-op inside a transaction, hence NO TRANSACTION above.
PRAGMA foreign_keys = OFF;

CREATE TABLE recurrences_new (
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
    next_date TEXT CHECK (next_date IS NULL OR length(next_date) = 10),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

INSERT INTO recurrences_new (
    id, account_id, category_id, description, amount_minor, currency, every, unit, day_of_month,
    start_date, end_date, next_date, created_at, updated_at
)
SELECT
    id, account_id, category_id, description, amount_minor, currency, every, unit, day_of_month,
    start_date, end_date, next_date, created_at, updated_at
FROM recurrences;

DROP TABLE recurrences;

ALTER TABLE recurrences_new RENAME TO recurrences;

CREATE INDEX idx_recurrences_next_date_id ON recurrences (next_date, id);
CREATE INDEX idx_recurrences_account_next_date ON recurrences (account_id, next_date, id);

PRAGMA foreign_key_check;

PRAGMA foreign_keys = ON;

PRAGMA user_version = 10;

-- +goose Down
SELECT 'forward-only migration';
