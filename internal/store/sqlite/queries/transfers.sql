-- name: InsertTransferGroup :one
INSERT INTO transfer_groups (fee_minor, fee_currency)
VALUES (@fee_minor, @fee_currency)
RETURNING *;

-- name: GetTransferGroup :one
SELECT *
FROM transfer_groups
WHERE id = @id;

-- name: DeleteTransferGroup :execrows
DELETE FROM transfer_groups
WHERE id = @id;
