-- name: InsertTag :one
INSERT INTO tags (name)
VALUES (@name)
RETURNING *;

-- name: UpdateTag :execrows
UPDATE tags
SET
    name = @name,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = @id;

-- name: DeleteTag :execrows
DELETE FROM tags
WHERE id = @id;

-- name: GetTag :one
SELECT *
FROM tags
WHERE id = @id;

-- name: ListTags :many
SELECT *
FROM tags
ORDER BY id;

-- name: TagsOfTransaction :many
SELECT t.*
FROM tags AS t
JOIN transaction_tags AS tt ON tt.tag_id = t.id
WHERE tt.transaction_id = @transaction_id
ORDER BY t.id;

-- name: ClearTransactionTags :exec
DELETE FROM transaction_tags
WHERE transaction_id = @transaction_id;

-- name: AddTransactionTag :exec
INSERT INTO transaction_tags (transaction_id, tag_id)
VALUES (@transaction_id, @tag_id);
