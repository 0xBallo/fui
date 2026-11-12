-- name: InsertRule :one
INSERT INTO rules (name, priority, enabled, conditions, actions)
VALUES (@name, @priority, @enabled, @conditions, @actions)
RETURNING *;

-- name: UpdateRule :execrows
UPDATE rules
SET
    priority = @priority,
    enabled = @enabled,
    conditions = @conditions,
    actions = @actions,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE name = @name;

-- name: DeleteRule :execrows
DELETE FROM rules
WHERE name = @name;

-- name: GetRule :one
SELECT *
FROM rules
WHERE name = @name;

-- name: ListRules :many
SELECT *
FROM rules
ORDER BY name COLLATE NOCASE, id;

-- name: ListEnabledRulesByPriority :many
SELECT *
FROM rules
WHERE enabled = 1
ORDER BY priority, name COLLATE NOCASE, id;
