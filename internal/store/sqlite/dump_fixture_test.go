package sqlite

import (
	"context"
	"testing"
)

// fixtureAllTables returns a store whose archive has at least one row in every user table,
// inserted with raw SQL (explicit ids) so the helper does not depend on repository APIs.
func fixtureAllTables(t *testing.T) *Store {
	t.Helper()
	store, _ := newTestStore(t)

	const script = `
INSERT INTO accounts (id, name, kind, currency, opening_balance_minor, opened_on, iban)
    VALUES (1, 'Main', 'checking', 'EUR', 100000, '2026-01-01', 'IT60X0542811101000000123456'),
           (2, 'Mortgage', 'loan', 'EUR', 0, '2026-01-01', NULL);
INSERT INTO categories (id, name, kind) VALUES (901, 'Fixture groceries', 'expense'), (902, 'Fixture salary', 'income');
INSERT INTO rules (id, name, priority, enabled, conditions, actions)
    VALUES (1, 'Fixture rule', 10, 1, '[]', '[]');
INSERT INTO transfer_groups (id, fee_minor, fee_currency) VALUES (1, 100, 'EUR');
INSERT INTO import_batches (id, connector, source, source_sha256, imported_at)
    VALUES (1, 'csv', 'fixture.csv', 'abc', '2026-01-02T03:04:05Z');
INSERT INTO fx_rates (id, currency, date, rate_scaled) VALUES (1, 'USD', '2026-01-02', 1100000);
INSERT INTO securities (id, name, ticker, kind, currency, quantity_scale)
    VALUES (1, 'Fixture ETF', 'FETF', 'etf', 'EUR', 2);
INSERT INTO security_prices (id, security_id, date, price_minor, currency)
    VALUES (1, 1, '2026-01-02', 1000, 'EUR');
INSERT INTO tags (id, name) VALUES (1, 'fixture-tag');
INSERT INTO payees (id, name) VALUES (1, 'Fixture Shop');
INSERT INTO payee_aliases (id, payee_id, alias) VALUES (1, 1, 'FIXTURE SHOP SRL');
INSERT INTO recurrences (id, account_id, category_id, description, amount_minor, currency, every, unit, start_date, next_date)
    VALUES (1, 1, 901, 'Rent', -50000, 'EUR', 1, 'months', '2026-01-01', '2026-02-01');
INSERT INTO transactions (id, account_id, date, amount_minor, currency, payee_id, description, category_id,
        status, kind, external_id, import_batch_id, transfer_group_id, search_text, recurrence_id)
    VALUES (1, 1, '2026-01-02', -1500, 'EUR', 1, 'Fixture purchase', 901, 'booked', 'payment', 'ext-1', 1, 1, 'fixture purchase', 1);
INSERT INTO security_ops (id, date, account_id, security_id, kind, quantity_units, quantity_scale,
        unit_price_minor, unit_price_currency, fees_minor, fees_currency, cash_tx_id, import_batch_id)
    VALUES (1, '2026-01-02', 1, 1, 'buy', 100, 2, 1000, 'EUR', 0, 'EUR', 1, 1);
INSERT INTO connector_account_refs (id, connector, ref, account_id) VALUES (1, 'csv', 'ref-1', 1);
INSERT INTO loans (id, kind, principal_minor, currency, disbursed_on, term_months, rate_type, debit_account_id,
        name, liability_account_id)
    VALUES (1, 'mortgage', 10000000, 'EUR', '2026-01-01', 240, 'fixed', 1, 'Fixture mortgage', 2);
INSERT INTO loan_schedules (id, loan_id, valid_from, origin, rate_basis_points)
    VALUES (1, 1, '2026-01-01', 'computed', 300);
INSERT INTO loan_installments (id, schedule_id, due_date, principal_minor, interest_minor, fees_minor, remaining_minor, transaction_id)
    VALUES (1, 1, '2026-02-01', 30000, 25000, 0, 9970000, 1);
INSERT INTO loan_events (id, loan_id, kind, event_date, new_rate_basis_points)
    VALUES (1, 1, 'rate_change', '2026-06-01', 250);
INSERT INTO saved_reports (id, name, kind, params) VALUES (1, 'Fixture report', 'summary', '{}');
INSERT INTO saved_simulations (id, name, kind, params) VALUES (1, 'Fixture sim', 'pac', '{}');
INSERT INTO dismissed_insights (key, kind, dismissed_at) VALUES ('fixture-key', 'anomaly', '2026-01-02T03:04:05Z');
INSERT INTO reconcile_ticks (account_id, transaction_id) VALUES (1, 1);
INSERT INTO reconciliations (id, account_id, date, statement_balance_minor, currency, closed_at)
    VALUES (1, 1, '2026-01-31', 98500, 'EUR', '2026-02-01T03:04:05Z');
INSERT INTO transaction_tags (transaction_id, tag_id) VALUES (1, 1);
INSERT INTO transaction_splits (id, transaction_id, category_id, amount_minor, note) VALUES (1, 1, 901, -1500, 'all');
INSERT INTO budgets (id, name, mode, amount_minor, currency) VALUES (1, 'Fixture budget', 'cap', 50000, 'EUR'),
    (2, 'Fixture envelope', 'envelope', 20000, 'EUR');
INSERT INTO budget_targets (id, budget_id, category_id) VALUES (1, 1, 901);
INSERT INTO budget_amounts (id, budget_id, year, month, amount_minor) VALUES (1, 1, 2026, 1, 50000);
INSERT INTO budget_moves (id, date, from_budget_id, to_budget_id, amount_minor, currency)
    VALUES (1, '2026-01-15', 1, 2, 1000, 'EUR');
INSERT INTO assets (id, name, kind, purchase_date, purchase_minor, currency, loan_id)
    VALUES (1, 'Fixture flat', 'property', '2026-01-01', 12000000, 'EUR', 1);
INSERT INTO asset_valuations (id, asset_id, date, value_minor, currency)
    VALUES (1, 1, '2026-06-01', 12500000, 'EUR');
INSERT INTO op_log (id, kind, payload, created_at) VALUES (1, 'fixture', '{}', '2026-01-02T03:04:05Z');
`
	if _, err := store.writeDB.ExecContext(context.Background(), script); err != nil {
		t.Fatalf("fixtureAllTables: %v", err)
	}
	return store
}
