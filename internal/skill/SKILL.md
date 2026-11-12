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
| `fui add AMOUNT DESC... -c CATEGORY -a ACCOUNT` | Add one transaction, default date today |
| `fui tx list / show / edit / rm` | List and change transactions |
| `fui tx search TEXT` | Search transactions by case/accent-insensitive partial text |
| `fui transfer AMOUNT --from A --to B` | Move money between two accounts |
| `fui account add / list / show / edit / archive` | Manage accounts and see balances |
| `fui category add / list / rename / merge / rm` | Manage categories |
| `fui fx set / list` | Store or list manual exchange rates used to convert balances |
| `fui import FILE --connector NAME [--profile P]` | Import a bank statement, deduplicated |
| `fui export --format json|csv|fui --all` | Export transactions or the whole archive |
| `fui report run categories\|cashflow [--month YYYY-MM]` | Spending-by-category or cash-flow report |
| `fui kpi runway [--at DATE]` | Months of liquidity left at current spending |
| `fui tag add / list / rename / merge / rm`, `fui tx tag / untag ID TAG...` | Manage tags and tag transactions |
| `fui payee add / list / rename / alias / merge` | Manage payees and their aliases |
| `fui tx split ID --row "cat=amount[:note]"...` / `fui tx unsplit ID` | Split a transaction across categories |
| `fui rule add / list / test / apply / edit / rm` | Categorization rules (`apply --dry-run` first) |
| `fui reconcile ACCOUNT --balance AMOUNT --date DATE` | Compare with a statement balance; closes only at zero difference |
| `fui budget set / list / status / assign / move / copy` | Budgets (module `budget`) |
| `fui recurring add / list / run / skip / confirm` | Recurring transactions (module `recurring`) |
| `fui forecast [--horizon DAYS]` | Cash-flow forecast (module `recurring`, needs `analytics`) |
| `fui loan add / show / schedule / event / run / confirm` | Loans and mortgages (module `loans`) |
| `fui report KIND` / `fui report save / list / run / rm` | Analytics and saved reports |
| `fui backup` / `fui restore PATH --yes` | Back up or restore the archive |
| `fui config get/set KEY [VALUE]` | Read or change configuration |
| `fui skill show` / `fui skill install` | Print or install this skill document |

`fui tx list`, `fui tx search`, and `fui export` share the same filter flags:
`--from`, `--to`, `--month`, `--year`, `-a/--account`, `-c/--category`, `--uncategorized`,
`--payee`, `--min`, `--max`, `--income`, `--expense`, `--status`, `--kind`, `--batch`, `--sort`,
`--desc`, `--limit`. Keyset paging flags (`--after-date`, `--after-id`) apply only to
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
a no-op: fui recognizes it by content and skips already-imported rows.

**Reviewing uncategorized transactions** — `fui tx list --category Uncategorized --json` to list
them, then `fui tx edit ID --category NAME` (with `--dry-run` first for a bulk-looking change)
to assign categories the user confirms.

**Reports and runway** — `fui report run categories --month 2026-10 --json` for spending by
category, `fui report run cashflow --json` for monthly inflow/outflow/net, `fui kpi runway --json`
for how many months the current liquidity covers at the recent spending rate. Prefer these
aggregated reports to dumping every transaction into the conversation.

## Safety rules

1. **Preview before writing.** Run the write command with `--dry-run` first and show the user
   what it would change before doing anything.
2. **Confirm, then `--yes`.** Only run the command for real, adding `--yes`, after the user has
   seen the preview and agreed.
3. **Never delete, reset, or restore without an explicit request.** Do not run `fui tx rm`,
   `fui category rm`, `fui reset`, or `fui restore`, and do not edit a reconciled transaction,
   unless the user asked for exactly that.
4. **Prefer aggregates to raw rows.** Use reports and totals rather than listing every
   transaction, to keep the user's financial detail out of the model's context when a summary
   answers the question.

If the agent runs on a remote model, anything it reads leaves the machine — that is the user's
choice to make, not fui's: fui itself never opens a network connection.
