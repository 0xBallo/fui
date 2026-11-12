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
