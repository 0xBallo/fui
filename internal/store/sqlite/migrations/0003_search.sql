-- +goose Up
ALTER TABLE transactions ADD COLUMN search_text TEXT NOT NULL DEFAULT '';

-- External-content index: the text lives in transactions.search_text, computed in Go (textnorm).
CREATE VIRTUAL TABLE transactions_fts USING fts5 (
    search_text,
    content = 'transactions',
    content_rowid = 'id',
    tokenize = 'trigram'
);

-- Index the existing rows (still empty text) so the later 'delete' of their old value is consistent.
INSERT INTO transactions_fts (transactions_fts) VALUES ('rebuild');

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

-- Existing rows are backfilled in Go right after this migration (SQL cannot do NFD folding).

PRAGMA user_version = 3;

-- +goose Down
SELECT 'forward-only migration';
