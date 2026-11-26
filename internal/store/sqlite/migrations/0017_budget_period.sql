-- +goose Up
-- A budget period is a (budget) month or a (budget) year (domain.BudgetPeriod). Existing
-- budgets stay monthly; yearly overrides are keyed on the January MonthKey of the year.
ALTER TABLE budgets ADD COLUMN period TEXT NOT NULL DEFAULT 'month' CHECK (period IN ('month', 'year'));

PRAGMA user_version = 17;

-- +goose Down
SELECT 'forward-only migration';
