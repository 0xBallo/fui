-- name: InsertBudget :one
INSERT INTO budgets (name, mode, amount_minor, currency, carry, threshold_warn, threshold_over)
VALUES (@name, @mode, @amount_minor, @currency, @carry, @threshold_warn, @threshold_over)
RETURNING *;

-- name: UpdateBudget :execrows
UPDATE budgets
SET
    name = @name,
    mode = @mode,
    amount_minor = @amount_minor,
    currency = @currency,
    carry = @carry,
    threshold_warn = @threshold_warn,
    threshold_over = @threshold_over,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = @id;

-- name: DeleteBudget :execrows
DELETE FROM budgets
WHERE id = @id;

-- name: GetBudget :one
SELECT *
FROM budgets
WHERE id = @id;

-- name: ListBudgets :many
SELECT *
FROM budgets
ORDER BY id;

-- name: ClearBudgetTargets :exec
DELETE FROM budget_targets
WHERE budget_id = @budget_id;

-- name: AddBudgetTarget :exec
INSERT INTO budget_targets (budget_id, category_id, tag_id)
VALUES (@budget_id, @category_id, @tag_id);

-- name: TargetsOfBudget :many
SELECT *
FROM budget_targets
WHERE budget_id = @budget_id
ORDER BY id;

-- name: ListBudgetTargets :many
SELECT *
FROM budget_targets
ORDER BY budget_id, id;

-- name: ClearBudgetAmounts :exec
DELETE FROM budget_amounts
WHERE budget_id = @budget_id;

-- name: AddBudgetAmount :exec
INSERT INTO budget_amounts (budget_id, year, month, amount_minor)
VALUES (@budget_id, @year, @month, @amount_minor);

-- name: AmountsOfBudget :many
SELECT *
FROM budget_amounts
WHERE budget_id = @budget_id
ORDER BY year, month;

-- name: ListBudgetAmounts :many
SELECT *
FROM budget_amounts
ORDER BY budget_id, year, month;

-- name: ListBudgetAmountsForMonth :many
SELECT *
FROM budget_amounts
WHERE year = @year AND month = @month;

-- name: InsertBudgetMove :exec
INSERT INTO budget_moves (date, from_budget_id, to_budget_id, amount_minor, currency)
VALUES (@date, @from_budget_id, @to_budget_id, @amount_minor, @currency);

-- name: BudgetMovesInRange :many
SELECT *
FROM budget_moves
WHERE date >= @from_date AND date <= @to_date
ORDER BY date, id;
