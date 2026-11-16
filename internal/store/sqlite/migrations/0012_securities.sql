-- +goose Up
CREATE TABLE securities (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    ticker TEXT,
    isin TEXT,
    kind TEXT NOT NULL CHECK (kind IN ('stock', 'etf', 'bond', 'fund', 'crypto')),
    currency TEXT NOT NULL CHECK (length(currency) = 3),
    quantity_scale INTEGER NOT NULL CHECK (quantity_scale >= 0 AND quantity_scale <= 8),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    CHECK (
        (ticker IS NOT NULL AND trim(ticker) <> '')
        OR (isin IS NOT NULL AND trim(isin) <> '')
    ),
    CHECK (ticker IS NULL OR trim(ticker) = ticker),
    CHECK (isin IS NULL OR trim(isin) = isin)
) STRICT;

CREATE UNIQUE INDEX idx_securities_ticker
    ON securities (ticker COLLATE NOCASE) WHERE ticker IS NOT NULL;

CREATE UNIQUE INDEX idx_securities_isin
    ON securities (isin COLLATE NOCASE) WHERE isin IS NOT NULL;

CREATE TABLE security_prices (
    id INTEGER PRIMARY KEY,
    security_id INTEGER NOT NULL REFERENCES securities (id) ON DELETE RESTRICT,
    date TEXT NOT NULL CHECK (length(date) = 10),
    price_minor INTEGER NOT NULL CHECK (price_minor > 0),
    currency TEXT NOT NULL CHECK (length(currency) = 3),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (security_id, date)
) STRICT;

CREATE INDEX idx_security_prices_security_date
    ON security_prices (security_id, date);

CREATE TABLE security_ops (
    id INTEGER PRIMARY KEY,
    date TEXT NOT NULL CHECK (length(date) = 10),
    account_id INTEGER NOT NULL REFERENCES accounts (id) ON DELETE RESTRICT,
    security_id INTEGER NOT NULL REFERENCES securities (id) ON DELETE RESTRICT,
    kind TEXT NOT NULL CHECK (kind IN ('buy', 'sell', 'dividend', 'coupon')),
    quantity_units INTEGER NOT NULL CHECK (quantity_units > 0),
    quantity_scale INTEGER NOT NULL CHECK (quantity_scale >= 0 AND quantity_scale <= 8),
    unit_price_minor INTEGER NOT NULL CHECK (unit_price_minor >= 0),
    unit_price_currency TEXT NOT NULL CHECK (length(unit_price_currency) = 3),
    fees_minor INTEGER NOT NULL DEFAULT 0 CHECK (fees_minor >= 0),
    fees_currency TEXT NOT NULL CHECK (length(fees_currency) = 3),
    cash_tx_id INTEGER REFERENCES transactions (id) ON DELETE SET NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    CHECK (fees_currency = unit_price_currency)
) STRICT;

CREATE INDEX idx_security_ops_security_date_id
    ON security_ops (security_id, date, id);

CREATE INDEX idx_security_ops_account_date_id
    ON security_ops (account_id, date, id);

CREATE INDEX idx_security_ops_cash_tx
    ON security_ops (cash_tx_id) WHERE cash_tx_id IS NOT NULL;

PRAGMA user_version = 12;

-- +goose Down
SELECT 'forward-only migration';
