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
(default 100%), set with `n` or `e`. They drive the progress icons here and
the Dashboard alerts; warn must be below over.

A yearly budget is tagged `[year]` in its row; the detail line under the table
shows its period and thresholds (for example `thresholds 80/100`), and for a
yearly envelope the suggested monthly quota (its balance spread over the
months left in the year).

## Dating of moves

`a` and `m` date the movement in the month on screen: today when today falls
inside that month (or budget period, if your month starts on a day other than
the 1st), otherwise the first day of the month shown. Browse to a past or
future month first to plan or correct it.

## Progress colors and icons

The bar color follows the budget's own thresholds. The icon and the OVER label
keep the state readable without color.

| Spent               | Color  | Icon   |
| ------------------- | ------ | ------ |
| below warn          | green  | ✓      |
| warn up to over     | yellow | !      |
| over or more        | red    | ✗ OVER |

With the default 80/100 thresholds, 80–99% shows `!` and 100% or more shows
`✗ OVER`.

## Keys on this screen

- `h`/`l` (or arrows): previous / next month
- `n`: create a budget: name, categories and tags (pick from the list or type, `enter` adds, `backspace` on an empty field removes the last), period (`month` or `year`), default amount, carry-over, warn % and over %
- `e`: edit the selected budget (name is fixed); an empty categories/tags field keeps the current targets
- `s`: set the amount of the selected budget for the month on screen only (a month override; `n`/`e` change the default amount)
- `a`: assign from the pool to an envelope (envelope mode)
- `m`: move money to another envelope (envelope mode)
- `c`: copy the budgets of the month before the one shown into it (yearly budgets are skipped)
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
