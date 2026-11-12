-- +goose Up
CREATE TABLE budgets (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    mode TEXT NOT NULL CHECK (mode IN ('cap', 'envelope')),
    amount_minor INTEGER NOT NULL CHECK (amount_minor >= 0),
    currency TEXT NOT NULL CHECK (length(currency) = 3),
    carry INTEGER NOT NULL DEFAULT 0 CHECK (carry IN (0, 1)),
    threshold_warn INTEGER NOT NULL DEFAULT 80 CHECK (threshold_warn >= 1),
    threshold_over INTEGER NOT NULL DEFAULT 100 CHECK (threshold_over >= threshold_warn),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

-- A budget covers exactly one category or one tag (domain.BudgetTarget). Categories can be
-- covered by at most one budget (domain.ValidateBudgets); tags may overlap across budgets.
CREATE TABLE budget_targets (
    id INTEGER PRIMARY KEY,
    budget_id INTEGER NOT NULL REFERENCES budgets (id) ON DELETE CASCADE,
    category_id INTEGER REFERENCES categories (id) ON DELETE RESTRICT,
    tag_id INTEGER REFERENCES tags (id) ON DELETE RESTRICT,
    CHECK ((category_id IS NULL) <> (tag_id IS NULL))
) STRICT;

CREATE INDEX idx_budget_targets_budget ON budget_targets (budget_id, id);
CREATE UNIQUE INDEX idx_budget_targets_category ON budget_targets (category_id) WHERE category_id IS NOT NULL;

-- Monthly overrides of a budget's default amount (domain.Budget.Overrides), keyed by the
-- calendar month the budget period starts in (domain.MonthKey).
CREATE TABLE budget_amounts (
    id INTEGER PRIMARY KEY,
    budget_id INTEGER NOT NULL REFERENCES budgets (id) ON DELETE CASCADE,
    year INTEGER NOT NULL,
    month INTEGER NOT NULL CHECK (month BETWEEN 1 AND 12),
    amount_minor INTEGER NOT NULL CHECK (amount_minor >= 0),
    UNIQUE (budget_id, year, month)
) STRICT;

-- Envelope assignments and moves (domain.BudgetMove). A NULL side is the unassigned pool
-- ("Da assegnare"); moves are a ledger, never updated in place.
CREATE TABLE budget_moves (
    id INTEGER PRIMARY KEY,
    date TEXT NOT NULL CHECK (length(date) = 10),
    from_budget_id INTEGER REFERENCES budgets (id) ON DELETE CASCADE,
    to_budget_id INTEGER REFERENCES budgets (id) ON DELETE CASCADE,
    amount_minor INTEGER NOT NULL CHECK (amount_minor > 0),
    currency TEXT NOT NULL CHECK (length(currency) = 3),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    CHECK (from_budget_id IS NOT to_budget_id),
    CHECK (from_budget_id IS NOT NULL OR to_budget_id IS NOT NULL)
) STRICT;

CREATE INDEX idx_budget_moves_date ON budget_moves (date, id);

PRAGMA user_version = 6;

-- +goose Down
SELECT 'forward-only migration';
