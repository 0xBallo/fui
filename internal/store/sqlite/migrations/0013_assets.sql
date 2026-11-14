-- +goose Up
CREATE TABLE assets (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    kind TEXT NOT NULL CHECK (kind IN ('property', 'vehicle', 'other')),
    purchase_date TEXT NOT NULL CHECK (length(purchase_date) = 10),
    purchase_minor INTEGER NOT NULL CHECK (purchase_minor >= 0),
    currency TEXT NOT NULL CHECK (length(currency) = 3),
    loan_id INTEGER REFERENCES loans (id) ON DELETE SET NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE INDEX idx_assets_loan ON assets (loan_id) WHERE loan_id IS NOT NULL;

CREATE TABLE asset_valuations (
    id INTEGER PRIMARY KEY,
    asset_id INTEGER NOT NULL REFERENCES assets (id) ON DELETE CASCADE,
    date TEXT NOT NULL CHECK (length(date) = 10),
    value_minor INTEGER NOT NULL CHECK (value_minor >= 0),
    currency TEXT NOT NULL CHECK (length(currency) = 3),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (asset_id, date)
) STRICT;

PRAGMA user_version = 13;

-- +goose Down
SELECT 'forward-only migration';
