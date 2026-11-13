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
