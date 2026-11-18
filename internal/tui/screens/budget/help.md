# Budgets

FUI has two budget modes, chosen globally with `fui config set budget.mode cap|envelope`.
The current mode is shown next to the month in the screen title.

## Caps (ceilings)

A cap is a spending limit for a category or tag over a month or a year.
Use caps to **watch** your spending: nothing is moved, you just compare
what you spent with the limit.

- Income does not count; only spending is measured against the cap.
- **Carry-over**: what is left (or overspent) at the end of a period is
  carried into the next one when carry is enabled.

## Envelopes

An envelope holds money you have **assigned** to it from the pool of
unassigned money ("To assign"). Use envelopes to **plan** every euro:
decide where it goes before you spend it.

- Spending a covered transaction lowers the envelope balance.
- To spend more than an envelope holds, move money from another one.
- The balance carries over to the next month, overspending included.
- Envelopes plus "To assign" always equal your budgeted cash accounts.

## Which one?

- Caps: simple monitoring, income is irrelevant to the limit.
- Envelopes: zero-based planning, every amount has a job.

## Thresholds

Each budget has a warning threshold (default 80%) and an over threshold
(default 100%). They drive the Dashboard alerts.

## Progress colors and icons

The bar color is a fixed visual tier. The icon and the OVER label keep the
state readable without color.

| Spent | Color  | Icon   |
| ----- | ------ | ------ |
| < 70% | green  | ✓      |
| 70–89 | yellow | !      |
| 90–99 | orange | ‼      |
| ≥ 100 | red    | ✗ OVER |

## Keys on this screen

- `h`/`l` (or arrows): previous / next month
- `s`: set the amount of the selected budget
- `a`: assign from the pool to an envelope (envelope mode)
- `m`: move money to another envelope (envelope mode)
- `c`: copy the previous month's budgets into this month
- `i`: open or close this explanation

## Same from the CLI

```
fui budget set Groceries 400
fui budget set Groceries 500 --month 2026-12 --carry
fui budget assign Groceries 400
fui budget move Groceries Transport 50
fui budget copy
fui budget status
```
