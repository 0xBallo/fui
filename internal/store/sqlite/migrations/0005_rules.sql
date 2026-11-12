-- +goose Up
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

PRAGMA user_version = 5;

-- +goose Down
SELECT 'forward-only migration';
