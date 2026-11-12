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

CREATE TABLE rules (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    priority INTEGER NOT NULL,
    enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
    conditions TEXT NOT NULL CHECK (json_valid(conditions)),
    actions TEXT NOT NULL CHECK (json_valid(actions)),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE INDEX idx_rules_enabled_priority_name
    ON rules (enabled, priority, name);

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
    search_text TEXT NOT NULL DEFAULT '', -- appended last by 0003; later migrations append new columns
    recurrence_id INTEGER REFERENCES recurrences (id) ON DELETE SET NULL,
    due_date TEXT CHECK (due_date IS NULL OR length(due_date) = 10),
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
CREATE INDEX idx_transactions_payee
    ON transactions (payee_id);
CREATE INDEX idx_transactions_recurrence_status_due_date
    ON transactions (recurrence_id, status, due_date, id);

CREATE VIRTUAL TABLE transactions_fts USING fts5 (
    search_text,
    content = 'transactions',
    content_rowid = 'id',
    tokenize = 'trigram'
);

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

CREATE TABLE budgets (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    mode TEXT NOT NULL CHECK (mode IN ('cap', 'envelope')),
    amount_minor INTEGER NOT NULL CHECK (amount_minor >= 0),
    currency TEXT NOT NULL CHECK (length(currency) = 3),
    carry INTEGER NOT NULL DEFAULT 0 CHECK (carry IN (0, 1)),
    threshold_warn INTEGER NOT NULL DEFAULT 80 CHECK (threshold_warn >= 1),
    threshold_over INTEGER NOT NULL DEFAULT 100 CHECK (threshold_over >= threshold_warn),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE TABLE budget_targets (
    id INTEGER PRIMARY KEY,
    budget_id INTEGER NOT NULL REFERENCES budgets (id) ON DELETE CASCADE,
    category_id INTEGER REFERENCES categories (id) ON DELETE RESTRICT,
    tag_id INTEGER REFERENCES tags (id) ON DELETE RESTRICT,
    CHECK ((category_id IS NULL) <> (tag_id IS NULL))
) STRICT;

CREATE INDEX idx_budget_targets_budget ON budget_targets (budget_id, id);
CREATE UNIQUE INDEX idx_budget_targets_category ON budget_targets (category_id) WHERE category_id IS NOT NULL;

CREATE TABLE budget_amounts (
    id INTEGER PRIMARY KEY,
    budget_id INTEGER NOT NULL REFERENCES budgets (id) ON DELETE CASCADE,
    year INTEGER NOT NULL,
    month INTEGER NOT NULL CHECK (month BETWEEN 1 AND 12),
    amount_minor INTEGER NOT NULL CHECK (amount_minor >= 0),
    UNIQUE (budget_id, year, month)
) STRICT;

CREATE TABLE budget_moves (
    id INTEGER PRIMARY KEY,
    date TEXT NOT NULL CHECK (length(date) = 10),
    from_budget_id INTEGER REFERENCES budgets (id) ON DELETE CASCADE,
    to_budget_id INTEGER REFERENCES budgets (id) ON DELETE CASCADE,
    amount_minor INTEGER NOT NULL CHECK (amount_minor > 0),
    currency TEXT NOT NULL CHECK (length(currency) = 3),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    CHECK (from_budget_id IS NOT to_budget_id),
    CHECK (from_budget_id IS NOT NULL OR to_budget_id IS NOT NULL)
) STRICT;

CREATE INDEX idx_budget_moves_date ON budget_moves (date, id);
