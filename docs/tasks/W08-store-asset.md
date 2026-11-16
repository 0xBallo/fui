# W08 — Store: assets (0013)

Branch `feat/w-store-asset`. Owned: store `migrations/0013_assets.sql`, `queries/asset*`, `repo_assets*`, new port in ports.go. Protocol: `docs/tasks/_PROTOCOL.md`. Plan: `docs/PLAN-v0.3.md`.

Spec: T Modello dati (`assets`, `asset_valuations` UNIQUE(asset_id,date), `loan_id` nullable FK to loans). Same method as W07; `reset`/dump/restore extended.
