# ADR 0020: v0.5 gap-closure product decisions

* Status: Accepted
* Date: 2026-11-25

## Context and Problem Statement

A spec-vs-code audit found 51 gaps between the v0.2/v0.3 specifications and the code. Closing them required product decisions that the specifications did not fix; they are recorded here (see `docs/PLAN-gaps.md`).

## Decision Outcome

| # | Decision | Reason |
| --- | --- | --- |
| P1 | Recurrences and loan installments are registered as pending at startup (CLI and TUI), idempotently, for enabled modules only; `recurring run --until` and `loan run` anticipate them; skipped for `help`, `completion`, `init`, `reset`, `restore`, `backup`, `version` and when the archive is missing or locked | The spec says they are registered automatically; users should not have to run a command |
| P2 | Budget carry-over defaults to on for new budgets (`--no-carry` turns it off); existing budgets keep their value | Matches the spec without changing stored behaviour |
| P3 | Undo covers every transaction write, with field masks and cleared dangling references (amends ADR 0016) | One undo for all the ways a transaction changes |
| P4 | Widiba "Rimborso Titoli" stays a transaction of kind `investment` | Cash-leg-less security operations are out of scope |
| P5 | Forecast horizon default 6m, `--horizon` accepts `Nd`, `Nw`, `Nm`, `--granularity day\|week`, `forecast.min_balance` | Spec examples (`12m`) must work; bare numbers stay days |
| P6 | Investment accounts (module off) and wallet accounts (crypto off) are excluded from net worth, runway, reports and Dashboard | Required by "effects of a disabled module"; lists still show them |
| P7 | New account kind `wallet`; security prices have at least 8 decimal digits, as integers | Crypto needs fractional quantities and prices without floats |
| P8 | Budget period `month` or `year`; annual envelopes show a suggested monthly quota | Spec: monthly or annual periods |
| P9 | Budget thresholds are per budget (`--warn`, `--over`, default 80/100); TUI progress bars keep fixed tiers | Alerts follow the user's thresholds, the bars stay readable and consistent |
| P10 | Reconciliation keeps the bulk mode and adds per-transaction ticking | Spec: unticked transactions are shown; ticks allow partial statements |
| P11 | One report filter set (`TxFilter`) plus `--group-by` and `--granularity`; saved reports keep them in `params` JSON | One filter vocabulary, no migration |
| P12 | `reset --keep-settings` also keeps connector associations, stored by account name (table `connector_ref_pending`) and re-linked at the next import | Settings survive a reset |
| P13 | `export --all` writes `dump.json` with every table plus config; `fui restore FILE.json` rebuilds the archive (config only with `--with-config`) | Backups must be restorable from a portable dump |
| P14 | Import preview shows warnings, new entities, transfer proposals, balance mismatches and records of disabled modules (`--accept-transfers`, `--enable-modules`) | Spec: the preview is complete before the write |
| P15 | CLI accepts the spec examples as written (`loan event --rate --from`, `rule test "TEXT"`, `budget set` creating the budget, `forecast --horizon 12m`) | Documentation and behaviour must agree |
| P16 | Period comparison accepts `YYYY-MM`, `YYYY-Qn`, `YYYY` and relative periods, the P11 filters, and can be saved; all net-worth reports use `NetWorthReport` | One definition of net worth |

### Consequences

* **Positive:** specifications, CLI and TUI agree; no new dependencies.
* **Negative:** behaviour changes for existing users: automatic pending transactions at startup, carry-over on by default for new budgets, investment and wallet accounts excluded from totals when their module is off.
