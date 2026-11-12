-- name: InsertTransaction :one
INSERT INTO transactions (
    account_id,
    date,
    operation_date,
    value_date,
    amount_minor,
    currency,
    payee_id,
    description,
    category_id,
    notes,
    status,
    kind,
    external_id,
    fingerprint,
    import_batch_id,
    transfer_group_id,
    search_text
)
VALUES (
    @account_id,
    @date,
    @operation_date,
    @value_date,
    @amount_minor,
    @currency,
    @payee_id,
    @description,
    @category_id,
    @notes,
    @status,
    @kind,
    @external_id,
    @fingerprint,
    @import_batch_id,
    @transfer_group_id,
    @search_text
)
RETURNING
    *;

-- name: UpdateTransaction :execrows
UPDATE transactions
SET
    account_id = @account_id,
    date = @date,
    operation_date = @operation_date,
    value_date = @value_date,
    amount_minor = @amount_minor,
    currency = @currency,
    payee_id = @payee_id,
    description = @description,
    category_id = @category_id,
    notes = @notes,
    status = @status,
    kind = @kind,
    external_id = @external_id,
    fingerprint = @fingerprint,
    import_batch_id = @import_batch_id,
    transfer_group_id = @transfer_group_id,
    search_text = @search_text,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = @id;

-- name: DeleteTransaction :execrows
DELETE FROM transactions
WHERE id = @id;

-- name: GetTransaction :one
SELECT
    *
FROM transactions
WHERE id = @id;

-- name: ListTransactions :many
-- Translation of domain.TxFilter; text_mode is 0 (none), 1 (instr on search_text) or 2 (FTS5 MATCH).
-- The abs bounds avoid abs(): it overflows on the minimum int64. sqlc does not rewrite named
-- parameters in ORDER BY, so that clause uses the numbers sqlc assigns to @desc (?19) and @sort_key
-- (?22); the payee sort has no key because payees have no names yet (ties fall to id).
SELECT
    *
FROM transactions
WHERE (@from_date = '' OR date >= @from_date)
    AND (@to_date = '' OR date <= @to_date)
    AND (json_array_length(@account_ids_json) = 0 OR account_id IN (SELECT value FROM json_each(@account_ids_json)))
    AND (
        (json_array_length(@category_ids_json) = 0 AND @uncategorized = 0)
        OR (category_id IS NULL AND @uncategorized = 1)
        OR category_id IN (SELECT value FROM json_each(@category_ids_json))
    )
    AND (json_array_length(@payee_ids_json) = 0 OR payee_id IN (SELECT value FROM json_each(@payee_ids_json)))
    AND (json_array_length(@statuses_json) = 0 OR status IN (SELECT value FROM json_each(@statuses_json)))
    AND (json_array_length(@kinds_json) = 0 OR kind IN (SELECT value FROM json_each(@kinds_json)))
    AND (@import_batch_id = 0 OR import_batch_id = @import_batch_id)
    AND (@sign = 'any' OR (@sign = 'income' AND amount_minor > 0) OR (@sign = 'expense' AND amount_minor < 0))
    AND (@min_abs_set = 0 OR amount_minor >= @min_abs OR amount_minor <= (0 - @min_abs))
    AND (@max_abs_set = 0 OR (amount_minor >= (0 - @max_abs) AND amount_minor <= @max_abs))
    AND (
        @text_mode = 0
        OR (@text_mode = 1 AND instr(search_text, @text) > 0)
        OR (@text_mode = 2 AND id IN (SELECT fts.rowid FROM transactions_fts AS fts WHERE fts.search_text MATCH @fts_query))
    )
    AND (
        @after_set = 0
        OR (@desc = 0 AND (date > @after_date OR (date = @after_date AND id > @after_id)))
        OR (@desc = 1 AND (date < @after_date OR (date = @after_date AND id > @after_id)))
    )
    AND (@sort_key = 'date' OR @sort_key = 'amount' OR @sort_key = 'payee')
ORDER BY
    CASE WHEN ?22 = 'date' AND ?19 = 0 THEN date END ASC,
    CASE WHEN ?22 = 'date' AND ?19 = 1 THEN date END DESC,
    CASE WHEN ?22 = 'amount' AND ?19 = 0 THEN amount_minor END ASC,
    CASE WHEN ?22 = 'amount' AND ?19 = 1 THEN amount_minor END DESC,
    id ASC
LIMIT CASE
    WHEN @limit_value <= 0 THEN -1
    ELSE @limit_value
END;

-- name: ListTransactionsForReport :many
SELECT
    *
FROM transactions
WHERE date >= @from_date
    AND date <= @to_date
    AND (json_array_length(@account_ids_json) = 0 OR account_id IN (SELECT value FROM json_each(@account_ids_json)))
ORDER BY date, id;

-- name: SumTransactionsUpTo :one
SELECT CAST(COALESCE(SUM(amount_minor), 0) AS INTEGER)
FROM transactions
WHERE account_id = @account_id
    AND date <= @date;

-- name: TransactionsByExternalID :many
SELECT
    *
FROM transactions
WHERE account_id = @account_id
    AND external_id = @external_id
ORDER BY id;

-- name: TransactionsByFingerprint :many
SELECT
    *
FROM transactions
WHERE fingerprint = @fingerprint
ORDER BY id;

-- name: TransactionsNearAmount :many
SELECT
    *
FROM transactions
WHERE account_id = @account_id
    AND amount_minor = @amount_minor
    AND date >= @from_date
    AND date <= @to_date
ORDER BY id;

-- name: TransactionsByTransferGroup :many
SELECT
    *
FROM transactions
WHERE transfer_group_id = @transfer_group_id
ORDER BY id;
