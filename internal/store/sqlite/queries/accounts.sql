-- name: InsertAccount :one
INSERT INTO accounts (
    name,
    description,
    kind,
    currency,
    opening_balance_minor,
    opened_on,
    archived
)
VALUES (
    @name,
    @description,
    @kind,
    @currency,
    @opening_balance_minor,
    @opened_on,
    @archived
)
RETURNING
    *;

-- name: UpdateAccount :execrows
UPDATE accounts
SET
    name = @name,
    description = @description,
    kind = @kind,
    currency = @currency,
    opening_balance_minor = @opening_balance_minor,
    opened_on = @opened_on,
    archived = @archived,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = @id;

-- name: GetAccount :one
SELECT
    *
FROM accounts
WHERE id = @id;

-- name: ListAccounts :many
SELECT
    *
FROM accounts
ORDER BY id;

-- name: ListAccountsForReport :many
SELECT
    *
FROM accounts
WHERE (@include_archived = 1 OR archived = 0)
    AND (json_array_length(@account_ids_json) = 0 OR id IN (SELECT value FROM json_each(@account_ids_json)))
ORDER BY id;

-- name: BalanceAt :one
SELECT
    a.currency AS currency,
    a.opening_balance_minor + COALESCE(SUM(t.amount_minor), 0) AS balance_minor
FROM accounts a
LEFT JOIN transactions t ON t.account_id = a.id AND t.date <= @date
WHERE a.id = @account_id
GROUP BY a.id, a.currency, a.opening_balance_minor;
