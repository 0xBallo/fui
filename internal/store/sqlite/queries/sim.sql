-- name: InsertSavedSimulation :one
INSERT INTO saved_simulations (name, kind, params)
VALUES (@name, @kind, @params)
RETURNING *;

-- name: DeleteSavedSimulation :execrows
DELETE FROM saved_simulations
WHERE id = @id;

-- name: GetSavedSimulation :one
SELECT *
FROM saved_simulations
WHERE id = @id;

-- name: GetSavedSimulationByName :one
SELECT *
FROM saved_simulations
WHERE name = @name;

-- name: ListSavedSimulations :many
SELECT *
FROM saved_simulations
ORDER BY name COLLATE NOCASE, id;

-- name: UpsertDismissedInsight :exec
INSERT INTO dismissed_insights (key, kind, dismissed_at)
VALUES (@key, @kind, @dismissed_at)
ON CONFLICT (key) DO UPDATE SET
    kind = excluded.kind,
    dismissed_at = excluded.dismissed_at;

-- name: ListDismissedInsights :many
SELECT *
FROM dismissed_insights
ORDER BY key;
