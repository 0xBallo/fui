-- name: InsertSecurity :one
INSERT INTO securities (
    name,
    ticker,
    isin,
    kind,
    currency,
    quantity_scale
)
VALUES (
    @name,
    @ticker,
    @isin,
    @kind,
    @currency,
    @quantity_scale
)
RETURNING *;

-- name: UpdateSecurity :execrows
UPDATE securities
SET
    name = @name,
    ticker = @ticker,
    isin = @isin,
    kind = @kind,
    currency = @currency,
    quantity_scale = @quantity_scale,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = @id;

-- name: DeleteSecurity :execrows
DELETE FROM securities
WHERE id = @id;

-- name: GetSecurity :one
SELECT *
FROM securities
WHERE id = @id;

-- name: ListSecurities :many
SELECT *
FROM securities
ORDER BY id;

-- name: UpsertSecurityPrice :exec
INSERT INTO security_prices (security_id, date, price_minor, currency)
VALUES (@security_id, @date, @price_minor, @currency)
ON CONFLICT (security_id, date) DO UPDATE SET
    price_minor = excluded.price_minor,
    currency = excluded.currency,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now');

-- name: DeleteSecurityPrice :execrows
DELETE FROM security_prices
WHERE security_id = @security_id
  AND date = @date;

-- name: ListSecurityPrices :many
SELECT *
FROM security_prices
WHERE security_id = @security_id
ORDER BY date;

-- name: SecurityPriceOnOrBefore :one
SELECT *
FROM security_prices
WHERE security_id = @security_id
  AND date <= @date
ORDER BY date DESC, id DESC
LIMIT 1;

-- name: InsertSecurityOp :one
INSERT INTO security_ops (
    date,
    account_id,
    security_id,
    kind,
    quantity_units,
    quantity_scale,
    unit_price_minor,
    unit_price_currency,
    fees_minor,
    fees_currency,
    cash_tx_id
)
VALUES (
    @date,
    @account_id,
    @security_id,
    @kind,
    @quantity_units,
    @quantity_scale,
    @unit_price_minor,
    @unit_price_currency,
    @fees_minor,
    @fees_currency,
    @cash_tx_id
)
RETURNING *;

-- name: UpdateSecurityOp :execrows
UPDATE security_ops
SET
    date = @date,
    account_id = @account_id,
    security_id = @security_id,
    kind = @kind,
    quantity_units = @quantity_units,
    quantity_scale = @quantity_scale,
    unit_price_minor = @unit_price_minor,
    unit_price_currency = @unit_price_currency,
    fees_minor = @fees_minor,
    fees_currency = @fees_currency,
    cash_tx_id = @cash_tx_id,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = @id;

-- name: DeleteSecurityOp :execrows
DELETE FROM security_ops
WHERE id = @id;

-- name: GetSecurityOp :one
SELECT *
FROM security_ops
WHERE id = @id;

-- name: ListSecurityOpsBySecurity :many
SELECT *
FROM security_ops
WHERE security_id = @security_id
  AND (@from_date = '' OR date >= @from_date)
  AND (@to_date = '' OR date <= @to_date)
ORDER BY date, id;
