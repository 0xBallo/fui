# Reports

Pick a report with `h`/`l` (or ←/→), run it again with `r`, and change its period or
parameters with `p`. With `f` you narrow the transactions a report counts (accounts, categories, tags,
payees, amount range, status, text, uncategorized, income/expense) and pick how it groups
(spending and cash flow: category, tag, payee, account, week, month, year) or buckets
(balance, net worth, forecast: day, week, month, year); the active choice shows under the title. On reports with rows, `enter` opens the Transactions
tab filtered on the selected row (its period, category, payee, account or tag) together with the
active filter; the row wins on a conflict. All amounts are shown in the base currency.

## Predefined reports

**Net worth** answers "how much am I worth?": assets minus debts at the end of the period.
Parameters: period.

**Budget vs actual** compares what you planned to spend with what you spent in each budget.
Parameters: period.

**Top payees** lists who you paid the most in the period. Parameters: period and how many
payees to list.

**Forecast** projects the account balances forward from your recurring items and warns when a
balance would drop under a minimum. Parameters: horizon and minimum balance.

**Balance over time** shows how one account's balance evolved. Parameters: account and period.

**Remaining debt** shows how much of a loan is still to be repaid over time. Parameters: loan
and period.

**Interest paid** shows the interest part of the loan installments paid in the period.
Parameters: loan and period.

**Spending by category** lists what you spent in the period for each category, with a bar chart.
Parameters: period.

**Cash flow** shows inflow, outflow and net for each month of the period. Parameters: period.

## Saved reports

Press `s` to save the report on screen under a name, `v` to list saved reports, `enter` to
run the selected one and `d` to delete it. A saved report keeps its filter, grouping and
granularity, and drills down with them.

## Compare and simulations

With the `analytics` module on, `c` compares two periods side by side and `t` toggles the
breakdown by category. With the `simulations` module on, `m` opens what-if scenarios
(investment, savings plan, cash flow); they never change your transactions. A cash-flow
scenario takes events (`date:amount` one-offs and monthly changes, a stop-income date, a
`date:capital:rate:years` loan; separate several with `;`) and an optional comma separated list
of accounts for a per-account view. In the saved simulations list (`v`), `d` deletes the selected
one after a confirmation.

## Keys

- `h`/`l`: previous / next report
- `j`/`k`: move in the table
- `r`: run again
- `enter`: open the selected row's transactions (reports with rows only)
- `p`: change period/params
- `f`: filter and group by / granularity
- `s`: save report, `v`: saved reports, `d`: delete selected
- `e`: export the report result (csv or json) or its transactions (csv, json or fui, narrowed by the active filter)
- `i`: this help, `esc`: back
