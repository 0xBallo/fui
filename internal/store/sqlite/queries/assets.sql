-- name: InsertAsset :one
INSERT INTO assets (name, kind, purchase_date, purchase_minor, currency, loan_id)
VALUES (@name, @kind, @purchase_date, @purchase_minor, @currency, @loan_id)
RETURNING *;

-- name: UpdateAsset :execrows
UPDATE assets
SET
    name = @name,
    kind = @kind,
    purchase_date = @purchase_date,
    purchase_minor = @purchase_minor,
    currency = @currency,
    loan_id = @loan_id,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = @id;

-- name: DeleteAsset :execrows
DELETE FROM assets
WHERE id = @id;

-- name: GetAsset :one
SELECT *
FROM assets
WHERE id = @id;

-- name: ListAssets :many
SELECT *
FROM assets
ORDER BY id;

-- name: UpsertAssetValuation :exec
INSERT INTO asset_valuations (asset_id, date, value_minor, currency)
VALUES (@asset_id, @date, @value_minor, @currency)
ON CONFLICT (asset_id, date) DO UPDATE
SET
    value_minor = excluded.value_minor,
    currency = excluded.currency,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now');

-- name: ListAssetValuations :many
SELECT *
FROM asset_valuations
WHERE asset_id = @asset_id
ORDER BY date, id;

-- name: DeleteAssetValuation :execrows
DELETE FROM asset_valuations
WHERE asset_id = @asset_id AND date = @date;
