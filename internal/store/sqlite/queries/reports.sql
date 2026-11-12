-- name: InsertSavedReport :one
INSERT INTO saved_reports (name, kind, params)
VALUES (@name, @kind, @params)
RETURNING *;

-- name: UpdateSavedReport :execrows
UPDATE saved_reports
SET
    name = @name,
    kind = @kind,
    params = @params,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = @id;

-- name: DeleteSavedReport :execrows
DELETE FROM saved_reports
WHERE id = @id;

-- name: GetSavedReport :one
SELECT *
FROM saved_reports
WHERE id = @id;

-- name: GetSavedReportByName :one
SELECT *
FROM saved_reports
WHERE name = @name;

-- name: ListSavedReports :many
SELECT *
FROM saved_reports
ORDER BY name COLLATE NOCASE, id;
