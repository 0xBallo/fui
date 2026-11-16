-- +goose Up
CREATE TABLE saved_simulations (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    kind TEXT NOT NULL CHECK (kind IN ('investment', 'pac', 'cashflow')),
    params TEXT NOT NULL CHECK (json_valid(params)),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE TABLE dismissed_insights (
    key TEXT PRIMARY KEY,
    kind TEXT NOT NULL CHECK (kind IN ('anomaly', 'subscription', 'price_change', 'trend')),
    dismissed_at TEXT NOT NULL CHECK (length(dismissed_at) >= 20)
) STRICT;

PRAGMA user_version = 14;

-- +goose Down
SELECT 'forward-only migration';
