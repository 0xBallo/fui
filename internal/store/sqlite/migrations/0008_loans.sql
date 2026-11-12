-- +goose Up
CREATE TABLE loans (
    id INTEGER PRIMARY KEY,
    kind TEXT NOT NULL CHECK (kind IN ('mortgage', 'loan')),
    principal_minor INTEGER NOT NULL CHECK (principal_minor > 0),
    currency TEXT NOT NULL CHECK (length(currency) = 3),
    disbursed_on TEXT NOT NULL CHECK (length(disbursed_on) = 10),
    term_months INTEGER NOT NULL CHECK (term_months >= 1),
    rate_type TEXT NOT NULL CHECK (rate_type IN ('fixed', 'variable', 'mixed')),
    debit_account_id INTEGER NOT NULL REFERENCES accounts (id) ON DELETE RESTRICT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE TABLE loan_schedules (
    id INTEGER PRIMARY KEY,
    loan_id INTEGER NOT NULL REFERENCES loans (id) ON DELETE CASCADE,
    valid_from TEXT NOT NULL CHECK (length(valid_from) = 10),
    origin TEXT NOT NULL CHECK (origin IN ('imported', 'computed')),
    rate_basis_points INTEGER NOT NULL CHECK (rate_basis_points > 0),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (loan_id, valid_from)
) STRICT;

CREATE INDEX idx_loan_schedules_loan_valid_from
    ON loan_schedules (loan_id, valid_from);

CREATE TABLE loan_installments (
    id INTEGER PRIMARY KEY,
    schedule_id INTEGER NOT NULL REFERENCES loan_schedules (id) ON DELETE CASCADE,
    due_date TEXT NOT NULL CHECK (length(due_date) = 10),
    principal_minor INTEGER NOT NULL CHECK (principal_minor >= 0),
    interest_minor INTEGER NOT NULL CHECK (interest_minor >= 0),
    fees_minor INTEGER NOT NULL CHECK (fees_minor >= 0),
    remaining_minor INTEGER NOT NULL CHECK (remaining_minor >= 0),
    transaction_id INTEGER REFERENCES transactions (id) ON DELETE SET NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (schedule_id, due_date)
) STRICT;

CREATE INDEX idx_loan_installments_schedule_due
    ON loan_installments (schedule_id, due_date, id);

CREATE INDEX idx_loan_installments_transaction
    ON loan_installments (transaction_id);

CREATE TABLE loan_events (
    id INTEGER PRIMARY KEY,
    loan_id INTEGER NOT NULL REFERENCES loans (id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('rate_change', 'renegotiation', 'prepayment')),
    event_date TEXT NOT NULL CHECK (length(event_date) = 10),
    new_rate_basis_points INTEGER CHECK (new_rate_basis_points IS NULL OR new_rate_basis_points > 0),
    new_term_months INTEGER CHECK (new_term_months IS NULL OR new_term_months >= 1),
    prepaid_minor INTEGER CHECK (prepaid_minor IS NULL OR prepaid_minor > 0),
    prepaid_currency TEXT CHECK (prepaid_currency IS NULL OR length(prepaid_currency) = 3),
    prepay_mode TEXT CHECK (prepay_mode IS NULL OR prepay_mode IN ('reduce_installment', 'reduce_term')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE INDEX idx_loan_events_loan_event_date
    ON loan_events (loan_id, event_date, id);

PRAGMA user_version = 8;

-- +goose Down
SELECT 'forward-only migration';
