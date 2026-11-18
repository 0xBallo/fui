# Changelog

All notable changes to FUI are listed here, newest first. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and versions follow [SemVer](https://semver.org/)
(`0.x` until the JSON output and the `fui/1` protocol are stable). Each release has a matching `vX.Y.Z` tag on
`main`; the section is written on the release branch from the squash-commit titles since the previous tag.

## [Unreleased]

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
