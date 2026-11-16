-- +goose Up
-- Undo journal: one row per undoable write, with the JSON snapshot needed to revert it.
CREATE TABLE op_log (
    id INTEGER PRIMARY KEY,
    kind TEXT NOT NULL,
    payload TEXT NOT NULL CHECK (json_valid(payload)),
    created_at TEXT NOT NULL,
    undone_at TEXT
) STRICT;

PRAGMA user_version = 15;

-- +goose Down
SELECT 'forward-only migration';
