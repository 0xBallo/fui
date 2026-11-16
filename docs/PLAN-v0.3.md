# FUI v0.3 — development plan

Same method as `PLAN-v0.2.md`: layers in order (domain → store → app → cli → tui → integration), one layer per task,
disjoint owned paths, ≤ ~800 changed lines per task, git flow of ADR 0013 (feature branch from `develop`, squash PR).
Workers follow `docs/tasks/_PROTOCOL.md` (v0.3 addendum: terse report + `RESULT` line, see bottom of that file).
Specs (Italian) stay the source of truth: `F:` = funzionali.md, `T:` = tecniche.md.

Baseline: v0.2.1 (migrations 0001–0011, module registry already knows `investments`, `crypto`, `assets`, `simulations`, `ai`).

## 1. Scope

| Item | Module | Spec |
| --- | --- | --- |
| OFX and CAMT.053 built-in connectors | core | F: Connettori; T: Connettori |
| Securities, prices, buy/sell/dividend ops, positions (avg cost), `fui security/invest/position` | investments (+crypto: scale 8) | F: Titolo, Operazione su titolo |
| Assets with manual valuations, `fui asset add/list/value`, net worth with positions + assets, runway `--include-investments` | assets | F: Bene, KPI; T: Patrimonio netto |
| Insights (anomalies, subscriptions, trends), dismiss, subscription → recurrence, `fui insights`, Dashboard panel | analytics | F: Analisi automatiche |
| Period comparison report | analytics | F: Report |
| Simulations: investment, PAC, cash flow; save/list/compare (≤3); `fui simulate`; Report screen | simulations | F: Simulazioni; T: `internal/domain/simulate` (only place with float64) |
| Generic undo `fui undo` for the last undoable write | core | F: roadmap "annullamento generico" |

Out of scope: `ai` module and Monte Carlo (spec "da definire"), live prices, taxes, v0.4+.

Defaults taken (spec silent; change if the product decision differs): generic undo = journal of inverse operations for
tx add/edit/rm/transfer/split (`op_log`), `import undo` unchanged; CLI names `fui security add/list/price`,
`fui invest buy/sell/dividend`, `fui position list`; cost basis = average cost; imported `kind=investment` rows are NOT
auto-converted to security ops; insight thresholds in config keys `insights.*`; `ai` module stays registry-only.

## 2. Schema plan

| Migration | Owner | Tables |
| --- | --- | --- |
| `0012_securities.sql` | S1 | `securities`, `security_prices`, `security_ops` (`cash_tx_id` FK) |
| `0013_assets.sql` | S2 | `assets`, `asset_valuations` (`UNIQUE(asset_id,date)`, `loan_id` nullable) |
| `0014_sim_insights.sql` | S3 | `saved_simulations`, `dismissed_insights` |
| `0015_op_log.sql` | S4 | `op_log` (undo journal) |

## 3. Tasks (wave = earliest start; max 3 agents in parallel, rolling pool)

Agent policy: effort ≤ medium, no flagship. Antigravity quota out until ~2026-11-19 → Copilot (`claude-sonnet-5`,
`gpt-5.3-codex`) and Claude Code (`sonnet`).

| Wave | Task | Title | Branch | Owned paths | Deps | Agent |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | W01 | Domain: security, quantity, ops, positions | `feat/w-domain-security` | `internal/domain/{security,quantity,position}*.go` | — | Copilot sonnet-5 |
| 1 | W02 | Domain: asset, valuation, net worth composer | `feat/w-domain-asset` | `internal/domain/{asset,networth}*.go` | — | Copilot codex |
| 1 | W03 | Domain: simulate engine | `feat/w-domain-simulate` | `internal/domain/simulate/` | — | Claude sonnet |
| 2 | W04 | Domain: insights + period comparison | `feat/w-domain-insight` | `internal/domain/{insight,compare}*.go` | — | Copilot sonnet-5 |
| 2 | W05 | Connector OFX | `feat/w-conn-ofx` | `internal/connector/ofx*.go`, `testdata/ofx*` | — | Copilot codex |
| 2 | W06 | Connector CAMT.053 | `feat/w-conn-camt` | `internal/connector/camt*.go`, `testdata/camt*` | — | Copilot sonnet-5 |
| 3 | W07 | Store: securities (0012) | `feat/w-store-security` | store `migrations/0012*`, `queries/security*`, `repo_securities*` | W01 | Copilot codex |
| 3 | W08 | Store: assets (0013) | `feat/w-store-asset` | `…0013*`, `queries/asset*`, `repo_assets*` | W02 | Copilot sonnet-5 |
| 3 | W09 | Store: simulations + dismissed insights (0014) | `feat/w-store-sim` | `…0014*`, `queries/sim*`, `repo_sim*` | W03,W04 | Copilot codex |
| 4 | W10 | Store + app: op_log undo (0015) | `feat/w-undo` | `…0015*`, `repo_oplog*`, `internal/app/undo*.go`, memstore | — | Claude sonnet |
| 4 | W11 | App: investing (securities, prices, ops, positions) | `feat/w-app-invest` | `internal/app/investing/`, `memstore/security*` | W01,W07 | Claude sonnet |
| 4 | W12 | App: assets | `feat/w-app-asset` | `internal/app/assetting/`, `memstore/asset*` | W02,W08 | Copilot sonnet-5 |
| 5 | W13 | App: simulations (run/save/list/compare) | `feat/w-app-sim` | `internal/app/simulating/`, `memstore/sim*` | W03,W09 | Copilot codex |
| 5 | W14 | App: insights + dismiss + subscription→recurrence + compare report | `feat/w-app-insight` | `internal/app/reporting/insight*.go,compare*.go`, `memstore/insight*` | W04,W09 | Claude sonnet |
| 5 | W15 | App: net worth with positions+assets, runway incl. investments | `feat/w-app-networth` | `internal/app/reporting/{networth,runway}*.go` | W11,W12 | Copilot sonnet-5 |
| 6 | W16 | CLI: `security`, `invest`, `position`, runway flag | `feat/w-cli-invest` | `internal/cli/{security,invest,position}*.go`, `render/` new | W11,W15 | Copilot sonnet-5 |
| 6 | W17 | CLI: `asset`, `kpi networth` update | `feat/w-cli-asset` | `internal/cli/{asset,kpi_networth}*.go` | W12,W15 | Copilot codex |
| 6 | W18 | CLI: `simulate` | `feat/w-cli-sim` | `internal/cli/simulate*.go`, `render/simulate.go` | W13 | Copilot sonnet-5 |
| 7 | W19 | CLI: `insights`, `report run compare` | `feat/w-cli-insight` | `internal/cli/{insights,report_compare}*.go` | W14 | Copilot codex |
| 7 | W20 | CLI: `undo` | `feat/w-cli-undo` | `internal/cli/undo*.go` | W10 | Copilot sonnet-5 |
| 7 | W21 | Proto: fui/1 security-op records round-trip | `feat/w-proto-sec` | `pkg/fuiproto/`, `internal/app/importing/` | W11 | Copilot codex |
| 8 | W22 | TUI: Dashboard insights panel + Report compare | `feat/w-tui-insight` | `internal/tui/screens/{dashboard,report}/` (insight/compare files) | W14 | Copilot sonnet-5 |
| 8 | W23 | TUI: Report simulations view | `feat/w-tui-sim` | `internal/tui/screens/report/sim*` | W13 | Copilot sonnet-5 |
| 8 | W24 | TUI: positions in Accounts + assets in Settings/Dashboard net worth | `feat/w-tui-invest` | `internal/tui/screens/accounts/pos*`, `dashboard/networth*` | W11,W12 | Copilot codex |
| 9 | W25 | Integration: register OFX/CAMT/commands/screens per module, e2e testscripts, module matrix goldens, skill, ADRs | `feat/w-integration` | `cmd/fui/`, `internal/cli/root.go`, `internal/tui/register.go`, `internal/connector/registry.go`, `internal/skill/`, `testdata/`, `docs/adr/` | all | Claude sonnet |
| 10 | — | Release PR `release/v0.3.0` (CHANGELOG), CI green on GitHub | — | — | W25 | coordinator |

To limit root.go/register.go conflicts, tasks W16–W24 expose `NewXCmd(deps)` / screen constructors only; W25 registers.
Lessons applied (memory): scratch under `./.scratch`, sandbox XDG for any manual `fui` run, no AI trailers, run full
`go test ./...`, `go mod tidy` after adds, regenerate sqlc with `go generate` after rebase.

## 4. Task briefs

Each `docs/tasks/W<nn>-*.md` holds the brief. Common acceptance: table-driven tests, rapid properties where noted,
`TZ=UTC` and `Pacific/Kiritimati`, coverage per package ≥ 80% (`internal/domain` ≥ 90%).

## 5. Status (2026-11-14)

Work stopped by request after the in-flight tasks; `develop` CI green (all jobs), coverage total 84.3%,
`internal/domain` 91.5%. Agents: Copilot (`claude-sonnet-5.5`, `gpt-5.3-codex`) and Claude Code `sonnet`, effort
medium; Antigravity quota exhausted.

| Task | Status | PR |
| --- | --- | --- |
| W01 Domain security/positions | merged | #8 |
| W02 Domain asset/net worth | merged | #6 |
| W03 Domain simulate | merged | #7 |
| W04 Domain insights/compare | merged | #9 |
| W05 Connector OFX | merged | #11 |
| W06 Connector CAMT.053 | merged | #10 |
| W07 Store securities (0012) | merged | #12 |
| W08 Store assets (0013) | merged | #13 |
| W09 Store simulations/insights (0014) | merged | #15 |
| W10 Undo journal (0015) | merged | #17 |
| W11 App investing | merged | #16 |
| W12 App assets | merged | #14 |
| W13 App simulations | merged | #20 |
| W14 App insights/compare | merged | #18 |
| W15 App net worth/runway | merged | #23 |
| W16 CLI security/invest/position | merged | #22 |
| W17 CLI asset, kpi networth, runway `--include-investments` | merged | #28 |
| W18 CLI simulate | merged | #25 |
| W19 CLI insights/compare + `insights.*` config | merged | #24 |
| W20 CLI undo | merged | #21 |
| W21 Proto fui/1 security ops | merged | #19 |
| W22 TUI insights/compare | merged | #27 |
| W23 TUI simulations | merged | #29 |
| W24 TUI positions/net worth | merged | #30 |
| W25 Integration | done on `feat/w-integration` | — |
| Release `release/v0.3.0` | **not started** | — |

Review fixes applied during the run: new-loan capital credited and simulation params bounded (W03); zero-quantity
sell no longer divides by zero, `insufficient_quantity` registered in codes and exit codes (W01); anomaly insight key
depends only on the transaction id (W04); dividends/coupons booked as `interest`, not `investment` (W11); simulation
"current positions" uses market value of positions (W13); amount edits on split transactions are rejected (W10,
fixes a v0.2 invariant break).

### Known gaps

- Resolved by W25: the v0.3 commands, services, OFX/CAMT connectors and TUI features are composed in `cmd/fui`
  and registered per module (ADR 0014); e2e scripts `v03_*` and the `help --json` goldens cover them.
- `report run compare` runs the period comparison, but a comparison cannot be saved (`report save NAME
  compare`); saved reports of kind net worth still use the old `Analytics.NetWorth` instead of
  `NetWorthReport` (W15 kept both for golden compatibility).
- Full JSON dump (`export --all`) lacks the v0.2 entities (budgets, loans, recurrences, rules) and the v0.3 securities,
  assets and op log; only saved simulations and dismissed insights were added (W09). Pre-existing gap.
- Subscription insights only skip transactions linked to a recurrence (`RecurrenceID`); a subscription with an existing
  recurrence but manually entered transactions is still reported.
- Insights and period comparison work in a single currency (no FX conversion).
- Undo journals only tx add/edit/rm, transfer, split/unsplit; categorize/tag/untag are not journaled, and a later
  tag/untag makes the previous entry stale. Restoring rows whose payee/batch/recurrence was deleted fails (rolled back).
- Investing: no edit/delete use cases for securities or operations (remove + re-add); cash account must share the
  security currency; positions are per (account, security).
- Assets: no update/delete use cases. Simulations: no per-account cash-flow scenarios.
- CAMT error records use the entry ordinal as line; batch split only when `TxDtls` amounts sum to the entry amount.
- Specs updated by W25 for the v0.3 choices (CLI names, undo scope in ADR 0016, fui/1 `security_op` record).

### Open points (product decisions; defaults in place)

1. Generic undo scope: transaction writes only (default) or every write (categories, budgets, rules, …)?
2. CLI names `fui security add/list/price`, `fui invest buy/sell/dividend`, `fui position list` (not in the spec).
3. Cost basis: average cost (PMC). FIFO needed?
4. Imported `kind=investment` rows (e.g. Widiba "Rimborso Titoli") stay plain transactions; auto-convert to security ops?
5. Insight thresholds as config keys `insights.{anomaly_factor_permille,subscription_tolerance_permille,
   subscription_min_occurrences,trend_permille,trend_min_amount}`: OK as permille integers?
6. Simulation `--rate` with one value uses base ± 2 pp for pessimistic/optimistic: acceptable default?
7. `ai` module and Monte Carlo stay out of v0.3 until the local model is chosen.
8. Next step: release v0.3.0 (`release/v0.3.0`, CHANGELOG, CI on GitHub). Choices 1–6 above are recorded in ADRs 0014–0018.
