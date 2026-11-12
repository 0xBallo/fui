-- +goose Up
CREATE TABLE tags (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE TABLE payees (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE TABLE payee_aliases (
    id INTEGER PRIMARY KEY,
    payee_id INTEGER NOT NULL REFERENCES payees (id) ON DELETE CASCADE,
    alias TEXT NOT NULL UNIQUE COLLATE NOCASE
) STRICT;

-- SQLite cannot add a foreign key to an existing column, so transactions is rebuilt. No payee could
-- exist before this migration, hence payee_id is reset to NULL. Column order is kept (search_text last).
-- Ids and search_text are copied as they are, so the external-content FTS index stays valid.
ALTER TABLE transactions RENAME TO transactions_old;

CREATE TABLE transactions (
    id INTEGER PRIMARY KEY,
    account_id INTEGER NOT NULL REFERENCES accounts (id) ON DELETE RESTRICT,
    date TEXT NOT NULL CHECK (length(date) = 10),
    operation_date TEXT CHECK (operation_date IS NULL OR length(operation_date) = 10),
    value_date TEXT CHECK (value_date IS NULL OR length(value_date) = 10),
    amount_minor INTEGER NOT NULL,
    currency TEXT NOT NULL CHECK (length(currency) = 3),
    payee_id INTEGER REFERENCES payees (id) ON DELETE RESTRICT,
    description TEXT NOT NULL DEFAULT '',
    category_id INTEGER REFERENCES categories (id) ON DELETE RESTRICT,
    notes TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('pending', 'booked', 'reconciled')),
    kind TEXT NOT NULL CHECK (kind IN ('payment', 'transfer', 'exchange', 'fee', 'interest', 'refund', 'investment', 'other')),
    external_id TEXT,
    fingerprint TEXT NOT NULL DEFAULT '',
    import_batch_id INTEGER REFERENCES import_batches (id) ON DELETE SET NULL,
    transfer_group_id INTEGER,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    search_text TEXT NOT NULL DEFAULT '',
    UNIQUE (account_id, external_id)
) STRICT;

INSERT INTO transactions (
    id, account_id, date, operation_date, value_date, amount_minor, currency, payee_id, description,
    category_id, notes, status, kind, external_id, fingerprint, import_batch_id, transfer_group_id,
    created_at, updated_at, search_text
)
SELECT
    id, account_id, date, operation_date, value_date, amount_minor, currency, NULL, description,
    category_id, notes, status, kind, external_id, fingerprint, import_batch_id, transfer_group_id,
    created_at, updated_at, search_text
FROM transactions_old;

DROP TABLE transactions_old;

CREATE INDEX idx_transactions_account_date_id
    ON transactions (account_id, date, id);
CREATE INDEX idx_transactions_category_date
    ON transactions (category_id, date);
CREATE INDEX idx_transactions_fingerprint
    ON transactions (fingerprint);
CREATE INDEX idx_transactions_date_id
    ON transactions (date, id);
CREATE INDEX idx_transactions_payee
    ON transactions (payee_id);

-- +goose StatementBegin
CREATE TRIGGER transactions_fts_ai AFTER INSERT ON transactions BEGIN
    INSERT INTO transactions_fts (rowid, search_text) VALUES (new.id, new.search_text);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER transactions_fts_ad AFTER DELETE ON transactions BEGIN
    INSERT INTO transactions_fts (transactions_fts, rowid, search_text) VALUES ('delete', old.id, old.search_text);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER transactions_fts_au AFTER UPDATE OF search_text ON transactions BEGIN
    INSERT INTO transactions_fts (transactions_fts, rowid, search_text) VALUES ('delete', old.id, old.search_text);
    INSERT INTO transactions_fts (rowid, search_text) VALUES (new.id, new.search_text);
END;
-- +goose StatementEnd

CREATE TABLE transaction_tags (
    transaction_id INTEGER NOT NULL REFERENCES transactions (id) ON DELETE CASCADE,
    tag_id INTEGER NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    PRIMARY KEY (transaction_id, tag_id)
) STRICT, WITHOUT ROWID;

CREATE INDEX idx_transaction_tags_tag ON transaction_tags (tag_id);

CREATE TABLE transaction_splits (
    id INTEGER PRIMARY KEY,
    transaction_id INTEGER NOT NULL REFERENCES transactions (id) ON DELETE CASCADE,
    category_id INTEGER NOT NULL REFERENCES categories (id) ON DELETE RESTRICT,
    amount_minor INTEGER NOT NULL CHECK (amount_minor <> 0),
    note TEXT NOT NULL DEFAULT ''
) STRICT;

CREATE INDEX idx_transaction_splits_transaction ON transaction_splits (transaction_id, id);
CREATE INDEX idx_transaction_splits_category ON transaction_splits (category_id);

PRAGMA user_version = 4;

-- +goose Down
SELECT 'forward-only migration';
