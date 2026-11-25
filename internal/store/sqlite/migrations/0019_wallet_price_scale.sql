-- +goose NO TRANSACTION
-- +goose Up
-- Adds the "wallet" account kind and a per-security price precision.
--
-- SQLite cannot widen a CHECK in place, so accounts is rebuilt following the
-- pattern of 0010: built as accounts_new, old table dropped, then renamed into
-- place so the REFERENCES clauses of every dependent table keep saying
-- "accounts". Foreign keys are off for the rebuild (a no-op inside a
-- transaction, hence NO TRANSACTION above) so dropping the old table does not
-- trigger ON DELETE actions of the dependents. Ids are preserved.
PRAGMA foreign_keys = OFF;

CREATE TABLE accounts_new (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    description TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL CHECK (kind IN ('cash', 'checking', 'credit_card', 'savings', 'investment', 'loan', 'wallet')),
    currency TEXT NOT NULL CHECK (length(currency) = 3),
    opening_balance_minor INTEGER NOT NULL,
    opened_on TEXT NOT NULL CHECK (length(opened_on) = 10),
    archived INTEGER NOT NULL DEFAULT 0 CHECK (archived IN (0, 1)),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    iban TEXT CHECK (iban IS NULL OR length(iban) BETWEEN 15 AND 34)
) STRICT;

INSERT INTO accounts_new (
    id, name, description, kind, currency, opening_balance_minor, opened_on, archived,
    created_at, updated_at, iban
)
SELECT
    id, name, description, kind, currency, opening_balance_minor, opened_on, archived,
    created_at, updated_at, iban
FROM accounts;

DROP TABLE accounts;

ALTER TABLE accounts_new RENAME TO accounts;

-- Unit prices are stored as integers of 10^-price_scale units of the security
-- currency. Existing rows are at the currency's decimals (JPY 0, others 2);
-- crypto securities are raised to 8 decimals, rescaling stored prices exactly.
-- Fees stay in minor units of the currency.
ALTER TABLE securities ADD COLUMN price_scale INTEGER NOT NULL DEFAULT 0 CHECK (price_scale BETWEEN 0 AND 8);

UPDATE security_prices
SET price_minor = price_minor * CASE currency WHEN 'JPY' THEN 100000000 ELSE 1000000 END
WHERE security_id IN (SELECT id FROM securities WHERE kind = 'crypto');

UPDATE security_ops
SET unit_price_minor = unit_price_minor * CASE unit_price_currency WHEN 'JPY' THEN 100000000 ELSE 1000000 END
WHERE security_id IN (SELECT id FROM securities WHERE kind = 'crypto');

UPDATE securities
SET price_scale = CASE
    WHEN kind = 'crypto' THEN 8
    WHEN currency = 'JPY' THEN 0
    ELSE 2
END;

PRAGMA foreign_key_check;

PRAGMA foreign_keys = ON;

PRAGMA user_version = 19;

-- +goose Down
SELECT 'forward-only migration';
