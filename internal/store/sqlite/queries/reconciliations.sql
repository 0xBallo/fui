-- name: InsertReconciliation :one
INSERT INTO reconciliations (account_id, date, statement_balance_minor, currency, closed_at)
VALUES (@account_id, @date, @statement_balance_minor, @currency, @closed_at)
RETURNING *;

-- name: CloseReconciliation :execrows
UPDATE reconciliations
SET
    closed_at = @closed_at,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE account_id = @account_id AND date = @date;

-- name: GetReconciliation :one
SELECT *
FROM reconciliations
WHERE account_id = @account_id AND date = @date;

-- name: LastClosedReconciliation :one
SELECT *
FROM reconciliations
WHERE account_id = @account_id AND closed_at IS NOT NULL
ORDER BY date DESC, id DESC
LIMIT 1;

-- name: ListReconcileTicks :many
SELECT transaction_id
FROM reconcile_ticks
WHERE account_id = @account_id
ORDER BY transaction_id;

-- name: InsertReconcileTick :exec
INSERT OR IGNORE INTO reconcile_ticks (account_id, transaction_id)
VALUES (@account_id, @transaction_id);

-- name: DeleteReconcileTick :exec
DELETE FROM reconcile_ticks
WHERE account_id = @account_id AND transaction_id = @transaction_id;
