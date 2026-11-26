# G10 — Integration, specs, release prep (v0.5.0)

Branch `feat/g10-integration` · worktree `../fui-wt/g10` · TL: Claude Sonnet medium · deps: G01–G06, G08, G09 merged;
G07 (dump/restore/reset) merges during this epic: rebase and include it before the PR.
Read first: `docs/PLAN-gaps.md` (§2 decisions P1–P16), `docs/tasks/G10-deviations.md` (deviations and known gaps
declared by PRs #68–#75), `docs/tasks/_PROTOCOL.md` (v0.5 addendum). Specs are Italian; write spec text in Italian
with correct accents, everything else in English.

## Work items

1. **Budget tiers fix (product decision, user-confirmed 2026-11-25)**: G01 replaced the v0.4 four fixed visual tiers
   of the TUI budget progress bars with three tiers from the budget thresholds. Restore the v0.4 behaviour for the
   bars: fixed tiers green < 70, yellow 70–89, orange 90–99 with `‼`, red ≥ 100 with `✗ OVER` (see
   `docs/PLAN-tui-revamp.md` §2 decision 8 and git history of `internal/tui/screens/budget/progress.go`). Per-budget
   thresholds stay for status/alerts (Dashboard warning/over) and the CLI. Goldens + test.
2. **Flaky tests**: `TestCompareTeatest` (`internal/tui/screens/report/compare_test.go:142`, WaitFor 3s fails on slow
   macOS CI) → raise the wait or make it deterministic; `TestImportTenThousandRowsUnderFiveSeconds`
   (`internal/connector/pipeline_test.go`) fails under parallel load → keep the 5 s budget but make the measure robust
   (e.g. best of 3 runs). Nothing else is skipped.
3. **Functional spec** `docs/specs/funzionali.md`: update every section touched by P1–P16 and by the deviations
   (CLI table and examples: `import detect`, `--accept-transfers`, `--enable-modules`, `tx add/edit --payee/--tag/--untag`,
   `recurring edit/rm ID`, `reconcile --tick/--untick`, `budget set` flags (`--category/--tag/--period/--warn/--over/--no-carry`),
   `budget copy --from`, report `--group-by/--granularity` and filters, `forecast --horizon Nd|Nw|Nm --granularity`,
   `report save NAME compare`, `security edit/rm`, `invest list/edit/rm/coupon`, `position list --by-security`,
   `asset show/edit/rm/value rm/link/unlink`, `simulate cashflow --account`, `restore FILE.json`, `undo` scope);
   Moduli (wallet account kind, investment/wallet excluded when the module is off); Ricorrenze registered at startup
   (P1); Budget (annual period, carry default on, thresholds per budget, TUI tiers as item 1); Formato FUI (`schedule`
   optional fields, `original`/`raw`, `category_hint` rule conditions, `error`/`balance` records from OFX/CAMT);
   Widiba "Rimborso Titoli" stays a transaction (P4); TUI keys (`u` undo global; Report `enter` drill / `r` run /
   filter form; Import `tab`/`t`/`m`/`ctrl+p`; Transactions `c`/`S` on pending rows; Settings › Recurrences `r`;
   Settings › Assets `enter`/`v`/`l`/`L`; Accounts `n`/`o`/`l`/`p` and reconcile `space`). Add "Versione 1.4" to the
   version history.
4. **Technical spec** `docs/specs/tecniche.md`: data model (migrations 0016–0019 + G07's), startup catch-up hook,
   undo journal scope and field masks, dump v2 and restore, module→account-kind exclusion, price scale.
5. **ADRs**: amend `0016-undo-scope-transaction-writes.md` (new scope); new `0020-v05-gap-closure-decisions.md`
   recording P1–P16 with the reasons (one table, short).
6. **Skill** (`internal/skill/`): add the new commands and flows (import with transfers/modules, budget creation,
   reconcile ticks, undo, invest/asset management, restore from dump) keeping the existing tone; golden updated.
7. **CHANGELOG** `[Unreleased]`: Added/Changed/Fixed entries for G01–G09 (+G07, item 1, item 2), user-facing wording.
8. **PLAN-gaps.md** §7: status table filled (PR numbers #68–#75 + G07), "Remaining known gaps" list from
   `G10-deviations.md` minus what was fixed later (e.g. hint conditions, edit form prefill, compare chart, TUI report
   filters, import account-kind rule, shared event parser were fixed in review). Delete `docs/tasks/G10-deviations.md`
   at the end (its content moves into PLAN-gaps.md).

## DoD

PLAN-gaps §6 items 3, 5, 6; `go test -race -count=1 ./...` green twice in a row locally with `TZ=UTC` and
`TZ=Pacific/Kiritimati`; spec text Italian, accents correct; PR title `docs: close the v0.5 gaps in specs, adrs and changelog`.
