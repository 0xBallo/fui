# Changelog

All notable changes to FUI are listed here, newest first. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and versions follow [SemVer](https://semver.org/)
(`0.x` until the JSON output and the `fui/1` protocol are stable). Each release has a matching `vX.Y.Z` tag on
`main`; the section is written on the release branch from the squash-commit titles since the previous tag.

## [Unreleased]

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

## [0.2] - 2026-11-12

### Added

- Tags, payees and aliases; split transactions; categorization rules; reconciliation.
- Budgets (caps and envelopes) with carry-over, moves and thresholds.
- Recurring transactions; loans and mortgages with amortization; cash forecast.
- Reports: budget vs actual, balance over time, remaining debt, interest paid, net worth, top payees; saved reports.
- TUI screens: Budget, Reports, Import and Settings (modules, theme, categories, tags, rules, recurrences, import
  profiles); dynamic shell completion of names.

[Unreleased]: https://github.com/0xBallo/fui/compare/v0.2...develop
[0.2]: https://github.com/0xBallo/fui/releases/tag/v0.2
