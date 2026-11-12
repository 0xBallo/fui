-- name: InsertPayee :one
INSERT INTO payees (name)
VALUES (@name)
RETURNING *;

-- name: UpdatePayee :execrows
UPDATE payees
SET
    name = @name,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = @id;

-- name: DeletePayee :execrows
DELETE FROM payees
WHERE id = @id;

-- name: GetPayee :one
SELECT *
FROM payees
WHERE id = @id;

-- name: ListPayees :many
SELECT *
FROM payees
ORDER BY id;

-- name: ListPayeeAliases :many
SELECT *
FROM payee_aliases
ORDER BY payee_id, id;

-- name: PayeeAliases :many
SELECT *
FROM payee_aliases
WHERE payee_id = @payee_id
ORDER BY id;

-- name: ClearPayeeAliases :exec
DELETE FROM payee_aliases
WHERE payee_id = @payee_id;

-- name: AddPayeeAlias :exec
INSERT INTO payee_aliases (payee_id, alias)
VALUES (@payee_id, @alias);

-- name: ListTransactionsOfPayee :many
SELECT *
FROM transactions
WHERE payee_id = @payee_id
ORDER BY id;

-- name: SetTransactionSearchText :exec
UPDATE transactions
SET search_text = @search_text
WHERE id = @id;
