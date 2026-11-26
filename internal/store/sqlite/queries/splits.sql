-- name: SplitsOfTransaction :many
SELECT *
FROM transaction_splits
WHERE transaction_id = @transaction_id
ORDER BY id;

-- name: ClearTransactionSplits :exec
DELETE FROM transaction_splits
WHERE transaction_id = @transaction_id;

-- name: AddTransactionSplit :exec
INSERT INTO transaction_splits (transaction_id, category_id, amount_minor, note)
VALUES (@transaction_id, @category_id, @amount_minor, @note);

-- name: ListTransactionsWithSplitsForReport :many
SELECT
    sqlc.embed(t),
    s.category_id AS split_category_id,
    s.amount_minor AS split_amount_minor
FROM transactions AS t
LEFT JOIN transaction_splits AS s ON s.transaction_id = t.id
WHERE t.date >= @from_date
    AND t.date <= @to_date
    AND (json_array_length(@account_ids_json) = 0 OR t.account_id IN (SELECT value FROM json_each(@account_ids_json)))
ORDER BY t.date, t.id, s.id;
