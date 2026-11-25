# Changelog

All notable changes to FUI are listed here, newest first. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and versions follow [SemVer](https://semver.org/)
(`0.x` until the JSON output and the `fui/1` protocol are stable). Each release has a matching `vX.Y.Z` tag on
`main`; the section is written on the release branch from the squash-commit titles since the previous tag.

## [Unreleased]

### Added

- Budget: annual periods with a suggested monthly quota, per-budget warning/over thresholds, `budget set` that
  creates the budget (`--category`, `--tag`, `--period`, `--warn`, `--over`, `--no-carry`), `budget copy --from`,
  and TUI forms to create and edit budgets.
- Import: complete preview (warnings, new categories/tags/payees, transfer proposals, balance mismatches, records
  of disabled modules), `import detect`, `--accept-transfers`, `--enable-modules`, CSV column mapper in the TUI, and
  original amount/currency and raw source data kept on transactions.
- Loans through import and export (`schedule` and `installment` records) with matching of pending installments.
- Recurrences and loan installments are registered as pending automatically at startup; `recurring edit/rm`,
  `tx` payee and tag options (`add`, `tx edit --payee/--tag/--untag`) and per-transaction reconciliation
  (`reconcile --tick/--untick`, `space` in the TUI).
- Undo covers every transaction write (categorize, tag/untag, rules apply, recurring and loan confirm) and is
  available in the TUI with `u`.
- Reports: shared filters, `--group-by` and `--granularity`, drill-down and filter form in the TUI, saved report
  parameters, period comparison with `YYYY-MM`, `YYYY-Qn`, `YYYY` and relative periods (`report save NAME compare`);
  forecast `--horizon Nd|Nw|Nm`, `--granularity` and `forecast.min_balance`.
- Investments and assets: `wallet` account kind, 8-digit security price precision, `security edit/rm`,
  `invest list/edit/rm/coupon`, `position list --by-security`, `asset show/edit/rm/value rm/link/unlink`.
- Insights, simulations and connectors: insight FX handling, known recurrences, `simulate cashflow --account`,
  OFX and CAMT.053 `error` and `balance` records.
- `fui export --all` full dump and `fui restore FILE.json` to rebuild the archive from it.
- ADR 0020 records the product decisions of the v0.5 gap closure.

### Changed

- Investment accounts (investments module off) and wallet accounts (crypto module off) are excluded from net worth,
  runway, reports and the Dashboard.
- Budget carry-over is on by default for new budgets; existing budgets keep their setting.
- Report `enter` drills into a row and `r` runs it; spending counts outflows only.
- `reset --keep-settings` keeps connector associations too.
- Functional and technical specifications updated (versions 1.4 and 1.5); ADR 0016 now covers every transaction
  write.
- Widiba "Rimborso Titoli" stays a transaction of kind `investment`.

### Fixed

- TUI budget progress bars use the fixed tiers again (green below 70%, yellow 70-89%, orange 90-99% with `‼`,
  red from 100% with `✗ OVER`); per-budget thresholds drive alerts and the CLI.
- Flaky tests: the compare report teatest waits longer and the 10,000-row import timing takes the best of three
  runs.

## [0.4.0] - 2026-11-18

### Added

- TUI header with an ASCII "FinanceUI" banner, the version and a link to the author (one line on short terminals).
- Dashboard calculation cycles (month, quarter, year, lifetime) switched with `p` and browsed with `h`/`l`;
  every section follows the selected period.
- Dashboard sections: gross net worth in big digits with the net of loans, runway countdown with the zero date,
  income/expenses/savings bar chart, last ten transactions with a jump to Transactions (`t`), and an alerts box
  (budgets over or near their threshold, anomalous transactions, upcoming recurrences, other insights).
- Transactions: income and expense totals of the current filter in big digits, tags shown as chips, and a date
  picker on the From/To filters (`ctrl+d`).
- Accounts: currency column, separate sections for wealth accounts and loans/mortgages, each with its total in
  the base currency.
- Budget and Reports: `i` opens a markdown explanation of the screen.
- Reports: line charts for net worth, balance over time, forecast and remaining debt; bar charts with legends for
  budget vs actual, top payees, interest paid and period comparison.
- `make demo` seeds a synthetic archive for manual runs.
- ADR 0019: TUI libraries (glamour, ntcharts, bubble-datepicker) on Bubble Tea v1.

### Changed

- Key help moved to a one-line bar at the bottom, cut with `…`; `?` expands it with global and screen commands.
- Transactions are colored by flow (income green, expenses red, transfers plain), pending ones in a lighter tone.
- Budget progress bars are colored by tier (< 70%, 70–89%, 90–99%, ≥ 100%) with an icon instead of the `OK`
  label; `OVER` stays as text.
- The dashboard drops the accounts table and the spending-by-category chart (still on Accounts and Reports) and
  the insights panel, now part of the alerts.
- Reports: the simulations view moved from `i` to `m`.

## [0.3.0] - 2026-11-16

### Added

- Built-in OFX and CAMT.053 import connectors.
- Investments: securities, manual prices, buy/sell/dividend operations and positions at average cost
  (`security`, `invest`, `position`); `security_op` records in the `fui/1` protocol.
- Assets with valuations, and net worth composed of liquidity, positions, assets and debts (`asset`,
  `kpi networth`); runway counts investments.
- Simulations of investments, PAC and cash flow, with saved scenarios and comparison of up to three (`simulate`).
- Automatic insights (anomalies, subscriptions, trends) with dismiss and subscription-to-recurrence accept
  (`insights`), configurable through the `insights.*` keys.
- Period comparison report (`report compare`, `report run compare`).
- Generic `undo` of the last transaction write (add, edit, remove, transfer, split).
- TUI: positions and net worth breakdown, insights panel, period comparison and simulations views.
- ADRs 0014–0018 for the v0.3 decisions.

### Fixed

- Editing the amount of a split transaction is rejected instead of breaking the split invariant.

## [0.2.1] - 2026-11-13

### Added

- TUI: bordered tabs, titles with a rule and tables drawn with lipgloss; forms, confirmations and help open in a
  centered modal.
- TUI: full-screen layout that fills the terminal; tables fit its width.
- TUI: mouse support (click a tab to switch, wheel to scroll lists, click to close the help).

### Changed

- Development flow: `develop` (PREVIEW) and `main` (PROD), squash-merged feature and hotfix branches, a
  `pre-commit` hook (`make hooks`) and this changelog (ADR 0013).

### Fixed

- TUI: the import screen no longer traps focus in its form; it opens on an idle page (`n` new import, `esc` back).
- TUI: no more lost keystrokes when the terminal does not answer the background-color query.
- TUI: snapshot tests no longer depend on frame timing.
- CI: the commit message check skips merge commits, which failed every pull request.

## [0.2] - 2026-11-12

### Added

- Tags, payees and aliases; split transactions; categorization rules; reconciliation.
- Budgets (caps and envelopes) with carry-over, moves and thresholds.
- Recurring transactions; loans and mortgages with amortization; cash forecast.
- Reports: budget vs actual, balance over time, remaining debt, interest paid, net worth, top payees; saved reports.
- TUI screens: Budget, Reports, Import and Settings (modules, theme, categories, tags, rules, recurrences, import
  profiles); dynamic shell completion of names.

[Unreleased]: https://github.com/0xBallo/fui/compare/v0.4.0...develop
[0.4.0]: https://github.com/0xBallo/fui/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/0xBallo/fui/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/0xBallo/fui/compare/v0.2...v0.2.1
[0.2]: https://github.com/0xBallo/fui/releases/tag/v0.2
