-- name: InsertCategory :one
INSERT INTO categories (name, kind)
VALUES (@name, @kind)
RETURNING *;

-- name: UpdateCategory :execrows
UPDATE categories
SET
    name = @name,
    kind = @kind,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = @id;

-- name: DeleteCategory :execrows
DELETE FROM categories
WHERE id = @id;

-- name: GetCategory :one
SELECT *
FROM categories
WHERE id = @id;

-- name: ListCategories :many
SELECT *
FROM categories
ORDER BY id;

-- name: ListCategoriesByIDs :many
SELECT *
FROM categories
WHERE json_array_length(@category_ids_json) > 0
    AND id IN (SELECT value FROM json_each(@category_ids_json))
ORDER BY id;

-- name: CountCategoryUsage :one
SELECT COUNT(*)
FROM transactions
WHERE category_id = @category_id;

-- name: CountCategorySplitUsage :one
SELECT COUNT(*)
FROM transaction_splits
WHERE category_id = @category_id;

-- name: ReassignCategory :execrows
UPDATE transactions
SET
    category_id = @to_category_id,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE category_id = @from_category_id;

-- name: ReassignSplitCategory :execrows
UPDATE transaction_splits
SET category_id = @to_category_id
WHERE category_id = @from_category_id;
