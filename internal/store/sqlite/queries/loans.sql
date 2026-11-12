-- name: InsertLoan :one
INSERT INTO loans (
    kind,
    principal_minor,
    currency,
    disbursed_on,
    term_months,
    rate_type,
    debit_account_id
)
VALUES (
    @kind,
    @principal_minor,
    @currency,
    @disbursed_on,
    @term_months,
    @rate_type,
    @debit_account_id
)
RETURNING *;

-- name: UpdateLoan :execrows
UPDATE loans
SET
    kind = @kind,
    principal_minor = @principal_minor,
    currency = @currency,
    disbursed_on = @disbursed_on,
    term_months = @term_months,
    rate_type = @rate_type,
    debit_account_id = @debit_account_id,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = @id;

-- name: DeleteLoan :execrows
DELETE FROM loans
WHERE id = @id;

-- name: GetLoan :one
SELECT *
FROM loans
WHERE id = @id;

-- name: ListLoans :many
SELECT *
FROM loans
ORDER BY id;

-- name: InsertLoanSchedule :one
INSERT INTO loan_schedules (
    loan_id,
    valid_from,
    origin,
    rate_basis_points
)
VALUES (
    @loan_id,
    @valid_from,
    @origin,
    @rate_basis_points
)
RETURNING *;

-- name: GetScheduleAt :one
SELECT *
FROM loan_schedules
WHERE loan_id = @loan_id
  AND valid_from <= @at_date
ORDER BY valid_from DESC, id DESC
LIMIT 1;

-- name: ListLoanInstallmentsBySchedule :many
SELECT *
FROM loan_installments
WHERE schedule_id = @schedule_id
ORDER BY due_date, id;

-- name: InsertLoanInstallment :exec
INSERT INTO loan_installments (
    schedule_id,
    due_date,
    principal_minor,
    interest_minor,
    fees_minor,
    remaining_minor,
    transaction_id
)
VALUES (
    @schedule_id,
    @due_date,
    @principal_minor,
    @interest_minor,
    @fees_minor,
    @remaining_minor,
    @transaction_id
);

-- name: InsertLoanEvent :one
INSERT INTO loan_events (
    loan_id,
    kind,
    event_date,
    new_rate_basis_points,
    new_term_months,
    prepaid_minor,
    prepaid_currency,
    prepay_mode
)
VALUES (
    @loan_id,
    @kind,
    @event_date,
    @new_rate_basis_points,
    @new_term_months,
    @prepaid_minor,
    @prepaid_currency,
    @prepay_mode
)
RETURNING *;

-- name: ListLoanEvents :many
SELECT *
FROM loan_events
WHERE loan_id = @loan_id
ORDER BY event_date, id;

-- name: LinkLoanInstallment :execrows
UPDATE loan_installments
SET
    transaction_id = @transaction_id,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE schedule_id = (
    SELECT id
    FROM loan_schedules
    WHERE loan_id = @loan_id
      AND valid_from = @schedule_from
)
  AND due_date = @due_date;

-- name: UnlinkLoanInstallment :execrows
UPDATE loan_installments
SET
    transaction_id = NULL,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE schedule_id = (
    SELECT id
    FROM loan_schedules
    WHERE loan_id = @loan_id
      AND valid_from = @schedule_from
)
  AND due_date = @due_date;
