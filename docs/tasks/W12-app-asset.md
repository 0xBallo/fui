# W12 — App: assets

Branch `feat/w-app-asset`. Owned: `internal/app/assetting/`, `internal/testutil/memstore/asset*`. Protocol: `docs/tasks/_PROTOCOL.md`. Plan: `docs/PLAN-v0.3.md`.

Plan/Apply: AddAsset (optional loan link), SetValuation (same date overwrites with confirm flag in the plan), List (with latest value, net-of-debt when linked), Show history. `Require(assets)`.
Accept: dry-run, valuation upsert semantics, loan link validated, coverage ≥ 80%.

Coordinator note: also add `Assets() AssetRepo` to `app.Repos` (+ memstore impl, sqlite repos.go) and drop the type assertion in `repo_assets_test.go` (REQ from W08). You may touch those files.
