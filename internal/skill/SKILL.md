---
name: fui
description: Use fui, a local-first personal-finance CLI, to record expenses, import bank
  statements, review and categorize transactions, and build spending/cash-flow/runway reports
  from natural-language requests. Invoke it whenever the user asks about their accounts,
  spending, budget, or wants to log a purchase or transfer.
version: {{FUI_VERSION}}
---

# fui — agent skill

fui is a single local binary: one SQLite archive, no network access, every write is a
Plan/Apply pair so a change can be previewed before it happens. Run `fui` commands with the
shell tool; never invent output, always run `--dry-run` first and read the real result.

## When to use this skill

Use fui whenever the conversation is about the user's own finances and an `fui` archive is
available: adding a transaction, importing a bank statement, checking balances or net worth,
reviewing uncategorized spending, or running a report. Do not use it for other users' data, for
general financial advice unrelated to the archive, or to open any network connection — fui never
does, and neither should you on its behalf.

## Command reference

Every command accepts `--json` for machine-readable output and `--csv` for RFC 4180 CSV; without
either it prints a table for a human. `fui help --json` returns the full, current command tree
with every flag and output schema — run it once per session instead of trusting this table
blindly, since it reflects the exact binary installed.

For agent runs, call `fui module list --json` first so you only propose commands from active
modules.

| Command | Purpose |
| --- | --- |
| `fui init` | Create the default categories (safe to run again) |
| `fui add AMOUNT DESC... -c CATEGORY -a ACCOUNT [--payee P] [--tag T]` | Add one transaction, default date today |
| `fui tx list / show / edit / rm` | List and change transactions; `tx edit` also takes `--payee`, `--tag`, `--untag` |
| `fui tx search TEXT` | Search transactions by case/accent-insensitive partial text |
| `fui transfer AMOUNT --from A --to B` | Move money between two accounts |
| `fui account add / list / show / edit / archive` | Manage accounts and see balances; kinds include `wallet` (needs the `crypto` module) |
| `fui category add / list / rename / merge / rm` | Manage categories |
| `fui fx set / list` | Store or list manual exchange rates used to convert balances |
| `fui import FILE --connector NAME [--profile P] [--account A]` | Import a bank statement, deduplicated; the preview lists transfer proposals and records for disabled modules |
| `fui import FILE --accept-transfers --enable-modules` | Accept the proposed transfers / enable the modules the file needs |
| `fui import detect FILE [--save NAME]` | Suggest (and optionally save) a CSV profile |
| `fui import list` / `fui import undo BATCH` | List import batches / undo one |
| `fui export --format json|csv|fui [--output PATH]` / `fui export --all --output DIR` | Export filtered transactions or the whole archive |
| `fui report run categories\|cashflow [--month YYYY-MM]` | Spending-by-category or cash-flow report |
| `fui kpi runway [--at DATE]` | Months of liquidity left at current spending |
| `fui tag add / list / rename / merge / rm`, `fui tx tag / untag ID TAG...` | Manage tags and tag transactions |
| `fui payee add / list / rename / alias / merge` | Manage payees and their aliases |
| `fui tx split ID --row "cat=amount[:note]"...` / `fui tx unsplit ID` | Split a transaction across categories |
| `fui rule add / list / test / apply / edit / rm` | Categorization rules (`apply --dry-run` first) |
| `fui reconcile ACCOUNT --balance AMOUNT --date DATE` | Compare with a statement balance; closes only at zero difference |
| `fui reconcile ACCOUNT --tick ID...` / `--untick ID...` | Tick or untick transactions against the statement while reconciling |
| `fui budget set NAME AMOUNT [--month M] [--category C]... [--tag T]... [--period month\|year] [--warn PCT] [--over PCT] [--no-carry]` | Create a budget or set its amount (module `budget`) |
| `fui budget list / status / assign / move` | Inspect budgets and move money between them |
| `fui budget copy --from YYYY-MM --to YYYY-MM` | Copy a month's budgets into another month (yearly ones are not copied) |
| `fui recurring add / list / edit / rm / run / skip / confirm` | Recurring transactions; `edit` and `rm` take the recurrence ID (module `recurring`) |
| `fui forecast [--horizon Nd\|Nw\|Nm] [--granularity G] [--min-balance AMOUNT]` | Cash-flow forecast, e.g. `--horizon 12m` (module `recurring`) |
| `fui loan add / show / schedule / event / run / confirm` | Loans and mortgages (module `loans`) |
| `fui report KIND [--group-by G] [--granularity G] [--period P]` | Analytics; filters: `--account`, `--category`, `--payee`, `--tag`, `--kind`, `--status`, `--min`, `--max`, `--income`, `--expense`, `--uncategorized` |
| `fui report save NAME KIND [flags]` / `fui report list / run / rm` | Saved reports; `report save NAME compare --a P --b P` saves a comparison |
| `fui import FILE --connector ofx\|camt` | Import an OFX/QFX or ISO 20022 camt.053 statement (built-in connectors) |
| `fui security add / list / edit / rm / price` | Securities and manual prices; `rm` is refused while the security has operations (module `investments`; `crypto` for crypto) |
| `fui invest buy / sell / dividend / coupon / list / edit / rm` | Investment operations; `list`, `edit`, `rm` work on existing ones |
| `fui position list [--by-security]` | Computed positions per account, or totals per security |
| `fui asset add / list / show / edit / rm` | Manage assets (module `assets`) |
| `fui asset value ASSET --date D` / `fui asset value rm ASSET DATE` | Record or remove an asset valuation |
| `fui asset link ASSET LOAN` / `fui asset unlink ASSET` | Link an asset to the loan that financed it, or remove the link |
| `fui kpi networth [--at DATE]` | Net worth with its components |
| `fui insights [--dismiss KEY\|--accept KEY]`, `fui report compare --a P --b P` | Automatic analyses and period comparison (module `analytics`) |
| `fui simulate investment / pac / cashflow / save / list / compare / rm` | What-if scenarios; never write transactions; `cashflow` accepts `--account` (module `simulations`) |
| `fui undo` | Revert the last undoable write: tx add/edit/rm, transfer, split, categorize, tag/untag, `rule apply`, recurring/loan confirm (also key `u` in the TUI); preview first |
| `fui backup` / `fui restore FILE --yes` | Back up the archive, or restore from a backup or a `fui export --all` dump (`FILE.json`) |
| `fui reset [--keep-settings]` | Reset the archive to its initial state (a backup is taken first) |
| `fui config get/set KEY [VALUE]` | Read or change configuration |
| `fui skill show` / `fui skill install` | Print or install this skill document |

`fui tx list`, `fui tx search`, and `fui export` share the same filter flags:
`--from`, `--to`, `--month`, `--year`, `-a/--account`, `-c/--category`, `--uncategorized`,
`--payee`, `--min`, `--max`, `--income`, `--expense`, `--status`, `--kind`, `--batch`, `--sort`,
`--desc`, `--limit` (plus `--tag`; `export` adds `--format`, `--output`, `--all`). Keyset paging flags (`--after-date`, `--after-id`) apply only to
`fui tx list`.

Accounts, categories and connector names are matched by case-insensitive partial name; an
ambiguous match lists the candidates and exits with an error instead of guessing.

## JSON output format

`--json` output is a stable, machine-parseable contract: snake_case field names, amounts as
signed integers in minor units (cents) with a separate currency field, dates as `YYYY-MM-DD`. A
failing command also writes JSON to stderr, e.g. `{"error":{"code":"not_found","message":"..."}}`,
and the process exit code is 0 for success, 1 for a user-input error (bad name, bad amount), 2
for a data or I/O error. Prefer `--json` over parsing table output.

### Agent mode

Set `FUI_AGENT=1` in the environment to make fui behave predictably for unattended or scripted
use: it never prompts, always renders `--json`, and refuses any write that lacks `--yes`
(`confirmation_required` error) so nothing is applied without an explicit, already-reviewed flag.

## Typical flows

**Monthly import** — `fui import statement.csv --connector csv --profile mybank --dry-run` to
preview what would be created (new transactions, matched transfers, any unmapped accounts), show
the plan to the user, then rerun with `--yes` once they confirm. Re-importing the same file is
a no-op: fui recognizes it by content and skips already-imported rows. Read the preview for
transfer proposals (rerun with `--accept-transfers` only if the user agrees) and for records that
belong to disabled modules (`--enable-modules` turns them on, again only on request). For an
unknown CSV layout, `fui import detect FILE` suggests a profile first. A wrong import can be
reverted with `fui import undo BATCH`. Recurrences and loan installments that fell due are
registered as pending automatically at startup; review them with `fui recurring list` and
`fui recurring confirm` (or `fui loan confirm`).

**Reviewing uncategorized transactions** — `fui tx list --category Uncategorized --json` to list
them, then `fui tx edit ID --category NAME` (with `--dry-run` first for a bulk-looking change)
to assign categories the user confirms.

**Reports and runway** — `fui report run categories --month 2026-10 --json` for spending by
category, `fui report run cashflow --json` for monthly inflow/outflow/net, `fui kpi runway --json`
for how many months the current liquidity covers at the recent spending rate. Prefer these
aggregated reports to dumping every transaction into the conversation.

**Creating a budget** — `fui budget set Food 400 --category Groceries --month 2026-11 --warn 80
--over 100 --dry-run`, then `--yes`; use `--period year` for a yearly budget and `--no-carry` to
stop unspent amounts rolling over. `fui budget copy --from 2026-10 --to 2026-11` reuses last month's
setup and `fui budget status --month 2026-11 --json` shows progress.

**Reconciling with ticks** — `fui reconcile Checking --balance 2340,18 --date 2026-10-31 --json`
shows the difference; `--tick ID...` marks transactions that appear on the statement and
`--untick ID...` removes the mark, until the difference is zero and the user confirms with `--yes`.

**Undo** — only when the user asks to revert the last write: `fui undo --dry-run` shows what would
be reverted, `fui undo --yes` applies it. `fui import undo BATCH` reverts a whole import.

**Investments and assets** — `fui invest buy BTP2030 --account Broker --quantity 10 --price 101.2
--cash-account Cash --dry-run`; fix mistakes with `fui invest list` then `fui invest edit ID` or
`fui invest rm ID`. `fui position list --by-security --json` gives totals per security,
`fui asset value House --date 2026-12-31 ...` records a valuation and `fui asset link House
Mortgage` ties it to its loan.

**Restoring from a dump** — only on explicit request: `fui export --all --output ./dump` writes a
full dump; `fui restore FILE.json --dry-run` previews restoring from a dump or backup, then
`--yes`. `fui reset --keep-settings` starts over but keeps the configuration.

**Positions and what-ifs** — `fui position list --json` for holdings and unrealized gain,
`fui kpi networth --json` for net worth by component, `fui simulate cashflow --months 12 --account Checking --json` projects the cash forecast, `fui simulate pac --monthly 200 --years 20
--rate 4 --json` for a scenario. Simulations are read-only: they are safe to run without `--dry-run`.

## Safety rules

1. **Preview before writing.** Run the write command with `--dry-run` first and show the user
   what it would change before doing anything.
2. **Confirm, then `--yes`.** Only run the command for real, adding `--yes`, after the user has
   seen the preview and agreed.
3. **Never delete, reset, or restore without an explicit request.** Do not run `fui tx rm`,
   `fui category rm`, `fui reset`, `fui restore`, `fui undo` or `fui import undo`, and do not edit a reconciled transaction,
   unless the user asked for exactly that.
4. **Prefer aggregates to raw rows.** Use reports and totals rather than listing every
   transaction, to keep the user's financial detail out of the model's context when a summary
   answers the question.

If the agent runs on a remote model, anything it reads leaves the machine — that is the user's
choice to make, not fui's: fui itself never opens a network connection.
