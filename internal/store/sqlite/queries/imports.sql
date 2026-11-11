-- name: InsertImportBatch :one
INSERT INTO import_batches (connector, source, source_sha256, imported_at)
VALUES (@connector, @source, @source_sha256, @imported_at)
RETURNING *;

-- name: GetImportBatch :one
SELECT *
FROM import_batches
WHERE id = @id;

-- name: ImportBatchBySHA256 :one
SELECT *
FROM import_batches
WHERE source_sha256 = @source_sha256
ORDER BY imported_at DESC, id DESC
LIMIT 1;

-- name: ListImportBatches :many
SELECT *
FROM import_batches
ORDER BY imported_at DESC, id DESC;

-- name: DeleteImportBatch :execrows
DELETE FROM import_batches
WHERE id = @id;

-- name: GetConnectorAccountRef :one
SELECT account_id
FROM connector_account_refs
WHERE connector = @connector
    AND ref = @ref;

-- name: UpsertConnectorAccountRef :exec
INSERT INTO connector_account_refs (connector, ref, account_id)
VALUES (@connector, @ref, @account_id)
ON CONFLICT (connector, ref) DO UPDATE SET
    account_id = excluded.account_id,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now');
