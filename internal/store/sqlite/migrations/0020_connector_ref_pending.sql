-- +goose Up
-- Connector associations a "reset --keep-settings" carries over: accounts are deleted, so the
-- association is kept by account name and consumed when the next import finds an account with it.
CREATE TABLE connector_ref_pending (
    id INTEGER PRIMARY KEY,
    connector TEXT NOT NULL,
    ref TEXT NOT NULL,
    account_name TEXT NOT NULL COLLATE NOCASE,
    UNIQUE (connector, ref)
) STRICT;

PRAGMA user_version = 20;

-- +goose Down
SELECT 'forward-only migration';
