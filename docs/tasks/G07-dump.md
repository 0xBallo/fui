# G07 — Full dump, restore from dump, reset keep-settings

Branch `feat/g07-dump` · worktree `../fui-wt/g07` · TL: Claude Sonnet medium · deps: **G01–G05 and G08 merged**
(the dump must cover every table they add).
Read first: `docs/PLAN-gaps.md` (§2 P12, P13; §6), `docs/tasks/_PROTOCOL.md` (v0.5 addendum),
F: "Import, export e riconciliazione" (Export), "Formato FUI", "Reset dell'archivio", "Persistenza, backup";
T: "Persistenza e migrazioni", ADR 0011.

## Owned paths

`internal/app/importing/dump*.go`, `internal/app/{backup.go,reset.go,restore*.go}`, `internal/store/sqlite/{reset*,
dump*,restore*}` + queries, `internal/cli/{export*,restore*,reset*}.go`, render files of those commands.

## Gaps

| Id | Gap | Evidence | Expected |
| --- | --- | --- | --- |
| a2-23, a3-2 | `export --all` lacks most entities | `importing/dump.go:96-113` | Dump schema v2 with every table in `schema.sql` except caches/FTS: budgets (targets, amounts, moves), recurrences, rules, tags, tx tags, payees, aliases, splits, loans (schedules, installments, events), saved reports, reconciliations, connector refs, securities, prices, ops, assets, valuations, op_log, dismissed insights, saved simulations, plus config (`config.toml` content) and import profiles. A test fails when a new table is not in the dump (compare against `sqlite_master`) |
| P13 | `fui restore` accepts only `.db` | `app/backup.go:34-53`, `cli/restore.go:17` | `fui restore FILE.json` validates the dump, takes a backup, rebuilds via the reset/rebuild path (ADR 0011) and loads all rows with ids preserved; `.db` restore unchanged |
| a2-6 | `reset --keep-settings` drops connector refs; rules keep stale ids | `store/sqlite/reset.go:163-170,196`, `reset_test.go:287`, `app/reset.go:107-125` | P12: refs kept as (connector, ref, account name) and re-linked at the next import when an account with that name exists; rule conditions/actions remapped to the new category/payee/tag/account ids (or dropped with a warning when the target was not kept); update the reset test |

## Suggested MTs

1. store: dump reader/writer for all tables + coverage test against `sqlite_master`.
2. app: dump v2 encode/decode (v1 dumps still readable), config + profiles section.
3. app + store: restore from dump via rebuild; rapid/round trip: dump → restore → dump is byte-identical.
4. store + app: reset keep-settings refs + rule id remapping (tests).
5. cli: `restore FILE.json`, confirmation and `--yes`, testscripts `g07_dump_restore.txtar`, `g07_reset_keep.txtar`.

## DoD

PLAN §6, plus the round-trip identity test on a fixture archive that has at least one row in every table.
