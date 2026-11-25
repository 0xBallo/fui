-- +goose Up
-- Import provenance: original amount/currency and raw source record on transactions, and the
-- import batch that created a security operation (so an import can be undone).
ALTER TABLE transactions ADD COLUMN original_amount_minor INTEGER;
ALTER TABLE transactions ADD COLUMN original_currency TEXT CHECK ((original_currency IS NULL) = (original_amount_minor IS NULL) AND (original_currency IS NULL OR length(original_currency) = 3));
ALTER TABLE transactions ADD COLUMN raw TEXT CHECK (raw IS NULL OR json_valid(raw));

ALTER TABLE security_ops ADD COLUMN import_batch_id INTEGER REFERENCES import_batches (id) ON DELETE SET NULL;
CREATE INDEX idx_security_ops_import_batch
    ON security_ops (import_batch_id) WHERE import_batch_id IS NOT NULL;

PRAGMA user_version = 16;

-- +goose Down
SELECT 'forward-only migration';
