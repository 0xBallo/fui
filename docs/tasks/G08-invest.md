# G08 — Investments, crypto, assets

Branch `feat/g08-invest` · worktree `../fui-wt/g08` · TL: Claude Sonnet medium · deps: none (P6 filter map comes
from G06; if G06 is not merged, add the `wallet` exclusion when rebasing).
Read first: `docs/PLAN-gaps.md` (§2 P6, P7; §5; §6), `docs/tasks/_PROTOCOL.md` (v0.5 addendum), ADR 0015,
F: "Moduli" (Investimenti, Crypto, Beni), "Modello di dominio" (Titolo, Operazione su titolo, Bene);
T: "Patrimonio netto", "Modello dati".

## Owned paths

`internal/domain/{account,security,quantity,position,asset}*.go`, store migration + `queries/{account,security*,
asset*}.sql` + repos, `internal/app/{investing,assetting}/`, memstore matching files,
`internal/cli/{security*,invest*,position*,asset*,kpi_networth*,account*}.go` + render files,
`internal/tui/screens/{accounts,settings}/` (new files for securities/ops/assets; existing files minimal edits).

## Gaps

| Id | Gap | Evidence | Expected |
| --- | --- | --- | --- |
| a3-6 | Missing investing use cases | `investing/investing.go:514-590,601-641`, `store/sqlite/repo_securities.go:38,64,213`, `cli/invest.go:106-108` | `security edit/rm` (rm refused when ops exist), `invest list [--account] [--security]`, `invest edit ID`, `invest rm ID` (removes the linked cash tx), `invest coupon`, `position list --by-security` (totals across accounts); ops only on `investment`/`wallet` accounts |
| a3-7 | Assets: no update/delete/show, valuations not deletable | `assetting/assetting.go:374`, `repo_assets.go:46,86,165`, `cli/asset.go:18-22` | `asset show/edit/rm`, `asset value rm NAME DATE` |
| a3-10 | Crypto: no wallet; gating holes; sub-cent prices become 0 | `domain/account.go:8-13`, `schema.sql:5`, `investing.go:125-134`, `importing/rows.go:71`, `domain/amount.go:42-44` | P7: account kind `wallet` (requires the crypto module; migration rebuilds the `accounts` CHECK following the existing forward-only pattern with FK checks); `security list` hides crypto when the module is off; import of crypto `security_op` requires crypto (reuse G02 module dialog if merged, else skip + warning); price precision ≥ 8 decimals (e.g. price scale per security), rapid: value = quantity × price exact at any scale |
| a3-11 | Asset ↔ mortgage link only at creation; not per-asset in net worth | `cli/kpi_networth_render.go:149` | `asset link NAME LOAN` / `asset unlink NAME`; `kpi networth` lists each asset with value, debt and net |
| a3-12 | TUI: no assets, no security/op entry | `tui/root.go:87-94,203-216`, `screens/accounts/ports.go:31-35` | Accounts › positions panel: add security, buy/sell/dividend/coupon form, op list with edit/delete; Settings › Assets (module assets): CRUD, valuations, link loan. Screen/section registration per module in `internal/tui/root.go` |

## Suggested MTs

1. domain + store: `wallet` kind migration, price precision, op account-kind rule (rapid exactness property).
2. app/investing: edit/rm security, list/edit/rm ops, coupon, by-security positions, crypto gating fixes.
3. app/assetting: show/edit/rm, valuation rm, link/unlink.
4. cli: new commands + `kpi networth` per-asset lines, goldens, testscripts `g08_invest.txtar`, `g08_assets.txtar`.
5. tui: Accounts securities/ops forms; Settings › Assets section; goldens.

## DoD

PLAN §6, plus: migration test on a populated archive with every account kind; e2e with crypto module on/off.
