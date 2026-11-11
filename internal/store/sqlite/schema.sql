CREATE TABLE accounts (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    description TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL CHECK (kind IN ('cash', 'checking', 'credit_card', 'savings', 'investment', 'loan')),
    currency TEXT NOT NULL CHECK (length(currency) = 3),
    opening_balance_minor INTEGER NOT NULL,
    opened_on TEXT NOT NULL CHECK (length(opened_on) = 10),
    archived INTEGER NOT NULL DEFAULT 0 CHECK (archived IN (0, 1)),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    iban TEXT CHECK (iban IS NULL OR length(iban) BETWEEN 15 AND 34)
) STRICT;

CREATE TABLE categories (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    kind TEXT NOT NULL CHECK (kind IN ('income', 'expense')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE TABLE transfer_groups (
    id INTEGER PRIMARY KEY,
    fee_minor INTEGER NOT NULL DEFAULT 0,
    fee_currency TEXT CHECK (fee_currency IS NULL OR length(fee_currency) = 3),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    CHECK ((fee_minor = 0 AND fee_currency IS NULL) OR (fee_currency IS NOT NULL))
) STRICT;

CREATE TABLE import_batches (
    id INTEGER PRIMARY KEY,
    connector TEXT NOT NULL,
    source TEXT NOT NULL,
    source_sha256 TEXT NOT NULL,
    imported_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE INDEX idx_import_batches_sha_imported_at
    ON import_batches (source_sha256, imported_at DESC, id DESC);

CREATE TABLE fx_rates (
    id INTEGER PRIMARY KEY,
    currency TEXT NOT NULL CHECK (length(currency) = 3),
    date TEXT NOT NULL CHECK (length(date) = 10),
    rate_scaled INTEGER NOT NULL CHECK (rate_scaled > 0),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (currency, date)
) STRICT;

CREATE TABLE connector_account_refs (
    id INTEGER PRIMARY KEY,
    connector TEXT NOT NULL,
    ref TEXT NOT NULL,
    account_id INTEGER NOT NULL REFERENCES accounts (id) ON DELETE RESTRICT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (connector, ref)
) STRICT;

CREATE TABLE transactions (
    id INTEGER PRIMARY KEY,
    account_id INTEGER NOT NULL REFERENCES accounts (id) ON DELETE RESTRICT,
    date TEXT NOT NULL CHECK (length(date) = 10),
    operation_date TEXT CHECK (operation_date IS NULL OR length(operation_date) = 10),
    value_date TEXT CHECK (value_date IS NULL OR length(value_date) = 10),
    amount_minor INTEGER NOT NULL,
    currency TEXT NOT NULL CHECK (length(currency) = 3),
    payee_id INTEGER,
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
    UNIQUE (account_id, external_id)
) STRICT;

CREATE INDEX idx_transactions_account_date_id
    ON transactions (account_id, date, id);
CREATE INDEX idx_transactions_category_date
    ON transactions (category_id, date);
CREATE INDEX idx_transactions_fingerprint
    ON transactions (fingerprint);
CREATE INDEX idx_transactions_date_id
    ON transactions (date, id);
