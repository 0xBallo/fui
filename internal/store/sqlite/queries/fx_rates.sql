-- name: UpsertFXRate :exec
INSERT INTO fx_rates (currency, date, rate_scaled)
VALUES (@currency, @date, @rate_scaled)
ON CONFLICT (currency, date) DO UPDATE SET
    rate_scaled = excluded.rate_scaled,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now');

-- name: ListFXRates :many
SELECT *
FROM fx_rates
WHERE (@currency = '' OR currency = @currency)
ORDER BY currency, date;

-- name: FXRateOnOrBefore :one
SELECT *
FROM fx_rates
WHERE currency = @currency
    AND date <= @date
ORDER BY date DESC
LIMIT 1;
