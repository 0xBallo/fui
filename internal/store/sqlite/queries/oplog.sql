-- name: InsertOpLog :one
INSERT INTO op_log (kind, payload, created_at)
VALUES (@kind, @payload, @created_at)
RETURNING *;

-- name: LatestOpLog :one
SELECT *
FROM op_log
WHERE undone_at IS NULL
ORDER BY id DESC
LIMIT 1;

-- name: MarkOpLogUndone :execrows
UPDATE op_log
SET undone_at = @undone_at
WHERE id = @id AND undone_at IS NULL;

-- name: SetTransactionID :execrows
UPDATE transactions
SET id = @new_id
WHERE id = @old_id;

-- name: SetTransferGroupID :execrows
UPDATE transfer_groups
SET id = @new_id
WHERE id = @old_id;
