-- name: InsertRecurrence :one
INSERT INTO recurrences (
    account_id,
    category_id,
    description,
    amount_minor,
    currency,
    every,
    unit,
    day_of_month,
    start_date,
    end_date,
    next_date
)
VALUES (
    @account_id,
    @category_id,
    @description,
    @amount_minor,
    @currency,
    @every,
    @unit,
    @day_of_month,
    @start_date,
    @end_date,
    @next_date
)
RETURNING
    *;

-- name: UpdateRecurrence :execrows
UPDATE recurrences
SET
    account_id = @account_id,
    category_id = @category_id,
    description = @description,
    amount_minor = @amount_minor,
    currency = @currency,
    every = @every,
    unit = @unit,
    day_of_month = @day_of_month,
    start_date = @start_date,
    end_date = @end_date,
    next_date = @next_date,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = @id;

-- name: DeleteRecurrence :execrows
DELETE FROM recurrences
WHERE id = @id;

-- name: GetRecurrence :one
SELECT
    *
FROM recurrences
WHERE id = @id;

-- name: ListRecurrences :many
SELECT
    *
FROM recurrences
ORDER BY id;

-- name: DueRecurrences :many
SELECT
    *
FROM recurrences
WHERE next_date <= @until_date
ORDER BY next_date, id;

-- name: PendingTransactionsOfRecurrence :many
SELECT
    *
FROM transactions
WHERE recurrence_id = @recurrence_id
    AND status = 'pending'
ORDER BY COALESCE(due_date, date), id;
